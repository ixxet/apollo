package visits

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/store"
	"github.com/ixxet/apollo/internal/testutil"
)

func TestRepositoryVisitPersistenceAndIsolation(t *testing.T) {
	ctx := context.Background()
	env := newPostgresEnv(t, ctx)
	defer func() {
		if err := env.Close(); err != nil {
			t.Fatalf("env.Close() error = %v", err)
		}
	}()

	repository := NewRepository(env.DB)
	user := lookupTracerUser(t, ctx, repository)
	arrivedAt := time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC)

	firstVisit, err := repository.CreateVisit(ctx, store.CreateVisitParams{
		UserID:        user.ID,
		FacilityKey:   "ashtonbee",
		SourceEventID: stringPtr("evt-001"),
		ArrivedAt:     timestamptz(arrivedAt),
	})
	if err != nil {
		t.Fatalf("CreateVisit(first) error = %v", err)
	}
	secondVisit, err := repository.CreateVisit(ctx, store.CreateVisitParams{
		UserID:        user.ID,
		FacilityKey:   "annex",
		SourceEventID: stringPtr("evt-002"),
		ArrivedAt:     timestamptz(arrivedAt.Add(time.Minute)),
	})
	if err != nil {
		t.Fatalf("CreateVisit(second) error = %v", err)
	}

	rows, err := repository.ListByStudentID(ctx, "tracer2-student-001")
	if err != nil {
		t.Fatalf("ListByStudentID() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if firstVisit.FacilityKey == secondVisit.FacilityKey {
		t.Fatal("expected visit persistence test to use distinct facilities")
	}
}

func TestRepositoryRejectsDuplicateSourceEventID(t *testing.T) {
	ctx := context.Background()
	env := newPostgresEnv(t, ctx)
	defer func() {
		if err := env.Close(); err != nil {
			t.Fatalf("env.Close() error = %v", err)
		}
	}()

	repository := NewRepository(env.DB)
	user := lookupTracerUser(t, ctx, repository)

	_, err := repository.CreateVisit(ctx, store.CreateVisitParams{
		UserID:        user.ID,
		FacilityKey:   "ashtonbee",
		SourceEventID: stringPtr("evt-duplicate"),
		ArrivedAt:     timestamptz(time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("CreateVisit(first) error = %v", err)
	}
	_, err = repository.CreateVisit(ctx, store.CreateVisitParams{
		UserID:        user.ID,
		FacilityKey:   "annex",
		SourceEventID: stringPtr("evt-duplicate"),
		ArrivedAt:     timestamptz(time.Date(2026, 4, 1, 12, 31, 0, 0, time.UTC)),
	})
	assertUniqueViolation(t, err)
}

func TestRepositoryRejectsSecondOpenVisitForSameFacility(t *testing.T) {
	ctx := context.Background()
	env := newPostgresEnv(t, ctx)
	defer func() {
		if err := env.Close(); err != nil {
			t.Fatalf("env.Close() error = %v", err)
		}
	}()

	repository := NewRepository(env.DB)
	user := lookupTracerUser(t, ctx, repository)

	_, err := repository.CreateVisit(ctx, store.CreateVisitParams{
		UserID:        user.ID,
		FacilityKey:   "ashtonbee",
		SourceEventID: stringPtr("evt-open-1"),
		ArrivedAt:     timestamptz(time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("CreateVisit(first) error = %v", err)
	}
	_, err = repository.CreateVisit(ctx, store.CreateVisitParams{
		UserID:        user.ID,
		FacilityKey:   "ashtonbee",
		SourceEventID: stringPtr("evt-open-2"),
		ArrivedAt:     timestamptz(time.Date(2026, 4, 1, 12, 31, 0, 0, time.UTC)),
	})
	assertUniqueViolation(t, err)
}

func newPostgresEnv(t *testing.T, ctx context.Context) *testutil.PostgresEnv {
	t.Helper()

	env, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	if err := testutil.ApplySQLFiles(
		ctx,
		env.DB,
		testutil.RepoFilePath("db", "migrations", "001_initial.up.sql"),
		testutil.RepoFilePath("db", "migrations", "002_open_visit_uniqueness.up.sql"),
		testutil.RepoFilePath("db", "migrations", "003_member_auth_and_sessions.up.sql"),
		testutil.RepoFilePath("db", "seeds", "tracer2.sql"),
	); err != nil {
		t.Fatalf("ApplySQLFiles() error = %v", err)
	}

	return env
}

func lookupTracerUser(t *testing.T, ctx context.Context, repository *Repository) *store.ApolloUser {
	t.Helper()

	user, err := repository.FindActiveUserByTagHash(ctx, "tag_tracer2_001")
	if err != nil {
		t.Fatalf("FindActiveUserByTagHash() error = %v", err)
	}
	if user == nil {
		t.Fatal("FindActiveUserByTagHash() returned nil user")
	}

	return user
}

func assertUniqueViolation(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("expected unique violation error, got nil")
	}

	pgErr, ok := err.(*pgconn.PgError)
	if !ok || pgErr.Code != "23505" {
		t.Fatalf("expected unique violation, got %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
