package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/testutil"
)

func TestWorkoutRuntimeMigrationUpgradesExistingRowsWithoutBreakingNewInProgressWorkouts(t *testing.T) {
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

	if err := testutil.ApplySQLFiles(
		ctx,
		postgresEnv.DB,
		testutil.RepoFilePath("db", "migrations", "001_initial.up.sql"),
		testutil.RepoFilePath("db", "migrations", "002_open_visit_uniqueness.up.sql"),
		testutil.RepoFilePath("db", "migrations", "003_member_auth_and_sessions.up.sql"),
		testutil.RepoFilePath("db", "migrations", "004_visit_departure_tracking.up.sql"),
	); err != nil {
		t.Fatalf("ApplySQLFiles(pre-005) error = %v", err)
	}

	userID := uuid.MustParse("b3448f4c-9656-4e1b-bf88-46eca9382607")
	workoutID := uuid.MustParse("2332ef63-d7e0-44a4-8a54-4cb2dd03e7df")

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.users (id, student_id, display_name, email)
VALUES ($1, $2, $3, $4)
`, userID, "student-workout-migration", "Workout Migration", "workout-migration@example.com"); err != nil {
		t.Fatalf("Exec(insert user) error = %v", err)
	}

	loggedAt := time.Date(2026, 4, 2, 18, 15, 0, 0, time.UTC)
	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.workouts (id, user_id, logged_at, notes, metadata)
VALUES ($1, $2, $3, $4, $5::jsonb)
`, workoutID, userID, loggedAt, "legacy workout", `{"source":"legacy"}`); err != nil {
		t.Fatalf("Exec(insert legacy workout) error = %v", err)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.exercises (workout_id, name, sets, reps, weight_kg, rpe, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`, workoutID, "bench press", 3, 8, 84.50, 8.5, "legacy exercise"); err != nil {
		t.Fatalf("Exec(insert legacy exercise) error = %v", err)
	}

	if err := testutil.ApplySQLFiles(
		ctx,
		postgresEnv.DB,
		testutil.RepoFilePath("db", "migrations", "005_workout_runtime.up.sql"),
	); err != nil {
		t.Fatalf("ApplySQLFiles(005) error = %v", err)
	}

	var status string
	var startedAt time.Time
	var finishedAt *time.Time
	if err := postgresEnv.DB.QueryRow(ctx, `
SELECT status, started_at, finished_at
FROM apollo.workouts
WHERE id = $1
`, workoutID).Scan(&status, &startedAt, &finishedAt); err != nil {
		t.Fatalf("QueryRow(select upgraded workout) error = %v", err)
	}

	if status != "finished" {
		t.Fatalf("status = %q, want finished", status)
	}
	if !startedAt.Equal(loggedAt) {
		t.Fatalf("startedAt = %s, want %s", startedAt, loggedAt)
	}
	if finishedAt == nil || !finishedAt.Equal(loggedAt) {
		t.Fatalf("finishedAt = %v, want %s", finishedAt, loggedAt)
	}

	var position int
	if err := postgresEnv.DB.QueryRow(ctx, `
SELECT position
FROM apollo.exercises
WHERE workout_id = $1
`, workoutID).Scan(&position); err != nil {
		t.Fatalf("QueryRow(select upgraded exercise) error = %v", err)
	}
	if position != 1 {
		t.Fatalf("position = %d, want 1", position)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.workouts (user_id, notes, metadata)
VALUES ($1, $2, $3::jsonb)
`, userID, "fresh runtime workout", `{"source":"runtime"}`); err != nil {
		t.Fatalf("Exec(insert in-progress workout) error = %v", err)
	}
}
