package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"

	"github.com/google/uuid"
	dbmigrations "github.com/ixxet/apollo/db/migrations"
	"github.com/ixxet/apollo/internal/ares"
	"github.com/ixxet/apollo/internal/athena"
	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/booking"
	"github.com/ixxet/apollo/internal/coaching"
	"github.com/ixxet/apollo/internal/competition"
	"github.com/ixxet/apollo/internal/config"
	"github.com/ixxet/apollo/internal/consumer"
	"github.com/ixxet/apollo/internal/eligibility"
	"github.com/ixxet/apollo/internal/exercises"
	"github.com/ixxet/apollo/internal/membership"
	"github.com/ixxet/apollo/internal/nutrition"
	"github.com/ixxet/apollo/internal/ops"
	"github.com/ixxet/apollo/internal/planner"
	"github.com/ixxet/apollo/internal/presence"
	"github.com/ixxet/apollo/internal/profile"
	"github.com/ixxet/apollo/internal/recommendations"
	"github.com/ixxet/apollo/internal/schedule"
	"github.com/ixxet/apollo/internal/server"
	"github.com/ixxet/apollo/internal/sports"
	"github.com/ixxet/apollo/internal/visits"
	"github.com/ixxet/apollo/internal/workouts"
	protoevents "github.com/ixxet/ashton-proto/events"
)

const (
	httpReadHeaderTimeout        = 5 * time.Second
	httpReadTimeout              = 15 * time.Second
	httpWriteTimeout             = 15 * time.Second
	httpIdleTimeout              = 60 * time.Second
	httpShutdownTimeout          = 10 * time.Second
	identifiedPresenceMsgTimeout = 5 * time.Second
	natsDrainTimeout             = 10 * time.Second
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "apollo",
		Short:         "APOLLO owns member auth, profile state, and visit history.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newMigrateCmd())
	rootCmd.AddCommand(newCompetitionCmd())
	rootCmd.AddCommand(newScheduleCmd())
	rootCmd.AddCommand(newSportCmd())
	rootCmd.AddCommand(newVisitCmd())

	return rootCmd
}

func newMigrateCmd() *cobra.Command {
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Apply APOLLO database migrations.",
	}

	migrateCmd.AddCommand(&cobra.Command{
		Use:   "up",
		Short: "Apply all pending up migrations.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool, err := openPool(cmd.Context(), cfg.DatabaseURL)
			if err != nil {
				return err
			}
			defer pool.Close()

			if err := dbmigrations.ApplyAll(cmd.Context(), pool); err != nil {
				return err
			}

			slog.Info("apollo migrations applied")
			return nil
		},
	})

	return migrateCmd
}

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the APOLLO HTTP server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			serveCtx, stopSignals := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stopSignals()

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool, err := openPool(cmd.Context(), cfg.DatabaseURL)
			if err != nil {
				return err
			}
			defer pool.Close()

			visitRepository := visits.NewRepository(pool)
			visitService := visits.NewService(visitRepository)
			presenceRepository := presence.NewRepository(pool)
			presenceService := presence.NewService(presenceRepository, visitService)

			consumerEnabled := false
			var natsConn *nats.Conn
			if cfg.NATSURL != "" {
				conn, err := nats.Connect(cfg.NATSURL)
				if err != nil {
					return err
				}
				natsConn = conn

				arrivalHandler := consumer.NewIdentifiedPresenceHandler(presenceService)
				if _, err := conn.Subscribe(protoevents.SubjectIdentifiedPresenceArrived, func(msg *nats.Msg) {
					msgCtx, cancel := context.WithTimeout(context.Background(), identifiedPresenceMsgTimeout)
					defer cancel()

					if _, err := arrivalHandler.HandleMessage(msgCtx, msg.Data); err != nil {
						slog.Error("identified arrival consumer failed", "error", err)
					}
				}); err != nil {
					conn.Close()
					return err
				}

				departureHandler := consumer.NewIdentifiedDepartureHandler(presenceService)
				if _, err := conn.Subscribe(protoevents.SubjectIdentifiedPresenceDeparted, func(msg *nats.Msg) {
					msgCtx, cancel := context.WithTimeout(context.Background(), identifiedPresenceMsgTimeout)
					defer cancel()

					if _, err := departureHandler.HandleMessage(msgCtx, msg.Data); err != nil {
						slog.Error("identified departure consumer failed", "error", err)
					}
				}); err != nil {
					conn.Close()
					return err
				}
				consumerEnabled = true
				slog.Info("identified presence consumer enabled", "subjects", []string{
					protoevents.SubjectIdentifiedPresenceArrived,
					protoevents.SubjectIdentifiedPresenceDeparted,
				})
			}
			if natsConn != nil {
				defer natsConn.Close()
			}

			if cfg.SessionCookieSecret == "" {
				return fmt.Errorf("APOLLO_SESSION_COOKIE_SECRET is required")
			}

			cookies, err := auth.NewSessionCookieManager(cfg.SessionCookieName, cfg.SessionCookieSecret, cfg.SessionCookieSecure)
			if err != nil {
				return err
			}

			var sender auth.EmailSender = auth.NoopEmailSender{}
			if cfg.LogVerificationTokens {
				sender = auth.LogEmailSender{}
			}

			deps, err := buildServerDependencies(pool, consumerEnabled, cookies, sender, cfg)
			if err != nil {
				return err
			}

			httpServer := &http.Server{
				Addr:              cfg.HTTPAddr,
				Handler:           server.NewHandler(deps),
				ReadHeaderTimeout: httpReadHeaderTimeout,
				ReadTimeout:       httpReadTimeout,
				WriteTimeout:      httpWriteTimeout,
				IdleTimeout:       httpIdleTimeout,
			}

			slog.Info("starting APOLLO server", "addr", cfg.HTTPAddr)

			serverErrors := make(chan error, 1)
			go func() {
				serverErrors <- httpServer.ListenAndServe()
			}()

			select {
			case err := <-serverErrors:
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
			case <-serveCtx.Done():
				slog.Info("apollo shutdown requested")
			}

			shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), httpShutdownTimeout)
			defer cancelShutdown()

			if err := httpServer.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("shutdown APOLLO HTTP server: %w", err)
			}
			if natsConn != nil {
				if err := drainNATS(natsConn, natsDrainTimeout); err != nil {
					return fmt.Errorf("drain APOLLO NATS connection: %w", err)
				}
			}

			return nil
		},
	}
}

func buildServerDependencies(pool *pgxpool.Pool, consumerEnabled bool, cookies *auth.SessionCookieManager, sender auth.EmailSender, cfg config.Config) (server.Dependencies, error) {
	authRepository := auth.NewRepository(pool)
	authService := auth.NewService(authRepository, cookies, sender, cfg.VerificationTokenTTL, cfg.SessionTTL)
	visitRepository := visits.NewRepository(pool)
	visitService := visits.NewService(visitRepository)
	exerciseRepository := exercises.NewRepository(pool)
	exerciseService := exercises.NewService(exerciseRepository)
	plannerRepository := planner.NewRepository(pool)
	plannerService := planner.NewService(plannerRepository, exerciseService)
	profileRepository := profile.NewRepository(pool)
	profileService := profile.NewService(profileRepository, exerciseService)
	eligibilityService := eligibility.NewService(profileRepository)
	membershipService := membership.NewService(membership.NewRepository(pool), eligibilityService)
	matchPreviewService := ares.NewService(ares.NewRepository(pool))
	recommendationService := recommendations.NewService(recommendations.NewRepository(pool))
	coachingService := coaching.NewService(coaching.NewRepository(pool), plannerService, profileService)
	nutritionService := nutrition.NewService(nutrition.NewRepository(pool), profileService)
	workoutService := workouts.NewService(workouts.NewRepository(pool))
	competitionService := competition.NewService(competition.NewRepository(pool))
	scheduleService := schedule.NewService(schedule.NewRepository(pool))
	bookingService := booking.NewService(booking.NewRepository(pool), scheduleService)
	presenceService := presence.NewService(presence.NewRepository(pool), visitService, presence.WithFacilityCalendar(scheduleService))

	var opsReader server.OpsOverviewReader
	if strings.TrimSpace(cfg.AthenaBaseURL) != "" {
		athenaClient, err := athena.NewClient(cfg.AthenaBaseURL, cfg.AthenaTimeout)
		if err != nil {
			return server.Dependencies{}, err
		}
		opsReader = ops.NewService(scheduleService, athenaClient, cfg.OpsAnalyticsMaxWindow)
	}

	return server.Dependencies{
		ConsumerEnabled:    consumerEnabled,
		Auth:               authService,
		Competition:        competitionService,
		CompetitionHistory: competitionService,
		Profile:            profileService,
		Presence:           presenceService,
		PresenceClaims:     presenceService,
		MemberFacilities:   presenceService,
		Exercises:          exerciseService,
		Planner:            plannerService,
		Eligibility:        eligibilityService,
		Membership:         membershipService,
		MatchPreview:       matchPreviewService,
		Recommendations:    recommendationService,
		Schedule:           scheduleService,
		Booking:            bookingService,
		Ops:                opsReader,
		Coaching:           coachingService,
		Nutrition:          nutritionService,
		Workouts:           workoutService,
	}, nil
}

