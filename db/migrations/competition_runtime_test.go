package migrations

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ixxet/apollo/internal/testutil"
)

func TestCompetitionContainerSchemaEnforcesSessionWideRosterExclusivity(t *testing.T) {
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

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	ownerUserID := insertCompetitionUser(t, ctx, postgresEnv, "competition-owner-001", "Competition Owner", "competition-owner-001@example.com")
	memberUserID := insertCompetitionUser(t, ctx, postgresEnv, "competition-member-001", "Competition Member", "competition-member-001@example.com")

	firstSessionID := insertCompetitionSession(t, ctx, postgresEnv, ownerUserID, "Tracer 20 Session One")
	secondSessionID := insertCompetitionSession(t, ctx, postgresEnv, ownerUserID, "Tracer 20 Session Two")

	firstTeamID := insertCompetitionTeam(t, ctx, postgresEnv, firstSessionID, 1)
	secondTeamID := insertCompetitionTeam(t, ctx, postgresEnv, firstSessionID, 2)
	otherSessionTeamID := insertCompetitionTeam(t, ctx, postgresEnv, secondSessionID, 1)

	insertCompetitionRosterMember(t, ctx, postgresEnv, firstSessionID, firstTeamID, memberUserID, 1)

	_, err = postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_team_roster_members (
    competition_session_id,
    competition_session_team_id,
    user_id,
    slot_index
)
VALUES ($1, $2, $3, $4)
`, firstSessionID, secondTeamID, memberUserID, 1)
	if err == nil {
		t.Fatal("Exec(duplicate same-session roster member) error = nil, want unique violation")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("duplicate same-session roster member error = %v, want pg error", err)
	}
	if pgErr.ConstraintName != "competition_team_roster_members_session_user_unique" {
		t.Fatalf("duplicate same-session roster member constraint = %q, want competition_team_roster_members_session_user_unique", pgErr.ConstraintName)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_team_roster_members (
    competition_session_id,
    competition_session_team_id,
    user_id,
    slot_index
)
VALUES ($1, $2, $3, $4)
`, secondSessionID, otherSessionTeamID, memberUserID, 1); err != nil {
		t.Fatalf("Exec(cross-session roster member) error = %v", err)
	}
}

func TestCompetitionExecutionSchemaSupportsQueueUniquenessAndLifecycleStates(t *testing.T) {
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

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	ownerUserID := insertCompetitionUser(t, ctx, postgresEnv, "competition-owner-021", "Competition Owner", "competition-owner-021@example.com")
	memberUserID := insertCompetitionUser(t, ctx, postgresEnv, "competition-member-021", "Competition Member", "competition-member-021@example.com")

	sessionID := insertCompetitionSession(t, ctx, postgresEnv, ownerUserID, "Tracer 21 Queue Session")
	if _, err := postgresEnv.DB.Exec(ctx, `
UPDATE apollo.competition_sessions
SET status = 'queue_open',
    queue_version = 1
WHERE id = $1
`, sessionID); err != nil {
		t.Fatalf("Exec(update queue_open session) error = %v", err)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_session_queue_members (
    competition_session_id,
    user_id
)
VALUES ($1, $2)
`, sessionID, memberUserID); err != nil {
		t.Fatalf("Exec(insert queue member) error = %v", err)
	}

	_, err = postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_session_queue_members (
    competition_session_id,
    user_id
)
VALUES ($1, $2)
`, sessionID, memberUserID)
	if err == nil {
		t.Fatal("Exec(duplicate queue member) error = nil, want unique violation")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("duplicate queue member error = %v, want pg error", err)
	}
	if pgErr.ConstraintName != "competition_session_queue_members_pkey" {
		t.Fatalf("duplicate queue member constraint = %q, want competition_session_queue_members_pkey", pgErr.ConstraintName)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
UPDATE apollo.competition_sessions
SET status = 'assigned'
WHERE id = $1
`, sessionID); err != nil {
		t.Fatalf("Exec(update assigned session) error = %v", err)
	}
	if _, err := postgresEnv.DB.Exec(ctx, `
UPDATE apollo.competition_sessions
SET status = 'in_progress'
WHERE id = $1
`, sessionID); err != nil {
		t.Fatalf("Exec(update in_progress session) error = %v", err)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_matches (
    competition_session_id,
    match_index,
    status
)
VALUES ($1, $2, 'assigned')
`, sessionID, 1); err != nil {
		t.Fatalf("Exec(insert assigned match) error = %v", err)
	}
}

