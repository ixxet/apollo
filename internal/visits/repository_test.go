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

func TestRepositoryCloseVisitPersistsDepartureTracking(t *testing.T) {
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
	departedAt := time.Date(2026, 4, 1, 13, 10, 0, 0, time.UTC)

	visit, err := repository.CreateVisit(ctx, store.CreateVisitParams{
		UserID:        user.ID,
		FacilityKey:   "ashtonbee",
		SourceEventID: stringPtr("evt-open-close-001"),
		ArrivedAt:     timestamptz(arrivedAt),
	})
	if err != nil {
		t.Fatalf("CreateVisit() error = %v", err)
	}

	closedVisit, err := repository.CloseVisit(ctx, store.CloseVisitParams{
		ID:                     visit.ID,
		DepartedAt:             timestamptz(departedAt),
		DepartureSourceEventID: stringPtr("evt-close-001"),
	})
	if err != nil {
		t.Fatalf("CloseVisit() error = %v", err)
	}
	if closedVisit == nil {
		t.Fatal("CloseVisit() returned nil visit")
	}
	if !closedVisit.DepartedAt.Valid || !closedVisit.DepartedAt.Time.Equal(departedAt) {
		t.Fatalf("closedVisit.DepartedAt = %#v, want %s", closedVisit.DepartedAt, departedAt)
	}
	if closedVisit.DepartureSourceEventID == nil || *closedVisit.DepartureSourceEventID != "evt-close-001" {
		t.Fatalf("closedVisit.DepartureSourceEventID = %#v, want evt-close-001", closedVisit.DepartureSourceEventID)
	}

	openVisit, err := repository.GetOpenVisitByUserAndFacility(ctx, user.ID, "ashtonbee")
	if err != nil {
		t.Fatalf("GetOpenVisitByUserAndFacility() error = %v", err)
	}
	if openVisit != nil {
		t.Fatalf("GetOpenVisitByUserAndFacility() = %#v, want nil after close", openVisit)
	}

	visitByDeparture, err := repository.GetVisitByDepartureSourceEventID(ctx, "evt-close-001")
	if err != nil {
		t.Fatalf("GetVisitByDepartureSourceEventID() error = %v", err)
	}
	if visitByDeparture == nil {
		t.Fatal("GetVisitByDepartureSourceEventID() returned nil visit")
	}
	if visitByDeparture.ID != visit.ID {
		t.Fatalf("visitByDeparture.ID = %s, want %s", visitByDeparture.ID, visit.ID)
	}
}

func TestRepositoryRejectsDuplicateDepartureSourceEventID(t *testing.T) {
	ctx := context.Background()
	env := newPostgresEnv(t, ctx)
	defer func() {
		if err := env.Close(); err != nil {
			t.Fatalf("env.Close() error = %v", err)
		}
	}()

	repository := NewRepository(env.DB)
	user := lookupTracerUser(t, ctx, repository)

	firstVisit, err := repository.CreateVisit(ctx, store.CreateVisitParams{
		UserID:        user.ID,
		FacilityKey:   "ashtonbee",
		SourceEventID: stringPtr("evt-open-a"),
		ArrivedAt:     timestamptz(time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("CreateVisit(first) error = %v", err)
	}
	secondVisit, err := repository.CreateVisit(ctx, store.CreateVisitParams{
		UserID:        user.ID,
		FacilityKey:   "annex",
		SourceEventID: stringPtr("evt-open-b"),
		ArrivedAt:     timestamptz(time.Date(2026, 4, 1, 12, 45, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("CreateVisit(second) error = %v", err)
	}

	if _, err := repository.CloseVisit(ctx, store.CloseVisitParams{
		ID:                     firstVisit.ID,
		DepartedAt:             timestamptz(time.Date(2026, 4, 1, 13, 0, 0, 0, time.UTC)),
		DepartureSourceEventID: stringPtr("evt-depart-duplicate"),
	}); err != nil {
		t.Fatalf("CloseVisit(first) error = %v", err)
	}

	_, err = repository.CloseVisit(ctx, store.CloseVisitParams{
		ID:                     secondVisit.ID,
		DepartedAt:             timestamptz(time.Date(2026, 4, 1, 13, 5, 0, 0, time.UTC)),
		DepartureSourceEventID: stringPtr("evt-depart-duplicate"),
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
		testutil.RepoFilePath("db", "migrations", "004_visit_departure_tracking.up.sql"),
		testutil.RepoFilePath("db", "migrations", "005_workout_runtime.up.sql"),
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