func drainNATS(conn *nats.Conn, timeout time.Duration) error {
	if conn == nil {
		return nil
	}

	drained := make(chan error, 1)
	go func() {
		drained <- conn.Drain()
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-drained:
		return err
	case <-timer.C:
		conn.Close()
		return fmt.Errorf("timeout after %s", timeout)
	}
}

func newVisitCmd() *cobra.Command {
	visitCmd := &cobra.Command{
		Use:   "visit",
		Short: "Query member visit history.",
	}

	var studentID string
	var format string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List visits for a student id.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool, err := openPool(cmd.Context(), cfg.DatabaseURL)
			if err != nil {
				return err
			}
			defer pool.Close()

			repository := visits.NewRepository(pool)
			rows, err := repository.ListByStudentID(cmd.Context(), studentID)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(rows)
			case "text":
				for _, row := range rows {
					sourceEventID := ""
					if row.SourceEventID != nil {
						sourceEventID = *row.SourceEventID
					}
					zoneKey := ""
					if row.ZoneKey != nil {
						zoneKey = *row.ZoneKey
					}
					if _, err := fmt.Fprintf(
						cmd.OutOrStdout(),
						"facility=%s zone=%s arrived_at=%s source_event_id=%s\n",
						row.FacilityKey,
						zoneKey,
						formatTimestamp(row.ArrivedAt),
						sourceEventID,
					); err != nil {
						return err
					}
				}
				return nil
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}

	listCmd.Flags().StringVar(&studentID, "student-id", "", "student id to query")
	_ = listCmd.MarkFlagRequired("student-id")
	listCmd.Flags().StringVar(&format, "format", "text", "output format: text or json")

	visitCmd.AddCommand(listCmd)

	return visitCmd
}

func newCompetitionCmd() *cobra.Command {
	competitionCmd := &cobra.Command{
		Use:   "competition",
		Short: "Read and command APOLLO competition runtime truth.",
	}

	publicCmd := &cobra.Command{
		Use:   "public",
		Short: "Read APOLLO public-safe competition projections.",
	}
	publicCmd.AddCommand(newCompetitionPublicReadinessCmd())
	publicCmd.AddCommand(newCompetitionPublicLeaderboardCmd())
	publicCmd.AddCommand(newCompetitionPublicGameIdentityCmd())

	memberCmd := &cobra.Command{
		Use:   "member",
		Short: "Read APOLLO member-safe competition projections for a supplied member id.",
	}
	memberCmd.AddCommand(newCompetitionMemberStatsCmd())
	memberCmd.AddCommand(newCompetitionMemberHistoryCmd())
	memberCmd.AddCommand(newCompetitionMemberGameIdentityCmd())

	sessionCmd := &cobra.Command{
		Use:   "session",
		Short: "Read APOLLO competition sessions.",
	}
	sessionCmd.AddCommand(newCompetitionSessionListCmd())
	sessionCmd.AddCommand(newCompetitionSessionShowCmd())

	tournamentCmd := &cobra.Command{
		Use:   "tournament",
		Short: "Read APOLLO internal tournament runtime truth.",
	}
	tournamentCmd.AddCommand(newCompetitionTournamentListCmd())
	tournamentCmd.AddCommand(newCompetitionTournamentShowCmd())

	commandCmd := &cobra.Command{
		Use:   "command",
		Short: "Run APOLLO competition command DTOs.",
	}
	commandCmd.AddCommand(newCompetitionCommandReadinessCmd())
	commandCmd.AddCommand(newCompetitionCommandRunCmd())

	safetyCmd := &cobra.Command{
		Use:   "safety",
		Short: "Read APOLLO manager/internal safety and reliability readiness.",
	}
	safetyCmd.AddCommand(newCompetitionSafetyReadinessCmd())
	safetyCmd.AddCommand(newCompetitionSafetyReviewCmd())

	competitionCmd.AddCommand(publicCmd)
	competitionCmd.AddCommand(memberCmd)
	competitionCmd.AddCommand(sessionCmd)
	competitionCmd.AddCommand(tournamentCmd)
	competitionCmd.AddCommand(commandCmd)
	competitionCmd.AddCommand(safetyCmd)
	return competitionCmd
}

func newCompetitionPublicReadinessCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "readiness",
		Short: "Show APOLLO public-safe competition readiness.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			readiness, err := service.PublicCompetitionReadiness(cmd.Context())
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeJSONOutput(cmd, readiness)
			case "text":
				return writePublicCompetitionReadinessText(cmd, readiness)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return cmd
}

func newCompetitionPublicLeaderboardCmd() *cobra.Command {
	var sportKey string
	var modeKey string
	var statType string
	var teamScope string
	var limit int
	var format string

	cmd := &cobra.Command{
		Use:   "leaderboard",
		Short: "Show APOLLO public-safe competition leaderboard projection.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			leaderboard, err := service.ListPublicCompetitionLeaderboard(cmd.Context(), competition.PublicCompetitionLeaderboardInput{
				SportKey:  sportKey,
				ModeKey:   modeKey,
				StatType:  statType,
				TeamScope: teamScope,
				Limit:     limit,
			})
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeJSONOutput(cmd, leaderboard)
			case "text":
				return writePublicCompetitionLeaderboardText(cmd, leaderboard)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&sportKey, "sport-key", "", "filter by sport key")
	cmd.Flags().StringVar(&modeKey, "mode-key", "", "filter by mode key")
	cmd.Flags().StringVar(&statType, "stat-type", "wins", "leaderboard stat type")
	cmd.Flags().StringVar(&teamScope, "team-scope", "all", "team scope: all, solo, or team")
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum rows to return")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return cmd
}

func newCompetitionPublicGameIdentityCmd() *cobra.Command {
	var input competition.PublicGameIdentityInput
	var format string

	cmd := &cobra.Command{
		Use:   "game-identity",
		Short: "Show APOLLO public-safe game identity projection.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			identity, err := service.PublicGameIdentity(cmd.Context(), input)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeJSONOutput(cmd, identity)
			case "text":
				return writeGameIdentityText(cmd, identity)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	addGameIdentityFlags(cmd, &input, &format)
	return cmd
}

func newCompetitionMemberStatsCmd() *cobra.Command {
	var userID string
	var format string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show APOLLO member-safe competition stats and active legacy rating projection.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			parsedUserID, err := parseUUIDFlag(userID, "user-id")
			if err != nil {
				return err
			}
			stats, err := service.ListMemberStats(cmd.Context(), parsedUserID)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeJSONOutput(cmd, stats)
			case "text":
				return writeMemberStatsText(cmd, stats)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&userID, "user-id", "", "member user id")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	_ = cmd.MarkFlagRequired("user-id")
	return cmd
}

func newCompetitionMemberHistoryCmd() *cobra.Command {
	var userID string
	var format string

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show APOLLO member-safe competition history.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			parsedUserID, err := parseUUIDFlag(userID, "user-id")
			if err != nil {
				return err
			}
			history, err := service.ListMemberHistory(cmd.Context(), parsedUserID)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeJSONOutput(cmd, history)
			case "text":
				return writeMemberHistoryText(cmd, history)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&userID, "user-id", "", "member user id")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	_ = cmd.MarkFlagRequired("user-id")
	return cmd
}

func newCompetitionMemberGameIdentityCmd() *cobra.Command {
	var userID string
	var input competition.PublicGameIdentityInput
	var format string

	cmd := &cobra.Command{
		Use:   "game-identity",
		Short: "Show APOLLO member-safe game identity projection for a supplied member id.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			parsedUserID, err := parseUUIDFlag(userID, "user-id")
			if err != nil {
				return err
			}
			identity, err := service.MemberGameIdentity(cmd.Context(), parsedUserID, input)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeJSONOutput(cmd, identity)
			case "text":
				return writeGameIdentityText(cmd, identity)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&userID, "user-id", "", "member user id")
	addGameIdentityFlags(cmd, &input, &format)
	_ = cmd.MarkFlagRequired("user-id")
	return cmd
}

func newCompetitionSessionListCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List APOLLO competition sessions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			sessions, err := service.ListSessions(cmd.Context())
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeJSONOutput(cmd, sessions)
			case "text":
				return writeCompetitionSessionListText(cmd, sessions)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return cmd
}

