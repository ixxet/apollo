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
		RepoFilePath("db", "migrations", "018_booking_request_runtime.up.sql"),
		RepoFilePath("db", "migrations", "019_approved_booking_cancellation.up.sql"),
		RepoFilePath("db", "migrations", "020_public_booking_intake.up.sql"),
		RepoFilePath("db", "migrations", "021_public_booking_receipts.up.sql"),
		RepoFilePath("db", "migrations", "022_booking_request_edit_rebook.up.sql"),
		RepoFilePath("db", "migrations", "023_competition_result_trust.up.sql"),
		RepoFilePath("db", "migrations", "024_competition_rating_foundation.up.sql"),
		RepoFilePath("db", "migrations", "025_competition_openskill_dual_run.up.sql"),
		RepoFilePath("db", "migrations", "026_competition_ares_v2_preview.up.sql"),
		RepoFilePath("db", "migrations", "027_competition_ares_input_watermark.up.sql"),
		RepoFilePath("db", "migrations", "028_competition_analytics_foundation.up.sql"),
		RepoFilePath("db", "migrations", "029_internal_tournament_runtime.up.sql"),
		RepoFilePath("db", "migrations", "030_internal_tournament_cohesion.up.sql"),
		RepoFilePath("db", "migrations", "031_competition_safety_reliability.up.sql"),
	)
}