func TestCompetitionContainerDownMigrationExecutesCleanly(t *testing.T) {
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

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	if err := testutil.ApplySQLFiles(
		ctx,
		postgresEnv.DB,
		testutil.RepoFilePath("db", "migrations", "026_competition_ares_v2_preview.down.sql"),
		testutil.RepoFilePath("db", "migrations", "025_competition_openskill_dual_run.down.sql"),
		testutil.RepoFilePath("db", "migrations", "024_competition_rating_foundation.down.sql"),
		testutil.RepoFilePath("db", "migrations", "023_competition_result_trust.down.sql"),
		testutil.RepoFilePath("db", "migrations", "010_competition_execution_runtime.down.sql"),
		testutil.RepoFilePath("db", "migrations", "009_competition_container_runtime.down.sql"),
	); err != nil {
		t.Fatalf("ApplySQLFiles(down migration) error = %v", err)
	}

	var remainingCompetitionTables int
	if err := postgresEnv.DB.QueryRow(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = 'apollo'
	AND table_name IN (
	    'competition_sessions',
	    'competition_session_queue_members',
	    'competition_queue_intents',
	    'competition_queue_intent_events',
	    'competition_session_teams',
	    'competition_team_roster_members',
	    'competition_matches',
	    'competition_match_side_slots',
	    'competition_match_previews',
	    'competition_match_preview_members',
	    'competition_match_preview_events'
	  )
`).Scan(&remainingCompetitionTables); err != nil {
		t.Fatalf("count competition tables after down migration error = %v", err)
	}

	if remainingCompetitionTables != 0 {
		t.Fatalf("remaining competition table count = %d, want 0", remainingCompetitionTables)
	}
}

func TestCompetitionOpenSkillDualRunSchemaEnforcesComparisonFacts(t *testing.T) {
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

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	ownerUserID := insertCompetitionUser(t, ctx, postgresEnv, "competition-owner-025", "Competition Owner", "competition-owner-025@example.com")
	sessionID := insertCompetitionSession(t, ctx, postgresEnv, ownerUserID, "Tracer 25 OpenSkill")
	matchID := insertCompetitionMatch(t, ctx, postgresEnv, sessionID, 1)
	resultID := insertCompetitionResult(t, ctx, postgresEnv, matchID, ownerUserID)

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_rating_events (
    event_type,
    rating_engine,
    engine_version,
    policy_version,
    sport_key,
    mode_key,
    user_id,
    source_result_id,
    legacy_mu,
    legacy_sigma,
    openskill_mu,
    openskill_sigma,
    delta_from_legacy,
    accepted_delta_budget,
    comparison_scenario,
    projection_watermark,
    occurred_at
)
VALUES (
    'competition.rating.openskill_computed',
    'openskill',
    'openskill_weng_lin_bradley_terry_full.v1',
    'apollo_openskill_dual_run_v1',
    'badminton',
    'head_to_head:s2-p1',
    $1,
    $2,
    27.0000,
    7.9999,
    27.6000,
    8.0600,
    0.6000,
    0.7500,
    'head_to_head:s2-p1:win_loss',
    '2026-04-27T10:30:00Z#openskill-schema',
    NOW()
)
`, ownerUserID, resultID); err != nil {
		t.Fatalf("insert OpenSkill computed event error = %v", err)
	}

	_, err = postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_rating_events (
    event_type,
    rating_engine,
    engine_version,
    policy_version,
    sport_key,
    mode_key,
    user_id,
    source_result_id,
    legacy_mu,
    legacy_sigma,
    openskill_mu,
    openskill_sigma,
    delta_from_legacy,
    accepted_delta_budget,
    comparison_scenario,
    projection_watermark,
    occurred_at
)
VALUES (
    'competition.rating.delta_flagged',
    'openskill',
    'openskill_weng_lin_bradley_terry_full.v1',
    'apollo_openskill_dual_run_v1',
    'badminton',
    'head_to_head:s2-p1',
    $1,
    $2,
    27.0000,
    7.9999,
    27.1000,
    8.0600,
    0.1000,
    0.7500,
    'head_to_head:s2-p1:win_loss',
    '2026-04-27T10:30:00Z#openskill-schema',
    NOW()
)
`, ownerUserID, resultID)
	if err == nil {
		t.Fatal("insert in-budget delta_flagged event error = nil, want check violation")
	}
	var eventPgErr *pgconn.PgError
	if !errors.As(err, &eventPgErr) {
		t.Fatalf("in-budget delta_flagged event error = %v, want pg error", err)
	}
	if eventPgErr.ConstraintName != "competition_rating_events_delta_flagged_budget_required" {
		t.Fatalf("delta_flagged event constraint = %q, want competition_rating_events_delta_flagged_budget_required", eventPgErr.ConstraintName)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_rating_comparisons (
    sport_key,
    mode_key,
    user_id,
    source_result_id,
    legacy_rating_engine,
    legacy_engine_version,
    legacy_policy_version,
    openskill_rating_engine,
    openskill_engine_version,
    openskill_policy_version,
    legacy_mu,
    legacy_sigma,
    openskill_mu,
    openskill_sigma,
    delta_from_legacy,
    accepted_delta_budget,
    comparison_scenario,
    delta_flagged,
    projection_watermark,
    occurred_at
)
VALUES (
    'badminton',
    'head_to_head:s2-p1',
    $1,
    $2,
    'legacy_elo_like',
    'legacy_elo_like.v1',
    'apollo_legacy_rating_v1',
    'openskill',
    'openskill_weng_lin_bradley_terry_full.v1',
    'apollo_openskill_dual_run_v1',
    28.5000,
    7.6800,
    29.6500,
    7.8000,
    1.1500,
    0.7500,
    'head_to_head:s2-p1:win_loss',
    true,
    '2026-04-27T11:30:00Z#openskill-schema',
    NOW()
)
`, ownerUserID, resultID); err != nil {
		t.Fatalf("insert flagged comparison error = %v", err)
	}

	_, err = postgresEnv.DB.Exec(ctx, `
UPDATE apollo.competition_rating_comparisons
SET delta_flagged = false
WHERE user_id = $1
  AND source_result_id = $2
`, ownerUserID, resultID)
	if err == nil {
		t.Fatal("update mismatched delta flag error = nil, want check violation")
	}
	var comparisonPgErr *pgconn.PgError
	if !errors.As(err, &comparisonPgErr) {
		t.Fatalf("mismatched delta flag error = %v, want pg error", err)
	}
	if comparisonPgErr.ConstraintName != "competition_rating_comparisons_flag_matches_budget" {
		t.Fatalf("comparison flag constraint = %q, want competition_rating_comparisons_flag_matches_budget", comparisonPgErr.ConstraintName)
	}
}