func newCompetitionSessionShowCmd() *cobra.Command {
	var sessionID string
	var format string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show one APOLLO competition session.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			parsedSessionID, err := uuid.Parse(strings.TrimSpace(sessionID))
			if err != nil {
				return err
			}
			session, err := service.GetSession(cmd.Context(), parsedSessionID)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeJSONOutput(cmd, session)
			case "text":
				return writeCompetitionSessionText(cmd, session)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&sessionID, "session-id", "", "competition session id")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	_ = cmd.MarkFlagRequired("session-id")
	return cmd
}

func newCompetitionTournamentListCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List APOLLO internal tournaments.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			tournaments, err := service.ListTournaments(cmd.Context())
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeJSONOutput(cmd, tournaments)
			case "text":
				return writeCompetitionTournamentListText(cmd, tournaments)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return cmd
}

func newCompetitionTournamentShowCmd() *cobra.Command {
	var tournamentID string
	var format string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show one APOLLO internal tournament.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			parsedTournamentID, err := uuid.Parse(strings.TrimSpace(tournamentID))
			if err != nil {
				return err
			}
			tournament, err := service.GetTournament(cmd.Context(), parsedTournamentID)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeJSONOutput(cmd, tournament)
			case "text":
				return writeCompetitionTournamentText(cmd, tournament)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&tournamentID, "tournament-id", "", "competition tournament id")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	_ = cmd.MarkFlagRequired("tournament-id")
	return cmd
}

func newCompetitionCommandReadinessCmd() *cobra.Command {
	var actorUserID string
	var actorSessionID string
	var actorRole string
	var format string

	cmd := &cobra.Command{
		Use:   "readiness",
		Short: "Show APOLLO competition command readiness for an actor role.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			actor, err := parseCompetitionActor(actorUserID, actorSessionID, actorRole, "", "", "")
			if err != nil {
				return err
			}
			readiness := service.CompetitionReadiness(actor)

			switch format {
			case "json":
				return writeJSONOutput(cmd, readiness)
			case "text":
				return writeCompetitionReadinessText(cmd, readiness)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&actorUserID, "actor-user-id", "", "actor user id for attribution")
	cmd.Flags().StringVar(&actorSessionID, "actor-session-id", "", "actor session id for attribution")
	cmd.Flags().StringVar(&actorRole, "actor-role", "", "actor role")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	_ = cmd.MarkFlagRequired("actor-role")
	return cmd
}

func newCompetitionCommandRunCmd() *cobra.Command {
	var name string
	var inputJSON string
	var dryRun bool
	var idempotencyKey string
	var expectedVersion int
	var sessionID string
	var teamID string
	var matchID string
	var tournamentID string
	var bracketID string
	var teamSnapshotID string
	var matchBindingID string
	var userID string
	var actorUserID string
	var actorSessionID string
	var actorRole string
	var trustedSurfaceKey string
	var trustedSurfaceLabel string
	var format string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run one APOLLO competition command DTO through the shared handler.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			var command competition.CompetitionCommand
			if strings.TrimSpace(inputJSON) != "" {
				if err := json.Unmarshal([]byte(inputJSON), &command); err != nil {
					return err
				}
			}
			if strings.TrimSpace(name) != "" {
				command.Name = competition.CommandName(strings.TrimSpace(name))
			}
			command.DryRun = dryRun
			if strings.TrimSpace(idempotencyKey) != "" {
				command.IdempotencyKey = strings.TrimSpace(idempotencyKey)
			}
			if cmd.Flags().Changed("expected-version") {
				command.ExpectedVersion = &expectedVersion
			}
			if err := applyCompetitionCommandIDFlags(&command, sessionID, teamID, matchID, tournamentID, bracketID, teamSnapshotID, matchBindingID, userID); err != nil {
				return err
			}
			actor, err := parseCompetitionCommandActor(actorUserID, actorSessionID, actorRole, trustedSurfaceKey, trustedSurfaceLabel, dryRun)
			if err != nil {
				return err
			}
			command.Actor = actor

			outcome, err := service.ExecuteCommand(cmd.Context(), command)
			switch format {
			case "json":
				writeErr := writeJSONOutput(cmd, outcome)
				if err != nil {
					return err
				}
				return writeErr
			case "text":
				writeErr := writeCompetitionCommandOutcomeText(cmd, outcome)
				if err != nil {
					return err
				}
				return writeErr
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "competition command name")
	cmd.Flags().StringVar(&inputJSON, "input-json", "", "competition command DTO JSON")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "return a command plan without mutation")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "idempotency key when the command supports it")
	cmd.Flags().IntVar(&expectedVersion, "expected-version", 0, "expected resource version when the command supports it")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "competition session id")
	cmd.Flags().StringVar(&teamID, "team-id", "", "competition team id")
	cmd.Flags().StringVar(&matchID, "match-id", "", "competition match id")
	cmd.Flags().StringVar(&tournamentID, "tournament-id", "", "competition tournament id")
	cmd.Flags().StringVar(&bracketID, "bracket-id", "", "competition tournament bracket id")
	cmd.Flags().StringVar(&teamSnapshotID, "team-snapshot-id", "", "competition tournament team snapshot id")
	cmd.Flags().StringVar(&matchBindingID, "match-binding-id", "", "competition tournament match binding id")
	cmd.Flags().StringVar(&userID, "user-id", "", "competition user id")
	cmd.Flags().StringVar(&actorUserID, "actor-user-id", "", "actor user id for attribution")
	cmd.Flags().StringVar(&actorSessionID, "actor-session-id", "", "actor session id for attribution")
	cmd.Flags().StringVar(&actorRole, "actor-role", "", "actor role")
	cmd.Flags().StringVar(&trustedSurfaceKey, "trusted-surface-key", "", "trusted surface key")
	cmd.Flags().StringVar(&trustedSurfaceLabel, "trusted-surface-label", "", "trusted surface label")
	cmd.Flags().StringVar(&format, "format", "json", "output format: text or json")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("actor-role")
	return cmd
}

func newCompetitionSafetyReadinessCmd() *cobra.Command {
	var actorUserID string
	var actorSessionID string
	var actorRole string
	var trustedSurfaceKey string
	var trustedSurfaceLabel string
	var format string

	cmd := &cobra.Command{
		Use:   "readiness",
		Short: "Show APOLLO manager/internal safety and reliability readiness.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			actor, err := parseRequiredCompetitionActor(actorUserID, actorSessionID, actorRole, authz.CapabilityCompetitionSafetyReview, trustedSurfaceKey, trustedSurfaceLabel)
			if err != nil {
				return err
			}
			readiness, err := service.CompetitionSafetyReadiness(cmd.Context(), actor)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeJSONOutput(cmd, readiness)
			case "text":
				return writeCompetitionSafetyReadinessText(cmd, readiness)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	addCompetitionSafetyActorFlags(cmd, &actorUserID, &actorSessionID, &actorRole, &trustedSurfaceKey, &trustedSurfaceLabel)
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return cmd
}

func newCompetitionSafetyReviewCmd() *cobra.Command {
	var actorUserID string
	var actorSessionID string
	var actorRole string
	var trustedSurfaceKey string
	var trustedSurfaceLabel string
	var limit int
	var format string

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Show APOLLO manager/internal safety and reliability review facts.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openCompetitionService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			actor, err := parseRequiredCompetitionActor(actorUserID, actorSessionID, actorRole, authz.CapabilityCompetitionSafetyReview, trustedSurfaceKey, trustedSurfaceLabel)
			if err != nil {
				return err
			}
			review, err := service.GetCompetitionSafetyReview(cmd.Context(), actor, limit)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeJSONOutput(cmd, review)
			case "text":
				return writeCompetitionSafetyReviewText(cmd, review)
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	addCompetitionSafetyActorFlags(cmd, &actorUserID, &actorSessionID, &actorRole, &trustedSurfaceKey, &trustedSurfaceLabel)
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum rows per safety review section")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return cmd
}

func newScheduleCmd() *cobra.Command {
	scheduleCmd := &cobra.Command{
		Use:   "schedule",
		Short: "Read and author APOLLO schedule substrate truth.",
	}

	blockCmd := &cobra.Command{
		Use:   "block",
		Short: "Manage schedule blocks.",
	}
	resourceCmd := &cobra.Command{
		Use:   "resource",
		Short: "Manage schedule resources.",
	}
	resourceEdgeCmd := &cobra.Command{
		Use:   "edge",
		Short: "Manage schedule resource edges.",
	}

	scheduleCmd.AddCommand(blockCmd)
	scheduleCmd.AddCommand(resourceCmd)
	scheduleCmd.AddCommand(newScheduleCalendarCmd())

	blockCmd.AddCommand(newScheduleBlockListCmd())
	blockCmd.AddCommand(newScheduleBlockCreateCmd())
	blockCmd.AddCommand(newScheduleBlockExceptCmd())
	blockCmd.AddCommand(newScheduleBlockCancelCmd())

	resourceCmd.AddCommand(newScheduleResourceListCmd())
	resourceCmd.AddCommand(newScheduleResourceShowCmd())
	resourceCmd.AddCommand(newScheduleResourceUpsertCmd())
	resourceCmd.AddCommand(resourceEdgeCmd)
	resourceEdgeCmd.AddCommand(newScheduleResourceEdgeUpsertCmd())

	return scheduleCmd
}

func newScheduleCalendarCmd() *cobra.Command {
	var facilityKey string
	var from string
	var until string
	var format string

	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Read schedule occurrences for a facility.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openScheduleService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			windowFrom, windowUntil, err := parseScheduleWindow(from, until)
			if err != nil {
				return err
			}
			occurrences, err := service.GetCalendar(cmd.Context(), facilityKey, schedule.CalendarWindow{From: windowFrom, Until: windowUntil})
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeJSONOutput(cmd, occurrences)
			case "text":
				for _, occurrence := range occurrences {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s %s %s\n", occurrence.OccurrenceDate, occurrence.Scope, occurrence.Kind, occurrence.StartsAt.UTC().Format(time.RFC3339), occurrence.EndsAt.UTC().Format(time.RFC3339)); err != nil {
						return err
					}
				}
				return nil
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&facilityKey, "facility-key", "", "facility key to query")
	cmd.Flags().StringVar(&from, "from", "", "window start (RFC3339)")
	cmd.Flags().StringVar(&until, "until", "", "window end (RFC3339)")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	_ = cmd.MarkFlagRequired("facility-key")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("until")
	return cmd
}

