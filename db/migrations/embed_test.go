package migrations

import (
	"context"
	"testing"

	"github.com/ixxet/apollo/internal/testutil"
)

func TestApplyAllAppliesEmbeddedMigrationsIdempotently(t *testing.T) {
	ctx := context.Background()

	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if closeErr := postgresEnv.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}()

	if err := ApplyAll(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyAll() first run error = %v", err)
	}
	if err := ApplyAll(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyAll() second run error = %v", err)
	}

	var appliedCount int
	if err := postgresEnv.DB.QueryRow(ctx, `SELECT count(*) FROM apollo.schema_migrations`).Scan(&appliedCount); err != nil {
		t.Fatalf("count applied migrations error = %v", err)
	}

	migrations, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if appliedCount != len(migrations) {
		t.Fatalf("applied migration count = %d, want %d", appliedCount, len(migrations))
	}

	var departureColumnCount int
	if err := postgresEnv.DB.QueryRow(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = 'apollo'
  AND table_name = 'visits'
  AND column_name = 'departure_source_event_id'
`).Scan(&departureColumnCount); err != nil {
		t.Fatalf("count departure_source_event_id column error = %v", err)
	}

	if departureColumnCount != 1 {
		t.Fatalf("departure_source_event_id column count = %d, want 1", departureColumnCount)
	}

	var workoutStatusColumnCount int
	if err := postgresEnv.DB.QueryRow(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = 'apollo'
  AND table_name = 'workouts'
  AND column_name = 'status'
`).Scan(&workoutStatusColumnCount); err != nil {
		t.Fatalf("count workout status column error = %v", err)
	}

	if workoutStatusColumnCount != 1 {
		t.Fatalf("status column count = %d, want 1", workoutStatusColumnCount)
	}

	var exercisePositionColumnCount int
	if err := postgresEnv.DB.QueryRow(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = 'apollo'
  AND table_name = 'exercises'
  AND column_name = 'position'
`).Scan(&exercisePositionColumnCount); err != nil {
		t.Fatalf("count exercise position column error = %v", err)
	}

	if exercisePositionColumnCount != 1 {
		t.Fatalf("position column count = %d, want 1", exercisePositionColumnCount)
	}

	var sportsTableCount int
	if err := postgresEnv.DB.QueryRow(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = 'apollo'
  AND table_name = 'sports'
`).Scan(&sportsTableCount); err != nil {
		t.Fatalf("count sports table error = %v", err)
	}

	if sportsTableCount != 1 {
		t.Fatalf("sports table count = %d, want 1", sportsTableCount)
	}

	var seededSportsCount int
	if err := postgresEnv.DB.QueryRow(ctx, `SELECT count(*) FROM apollo.sports`).Scan(&seededSportsCount); err != nil {
		t.Fatalf("count seeded sports error = %v", err)
	}

	if seededSportsCount != 2 {
		t.Fatalf("seeded sports count = %d, want 2", seededSportsCount)
	}

	var seededCapabilityCount int
	if err := postgresEnv.DB.QueryRow(ctx, `SELECT count(*) FROM apollo.sport_facility_capabilities`).Scan(&seededCapabilityCount); err != nil {
		t.Fatalf("count seeded sport facility capabilities error = %v", err)
	}

	if seededCapabilityCount != 2 {
		t.Fatalf("seeded sport facility capability count = %d, want 2", seededCapabilityCount)
	}

	var competitionSessionsTableCount int
	if err := postgresEnv.DB.QueryRow(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = 'apollo'
  AND table_name = 'competition_sessions'
`).Scan(&competitionSessionsTableCount); err != nil {
		t.Fatalf("count competition_sessions table error = %v", err)
	}

	if competitionSessionsTableCount != 1 {
		t.Fatalf("competition_sessions table count = %d, want 1", competitionSessionsTableCount)
	}
}