func insertCompetitionUser(t *testing.T, ctx context.Context, postgresEnv *testutil.PostgresEnv, studentID string, displayName string, email string) uuid.UUID {
	t.Helper()

	var userID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.users (student_id, display_name, email)
VALUES ($1, $2, $3)
RETURNING id
`, studentID, displayName, email).Scan(&userID); err != nil {
		t.Fatalf("insert competition user error = %v", err)
	}

	return userID
}

func insertCompetitionSession(t *testing.T, ctx context.Context, postgresEnv *testutil.PostgresEnv, ownerUserID uuid.UUID, displayName string) uuid.UUID {
	t.Helper()

	var sessionID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.competition_sessions (
    owner_user_id,
    display_name,
    sport_key,
    facility_key,
    zone_key,
    participants_per_side,
    status
)
VALUES ($1, $2, 'badminton', 'ashtonbee', 'gym-floor', 1, 'draft')
RETURNING id
`, ownerUserID, displayName).Scan(&sessionID); err != nil {
		t.Fatalf("insert competition session error = %v", err)
	}

	return sessionID
}

func insertCompetitionMatch(t *testing.T, ctx context.Context, postgresEnv *testutil.PostgresEnv, sessionID uuid.UUID, matchIndex int) uuid.UUID {
	t.Helper()

	var matchID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.competition_matches (
    competition_session_id,
    match_index,
    status
)
VALUES ($1, $2, 'completed')
RETURNING id
`, sessionID, matchIndex).Scan(&matchID); err != nil {
		t.Fatalf("insert competition match error = %v", err)
	}

	return matchID
}

func insertCompetitionResult(t *testing.T, ctx context.Context, postgresEnv *testutil.PostgresEnv, matchID uuid.UUID, recordedByUserID uuid.UUID) uuid.UUID {
	t.Helper()

	var resultID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.competition_match_results (
    competition_match_id,
    recorded_by_user_id,
    recorded_at,
    result_status,
    dispute_status,
    finalized_at
)
VALUES ($1, $2, NOW(), 'finalized', 'none', NOW())
RETURNING id
`, matchID, recordedByUserID).Scan(&resultID); err != nil {
		t.Fatalf("insert competition result error = %v", err)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
UPDATE apollo.competition_matches
SET canonical_result_id = $2,
    result_version = 1
WHERE id = $1
`, matchID, resultID); err != nil {
		t.Fatalf("update competition match canonical result error = %v", err)
	}

	return resultID
}

func insertCompetitionTeam(t *testing.T, ctx context.Context, postgresEnv *testutil.PostgresEnv, sessionID uuid.UUID, sideIndex int) uuid.UUID {
	t.Helper()

	var teamID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.competition_session_teams (competition_session_id, side_index)
VALUES ($1, $2)
RETURNING id
`, sessionID, sideIndex).Scan(&teamID); err != nil {
		t.Fatalf("insert competition team error = %v", err)
	}

	return teamID
}

func insertCompetitionRosterMember(t *testing.T, ctx context.Context, postgresEnv *testutil.PostgresEnv, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, slotIndex int) {
	t.Helper()

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_team_roster_members (
    competition_session_id,
    competition_session_team_id,
    user_id,
    slot_index
)
VALUES ($1, $2, $3, $4)
`, sessionID, teamID, userID, slotIndex); err != nil {
		t.Fatalf("insert competition roster member error = %v", err)
	}
}