func newScheduleBlockListCmd() *cobra.Command {
	var facilityKey string
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List schedule blocks for a facility.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openScheduleService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			blocks, err := service.ListBlocks(cmd.Context(), facilityKey)
			if err != nil {
				return err
			}
			switch format {
			case "json":
				return writeJSONOutput(cmd, blocks)
			case "text":
				for _, block := range blocks {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s %s %s %s\n", block.ID, block.Scope, block.ScheduleType, block.Kind, block.Effect, block.Visibility); err != nil {
						return err
					}
				}
				return nil
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&facilityKey, "facility-key", "", "facility key to query")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	_ = cmd.MarkFlagRequired("facility-key")
	return cmd
}

func newScheduleBlockCreateCmd() *cobra.Command {
	var facilityKey string
	var zoneKey string
	var resourceKey string
	var scope string
	var kind string
	var effect string
	var visibility string
	var oneOffStartsAt string
	var oneOffEndsAt string
	var weeklyWeekday int
	var weeklyStartTime string
	var weeklyEndTime string
	var weeklyTimezone string
	var weeklyRecurrenceStartDate string
	var weeklyRecurrenceEndDate string
	var actorUserID string
	var actorSessionID string
	var actorRole string
	var trustedSurfaceKey string
	var trustedSurfaceLabel string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a schedule block.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openScheduleService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			actor, err := parseScheduleActor(cmd, actorUserID, actorSessionID, actorRole, trustedSurfaceKey, trustedSurfaceLabel, authz.CapabilityScheduleManage, false)
			if err != nil {
				return err
			}

			var input schedule.BlockInput
			input.FacilityKey = facilityKey
			input.Scope = scope
			input.Kind = kind
			input.Effect = effect
			input.Visibility = visibility
			if zoneKey != "" {
				input.ZoneKey = &zoneKey
			}
			if resourceKey != "" {
				input.ResourceKey = &resourceKey
			}

			if oneOffStartsAt != "" || oneOffEndsAt != "" {
				startsAt, err := time.Parse(time.RFC3339, oneOffStartsAt)
				if err != nil {
					return err
				}
				endsAt, err := time.Parse(time.RFC3339, oneOffEndsAt)
				if err != nil {
					return err
				}
				input.OneOff = &schedule.OneOffInput{StartsAt: startsAt, EndsAt: endsAt}
			} else {
				weekly := schedule.WeeklyInput{
					Weekday:             weeklyWeekday,
					StartTime:           weeklyStartTime,
					EndTime:             weeklyEndTime,
					Timezone:            weeklyTimezone,
					RecurrenceStartDate: weeklyRecurrenceStartDate,
				}
				if weeklyRecurrenceEndDate != "" {
					weekly.RecurrenceEndDate = &weeklyRecurrenceEndDate
				}
				input.Weekly = &weekly
			}

			block, err := service.CreateBlock(cmd.Context(), actor, input)
			if err != nil {
				return err
			}
			return writeJSONOutput(cmd, block)
		},
	}
	cmd.Flags().StringVar(&facilityKey, "facility-key", "", "facility key")
	cmd.Flags().StringVar(&zoneKey, "zone-key", "", "zone key")
	cmd.Flags().StringVar(&resourceKey, "resource-key", "", "resource key")
	cmd.Flags().StringVar(&scope, "scope", "", "scope")
	cmd.Flags().StringVar(&kind, "kind", "", "block kind")
	cmd.Flags().StringVar(&effect, "effect", "", "block effect")
	cmd.Flags().StringVar(&visibility, "visibility", "", "visibility")
	cmd.Flags().StringVar(&oneOffStartsAt, "one-off-starts-at", "", "one-off start RFC3339")
	cmd.Flags().StringVar(&oneOffEndsAt, "one-off-ends-at", "", "one-off end RFC3339")
	cmd.Flags().IntVar(&weeklyWeekday, "weekly-weekday", 0, "weekly weekday 1-7")
	cmd.Flags().StringVar(&weeklyStartTime, "weekly-start-time", "", "weekly start HH:MM")
	cmd.Flags().StringVar(&weeklyEndTime, "weekly-end-time", "", "weekly end HH:MM")
	cmd.Flags().StringVar(&weeklyTimezone, "weekly-timezone", "", "weekly timezone")
	cmd.Flags().StringVar(&weeklyRecurrenceStartDate, "weekly-recurrence-start-date", "", "weekly recurrence start date")
	cmd.Flags().StringVar(&weeklyRecurrenceEndDate, "weekly-recurrence-end-date", "", "weekly recurrence end date")
	cmd.Flags().StringVar(&actorUserID, "actor-user-id", "", "actor user id")
	cmd.Flags().StringVar(&actorSessionID, "actor-session-id", "", "actor session id")
	cmd.Flags().StringVar(&actorRole, "actor-role", "", "actor role")
	cmd.Flags().StringVar(&trustedSurfaceKey, "trusted-surface-key", "", "trusted surface key")
	cmd.Flags().StringVar(&trustedSurfaceLabel, "trusted-surface-label", "", "trusted surface label")
	_ = cmd.MarkFlagRequired("facility-key")
	_ = cmd.MarkFlagRequired("scope")
	_ = cmd.MarkFlagRequired("kind")
	_ = cmd.MarkFlagRequired("effect")
	_ = cmd.MarkFlagRequired("visibility")
	_ = cmd.MarkFlagRequired("actor-user-id")
	_ = cmd.MarkFlagRequired("actor-session-id")
	_ = cmd.MarkFlagRequired("actor-role")
	_ = cmd.MarkFlagRequired("trusted-surface-key")
	return cmd
}

func newScheduleBlockExceptCmd() *cobra.Command {
	var blockID string
	var expectedVersion int
	var exceptionDate string
	var actorUserID string
	var actorSessionID string
	var actorRole string
	var trustedSurfaceKey string
	var trustedSurfaceLabel string

	cmd := &cobra.Command{
		Use:   "except",
		Short: "Add a date exception to a weekly block.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openScheduleService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			id, err := uuid.Parse(blockID)
			if err != nil {
				return err
			}
			actor, err := parseScheduleActor(cmd, actorUserID, actorSessionID, actorRole, trustedSurfaceKey, trustedSurfaceLabel, authz.CapabilityScheduleManage, false)
			if err != nil {
				return err
			}
			block, err := service.AddException(cmd.Context(), actor, id, expectedVersion, schedule.BlockExceptionInput{ExceptionDate: exceptionDate})
			if err != nil {
				return err
			}
			return writeJSONOutput(cmd, block)
		},
	}
	cmd.Flags().StringVar(&blockID, "block-id", "", "block id")
	cmd.Flags().IntVar(&expectedVersion, "expected-version", 0, "expected version")
	cmd.Flags().StringVar(&exceptionDate, "exception-date", "", "exception date")
	cmd.Flags().StringVar(&actorUserID, "actor-user-id", "", "actor user id")
	cmd.Flags().StringVar(&actorSessionID, "actor-session-id", "", "actor session id")
	cmd.Flags().StringVar(&actorRole, "actor-role", "", "actor role")
	cmd.Flags().StringVar(&trustedSurfaceKey, "trusted-surface-key", "", "trusted surface key")
	cmd.Flags().StringVar(&trustedSurfaceLabel, "trusted-surface-label", "", "trusted surface label")
	_ = cmd.MarkFlagRequired("block-id")
	_ = cmd.MarkFlagRequired("expected-version")
	_ = cmd.MarkFlagRequired("exception-date")
	_ = cmd.MarkFlagRequired("actor-user-id")
	_ = cmd.MarkFlagRequired("actor-session-id")
	_ = cmd.MarkFlagRequired("actor-role")
	_ = cmd.MarkFlagRequired("trusted-surface-key")
	return cmd
}

