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
		testutil.RepoFilePath("db", "migrations", "029_internal_tournament_runtime.down.sql"),
		testutil.RepoFilePath("db", "migrations", "028_competition_analytics_foundation.down.sql"),
		testutil.RepoFilePath("db", "migrations", "027_competition_ares_input_watermark.down.sql"),
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
	    'competition_match_preview_events',
	    'competition_analytics_events',
	    'competition_analytics_projections',
	    'competition_tournaments',
	    'competition_tournament_brackets',
	    'competition_tournament_seeds',
	    'competition_tournament_team_snapshots',
	    'competition_tournament_team_snapshot_members',
	    'competition_tournament_match_bindings',
	    'competition_tournament_advancements',
	    'competition_tournament_events'
	  )
`).Scan(&remainingCompetitionTables); err != nil {
		t.Fatalf("count competition tables after down migration error = %v", err)
	}

	if remainingCompetitionTables != 0 {
		t.Fatalf("remaining competition table count = %d, want 0", remainingCompetitionTables)
	}
}

func TestCompetitionTournamentSchemaEnforcesInternalRuntimeFacts(t *testing.T) {
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

	ownerUserID := insertCompetitionUser(t, ctx, postgresEnv, "competition-owner-029", "Competition Owner", "competition-owner-029@example.com")
	memberOneID := insertCompetitionUser(t, ctx, postgresEnv, "competition-member-029-a", "Competition Member A", "competition-member-029-a@example.com")
	memberTwoID := insertCompetitionUser(t, ctx, postgresEnv, "competition-member-029-b", "Competition Member B", "competition-member-029-b@example.com")
	sessionID := insertCompetitionSession(t, ctx, postgresEnv, ownerUserID, "Tracer 29 Tournament Source")
	teamOneID := insertCompetitionTeam(t, ctx, postgresEnv, sessionID, 1)
	teamTwoID := insertCompetitionTeam(t, ctx, postgresEnv, sessionID, 2)
	insertCompetitionRosterMember(t, ctx, postgresEnv, sessionID, teamOneID, memberOneID, 1)
	insertCompetitionRosterMember(t, ctx, postgresEnv, sessionID, teamTwoID, memberTwoID, 1)
	matchID := insertCompetitionMatch(t, ctx, postgresEnv, sessionID, 1)
	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_match_side_slots (
    competition_match_id,
    competition_session_team_id,
    side_index
)
VALUES ($1, $2, 1), ($1, $3, 2)
`, matchID, teamOneID, teamTwoID); err != nil {
		t.Fatalf("insert tournament source match side slots error = %v", err)
	}
	resultID := insertCompetitionResult(t, ctx, postgresEnv, matchID, ownerUserID)

	var tournamentID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.competition_tournaments (
    owner_user_id,
    display_name,
    format,
    sport_key,
    facility_key,
    zone_key,
    participants_per_side,
    updated_at
)
VALUES ($1, 'Tracer 29 Tournament', 'single_elimination', 'badminton', 'ashtonbee', 'gym-floor', 1, NOW())
RETURNING id
`, ownerUserID).Scan(&tournamentID); err != nil {
		t.Fatalf("insert tournament error = %v", err)
	}

	_, err = postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_tournaments (
    owner_user_id,
    display_name,
    format,
    visibility,
    sport_key,
    facility_key,
    participants_per_side,
    updated_at
)
VALUES ($1, 'Public Drift Tournament', 'single_elimination', 'public', 'badminton', 'ashtonbee', 1, NOW())
`, ownerUserID)
	if err == nil {
		t.Fatal("insert public tournament visibility error = nil, want check violation")
	}
	var visibilityPgErr *pgconn.PgError
	if !errors.As(err, &visibilityPgErr) {
		t.Fatalf("public tournament visibility error = %v, want pg error", err)
	}
	if visibilityPgErr.ConstraintName != "competition_tournaments_visibility_internal_only" {
		t.Fatalf("visibility constraint = %q, want competition_tournaments_visibility_internal_only", visibilityPgErr.ConstraintName)
	}

	var bracketID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.competition_tournament_brackets (
    tournament_id,
    bracket_index,
    format,
    status,
    updated_at
)
VALUES ($1, 1, 'single_elimination', 'draft', NOW())
RETURNING id
`, tournamentID).Scan(&bracketID); err != nil {
		t.Fatalf("insert tournament bracket error = %v", err)
	}

	var seedOneID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.competition_tournament_seeds (
    tournament_id,
    bracket_id,
    seed,
    competition_session_team_id,
    seeded_at
)
VALUES ($1, $2, 1, $3, NOW())
RETURNING id
`, tournamentID, bracketID, teamOneID).Scan(&seedOneID); err != nil {
		t.Fatalf("insert tournament seed one error = %v", err)
	}
	_, err = postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_tournament_seeds (
    tournament_id,
    bracket_id,
    seed,
    competition_session_team_id,
    seeded_at
)
VALUES ($1, $2, 1, $3, NOW())
`, tournamentID, bracketID, teamTwoID)
	if err == nil {
		t.Fatal("insert duplicate tournament seed error = nil, want unique violation")
	}

	var snapshotOneID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.competition_tournament_team_snapshots (
    tournament_id,
    bracket_id,
    tournament_seed_id,
    seed,
    competition_session_id,
    competition_session_team_id,
    roster_hash,
    locked_at
)
VALUES ($1, $2, $3, 1, $4, $5, 'hash-one', NOW())
RETURNING id
`, tournamentID, bracketID, seedOneID, sessionID, teamOneID).Scan(&snapshotOneID); err != nil {
		t.Fatalf("insert tournament team snapshot error = %v", err)
	}
	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_tournament_team_snapshot_members (
    team_snapshot_id,
    user_id,
    display_name,
    slot_index
)
VALUES ($1, $2, 'Competition Member A', 1)
`, snapshotOneID, memberOneID); err != nil {
		t.Fatalf("insert tournament snapshot member error = %v", err)
	}
	_, err = postgresEnv.DB.Exec(ctx, `
