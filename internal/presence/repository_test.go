package presence

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/store"
	"github.com/ixxet/apollo/internal/testutil"
	"github.com/ixxet/apollo/internal/visits"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEnsureLinkedVisitAndCreditStaysIdempotentForAlreadyLinkedVisitAfterTagDeactivation(t *testing.T) {
	ctx := context.Background()
	env, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if closeErr := env.Close(); closeErr != nil {
			t.Fatalf("env.Close() error = %v", closeErr)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, env.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}
	if err := testutil.ApplySQLFiles(ctx, env.DB, testutil.RepoFilePath("db", "seeds", "tracer2.sql")); err != nil {
		t.Fatalf("ApplySQLFiles(seed) error = %v", err)
	}

	visitRepository := visits.NewRepository(env.DB)
	user, err := visitRepository.FindActiveUserByTagHash(ctx, "tag_tracer2_001")
	if err != nil {
		t.Fatalf("FindActiveUserByTagHash() error = %v", err)
	}
	if user == nil {
		t.Fatal("FindActiveUserByTagHash() returned nil user")
	}

	visit, err := visitRepository.CreateVisit(ctx, store.CreateVisitParams{
		UserID:        user.ID,
		FacilityKey:   "ashtonbee",
		SourceEventID: stringPtr("presence-replay-visit-001"),
		ArrivedAt:     pgTimestamp(time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("CreateVisit() error = %v", err)
	}

	repository := NewRepository(env.DB)
	if err := repository.EnsureLinkedVisitAndCredit(ctx, *visit, "tag_tracer2_001", time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("EnsureLinkedVisitAndCredit(first) error = %v", err)
	}

	if _, err := env.DB.Exec(ctx, "UPDATE apollo.claimed_tags SET is_active = FALSE WHERE tag_hash = $1", "tag_tracer2_001"); err != nil {
		t.Fatalf("Exec(deactivate claimed tag) error = %v", err)
	}

	if err := repository.EnsureLinkedVisitAndCredit(ctx, *visit, "tag_tracer2_001", time.Date(2026, 4, 10, 12, 5, 0, 0, time.UTC)); err != nil {
		t.Fatalf("EnsureLinkedVisitAndCredit(replay) error = %v", err)
	}

	assertTableCount(t, ctx, env.DB, "apollo.visit_tap_links", 1)
	assertTableCount(t, ctx, env.DB, "apollo.member_presence_streaks", 1)
	assertTableCount(t, ctx, env.DB, "apollo.member_presence_streak_events", 1)
}

func TestEnsureLinkedVisitAndCreditRejectsSpoofedUnclaimedTag(t *testing.T) {
	ctx := context.Background()
	env, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if closeErr := env.Close(); closeErr != nil {
			t.Fatalf("env.Close() error = %v", closeErr)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, env.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}
	if err := testutil.ApplySQLFiles(ctx, env.DB, testutil.RepoFilePath("db", "seeds", "tracer2.sql")); err != nil {
		t.Fatalf("ApplySQLFiles(seed) error = %v", err)
	}

	visitRepository := visits.NewRepository(env.DB)
	user, err := visitRepository.FindActiveUserByTagHash(ctx, "tag_tracer2_001")
	if err != nil {
		t.Fatalf("FindActiveUserByTagHash() error = %v", err)
	}
	if user == nil {
		t.Fatal("FindActiveUserByTagHash() returned nil user")
	}

	visit, err := visitRepository.CreateVisit(ctx, store.CreateVisitParams{
		UserID:        user.ID,
		FacilityKey:   "ashtonbee",
		SourceEventID: stringPtr("presence-spoof-visit-001"),
		ArrivedAt:     pgTimestamp(time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("CreateVisit() error = %v", err)
	}

	repository := NewRepository(env.DB)
	err = repository.EnsureLinkedVisitAndCredit(ctx, *visit, "spoofed-tag-hash", time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("EnsureLinkedVisitAndCredit() error = nil, want spoofed tag failure")
	}
	if !strings.Contains(err.Error(), "requires active claimed tag") {
		t.Fatalf("EnsureLinkedVisitAndCredit() error = %v, want active claimed tag failure", err)
	}

	assertTableCount(t, ctx, env.DB, "apollo.visit_tap_links", 0)
	assertTableCount(t, ctx, env.DB, "apollo.member_presence_streaks", 0)
	assertTableCount(t, ctx, env.DB, "apollo.member_presence_streak_events", 0)
}

func assertTableCount(t *testing.T, ctx context.Context, db *pgxpool.Pool, table string, want int) {
	t.Helper()

	var got int
	query := "SELECT count(*) FROM " + table
	if err := db.QueryRow(ctx, query).Scan(&got); err != nil {
		t.Fatalf("QueryRow(%s) error = %v", table, err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}

func stringPtr(value string) *string {
	return &value
}