func newScheduleBlockCancelCmd() *cobra.Command {
	var blockID string
	var expectedVersion int
	var actorUserID string
	var actorSessionID string
	var actorRole string
	var trustedSurfaceKey string
	var trustedSurfaceLabel string

	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a schedule block.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openScheduleService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			id, err := uuid.Parse(blockID)
			if err != nil {
				return err
			}
			actor, err := parseScheduleActor(cmd, actorUserID, actorSessionID, actorRole, trustedSurfaceKey, trustedSurfaceLabel, authz.CapabilityScheduleManage, false)
			if err != nil {
				return err
			}
			block, err := service.CancelBlock(cmd.Context(), actor, id, expectedVersion)
			if err != nil {
				return err
			}
			return writeJSONOutput(cmd, block)
		},
	}
	cmd.Flags().StringVar(&blockID, "block-id", "", "block id")
	cmd.Flags().IntVar(&expectedVersion, "expected-version", 0, "expected version")
	cmd.Flags().StringVar(&actorUserID, "actor-user-id", "", "actor user id")
	cmd.Flags().StringVar(&actorSessionID, "actor-session-id", "", "actor session id")
	cmd.Flags().StringVar(&actorRole, "actor-role", "", "actor role")
	cmd.Flags().StringVar(&trustedSurfaceKey, "trusted-surface-key", "", "trusted surface key")
	cmd.Flags().StringVar(&trustedSurfaceLabel, "trusted-surface-label", "", "trusted surface label")
	_ = cmd.MarkFlagRequired("block-id")
	_ = cmd.MarkFlagRequired("expected-version")
	_ = cmd.MarkFlagRequired("actor-user-id")
	_ = cmd.MarkFlagRequired("actor-session-id")
	_ = cmd.MarkFlagRequired("actor-role")
	_ = cmd.MarkFlagRequired("trusted-surface-key")
	return cmd
}

func newScheduleResourceListCmd() *cobra.Command {
	var facilityKey string
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List schedule resources for a facility.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openScheduleService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			rows, err := service.ListResources(cmd.Context(), facilityKey)
			if err != nil {
				return err
			}
			switch format {
			case "json":
				return writeJSONOutput(cmd, rows)
			case "text":
				for _, row := range rows {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", row.ResourceKey, row.ResourceType, row.DisplayName); err != nil {
						return err
					}
				}
				return nil
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&facilityKey, "facility-key", "", "facility key")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	_ = cmd.MarkFlagRequired("facility-key")
	return cmd
}

func newScheduleResourceShowCmd() *cobra.Command {
	var resourceKey string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show a schedule resource.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openScheduleService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			row, err := service.GetResource(cmd.Context(), resourceKey)
			if err != nil {
				return err
			}
			return writeJSONOutput(cmd, row)
		},
	}
	cmd.Flags().StringVar(&resourceKey, "resource-key", "", "resource key")
	_ = cmd.MarkFlagRequired("resource-key")
	return cmd
}

func newScheduleResourceUpsertCmd() *cobra.Command {
	var resourceKey string
	var facilityKey string
	var zoneKey string
	var resourceType string
	var displayName string
	var publicLabel string
	var bookable bool
	var active bool
	var actorUserID string
	var actorSessionID string
	var actorRole string
	var trustedSurfaceKey string
	var trustedSurfaceLabel string

	cmd := &cobra.Command{
		Use:   "upsert",
		Short: "Upsert a schedule resource.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openScheduleService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			actor, err := parseScheduleActor(cmd, actorUserID, actorSessionID, actorRole, trustedSurfaceKey, trustedSurfaceLabel, authz.CapabilityScheduleManage, true)
			if err != nil {
				return err
			}
			input := schedule.ResourceInput{
				ResourceKey:  resourceKey,
				FacilityKey:  facilityKey,
				ResourceType: resourceType,
				DisplayName:  displayName,
				Bookable:     bookable,
				Active:       active,
			}
			if zoneKey != "" {
				input.ZoneKey = &zoneKey
			}
			if publicLabel != "" {
				input.PublicLabel = &publicLabel
			}

			row, err := service.UpsertResource(cmd.Context(), actor, input)
			if err != nil {
				return err
			}
			return writeJSONOutput(cmd, row)
		},
	}
	cmd.Flags().StringVar(&resourceKey, "resource-key", "", "resource key")
	cmd.Flags().StringVar(&facilityKey, "facility-key", "", "facility key")
	cmd.Flags().StringVar(&zoneKey, "zone-key", "", "zone key")
	cmd.Flags().StringVar(&resourceType, "resource-type", "", "resource type")
	cmd.Flags().StringVar(&displayName, "display-name", "", "display name")
	cmd.Flags().StringVar(&publicLabel, "public-label", "", "public label")
	cmd.Flags().BoolVar(&bookable, "bookable", true, "bookable")
	cmd.Flags().BoolVar(&active, "active", true, "active")
	cmd.Flags().StringVar(&actorUserID, "actor-user-id", "", "actor user id")
	cmd.Flags().StringVar(&actorSessionID, "actor-session-id", "", "actor session id")
	cmd.Flags().StringVar(&actorRole, "actor-role", "", "actor role")
	cmd.Flags().StringVar(&trustedSurfaceKey, "trusted-surface-key", "", "trusted surface key")
	cmd.Flags().StringVar(&trustedSurfaceLabel, "trusted-surface-label", "", "trusted surface label")
	_ = cmd.MarkFlagRequired("resource-key")
	_ = cmd.MarkFlagRequired("facility-key")
	_ = cmd.MarkFlagRequired("resource-type")
	_ = cmd.MarkFlagRequired("display-name")
	_ = cmd.MarkFlagRequired("actor-user-id")
	_ = cmd.MarkFlagRequired("actor-session-id")
	_ = cmd.MarkFlagRequired("actor-role")
	_ = cmd.MarkFlagRequired("trusted-surface-key")
	return cmd
}

func newScheduleResourceEdgeUpsertCmd() *cobra.Command {
	var resourceKey string
	var relatedResourceKey string
	var edgeType string
	var actorUserID string
	var actorSessionID string
	var actorRole string
	var trustedSurfaceKey string
	var trustedSurfaceLabel string

	cmd := &cobra.Command{
		Use:   "upsert",
		Short: "Upsert a schedule resource edge.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openScheduleService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			actor, err := parseScheduleActor(cmd, actorUserID, actorSessionID, actorRole, trustedSurfaceKey, trustedSurfaceLabel, authz.CapabilityScheduleManage, true)
			if err != nil {
				return err
			}
			row, err := service.UpsertResourceEdge(cmd.Context(), actor, schedule.ResourceEdgeInput{
				ResourceKey:        resourceKey,
				RelatedResourceKey: relatedResourceKey,
				EdgeType:           edgeType,
			})
			if err != nil {
				return err
			}
			return writeJSONOutput(cmd, row)
		},
	}
	cmd.Flags().StringVar(&resourceKey, "resource-key", "", "resource key")
	cmd.Flags().StringVar(&relatedResourceKey, "related-resource-key", "", "related resource key")
	cmd.Flags().StringVar(&edgeType, "edge-type", "", "edge type")
	cmd.Flags().StringVar(&actorUserID, "actor-user-id", "", "actor user id")
	cmd.Flags().StringVar(&actorSessionID, "actor-session-id", "", "actor session id")
	cmd.Flags().StringVar(&actorRole, "actor-role", "", "actor role")
	cmd.Flags().StringVar(&trustedSurfaceKey, "trusted-surface-key", "", "trusted surface key")
	cmd.Flags().StringVar(&trustedSurfaceLabel, "trusted-surface-label", "", "trusted surface label")
	_ = cmd.MarkFlagRequired("resource-key")
	_ = cmd.MarkFlagRequired("related-resource-key")
	_ = cmd.MarkFlagRequired("edge-type")
	_ = cmd.MarkFlagRequired("actor-user-id")
	_ = cmd.MarkFlagRequired("actor-session-id")
	_ = cmd.MarkFlagRequired("actor-role")
	_ = cmd.MarkFlagRequired("trusted-surface-key")
	return cmd
}