UPDATE apollo.competition_tournament_team_snapshots
SET roster_hash = 'rewritten'
WHERE id = $1
`, snapshotOneID)
	if err == nil {
		t.Fatal("update tournament snapshot row error = nil, want immutable fact rejection")
	}

	var seedTwoID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.competition_tournament_seeds (
    tournament_id,
    bracket_id,
    seed,
    competition_session_team_id,
    seeded_at
)
VALUES ($1, $2, 2, $3, NOW())
RETURNING id
`, tournamentID, bracketID, teamTwoID).Scan(&seedTwoID); err != nil {
		t.Fatalf("insert tournament seed two error = %v", err)
	}
	var snapshotTwoID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.competition_tournament_team_snapshots (
    tournament_id,
    bracket_id,
    tournament_seed_id,
    seed,
    competition_session_id,
    competition_session_team_id,
    roster_hash,
    locked_at
)
VALUES ($1, $2, $3, 2, $4, $5, 'hash-two', NOW())
RETURNING id
`, tournamentID, bracketID, seedTwoID, sessionID, teamTwoID).Scan(&snapshotTwoID); err != nil {
		t.Fatalf("insert tournament team snapshot two error = %v", err)
	}
	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_tournament_match_bindings (
    tournament_id,
    bracket_id,
    round,
    match_number,
    competition_match_id,
    side_one_team_snapshot_id,
    side_two_team_snapshot_id,
    bound_at
)
VALUES ($1, $2, 1, 1, $3, $4, $5, NOW())
`, tournamentID, bracketID, matchID, snapshotOneID, snapshotTwoID); err != nil {
		t.Fatalf("insert tournament match binding error = %v", err)
	}
	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_tournament_advancements (
    tournament_id,
    bracket_id,
    match_binding_id,
    round,
    winning_team_snapshot_id,
    losing_team_snapshot_id,
    competition_match_id,
    canonical_result_id,
    advance_reason,
    advanced_at
)
SELECT tournament_id,
       bracket_id,
       id,
       round,
       side_one_team_snapshot_id,
       side_two_team_snapshot_id,
       competition_match_id,
       $1,
       'canonical_result_win',
       NOW()
