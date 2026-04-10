package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"

	dbmigrations "github.com/ixxet/apollo/db/migrations"
	"github.com/ixxet/apollo/internal/ares"
	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/competition"
	"github.com/ixxet/apollo/internal/config"
	"github.com/ixxet/apollo/internal/consumer"
	"github.com/ixxet/apollo/internal/eligibility"
	"github.com/ixxet/apollo/internal/exercises"
	"github.com/ixxet/apollo/internal/membership"
	"github.com/ixxet/apollo/internal/planner"
	"github.com/ixxet/apollo/internal/profile"
	"github.com/ixxet/apollo/internal/recommendations"
	"github.com/ixxet/apollo/internal/server"
	"github.com/ixxet/apollo/internal/sports"
	"github.com/ixxet/apollo/internal/visits"
	"github.com/ixxet/apollo/internal/workouts"
	protoevents "github.com/ixxet/ashton-proto/events"
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
			service := visits.NewService(repository)

			consumerEnabled := false
			var closeNATS func()
			if cfg.NATSURL != "" {
				conn, err := nats.Connect(cfg.NATSURL)
				if err != nil {
					return err
				}
				closeNATS = conn.Close

				arrivalHandler := consumer.NewIdentifiedPresenceHandler(service)
				if _, err := conn.Subscribe(protoevents.SubjectIdentifiedPresenceArrived, func(msg *nats.Msg) {
					if _, err := arrivalHandler.HandleMessage(context.Background(), msg.Data); err != nil {
						slog.Error("identified arrival consumer failed", "error", err)
					}
				}); err != nil {
					closeNATS()
					return err
				}

				departureHandler := consumer.NewIdentifiedDepartureHandler(service)
				if _, err := conn.Subscribe(protoevents.SubjectIdentifiedPresenceDeparted, func(msg *nats.Msg) {
					if _, err := departureHandler.HandleMessage(context.Background(), msg.Data); err != nil {
						slog.Error("identified departure consumer failed", "error", err)
					}
				}); err != nil {
					closeNATS()
					return err
				}
				consumerEnabled = true
				slog.Info("identified presence consumer enabled", "subjects", []string{
					protoevents.SubjectIdentifiedPresenceArrived,
					protoevents.SubjectIdentifiedPresenceDeparted,
				})
			}
			if closeNATS != nil {
				defer closeNATS()
			}

			if cfg.SessionCookieSecret == "" {
				return fmt.Errorf("APOLLO_SESSION_COOKIE_SECRET is required")
			}

			cookies, err := auth.NewSessionCookieManager(cfg.SessionCookieName, cfg.SessionCookieSecret, cfg.SessionCookieSecure)
			if err != nil {
				return err
			}

			var sender auth.EmailSender
			if cfg.LogVerificationTokens {
				sender = auth.LogEmailSender{}
			}

			httpServer := &http.Server{
				Addr:    cfg.HTTPAddr,
				Handler: server.NewHandler(buildServerDependencies(pool, consumerEnabled, cookies, sender, cfg)),
			}

			slog.Info("starting APOLLO server", "addr", cfg.HTTPAddr)
			if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}

			return nil
		},
	}
}

func buildServerDependencies(pool *pgxpool.Pool, consumerEnabled bool, cookies *auth.SessionCookieManager, sender auth.EmailSender, cfg config.Config) server.Dependencies {
	authRepository := auth.NewRepository(pool)
	authService := auth.NewService(authRepository, cookies, sender, cfg.VerificationTokenTTL, cfg.SessionTTL)
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
	workoutService := workouts.NewService(workouts.NewRepository(pool))
	competitionService := competition.NewService(competition.NewRepository(pool))

	return server.Dependencies{
		ConsumerEnabled: consumerEnabled,
		Auth:            authService,
		Competition:     competitionService,
		Profile:         profileService,
		Exercises:       exerciseService,
		Planner:         plannerService,
		Eligibility:     eligibilityService,
		Membership:      membershipService,
		MatchPreview:    matchPreviewService,
		Recommendations: recommendationService,
		Workouts:        workoutService,
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