func openScheduleService(ctx context.Context) (*schedule.Service, *pgxpool.Pool, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}

	pool, err := openPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}

	return schedule.NewService(schedule.NewRepository(pool)), pool, nil
}

func openCompetitionService(ctx context.Context) (*competition.Service, *pgxpool.Pool, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}

	pool, err := openPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}

	return competition.NewService(competition.NewRepository(pool)), pool, nil
}

func parseCompetitionCommandActor(actorUserID string, actorSessionID string, actorRole string, trustedSurfaceKey string, trustedSurfaceLabel string, dryRun bool) (competition.CompetitionCommandActor, error) {
	actor, err := parseCompetitionActor(actorUserID, actorSessionID, actorRole, "", trustedSurfaceKey, trustedSurfaceLabel)
	if err != nil {
		return competition.CompetitionCommandActor{}, err
	}
	if !dryRun {
		if actor.UserID == uuid.Nil || actor.SessionID == uuid.Nil {
			return competition.CompetitionCommandActor{}, fmt.Errorf("actor attribution is required")
		}
		if strings.TrimSpace(actor.TrustedSurfaceKey) == "" {
			return competition.CompetitionCommandActor{}, fmt.Errorf("trusted surface key is required")
		}
	}
	return competition.CompetitionCommandActor{
		UserID:              actor.UserID,
		Role:                actor.Role,
		SessionID:           actor.SessionID,
		Capability:          actor.Capability,
		TrustedSurfaceKey:   actor.TrustedSurfaceKey,
		TrustedSurfaceLabel: actor.TrustedSurfaceLabel,
	}, nil
}

func parseCompetitionActor(actorUserID string, actorSessionID string, actorRole string, capability authz.Capability, trustedSurfaceKey string, trustedSurfaceLabel string) (competition.StaffActor, error) {
	role, err := authz.NormalizeRole(strings.TrimSpace(actorRole))
	if err != nil {
		return competition.StaffActor{}, err
	}

	var userID uuid.UUID
	if strings.TrimSpace(actorUserID) != "" {
		userID, err = uuid.Parse(strings.TrimSpace(actorUserID))
		if err != nil {
			return competition.StaffActor{}, err
		}
	}

	var sessionID uuid.UUID
	if strings.TrimSpace(actorSessionID) != "" {
		sessionID, err = uuid.Parse(strings.TrimSpace(actorSessionID))
		if err != nil {
			return competition.StaffActor{}, err
		}
	}

	label := strings.TrimSpace(trustedSurfaceLabel)
	if label == "" {
		label = strings.TrimSpace(trustedSurfaceKey)
	}

	return competition.StaffActor{
		UserID:              userID,
		SessionID:           sessionID,
		Role:                role,
		Capability:          capability,
		TrustedSurfaceKey:   strings.TrimSpace(trustedSurfaceKey),
		TrustedSurfaceLabel: label,
	}, nil
}

func parseRequiredCompetitionActor(actorUserID string, actorSessionID string, actorRole string, capability authz.Capability, trustedSurfaceKey string, trustedSurfaceLabel string) (competition.StaffActor, error) {
	actor, err := parseCompetitionActor(actorUserID, actorSessionID, actorRole, capability, trustedSurfaceKey, trustedSurfaceLabel)
	if err != nil {
		return competition.StaffActor{}, err
	}
	if actor.UserID == uuid.Nil || actor.SessionID == uuid.Nil {
		return competition.StaffActor{}, fmt.Errorf("actor attribution is required")
	}
	if strings.TrimSpace(actor.TrustedSurfaceKey) == "" {
		return competition.StaffActor{}, fmt.Errorf("trusted surface key is required")
	}
	return actor, nil
}

func parseUUIDFlag(value string, flagName string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s must be a UUID: %w", flagName, err)
	}
	return parsed, nil
}

func addGameIdentityFlags(cmd *cobra.Command, input *competition.PublicGameIdentityInput, format *string) {
	cmd.Flags().StringVar(&input.SportKey, "sport-key", "", "filter by sport key")
	cmd.Flags().StringVar(&input.ModeKey, "mode-key", "", "filter by mode key")
	cmd.Flags().StringVar(&input.FacilityKey, "facility-key", "", "filter by facility key")
	cmd.Flags().StringVar(&input.TeamScope, "team-scope", "all", "team scope: all, solo, or team")
	cmd.Flags().IntVar(&input.Limit, "limit", 10, "maximum rows to return")
	cmd.Flags().StringVar(format, "format", "text", "output format: text or json")
}

func addCompetitionSafetyActorFlags(cmd *cobra.Command, actorUserID *string, actorSessionID *string, actorRole *string, trustedSurfaceKey *string, trustedSurfaceLabel *string) {
	cmd.Flags().StringVar(actorUserID, "actor-user-id", "", "actor user id for attribution")
	cmd.Flags().StringVar(actorSessionID, "actor-session-id", "", "actor session id for attribution")
	cmd.Flags().StringVar(actorRole, "actor-role", "", "actor role")
	cmd.Flags().StringVar(trustedSurfaceKey, "trusted-surface-key", "", "trusted surface key")
	cmd.Flags().StringVar(trustedSurfaceLabel, "trusted-surface-label", "", "trusted surface label")
	_ = cmd.MarkFlagRequired("actor-user-id")
	_ = cmd.MarkFlagRequired("actor-session-id")
	_ = cmd.MarkFlagRequired("actor-role")
	_ = cmd.MarkFlagRequired("trusted-surface-key")
}

func applyCompetitionCommandIDFlags(command *competition.CompetitionCommand, sessionID string, teamID string, matchID string, tournamentID string, bracketID string, teamSnapshotID string, matchBindingID string, userID string) error {
	if strings.TrimSpace(sessionID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(sessionID))
		if err != nil {
			return err
		}
		command.SessionID = parsed
	}
	if strings.TrimSpace(teamID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(teamID))
		if err != nil {
			return err
		}
		command.TeamID = parsed
	}
	if strings.TrimSpace(matchID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(matchID))
		if err != nil {
			return err
		}
		command.MatchID = parsed
	}
	if strings.TrimSpace(tournamentID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(tournamentID))
		if err != nil {
			return err
		}
		command.TournamentID = parsed
	}
	if strings.TrimSpace(bracketID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(bracketID))
		if err != nil {
			return err
		}
		command.BracketID = parsed
	}
	if strings.TrimSpace(teamSnapshotID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(teamSnapshotID))
		if err != nil {
			return err
		}
		command.TeamSnapshotID = parsed
	}
	if strings.TrimSpace(matchBindingID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(matchBindingID))
		if err != nil {
			return err
		}
		command.MatchBindingID = parsed
	}
	if strings.TrimSpace(userID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(userID))
		if err != nil {
			return err
		}
		command.UserID = parsed
	}
	return nil
}

func parseScheduleActor(cmd *cobra.Command, actorUserID string, actorSessionID string, actorRole string, trustedSurfaceKey string, trustedSurfaceLabel string, capability authz.Capability, requireOwner bool) (schedule.StaffActor, error) {
	userID, err := uuid.Parse(strings.TrimSpace(actorUserID))
	if err != nil {
		return schedule.StaffActor{}, err
	}
	sessionID, err := uuid.Parse(strings.TrimSpace(actorSessionID))
	if err != nil {
		return schedule.StaffActor{}, err
	}
	role, err := authz.NormalizeRole(strings.TrimSpace(actorRole))
	if err != nil {
		return schedule.StaffActor{}, err
	}
	if requireOwner && role != authz.RoleOwner {
		return schedule.StaffActor{}, fmt.Errorf("schedule graph authoring requires owner role")
	}
	if !requireOwner && capability == authz.CapabilityScheduleManage && role != authz.RoleManager && role != authz.RoleOwner {
		return schedule.StaffActor{}, fmt.Errorf("schedule writes require manager or owner role")
	}
	label := strings.TrimSpace(trustedSurfaceLabel)
	if label == "" {
		label = strings.TrimSpace(trustedSurfaceKey)
	}
	actor := schedule.StaffActor{
		UserID:              userID,
		SessionID:           sessionID,
		Role:                role,
		Capability:          capability,
		TrustedSurfaceKey:   strings.TrimSpace(trustedSurfaceKey),
		TrustedSurfaceLabel: label,
	}
	if err := scheduleValidateActor(actor); err != nil {
		return schedule.StaffActor{}, err
	}
	_ = cmd
	return actor, nil
}

func scheduleValidateActor(actor schedule.StaffActor) error {
	if actor.UserID == uuid.Nil || actor.SessionID == uuid.Nil {
		return fmt.Errorf("actor attribution is required")
	}
	if strings.TrimSpace(actor.TrustedSurfaceKey) == "" {
		return fmt.Errorf("trusted surface key is required")
	}
	return nil
}