FROM apollo.competition_tournament_match_bindings
WHERE bracket_id = $2
`, resultID, bracketID); err != nil {
		t.Fatalf("insert tournament advancement error = %v", err)
	}

	_, err = postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_tournament_events (
    tournament_id,
    event_type,
    occurred_at
)
VALUES ($1, 'competition.tournament.publicized', NOW())
`, tournamentID)
	if err == nil {
		t.Fatal("insert invalid tournament event error = nil, want check violation")
	}
}

func TestCompetitionAnalyticsSchemaEnforcesDerivedFactMetadata(t *testing.T) {
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

	ownerUserID := insertCompetitionUser(t, ctx, postgresEnv, "competition-owner-028", "Competition Owner", "competition-owner-028@example.com")
	sessionID := insertCompetitionSession(t, ctx, postgresEnv, ownerUserID, "Tracer 28 Analytics")
	matchID := insertCompetitionMatch(t, ctx, postgresEnv, sessionID, 1)
	resultID := insertCompetitionResult(t, ctx, postgresEnv, matchID, ownerUserID)

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_analytics_events (
    event_type,
    projection_version,
    projection_watermark,
    user_id,
    sport_key,
    facility_key,
    mode_key,
    team_scope,
    stat_type,
    stat_value,
    source_match_id,
    source_result_id,
    sample_size,
    confidence,
    computed_at
)
VALUES (
    'competition.analytics.stat_computed',
    'competition_analytics_v1',
    '2026-04-28T10:00:00Z#match#result',
    $1,
    'badminton',
    'ashtonbee',
    'head_to_head:s2-p1',
    'solo',
    'matches_played',
    1.0000,
    $2,
    $3,
    1,
    0.1000,
    NOW()
)
`, ownerUserID, matchID, resultID); err != nil {
		t.Fatalf("insert analytics stat event error = %v", err)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_analytics_projections (
    user_id,
    sport_key,
    facility_key,
    mode_key,
    team_scope,
    stat_type,
    stat_value,
    source_match_id,
    source_result_id,
    sample_size,
    confidence,
    computed_at,
    projection_version,
    projection_watermark
)
VALUES (
    $1,
    'badminton',
    'all',
    'all',
    'all',
    'matches_played',
    1.0000,
    $2,
    $3,
    1,
    0.1000,
    NOW(),
    'competition_analytics_v1',
    '2026-04-28T10:00:00Z#match#result'
)
`, ownerUserID, matchID, resultID); err != nil {
		t.Fatalf("insert analytics projection error = %v", err)
	}

	_, err = postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.competition_analytics_events (
    event_type,
    projection_version,
    projection_watermark,
    sport_key,
    stat_type,
    stat_value,
    sample_size,
    confidence,
    computed_at
)
VALUES (
    'competition.analytics.stat_computed',
    'competition_analytics_v1',
    '2026-04-28T10:00:00Z#match#result',
    'badminton',
    'matches_played',
    1.0000,
    1,
    0.1000,
    NOW()
)
`)
	if err == nil {
		t.Fatal("insert analytics stat without source error = nil, want check violation")
	}
	var statPgErr *pgconn.PgError
	if !errors.As(err, &statPgErr) {
		t.Fatalf("analytics stat without source error = %v, want pg error", err)
	}
	if statPgErr.ConstraintName != "competition_analytics_events_stat_payload_required" {
		t.Fatalf("analytics stat constraint = %q, want competition_analytics_events_stat_payload_required", statPgErr.ConstraintName)
	}

	_, err = postgresEnv.DB.Exec(ctx, `
UPDATE apollo.competition_analytics_projections
SET confidence = 1.1000
WHERE user_id = $1
`, ownerUserID)
	if err == nil {
		t.Fatal("update analytics confidence out of range error = nil, want check violation")
	}
	var projectionPgErr *pgconn.PgError
	if !errors.As(err, &projectionPgErr) {
		t.Fatalf("analytics confidence error = %v, want pg error", err)
	}
	if projectionPgErr.ConstraintName != "competition_analytics_projections_confidence_range" {
		t.Fatalf("analytics confidence constraint = %q, want competition_analytics_projections_confidence_range", projectionPgErr.ConstraintName)
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
