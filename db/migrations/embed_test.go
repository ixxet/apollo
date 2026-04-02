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
}
