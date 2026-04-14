package testutil

import (
	"context"
	"path/filepath"
	"runtime"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RepoFilePath(parts ...string) string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller() = false")
	}

	allParts := []string{filepath.Dir(currentFile), "..", ".."}
	allParts = append(allParts, parts...)

	return filepath.Join(allParts...)
}

func ApplyApolloSchema(ctx context.Context, db *pgxpool.Pool) error {
	return ApplySQLFiles(
		ctx,
		db,
		RepoFilePath("db", "migrations", "001_initial.up.sql"),
		RepoFilePath("db", "migrations", "002_open_visit_uniqueness.up.sql"),
		RepoFilePath("db", "migrations", "003_member_auth_and_sessions.up.sql"),
		RepoFilePath("db", "migrations", "004_visit_departure_tracking.up.sql"),
		RepoFilePath("db", "migrations", "005_workout_runtime.up.sql"),
		RepoFilePath("db", "migrations", "006_lobby_membership_runtime.up.sql"),
		RepoFilePath("db", "migrations", "007_match_preview_runtime.up.sql"),
		RepoFilePath("db", "migrations", "008_sport_substrate_runtime.up.sql"),
		RepoFilePath("db", "migrations", "009_competition_container_runtime.up.sql"),
		RepoFilePath("db", "migrations", "010_competition_execution_runtime.up.sql"),
		RepoFilePath("db", "migrations", "011_competition_history_runtime.up.sql"),
		RepoFilePath("db", "migrations", "012_planner_runtime_substrate.up.sql"),
		RepoFilePath("db", "migrations", "013_coaching_feedback_runtime.up.sql"),
		RepoFilePath("db", "migrations", "014_nutrition_runtime_substrate.up.sql"),
		RepoFilePath("db", "migrations", "015_presence_runtime.up.sql"),
		RepoFilePath("db", "migrations", "016_competition_authz_runtime.up.sql"),
		RepoFilePath("db", "migrations", "017_schedule_substrate_runtime.up.sql"),
	)
}
