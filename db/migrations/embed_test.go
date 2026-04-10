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

	var equipmentDefinitionsTableCount int
	if err := postgresEnv.DB.QueryRow(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = 'apollo'
  AND table_name = 'equipment_definitions'
`).Scan(&equipmentDefinitionsTableCount); err != nil {
		t.Fatalf("count equipment_definitions table error = %v", err)
	}

	if equipmentDefinitionsTableCount != 1 {
		t.Fatalf("equipment_definitions table count = %d, want 1", equipmentDefinitionsTableCount)
	}

	var plannerWeeksTableCount int
	if err := postgresEnv.DB.QueryRow(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = 'apollo'
  AND table_name = 'planner_weeks'
`).Scan(&plannerWeeksTableCount); err != nil {
		t.Fatalf("count planner_weeks table error = %v", err)
	}

	if plannerWeeksTableCount != 1 {
		t.Fatalf("planner_weeks table count = %d, want 1", plannerWeeksTableCount)
	}

	var seededEquipmentCount int
	if err := postgresEnv.DB.QueryRow(ctx, `SELECT count(*) FROM apollo.equipment_definitions`).Scan(&seededEquipmentCount); err != nil {
		t.Fatalf("count seeded equipment error = %v", err)
	}

	if seededEquipmentCount != 5 {
		t.Fatalf("seeded equipment count = %d, want 5", seededEquipmentCount)
	}

	var seededExerciseCount int
	if err := postgresEnv.DB.QueryRow(ctx, `SELECT count(*) FROM apollo.exercise_definitions`).Scan(&seededExerciseCount); err != nil {
		t.Fatalf("count seeded exercise definitions error = %v", err)
	}

	if seededExerciseCount != 6 {
		t.Fatalf("seeded exercise definition count = %d, want 6", seededExerciseCount)
	}

	var effortFeedbackTableCount int
	if err := postgresEnv.DB.QueryRow(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = 'apollo'
  AND table_name = 'workout_effort_feedback'
`).Scan(&effortFeedbackTableCount); err != nil {
		t.Fatalf("count workout_effort_feedback table error = %v", err)
	}

	if effortFeedbackTableCount != 1 {
		t.Fatalf("workout_effort_feedback table count = %d, want 1", effortFeedbackTableCount)
	}

	var recoveryFeedbackTableCount int
	if err := postgresEnv.DB.QueryRow(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = 'apollo'
  AND table_name = 'workout_recovery_feedback'
`).Scan(&recoveryFeedbackTableCount); err != nil {
		t.Fatalf("count workout_recovery_feedback table error = %v", err)
	}

	if recoveryFeedbackTableCount != 1 {
		t.Fatalf("workout_recovery_feedback table count = %d, want 1", recoveryFeedbackTableCount)
	}
}
