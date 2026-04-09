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

	if err := testutil.ApplySQLFiles(ctx, postgresEnv.DB, testutil.RepoFilePath("db", "migrations", "009_competition_container_runtime.down.sql")); err != nil {
		t.Fatalf("ApplySQLFiles(down migration) error = %v", err)
	}

	var remainingCompetitionTables int
	if err := postgresEnv.DB.QueryRow(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = 'apollo'
  AND table_name IN (
    'competition_sessions',
    'competition_session_teams',
    'competition_team_roster_members',
    'competition_matches',
    'competition_match_side_slots'
  )
`).Scan(&remainingCompetitionTables); err != nil {
		t.Fatalf("count competition tables after down migration error = %v", err)
	}

	if remainingCompetitionTables != 0 {
		t.Fatalf("remaining competition table count = %d, want 0", remainingCompetitionTables)
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