func parseScheduleWindow(from string, until string) (time.Time, time.Time, error) {
	windowFrom, err := parseScheduleBoundary(from, false)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	windowUntil, err := parseScheduleBoundary(until, true)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return windowFrom, windowUntil, nil
}

func parseScheduleBoundary(raw string, _ bool) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, fmt.Errorf("window boundary is required")
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("window boundary must be RFC3339")
}

func newSportCmd() *cobra.Command {
	sportCmd := &cobra.Command{
		Use:   "sport",
		Short: "Read APOLLO sport substrate truth.",
	}

	var listFormat string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List APOLLO sport definitions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openSportsService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			entries, err := service.ListSports(cmd.Context())
			if err != nil {
				return err
			}

			switch listFormat {
			case "json":
				return writeJSONOutput(cmd, entries)
			case "text":
				return writeSportListText(cmd, entries)
			default:
				return fmt.Errorf("unsupported format %q", listFormat)
			}
		},
	}
	listCmd.Flags().StringVar(&listFormat, "format", "text", "output format: text or json")

	var showSportKey string
	var showFormat string
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show one APOLLO sport definition and its facility capabilities.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openSportsService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			entry, err := service.GetSport(cmd.Context(), showSportKey)
			if err != nil {
				if errors.Is(err, sports.ErrSportNotFound) {
					return fmt.Errorf("sport %q not found", strings.TrimSpace(showSportKey))
				}
				return err
			}

			switch showFormat {
			case "json":
				return writeJSONOutput(cmd, entry)
			case "text":
				return writeSportDetailText(cmd, entry)
			default:
				return fmt.Errorf("unsupported format %q", showFormat)
			}
		},
	}
	showCmd.Flags().StringVar(&showSportKey, "sport-key", "", "sport key to show")
	showCmd.Flags().StringVar(&showFormat, "format", "text", "output format: text or json")
	_ = showCmd.MarkFlagRequired("sport-key")

	capabilityCmd := &cobra.Command{
		Use:   "capability",
		Short: "Read APOLLO facility-sport capability mappings.",
	}

	var capabilitySportKey string
	var capabilityFacilityKey string
	var capabilityFormat string
	capabilityListCmd := &cobra.Command{
		Use:   "list",
		Short: "List facility-sport capability mappings.",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, pool, err := openSportsService(cmd.Context())
			if err != nil {
				return err
			}
			defer pool.Close()

			entries, err := service.ListFacilityCapabilities(cmd.Context(), sports.CapabilityFilter{
				SportKey:    capabilitySportKey,
				FacilityKey: capabilityFacilityKey,
			})
			if err != nil {
				if errors.Is(err, sports.ErrSportNotFound) {
					return fmt.Errorf("sport %q not found", strings.TrimSpace(capabilitySportKey))
				}
				if errors.Is(err, sports.ErrFacilityNotFound) {
					return fmt.Errorf("facility %q not found", strings.TrimSpace(capabilityFacilityKey))
				}
				return err
			}

			switch capabilityFormat {
			case "json":
				return writeJSONOutput(cmd, entries)
			case "text":
				return writeSportCapabilityListText(cmd, entries)
			default:
				return fmt.Errorf("unsupported format %q", capabilityFormat)
			}
		},
	}
	capabilityListCmd.Flags().StringVar(&capabilitySportKey, "sport-key", "", "filter by sport key")
	capabilityListCmd.Flags().StringVar(&capabilityFacilityKey, "facility-key", "", "filter by facility key")
	capabilityListCmd.Flags().StringVar(&capabilityFormat, "format", "text", "output format: text or json")

	capabilityCmd.AddCommand(capabilityListCmd)
	sportCmd.AddCommand(listCmd)
	sportCmd.AddCommand(showCmd)
	sportCmd.AddCommand(capabilityCmd)

	return sportCmd
}

func openPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("APOLLO_DATABASE_URL is required")
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

func openSportsService(ctx context.Context) (*sports.Service, *pgxpool.Pool, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}

	pool, err := openPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}

	return sports.NewService(sports.NewRepository(pool)), pool, nil
}

func writeJSONOutput(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeCompetitionSessionListText(cmd *cobra.Command, sessions []competition.SessionSummary) error {
	for _, session := range sessions {
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"id=%s name=%q sport=%s facility=%s status=%s queue_version=%d participants_per_side=%d\n",
			session.ID,
			session.DisplayName,
			session.SportKey,
			session.FacilityKey,
			session.Status,
			session.QueueVersion,
			session.ParticipantsPerSide,
		); err != nil {
			return err
		}
	}
	return nil
}

func writeCompetitionSessionText(cmd *cobra.Command, session competition.Session) error {
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"id=%s name=%q sport=%s facility=%s status=%s queue_version=%d teams=%d matches=%d queue_members=%d\n",
		session.ID,
		session.DisplayName,
		session.SportKey,
		session.FacilityKey,
		session.Status,
		session.QueueVersion,
		len(session.Teams),
		len(session.Matches),
		len(session.Queue),
	); err != nil {
		return err
	}
	return nil
}

func writeCompetitionTournamentListText(cmd *cobra.Command, tournaments []competition.TournamentSummary) error {
	for _, tournament := range tournaments {
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"id=%s name=%q format=%s visibility=%s sport=%s facility=%s status=%s tournament_version=%d participants_per_side=%d\n",
			tournament.ID,
			tournament.DisplayName,
			tournament.Format,
			tournament.Visibility,
			tournament.SportKey,
			tournament.FacilityKey,
			tournament.Status,
			tournament.TournamentVersion,
			tournament.ParticipantsPerSide,
		); err != nil {
			return err
		}
	}
	return nil
}

func writeCompetitionTournamentText(cmd *cobra.Command, tournament competition.Tournament) error {
	seedCount := 0
	snapshotCount := 0
	bindingCount := 0
	advancementCount := 0
	for _, bracket := range tournament.Brackets {
		seedCount += len(bracket.Seeds)
		snapshotCount += len(bracket.TeamSnapshots)
		bindingCount += len(bracket.MatchBindings)
		advancementCount += len(bracket.Advancements)
	}
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"id=%s name=%q format=%s visibility=%s sport=%s facility=%s status=%s tournament_version=%d brackets=%d seeds=%d snapshots=%d bindings=%d advancements=%d\n",
		tournament.ID,
		tournament.DisplayName,
		tournament.Format,
		tournament.Visibility,
		tournament.SportKey,
		tournament.FacilityKey,
		tournament.Status,
		tournament.TournamentVersion,
		len(tournament.Brackets),
		seedCount,
		snapshotCount,
		bindingCount,
		advancementCount,
	); err != nil {
		return err
	}
	return nil
}

func writeCompetitionReadinessText(cmd *cobra.Command, readiness competition.CompetitionCommandReadiness) error {
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "status=%s role=%s message=%q\n", readiness.Status, readiness.Actor.Role, readiness.Message); err != nil {
		return err
	}
	for _, item := range readiness.Commands {
		reason := item.UnavailableReason
		if reason == "" {
			reason = "available"
		}
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"command=%s available=%t capability=%s dry_run=%t apply=%t reason=%q\n",
			item.Name,
			item.Available,
			item.RequiredCapability,
			item.DryRunSupported,
			item.ApplySupported,
			reason,
		); err != nil {
			return err
		}
	}
	return nil
}

func writeCompetitionCommandOutcomeText(cmd *cobra.Command, outcome competition.CompetitionCommandOutcome) error {
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"name=%s status=%s dry_run=%t mutated=%t capability=%s message=%q\n",
		outcome.Name,
		outcome.Status,
		outcome.DryRun,
		outcome.Mutated,
		outcome.RequiredCapability,
		outcome.Message,
	); err != nil {
		return err
	}
	if outcome.Error != "" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "error=%q\n", outcome.Error); err != nil {
			return err
		}
	}
	for _, step := range outcome.Plan {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "plan=%s resource_type=%s resource_id=%s description=%q\n", step.Action, step.ResourceType, step.ResourceID, step.Description); err != nil {
			return err
		}
	}
	return nil
}

func writePublicCompetitionReadinessText(cmd *cobra.Command, readiness competition.PublicCompetitionReadiness) error {
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"status=%s contract=%s projection=%s result_source=%s rating_source=%s leaderboards=%d canonical_results=%d\n",
		readiness.Status,
		readiness.ContractVersion,
		readiness.ProjectionVersion,
		readiness.ResultSource,
		readiness.RatingSource,
		readiness.AvailableLeaderboards,
		readiness.AvailableCanonicalResults,
	); err != nil {
		return err
	}
	if len(readiness.Deferred) > 0 {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "deferred=%s\n", strings.Join(readiness.Deferred, ",")); err != nil {
			return err
		}
	}
	return nil
}

