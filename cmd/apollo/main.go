package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"

	"github.com/ixxet/apollo/internal/config"
	"github.com/ixxet/apollo/internal/consumer"
	"github.com/ixxet/apollo/internal/server"
	"github.com/ixxet/apollo/internal/visits"
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
		Short:         "APOLLO records member visit history from ATHENA presence.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newVisitCmd())

	return rootCmd
}

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the APOLLO HTTP server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

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

				handler := consumer.NewIdentifiedPresenceHandler(service)
				if _, err := conn.Subscribe(consumer.SubjectIdentifiedPresenceArrived, func(msg *nats.Msg) {
					if _, err := handler.HandleMessage(context.Background(), msg.Data); err != nil {
						slog.Error("identified presence consumer failed", "error", err)
					}
				}); err != nil {
					closeNATS()
					return err
				}
				consumerEnabled = true
				slog.Info("identified presence consumer enabled", "subject", consumer.SubjectIdentifiedPresenceArrived)
			}
			if closeNATS != nil {
				defer closeNATS()
			}

			httpServer := &http.Server{
				Addr:    cfg.HTTPAddr,
				Handler: server.NewHandler(consumerEnabled),
			}

			slog.Info("starting APOLLO server", "addr", cfg.HTTPAddr)
			if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}

			return nil
		},
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
			cfg := config.Load()

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

func formatTimestamp(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}

	return value.Time.Format(time.RFC3339)
}