func writePublicCompetitionLeaderboardText(cmd *cobra.Command, leaderboard competition.PublicCompetitionLeaderboard) error {
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"contract=%s projection=%s result_source=%s rating_source=%s rows=%d\n",
		leaderboard.ContractVersion,
		leaderboard.ProjectionVersion,
		leaderboard.ResultSource,
		leaderboard.RatingSource,
		len(leaderboard.Leaderboard),
	); err != nil {
		return err
	}
	for _, row := range leaderboard.Leaderboard {
		lastResultAt := ""
		if row.LastResultAt != nil {
			lastResultAt = formatCompetitionTime(*row.LastResultAt)
		}
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"rank=%d participant=%s sport=%s mode=%s facility=%s team_scope=%s stat=%s value=%.4f last_result_at=%s computed_at=%s\n",
			row.Rank,
			row.Participant,
			row.SportKey,
			row.ModeKey,
			row.FacilityKey,
			row.TeamScope,
			row.StatType,
			row.StatValue,
			lastResultAt,
			formatCompetitionTime(row.ComputedAt),
		); err != nil {
			return err
		}
	}
	return nil
}

func writeGameIdentityText(cmd *cobra.Command, identity competition.GameIdentityProjection) error {
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"status=%s contract=%s projection=%s result_source=%s rating_source=%s cp_policy=%s badge_policy=%s rivalry_policy=%s squad_policy=%s cp_rows=%d badges=%d rivalries=%d squads=%d\n",
		identity.Status,
		identity.ContractVersion,
		identity.ProjectionVersion,
		identity.ResultSource,
		identity.RatingSource,
		identity.CPPolicyVersion,
		identity.BadgePolicyVersion,
		identity.RivalryPolicyVersion,
		identity.SquadPolicyVersion,
		len(identity.CP),
		len(identity.BadgeAwards),
		len(identity.RivalryStates),
		len(identity.SquadIdentities),
	); err != nil {
		return err
	}
	for _, row := range identity.CP {
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"rank=%d participant=%s sport=%s mode=%s facility=%s team_scope=%s cp=%d computed_at=%s\n",
			row.Rank,
			row.Participant,
			row.SportKey,
			row.ModeKey,
			row.FacilityKey,
			row.TeamScope,
			row.CP,
			formatCompetitionTime(row.ComputedAt),
		); err != nil {
			return err
		}
	}
	for _, award := range identity.BadgeAwards {
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"badge=%s participant=%s sport=%s mode=%s facility=%s team_scope=%s policy=%s computed_at=%s\n",
			award.BadgeKey,
			award.Participant,
			award.SportKey,
			award.ModeKey,
			award.FacilityKey,
			award.TeamScope,
			award.PolicyVersion,
			formatCompetitionTime(award.ComputedAt),
		); err != nil {
			return err
		}
	}
	return nil
}

func writeMemberStatsText(cmd *cobra.Command, stats []competition.MemberStat) error {
	for _, stat := range stats {
		lastPlayedAt := ""
		if stat.LastPlayedAt != nil {
			lastPlayedAt = formatCompetitionTime(*stat.LastPlayedAt)
		}
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"user_id=%s sport=%s mode=%s played=%d wins=%d losses=%d draws=%d rating_mu=%.4f rating_sigma=%.4f last_played_at=%s\n",
			stat.UserID,
			stat.SportKey,
			stat.ModeKey,
			stat.MatchesPlayed,
			stat.Wins,
			stat.Losses,
			stat.Draws,
			stat.CurrentRatingMu,
			stat.CurrentRatingSigma,
			lastPlayedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func writeMemberHistoryText(cmd *cobra.Command, history []competition.MemberHistoryEntry) error {
	for _, entry := range history {
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"match_id=%s source_result_id=%s canonical_result_id=%s name=%q sport=%s mode=%s facility=%s outcome=%s recorded_at=%s\n",
			entry.CompetitionMatchID,
			entry.SourceResultID,
			entry.CanonicalResultID,
			entry.DisplayName,
			entry.SportKey,
			entry.ModeKey,
			entry.FacilityKey,
			entry.Outcome,
			formatCompetitionTime(entry.RecordedAt),
		); err != nil {
			return err
		}
	}
	return nil
}

func writeCompetitionSafetyReadinessText(cmd *cobra.Command, readiness competition.CompetitionSafetyReadiness) error {
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"status=%s role=%s reports=%d blocks=%d reliability_events=%d audit_events=%d message=%q\n",
		readiness.Status,
		readiness.Actor.Role,
		readiness.Summary.ReportCount,
		readiness.Summary.BlockCount,
		readiness.Summary.ReliabilityEventCount,
		readiness.Summary.AuditEventCount,
		readiness.Message,
	); err != nil {
		return err
	}
	for _, item := range readiness.Commands {
		reason := item.UnavailableReason
		if reason == "" {
			reason = "available"
		}
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"command=%s available=%t capability=%s dry_run=%t apply=%t reason=%q\n",
			item.Name,
			item.Available,
			item.RequiredCapability,
			item.DryRunSupported,
			item.ApplySupported,
			reason,
		); err != nil {
			return err
		}
	}
	return nil
}

func writeCompetitionSafetyReviewText(cmd *cobra.Command, review competition.CompetitionSafetyReview) error {
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"reports=%d blocks=%d reliability_events=%d audit_events=%d\n",
		review.Summary.ReportCount,
		review.Summary.BlockCount,
		review.Summary.ReliabilityEventCount,
		review.Summary.AuditEventCount,
	); err != nil {
		return err
	}
	for _, report := range review.Reports {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "report=%s target_type=%s reason=%s status=%s occurred_at=%s\n", report.ID, report.TargetType, report.ReasonCode, report.Status, formatCompetitionTime(report.OccurredAt)); err != nil {
			return err
		}
	}
	for _, block := range review.Blocks {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "block=%s reason=%s status=%s occurred_at=%s\n", block.ID, block.ReasonCode, block.Status, formatCompetitionTime(block.OccurredAt)); err != nil {
			return err
		}
	}
	for _, reliability := range review.ReliabilityEvents {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "reliability_event=%s type=%s severity=%s occurred_at=%s\n", reliability.ID, reliability.ReliabilityType, reliability.Severity, formatCompetitionTime(reliability.OccurredAt)); err != nil {
			return err
		}
	}
	for _, event := range review.AuditEvents {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "audit_event=%s type=%s privacy_scope=%s occurred_at=%s\n", event.ID, event.EventType, event.PrivacyScope, formatCompetitionTime(event.OccurredAt)); err != nil {
			return err
		}
	}
	return nil
}

func writeSportListText(cmd *cobra.Command, sportsList []sports.Sport) error {
	for _, entry := range sportsList {
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"key=%s name=%q mode=%s sides=%d participants_per_side=%s scoring=%s default_duration_minutes=%d\n",
			entry.SportKey,
			entry.DisplayName,
			entry.CompetitionMode,
			entry.SidesPerMatch,
			formatParticipantRange(entry.ParticipantsPerSideMin, entry.ParticipantsPerSideMax),
			entry.ScoringModel,
			entry.DefaultMatchDurationMinutes,
		); err != nil {
			return err
		}
	}

	return nil
}

func writeSportDetailText(cmd *cobra.Command, entry sports.SportDetail) error {
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"key=%s name=%q description=%q mode=%s sides=%d participants_per_side=%s scoring=%s default_duration_minutes=%d rules=%q\n",
		entry.SportKey,
		entry.DisplayName,
		entry.Description,
		entry.CompetitionMode,
		entry.SidesPerMatch,
		formatParticipantRange(entry.ParticipantsPerSideMin, entry.ParticipantsPerSideMax),
		entry.ScoringModel,
		entry.DefaultMatchDurationMinutes,
		entry.RulesSummary,
	); err != nil {
		return err
	}

	if len(entry.FacilityCapabilities) == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "facility_capabilities=none")
		return err
	}

	return writeSportCapabilityListText(cmd, entry.FacilityCapabilities)
}

func writeSportCapabilityListText(cmd *cobra.Command, capabilities []sports.FacilityCapability) error {
	for _, entry := range capabilities {
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"sport=%s facility=%s zones=%s\n",
			entry.SportKey,
			entry.FacilityKey,
			formatZoneKeys(entry.ZoneKeys),
		); err != nil {
			return err
		}
	}

	return nil
}

func formatTimestamp(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}

	return value.Time.Format(time.RFC3339)
}

func formatCompetitionTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func formatParticipantRange(minimum int, maximum int) string {
	if minimum == maximum {
		return fmt.Sprintf("%d", minimum)
	}

	return fmt.Sprintf("%d-%d", minimum, maximum)
}

func formatZoneKeys(zoneKeys []string) string {
	if len(zoneKeys) == 0 {
		return "-"
	}

	return strings.Join(zoneKeys, ",")
}
