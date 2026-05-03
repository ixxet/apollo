package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/competition"
	"github.com/ixxet/apollo/internal/config"
	"github.com/ixxet/apollo/internal/schedule"
	"github.com/ixxet/apollo/internal/testutil"
)

func TestBuildServerDependenciesWiresCompetitionMembershipMatchPreviewAndPresenceRuntime(t *testing.T) {
	cookies, err := auth.NewSessionCookieManager("apollo_session", "0123456789abcdef0123456789abcdef", true)
	if err != nil {
		t.Fatalf("NewSessionCookieManager() error = %v", err)
	}

	deps, err := buildServerDependencies(nil, false, cookies, auth.LogEmailSender{}, config.Config{
		VerificationTokenTTL: 15 * time.Minute,
		SessionTTL:           7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("buildServerDependencies() error = %v", err)
	}

	if deps.Membership == nil {
		t.Fatal("deps.Membership = nil, want lobby membership runtime wired")
	}
	if deps.MatchPreview == nil {
		t.Fatal("deps.MatchPreview = nil, want match preview runtime wired")
	}
	if deps.Competition == nil {
		t.Fatal("deps.Competition = nil, want competition container runtime wired")
	}
	if deps.Schedule == nil {
		t.Fatal("deps.Schedule = nil, want schedule substrate runtime wired")
	}
	if deps.Coaching == nil {
		t.Fatal("deps.Coaching = nil, want deterministic coaching runtime wired")
	}
	if deps.Presence == nil {
		t.Fatal("deps.Presence = nil, want member presence runtime wired")
	}
	if deps.Ops != nil {
		t.Fatal("deps.Ops != nil without ATHENA config")
	}

	deps, err = buildServerDependencies(nil, false, cookies, auth.LogEmailSender{}, config.Config{
		VerificationTokenTTL:  15 * time.Minute,
		SessionTTL:            7 * 24 * time.Hour,
		AthenaBaseURL:         "http://127.0.0.1:8080",
		AthenaTimeout:         2 * time.Second,
		OpsAnalyticsMaxWindow: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("buildServerDependencies(with ATHENA) error = %v", err)
	}
	if deps.Ops == nil {
		t.Fatal("deps.Ops = nil, want ops overview runtime wired when ATHENA config is present")
	}
}

func TestNewRootCmdIncludesSportCommand(t *testing.T) {
	rootCmd := newRootCmd()

	if _, _, err := rootCmd.Find([]string{"sport"}); err != nil {
		t.Fatalf("rootCmd.Find(sport) error = %v", err)
	}
}

func TestNewRootCmdIncludesScheduleCommand(t *testing.T) {
	rootCmd := newRootCmd()

	if _, _, err := rootCmd.Find([]string{"schedule"}); err != nil {
		t.Fatalf("rootCmd.Find(schedule) error = %v", err)
	}
}

func TestNewRootCmdIncludesCompetitionCommand(t *testing.T) {
	rootCmd := newRootCmd()

	if _, _, err := rootCmd.Find([]string{"competition"}); err != nil {
		t.Fatalf("rootCmd.Find(competition) error = %v", err)
	}
}

func TestCompetitionCommandCLIDryRunAndCreateSession(t *testing.T) {
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

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	actorUserID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	actorSessionID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	if _, err := postgresEnv.DB.Exec(ctx, `INSERT INTO apollo.users (id, student_id, display_name, email, role) VALUES ($1, $2, $3, $4, $5)`, actorUserID, "competition-cli-manager", "Competition CLI Manager", "competition-cli-manager@example.com", "manager"); err != nil {
		t.Fatalf("insert actor user error = %v", err)
	}
	if _, err := postgresEnv.DB.Exec(ctx, `INSERT INTO apollo.sessions (id, user_id, expires_at, revoked_at) VALUES ($1, $2, NOW() + INTERVAL '1 hour', NULL)`, actorSessionID, actorUserID); err != nil {
		t.Fatalf("insert actor session error = %v", err)
	}

	input := `{"create_session":{"display_name":"CLI Command Session","sport_key":"badminton","facility_key":"ashtonbee","zone_key":"gym-floor","participants_per_side":1}}`
	dryRunOutput := runRootCommand(t,
		"competition", "command", "run",
		"--name", "create_session",
		"--dry-run",
		"--actor-role", "manager",
		"--input-json", input,
		"--format", "json",
	)
	var dryRunOutcome competition.CompetitionCommandOutcome
	if err := json.Unmarshal([]byte(dryRunOutput), &dryRunOutcome); err != nil {
		t.Fatalf("json.Unmarshal(dryRunOutput) error = %v output=%s", err, dryRunOutput)
	}
	if dryRunOutcome.Status != competition.CommandStatusPlanned || dryRunOutcome.Mutated {
		t.Fatalf("dryRunOutcome = %#v, want planned non-mutating", dryRunOutcome)
	}

	sessionList := runRootCommand(t, "competition", "session", "list", "--format", "json")
	var sessions []competition.SessionSummary
	if err := json.Unmarshal([]byte(sessionList), &sessions); err != nil {
		t.Fatalf("json.Unmarshal(sessionList) error = %v output=%s", err, sessionList)
	}
	if len(sessions) != 0 {
		t.Fatalf("len(sessions) = %d, want 0 after dry-run", len(sessions))
	}

	createOutput := runRootCommand(t,
		"competition", "command", "run",
		"--name", "create_session",
		"--actor-user-id", actorUserID.String(),
		"--actor-session-id", actorSessionID.String(),
		"--actor-role", "manager",
		"--trusted-surface-key", "staff-console",
		"--input-json", input,
		"--format", "json",
	)
	var createOutcome competition.CompetitionCommandOutcome
	if err := json.Unmarshal([]byte(createOutput), &createOutcome); err != nil {
		t.Fatalf("json.Unmarshal(createOutput) error = %v output=%s", err, createOutput)
	}
	if createOutcome.Status != competition.CommandStatusSucceeded || !createOutcome.Mutated {
		t.Fatalf("createOutcome = %#v, want succeeded mutating", createOutcome)
	}

	sessionList = runRootCommand(t, "competition", "session", "list", "--format", "text")
	if !strings.Contains(sessionList, "CLI Command Session") {
		t.Fatalf("sessionList = %q, want CLI-created session", sessionList)
	}
}

func TestCompetitionCommandCLIResultLifecycleSmoke(t *testing.T) {
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

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	actorUserID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	actorSessionID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	memberOneID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	memberTwoID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	for _, user := range []struct {
		id        uuid.UUID
		studentID string
		name      string
		email     string
		role      string
	}{
		{actorUserID, "competition-cli-result-manager", "Competition CLI Result Manager", "competition-cli-result-manager@example.com", "manager"},
		{memberOneID, "competition-cli-result-one", "Competition CLI Result One", "competition-cli-result-one@example.com", "member"},
		{memberTwoID, "competition-cli-result-two", "Competition CLI Result Two", "competition-cli-result-two@example.com", "member"},
	} {
		if _, err := postgresEnv.DB.Exec(ctx, `INSERT INTO apollo.users (id, student_id, display_name, email, role) VALUES ($1, $2, $3, $4, $5)`, user.id, user.studentID, user.name, user.email, user.role); err != nil {
			t.Fatalf("insert user %s error = %v", user.studentID, err)
		}
	}
	if _, err := postgresEnv.DB.Exec(ctx, `INSERT INTO apollo.sessions (id, user_id, expires_at, revoked_at) VALUES ($1, $2, NOW() + INTERVAL '1 hour', NULL)`, actorSessionID, actorUserID); err != nil {
		t.Fatalf("insert actor session error = %v", err)
	}

	service := competition.NewService(competition.NewRepository(postgresEnv.DB))
	actor := competition.StaffActor{
		UserID:              actorUserID,
		Role:                authz.RoleManager,
		SessionID:           actorSessionID,
		Capability:          authz.CapabilityCompetitionStructureManage,
		TrustedSurfaceKey:   "staff-console",
		TrustedSurfaceLabel: "staff console",
	}
	zoneKey := "gym-floor"
	session, err := service.CreateSession(ctx, actor, competition.CreateSessionInput{
		DisplayName:         "CLI Result Lifecycle",
		SportKey:            "badminton",
		FacilityKey:         "ashtonbee",
		ZoneKey:             &zoneKey,
		ParticipantsPerSide: 1,
	})
	if err != nil {
		t.Fatalf("CreateSession error = %v", err)
	}
	teamOne, err := service.CreateTeam(ctx, actor, session.ID, competition.CreateTeamInput{SideIndex: 1})
	if err != nil {
		t.Fatalf("CreateTeam one error = %v", err)
	}
	teamTwo, err := service.CreateTeam(ctx, actor, session.ID, competition.CreateTeamInput{SideIndex: 2})
	if err != nil {
		t.Fatalf("CreateTeam two error = %v", err)
	}
	if _, err := service.AddRosterMember(ctx, actor, session.ID, teamOne.ID, competition.AddRosterMemberInput{UserID: memberOneID, SlotIndex: 1}); err != nil {
		t.Fatalf("AddRosterMember one error = %v", err)
	}
	if _, err := service.AddRosterMember(ctx, actor, session.ID, teamTwo.ID, competition.AddRosterMemberInput{UserID: memberTwoID, SlotIndex: 1}); err != nil {
		t.Fatalf("AddRosterMember two error = %v", err)
	}
	match, err := service.CreateMatch(ctx, actor, session.ID, competition.CreateMatchInput{
		MatchIndex: 1,
		SideSlots: []competition.MatchSideInput{
			{TeamID: teamOne.ID, SideIndex: 1},
			{TeamID: teamTwo.ID, SideIndex: 2},
		},
	})
	if err != nil {
		t.Fatalf("CreateMatch error = %v", err)
	}
	if _, err := postgresEnv.DB.Exec(ctx, `UPDATE apollo.competition_sessions SET status = $2 WHERE id = $1`, session.ID, competition.SessionStatusInProgress); err != nil {
		t.Fatalf("mark session in progress error = %v", err)
	}
	if _, err := postgresEnv.DB.Exec(ctx, `UPDATE apollo.competition_matches SET status = $2 WHERE id = $1`, match.ID, competition.MatchStatusInProgress); err != nil {
		t.Fatalf("mark match in progress error = %v", err)
	}

	smokeStartedAt := time.Now()
	readinessOutput := runRootCommand(t, "competition", "command", "readiness", "--actor-role", "manager", "--format", "json")
	var readiness competition.CompetitionCommandReadiness
	if err := json.Unmarshal([]byte(readinessOutput), &readiness); err != nil {
		t.Fatalf("json.Unmarshal(readinessOutput) error = %v output=%s", err, readinessOutput)
	}
	if readiness.Status != "ready" {
		t.Fatalf("readiness.Status = %q, want ready", readiness.Status)
	}

	resultInput := fmt.Sprintf(`{"match_result":{"sides":[{"side_index":1,"competition_session_team_id":"%s","outcome":"win"},{"side_index":2,"competition_session_team_id":"%s","outcome":"loss"}]}}`, teamOne.ID, teamTwo.ID)
	correctionInput := fmt.Sprintf(`{"match_result":{"sides":[{"side_index":1,"competition_session_team_id":"%s","outcome":"loss"},{"side_index":2,"competition_session_team_id":"%s","outcome":"win"}]}}`, teamOne.ID, teamTwo.ID)
	dryRun := runCompetitionResultCLICommand(t, "record_match_result", session.ID, match.ID, 0, resultInput, true, actorUserID, actorSessionID)
	if dryRun.Status != competition.CommandStatusPlanned || dryRun.Mutated {
		t.Fatalf("dryRun = %#v, want planned non-mutating", dryRun)
	}

	for _, step := range []struct {
		name    string
		version int
		input   string
		actual  int
	}{
		{"record_match_result", 0, resultInput, 1},
		{"finalize_match_result", 1, `{}`, 2},
		{"dispute_match_result", 2, `{}`, 3},
		{"correct_match_result", 3, correctionInput, 4},
		{"void_match_result", 4, `{}`, 5},
	} {
		outcome := runCompetitionResultCLICommand(t, step.name, session.ID, match.ID, step.version, step.input, false, actorUserID, actorSessionID)
		if outcome.Status != competition.CommandStatusSucceeded || !outcome.Mutated {
			t.Fatalf("%s outcome = %#v, want succeeded mutating", step.name, outcome)
		}
		if outcome.ActualVersion == nil || *outcome.ActualVersion != step.actual {
			t.Fatalf("%s ActualVersion = %v, want %d", step.name, outcome.ActualVersion, step.actual)
		}
	}
	smokeDuration := time.Since(smokeStartedAt)
	if smokeDuration > 30*time.Second {
		t.Fatalf("CLI smoke duration = %s, ceiling 30s", smokeDuration)
	}
	t.Logf("scale_ceiling path=cli_smoke sequence_duration=%s hard_ceiling=%s", smokeDuration, 30*time.Second)
}

func TestCompetitionCLIDemoProjectionSafetyAndPreviewReads(t *testing.T) {
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

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	actorUserID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	actorSessionID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	memberOneID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	memberTwoID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	eligiblePreferences := `{"visibility_mode":"discoverable","availability_mode":"available_now","coaching_profile":{},"nutrition_profile":{}}`
	for _, user := range []struct {
		id          uuid.UUID
		studentID   string
		name        string
		email       string
		role        string
		preferences string
	}{
		{actorUserID, "competition-cli-demo-manager", "Competition CLI Demo Manager", "competition-cli-demo-manager@example.com", "manager", eligiblePreferences},
		{memberOneID, "competition-cli-demo-one", "Competition CLI Demo One", "competition-cli-demo-one@example.com", "member", eligiblePreferences},
		{memberTwoID, "competition-cli-demo-two", "Competition CLI Demo Two", "competition-cli-demo-two@example.com", "member", eligiblePreferences},
	} {
		if _, err := postgresEnv.DB.Exec(ctx, `INSERT INTO apollo.users (id, student_id, display_name, email, role, preferences) VALUES ($1, $2, $3, $4, $5, $6::jsonb)`, user.id, user.studentID, user.name, user.email, user.role, user.preferences); err != nil {
			t.Fatalf("insert user %s error = %v", user.studentID, err)
		}
		if _, err := postgresEnv.DB.Exec(ctx, `INSERT INTO apollo.lobby_memberships (user_id, status, joined_at, updated_at) VALUES ($1, 'joined', NOW(), NOW())`, user.id); err != nil {
			t.Fatalf("insert lobby membership %s error = %v", user.studentID, err)
		}
	}
	if _, err := postgresEnv.DB.Exec(ctx, `INSERT INTO apollo.sessions (id, user_id, expires_at, revoked_at) VALUES ($1, $2, NOW() + INTERVAL '1 hour', NULL)`, actorSessionID, actorUserID); err != nil {
		t.Fatalf("insert actor session error = %v", err)
	}

	service := competition.NewService(competition.NewRepository(postgresEnv.DB))
	structureActor := competition.StaffActor{
		UserID:              actorUserID,
		Role:                authz.RoleManager,
		SessionID:           actorSessionID,
		Capability:          authz.CapabilityCompetitionStructureManage,
		TrustedSurfaceKey:   "staff-console",
		TrustedSurfaceLabel: "staff console",
	}
	liveActor := structureActor
	liveActor.Capability = authz.CapabilityCompetitionLiveManage
	zoneKey := "gym-floor"
	session, err := service.CreateSession(ctx, structureActor, competition.CreateSessionInput{
		DisplayName:         "CLI Demo Spine Result",
		SportKey:            "badminton",
		FacilityKey:         "ashtonbee",
		ZoneKey:             &zoneKey,
		ParticipantsPerSide: 1,
	})
	if err != nil {
		t.Fatalf("CreateSession result fixture error = %v", err)
	}
	teamOne, err := service.CreateTeam(ctx, structureActor, session.ID, competition.CreateTeamInput{SideIndex: 1})
	if err != nil {
		t.Fatalf("CreateTeam one error = %v", err)
	}
	teamTwo, err := service.CreateTeam(ctx, structureActor, session.ID, competition.CreateTeamInput{SideIndex: 2})
	if err != nil {
		t.Fatalf("CreateTeam two error = %v", err)
	}
	if _, err := service.AddRosterMember(ctx, structureActor, session.ID, teamOne.ID, competition.AddRosterMemberInput{UserID: memberOneID, SlotIndex: 1}); err != nil {
		t.Fatalf("AddRosterMember one error = %v", err)
	}
	if _, err := service.AddRosterMember(ctx, structureActor, session.ID, teamTwo.ID, competition.AddRosterMemberInput{UserID: memberTwoID, SlotIndex: 1}); err != nil {
		t.Fatalf("AddRosterMember two error = %v", err)
	}
	match, err := service.CreateMatch(ctx, structureActor, session.ID, competition.CreateMatchInput{
		MatchIndex: 1,
		SideSlots: []competition.MatchSideInput{
			{TeamID: teamOne.ID, SideIndex: 1},
			{TeamID: teamTwo.ID, SideIndex: 2},
		},
	})
	if err != nil {
		t.Fatalf("CreateMatch error = %v", err)
	}
	if _, err := postgresEnv.DB.Exec(ctx, `UPDATE apollo.competition_sessions SET status = $2 WHERE id = $1`, session.ID, competition.SessionStatusInProgress); err != nil {
		t.Fatalf("mark session in progress error = %v", err)
	}
	if _, err := postgresEnv.DB.Exec(ctx, `UPDATE apollo.competition_matches SET status = $2 WHERE id = $1`, match.ID, competition.MatchStatusInProgress); err != nil {
		t.Fatalf("mark match in progress error = %v", err)
	}

	resultInput := fmt.Sprintf(`{"match_result":{"sides":[{"side_index":1,"competition_session_team_id":"%s","outcome":"win"},{"side_index":2,"competition_session_team_id":"%s","outcome":"loss"}]}}`, teamOne.ID, teamTwo.ID)
	recordOutcome := runCompetitionResultCLICommand(t, "record_match_result", session.ID, match.ID, 0, resultInput, false, actorUserID, actorSessionID)
	if recordOutcome.Status != competition.CommandStatusSucceeded {
		t.Fatalf("recordOutcome.Status = %q, want succeeded", recordOutcome.Status)
	}
	finalizeOutcome := runCompetitionResultCLICommand(t, "finalize_match_result", session.ID, match.ID, 1, `{}`, false, actorUserID, actorSessionID)
	if finalizeOutcome.Status != competition.CommandStatusSucceeded {
		t.Fatalf("finalizeOutcome.Status = %q, want succeeded", finalizeOutcome.Status)
	}

	sessionJSON := runRootCommand(t, "competition", "session", "show", "--session-id", session.ID.String(), "--format", "json")
	var shownSession competition.Session
	if err := json.Unmarshal([]byte(sessionJSON), &shownSession); err != nil {
		t.Fatalf("json.Unmarshal(session show) error = %v output=%s", err, sessionJSON)
	}
	if len(shownSession.Matches) != 1 || shownSession.Matches[0].Result == nil || shownSession.Matches[0].Result.ResultStatus != competition.ResultStatusFinalized {
		t.Fatalf("shownSession.Matches = %#v, want finalized result inspection", shownSession.Matches)
	}

	readinessJSON := runRootCommand(t, "competition", "public", "readiness", "--format", "json")
	var readiness competition.PublicCompetitionReadiness
	if err := json.Unmarshal([]byte(readinessJSON), &readiness); err != nil {
		t.Fatalf("json.Unmarshal(public readiness) error = %v output=%s", err, readinessJSON)
	}
	if readiness.Status != "available" || readiness.AvailableLeaderboards == 0 || readiness.AvailableCanonicalResults == 0 {
		t.Fatalf("readiness = %#v, want available public projection", readiness)
	}

	leaderboardJSON := runRootCommand(t,
		"competition", "public", "leaderboard",
		"--sport-key", "badminton",
		"--mode-key", "head_to_head:s2-p1",
		"--stat-type", "wins",
		"--team-scope", "all",
		"--limit", "10",
		"--format", "json",
	)
	var leaderboard competition.PublicCompetitionLeaderboard
	if err := json.Unmarshal([]byte(leaderboardJSON), &leaderboard); err != nil {
		t.Fatalf("json.Unmarshal(public leaderboard) error = %v output=%s", err, leaderboardJSON)
	}
	if len(leaderboard.Leaderboard) == 0 || leaderboard.Leaderboard[0].Participant != "participant_1" {
		t.Fatalf("leaderboard = %#v, want redacted participant row", leaderboard)
	}
	if strings.Contains(leaderboardJSON, memberOneID.String()) || strings.Contains(leaderboardJSON, memberTwoID.String()) {
		t.Fatalf("leaderboardJSON exposed raw member id: %s", leaderboardJSON)
	}

	publicIdentityJSON := runRootCommand(t,
		"competition", "public", "game-identity",
		"--sport-key", "badminton",
		"--mode-key", "head_to_head:s2-p1",
		"--facility-key", "ashtonbee",
		"--team-scope", "all",
		"--format", "json",
	)
	var publicIdentity competition.GameIdentityProjection
	if err := json.Unmarshal([]byte(publicIdentityJSON), &publicIdentity); err != nil {
		t.Fatalf("json.Unmarshal(public game identity) error = %v output=%s", err, publicIdentityJSON)
	}
	if publicIdentity.Status != "available" || len(publicIdentity.CP) == 0 || !strings.HasPrefix(publicIdentity.CP[0].Participant, "participant_") {
		t.Fatalf("publicIdentity = %#v, want public redacted projection", publicIdentity)
	}

	memberStatsJSON := runRootCommand(t, "competition", "member", "stats", "--user-id", memberOneID.String(), "--format", "json")
	var memberStats []competition.MemberStat
	if err := json.Unmarshal([]byte(memberStatsJSON), &memberStats); err != nil {
		t.Fatalf("json.Unmarshal(member stats) error = %v output=%s", err, memberStatsJSON)
	}
	if len(memberStats) == 0 || memberStats[0].UserID != memberOneID || memberStats[0].CurrentRatingMu <= 0 {
		t.Fatalf("memberStats = %#v, want self-scoped rating projection", memberStats)
	}

	memberHistoryText := runRootCommand(t, "competition", "member", "history", "--user-id", memberOneID.String(), "--format", "text")
	if !strings.Contains(memberHistoryText, "CLI Demo Spine Result") || !strings.Contains(memberHistoryText, "outcome=win") {
		t.Fatalf("memberHistoryText = %q, want finalized member history", memberHistoryText)
	}

	memberIdentityJSON := runRootCommand(t,
		"competition", "member", "game-identity",
		"--user-id", memberOneID.String(),
		"--sport-key", "badminton",
		"--mode-key", "head_to_head:s2-p1",
		"--facility-key", "ashtonbee",
		"--team-scope", "all",
		"--format", "json",
	)
	var memberIdentity competition.GameIdentityProjection
	if err := json.Unmarshal([]byte(memberIdentityJSON), &memberIdentity); err != nil {
		t.Fatalf("json.Unmarshal(member game identity) error = %v output=%s", err, memberIdentityJSON)
	}
	if memberIdentity.Status != "available" || len(memberIdentity.CP) != 1 || memberIdentity.CP[0].Participant != "member_self" {
		t.Fatalf("memberIdentity = %#v, want member self-scoped projection", memberIdentity)
	}

	safetyInput := fmt.Sprintf(`{"safety_report":{"competition_session_id":"%s","reporter_user_id":"%s","subject_user_id":"%s","target_type":"competition_member","reason_code":"conduct","note":"CLI demo safety note"}}`, session.ID, memberOneID, memberTwoID)
	safetyOutput := runRootCommand(t,
		"competition", "command", "run",
		"--name", "record_safety_report",
		"--actor-user-id", actorUserID.String(),
		"--actor-session-id", actorSessionID.String(),
		"--actor-role", "manager",
		"--trusted-surface-key", "staff-console",
		"--input-json", safetyInput,
		"--format", "json",
	)
	var safetyOutcome competition.CompetitionCommandOutcome
	if err := json.Unmarshal([]byte(safetyOutput), &safetyOutcome); err != nil {
		t.Fatalf("json.Unmarshal(safety command) error = %v output=%s", err, safetyOutput)
	}
	if safetyOutcome.Status != competition.CommandStatusSucceeded || !safetyOutcome.Mutated {
		t.Fatalf("safetyOutcome = %#v, want succeeded mutating safety command", safetyOutcome)
	}

	safetyReadinessText := runRootCommand(t,
		"competition", "safety", "readiness",
		"--actor-user-id", actorUserID.String(),
		"--actor-session-id", actorSessionID.String(),
		"--actor-role", "manager",
		"--trusted-surface-key", "staff-console",
		"--format", "text",
	)
	if !strings.Contains(safetyReadinessText, "reports=1") || !strings.Contains(safetyReadinessText, "record_safety_report") {
		t.Fatalf("safetyReadinessText = %q, want safety readiness summary and commands", safetyReadinessText)
	}

	safetyReviewJSON := runRootCommand(t,
		"competition", "safety", "review",
		"--actor-user-id", actorUserID.String(),
		"--actor-session-id", actorSessionID.String(),
		"--actor-role", "manager",
		"--trusted-surface-key", "staff-console",
		"--limit", "5",
		"--format", "json",
	)
	var safetyReview competition.CompetitionSafetyReview
	if err := json.Unmarshal([]byte(safetyReviewJSON), &safetyReview); err != nil {
		t.Fatalf("json.Unmarshal(safety review) error = %v output=%s", err, safetyReviewJSON)
	}
	if safetyReview.Summary.ReportCount != 1 || len(safetyReview.Reports) != 1 {
		t.Fatalf("safetyReview = %#v, want one authorized manager/internal report", safetyReview)
	}

	previewSession, err := service.CreateSession(ctx, structureActor, competition.CreateSessionInput{
		DisplayName:         "CLI Demo Spine Preview",
		SportKey:            "badminton",
		FacilityKey:         "ashtonbee",
		ZoneKey:             &zoneKey,
		ParticipantsPerSide: 1,
	})
	if err != nil {
		t.Fatalf("CreateSession preview fixture error = %v", err)
	}
	previewSession, err = service.OpenQueue(ctx, liveActor, previewSession.ID)
	if err != nil {
		t.Fatalf("OpenQueue preview fixture error = %v", err)
	}
	previewSession, err = service.AddQueueMember(ctx, liveActor, previewSession.ID, competition.QueueMemberInput{UserID: memberOneID, Tier: "competitive"})
	if err != nil {
		t.Fatalf("AddQueueMember preview one error = %v", err)
	}
	previewSession, err = service.AddQueueMember(ctx, liveActor, previewSession.ID, competition.QueueMemberInput{UserID: memberTwoID, Tier: "competitive"})
	if err != nil {
		t.Fatalf("AddQueueMember preview two error = %v", err)
	}
	previewOutput := runRootCommand(t,
		"competition", "command", "run",
		"--name", "generate_match_preview",
		"--session-id", previewSession.ID.String(),
		"--expected-version", fmt.Sprint(previewSession.QueueVersion),
		"--actor-user-id", actorUserID.String(),
		"--actor-session-id", actorSessionID.String(),
		"--actor-role", "manager",
		"--trusted-surface-key", "staff-console",
		"--format", "json",
	)
	var previewOutcome competition.CompetitionCommandOutcome
	if err := json.Unmarshal([]byte(previewOutput), &previewOutcome); err != nil {
		t.Fatalf("json.Unmarshal(preview output) error = %v output=%s", err, previewOutput)
	}
	if previewOutcome.Status != competition.CommandStatusSucceeded || previewOutcome.ActualVersion == nil || *previewOutcome.ActualVersion != previewSession.QueueVersion {
		t.Fatalf("previewOutcome = %#v, want succeeded ARES preview generation at current queue version", previewOutcome)
	}
	if !strings.Contains(previewOutput, `"preview_version": "v2"`) || !strings.Contains(previewOutput, `"active_rating_read_path": "apollo_legacy_rating_v1"`) {
		t.Fatalf("previewOutput = %s, want APOLLO ARES preview result", previewOutput)
	}
}

func TestScheduleCommandsRoundTripResourcesBlocksAndCalendar(t *testing.T) {
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

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	actorUserID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	actorSessionID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	if _, err := postgresEnv.DB.Exec(ctx, `INSERT INTO apollo.users (id, student_id, display_name, email) VALUES ($1, $2, $3, $4)`, actorUserID, "schedule-cli-owner", "Schedule CLI Owner", "schedule-cli-owner@example.com"); err != nil {
		t.Fatalf("insert actor user error = %v", err)
	}
	if _, err := postgresEnv.DB.Exec(ctx, `INSERT INTO apollo.sessions (id, user_id, expires_at, revoked_at) VALUES ($1, $2, NOW() + INTERVAL '1 hour', NULL)`, actorSessionID, actorUserID); err != nil {
		t.Fatalf("insert actor session error = %v", err)
	}

	actorFlags := []string{
		"--actor-user-id", actorUserID.String(),
		"--actor-session-id", actorSessionID.String(),
		"--actor-role", "owner",
		"--trusted-surface-key", "staff-console",
	}
	managerFlags := []string{
		"--actor-user-id", actorUserID.String(),
		"--actor-session-id", actorSessionID.String(),
		"--actor-role", "manager",
		"--trusted-surface-key", "staff-console",
	}

	if _, _, err := runRootCommandWithResult(t, append([]string{"schedule", "resource", "upsert", "--resource-key", "manager-graph-attempt", "--facility-key", "ashtonbee", "--zone-key", "gym-floor", "--resource-type", "court", "--display-name", "Manager Graph Attempt"}, managerFlags...)...); err == nil || !strings.Contains(err.Error(), "schedule graph authoring requires owner role") {
		t.Fatalf("manager graph authoring error = %v, want owner-only refusal", err)
	}

	runRootCommand(t, append([]string{"schedule", "resource", "upsert", "--resource-key", "full-court", "--facility-key", "ashtonbee", "--zone-key", "gym-floor", "--resource-type", "court", "--display-name", "Full Court"}, actorFlags...)...)
	runRootCommand(t, append([]string{"schedule", "resource", "upsert", "--resource-key", "half-court-a", "--facility-key", "ashtonbee", "--zone-key", "gym-floor", "--resource-type", "court", "--display-name", "Half Court A"}, actorFlags...)...)
	runRootCommand(t, append([]string{"schedule", "resource", "edge", "upsert", "--resource-key", "full-court", "--related-resource-key", "half-court-a", "--edge-type", "contains"}, actorFlags...)...)

	resourceListJSON := runRootCommand(t, "schedule", "resource", "list", "--facility-key", "ashtonbee", "--format", "json")
	var resources []schedule.Resource
	if err := json.Unmarshal([]byte(resourceListJSON), &resources); err != nil {
		t.Fatalf("json.Unmarshal(resourceListJSON) error = %v output=%s", err, resourceListJSON)
	}
	if len(resources) != 2 {
		t.Fatalf("len(resources) = %d, want 2", len(resources))
	}
	resourceKeys := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		resourceKeys[resource.ResourceKey] = struct{}{}
	}
	if _, ok := resourceKeys["full-court"]; !ok {
		t.Fatalf("resource list missing full-court: %#v", resourceKeys)
	}
	if _, ok := resourceKeys["half-court-a"]; !ok {
		t.Fatalf("resource list missing half-court-a: %#v", resourceKeys)
	}

	resourceListText := runRootCommand(t, "schedule", "resource", "list", "--facility-key", "ashtonbee", "--format", "text")
	if !strings.Contains(resourceListText, "full-court court Full Court") {
		t.Fatalf("resourceListText = %q, want full-court line", resourceListText)
	}
	if !strings.Contains(resourceListText, "half-court-a court Half Court A") {
		t.Fatalf("resourceListText = %q, want half-court-a line", resourceListText)
	}

	resourceShowOutput := runRootCommand(t, "schedule", "resource", "show", "--resource-key", "full-court")
	var shown schedule.Resource
	if err := json.Unmarshal([]byte(resourceShowOutput), &shown); err != nil {
		t.Fatalf("json.Unmarshal(resourceShowOutput) error = %v output=%s", err, resourceShowOutput)
	}
	if shown.ResourceKey != "full-court" {
		t.Fatalf("shown.ResourceKey = %q, want %q", shown.ResourceKey, "full-court")
	}
	if len(shown.Edges) != 1 {
		t.Fatalf("len(shown.Edges) = %d, want 1", len(shown.Edges))
	}
	if shown.Edges[0].EdgeType != schedule.EdgeContains || shown.Edges[0].RelatedResourceKey != "half-court-a" {
		t.Fatalf("shown.Edges[0] = %#v, want contains edge to half-court-a", shown.Edges[0])
	}

	blockOutput := runRootCommand(t, append([]string{"schedule", "block", "create",
		"--facility-key", "ashtonbee",
		"--scope", "facility",
		"--kind", "operating_hours",
		"--effect", "informational",
		"--visibility", "public_labeled",
		"--weekly-weekday", "1",
		"--weekly-start-time", "09:00",
		"--weekly-end-time", "10:00",
		"--weekly-timezone", "America/Toronto",
		"--weekly-recurrence-start-date", "2026-04-06",
		"--weekly-recurrence-end-date", "2026-04-20",
	}, actorFlags...)...)

	var created schedule.Block
	if err := json.Unmarshal([]byte(blockOutput), &created); err != nil {
		t.Fatalf("json.Unmarshal(blockOutput) error = %v output=%s", err, blockOutput)
	}

	blockListJSON := runRootCommand(t, "schedule", "block", "list", "--facility-key", "ashtonbee", "--format", "json")
	var listedBlocks []schedule.Block
	if err := json.Unmarshal([]byte(blockListJSON), &listedBlocks); err != nil {
		t.Fatalf("json.Unmarshal(blockListJSON) error = %v output=%s", err, blockListJSON)
	}
	foundCreated := false
	for _, block := range listedBlocks {
		if block.ID == created.ID {
			foundCreated = true
			break
		}
	}
	if !foundCreated {
		t.Fatalf("block list missing created block %s", created.ID)
	}

	blockListText := runRootCommand(t, "schedule", "block", "list", "--facility-key", "ashtonbee", "--format", "text")
	if !strings.Contains(blockListText, created.ID.String()) || !strings.Contains(blockListText, "operating_hours") {
		t.Fatalf("blockListText = %q, want created operating-hours block", blockListText)
	}

	calendarOutput := runRootCommand(t, "schedule", "calendar", "--facility-key", "ashtonbee", "--from", "2026-04-06T00:00:00Z", "--until", "2026-04-27T00:00:00Z", "--format", "json")
	var occurrences []schedule.Occurrence
	if err := json.Unmarshal([]byte(calendarOutput), &occurrences); err != nil {
		t.Fatalf("json.Unmarshal(calendarOutput) error = %v output=%s", err, calendarOutput)
	}
	if len(occurrences) != 3 {
		t.Fatalf("len(occurrences) = %d, want 3", len(occurrences))
	}

	calendarText := runRootCommand(t, "schedule", "calendar", "--facility-key", "ashtonbee", "--from", "2026-04-06T00:00:00Z", "--until", "2026-04-27T00:00:00Z", "--format", "text")
	if !strings.Contains(calendarText, "2026-04-06 facility operating_hours 2026-04-06T13:00:00Z 2026-04-06T14:00:00Z") {
		t.Fatalf("calendarText = %q, want first operating-hours occurrence", calendarText)
	}

	exceptionOutput := runRootCommand(t, append([]string{"schedule", "block", "except", "--block-id", created.ID.String(), "--expected-version", "1", "--exception-date", "2026-04-13"}, actorFlags...)...)
	if err := json.Unmarshal([]byte(exceptionOutput), &created); err != nil {
		t.Fatalf("json.Unmarshal(exceptionOutput) error = %v output=%s", err, exceptionOutput)
	}
	if created.Version != 2 {
		t.Fatalf("created.Version = %d, want 2", created.Version)
	}

	calendarOutput = runRootCommand(t, "schedule", "calendar", "--facility-key", "ashtonbee", "--from", "2026-04-06T00:00:00Z", "--until", "2026-04-27T00:00:00Z", "--format", "json")
	occurrences = nil
	if err := json.Unmarshal([]byte(calendarOutput), &occurrences); err != nil {
		t.Fatalf("json.Unmarshal(calendarOutput after exception) error = %v output=%s", err, calendarOutput)
	}
	if len(occurrences) != 2 {
		t.Fatalf("len(occurrences) = %d, want 2", len(occurrences))
	}

	cancelOutput := runRootCommand(t, append([]string{"schedule", "block", "cancel", "--block-id", created.ID.String(), "--expected-version", "2"}, actorFlags...)...)
	if err := json.Unmarshal([]byte(cancelOutput), &created); err != nil {
		t.Fatalf("json.Unmarshal(cancelOutput) error = %v output=%s", err, cancelOutput)
	}
	if created.Status != schedule.StatusCancelled {
		t.Fatalf("created.Status = %q, want %q", created.Status, schedule.StatusCancelled)
	}

	_, _, err = runRootCommandWithResult(t, "schedule", "calendar", "--facility-key", "ashtonbee", "--from", "2026-04-06", "--until", "2026-04-27", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "RFC3339") {
		t.Fatalf("date-only schedule calendar error = %v, want RFC3339 refusal", err)
	}

	timezoneBlockOutput := runRootCommand(t, append([]string{"schedule", "block", "create",
		"--facility-key", "ashtonbee",
		"--scope", "facility",
		"--kind", "event",
		"--effect", "informational",
		"--visibility", "internal",
		"--weekly-weekday", "7",
		"--weekly-start-time", "23:30",
		"--weekly-end-time", "23:50",
		"--weekly-timezone", "America/Toronto",
		"--weekly-recurrence-start-date", "2026-04-05",
		"--weekly-recurrence-end-date", "2026-04-12",
	}, actorFlags...)...)
	var timezoneBlock schedule.Block
	if err := json.Unmarshal([]byte(timezoneBlockOutput), &timezoneBlock); err != nil {
		t.Fatalf("json.Unmarshal(timezoneBlockOutput) error = %v output=%s", err, timezoneBlockOutput)
	}
	if timezoneBlock.ID == uuid.Nil {
		t.Fatal("timezoneBlock.ID = nil")
	}

	calendarOutput = runRootCommand(t, "schedule", "calendar", "--facility-key", "ashtonbee", "--from", "2026-04-06T03:00:00Z", "--until", "2026-04-06T04:00:00Z", "--format", "json")
	occurrences = nil
	if err := json.Unmarshal([]byte(calendarOutput), &occurrences); err != nil {
		t.Fatalf("json.Unmarshal(calendarOutput timezone window) error = %v output=%s", err, calendarOutput)
	}
	if len(occurrences) != 1 {
		t.Fatalf("len(occurrences) = %d, want 1 for timezone edge window", len(occurrences))
	}
	if occurrences[0].OccurrenceDate != "2026-04-05" {
		t.Fatalf("occurrences[0].OccurrenceDate = %q, want %q", occurrences[0].OccurrenceDate, "2026-04-05")
	}
	if got := occurrences[0].StartsAt.UTC().Format(time.RFC3339); got != "2026-04-06T03:30:00Z" {
		t.Fatalf("occurrences[0].StartsAt = %q, want %q", got, "2026-04-06T03:30:00Z")
	}

	calendarOutput = runRootCommand(t, "schedule", "calendar", "--facility-key", "ashtonbee", "--from", "2026-04-06T04:00:00Z", "--until", "2026-04-06T05:00:00Z", "--format", "json")
	occurrences = nil
	if err := json.Unmarshal([]byte(calendarOutput), &occurrences); err != nil {
		t.Fatalf("json.Unmarshal(calendarOutput narrow timezone window) error = %v output=%s", err, calendarOutput)
	}
	if len(occurrences) != 0 {
		t.Fatalf("len(occurrences) = %d, want 0 for narrow timezone window", len(occurrences))
	}

	nonOwnerBlockOutput := runRootCommand(t, append([]string{"schedule", "block", "create",
		"--facility-key", "ashtonbee",
		"--scope", "facility",
		"--kind", "closure",
		"--effect", "closed",
		"--visibility", "internal",
		"--one-off-starts-at", "2026-04-07T10:00:00Z",
		"--one-off-ends-at", "2026-04-07T11:00:00Z",
	}, managerFlags...)...)
	if !strings.Contains(nonOwnerBlockOutput, `"kind": "closure"`) {
		t.Fatalf("manager block write output = %q, want created closure block", nonOwnerBlockOutput)
	}
}

func TestSportListCommandReturnsDeterministicJSON(t *testing.T) {
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

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	first := runRootCommand(t, "sport", "list", "--format", "json")
	second := runRootCommand(t, "sport", "list", "--format", "json")
	if first != second {
		t.Fatalf("sport list output changed between runs:\nfirst=%s\nsecond=%s", first, second)
	}

	var decoded []map[string]any
	if err := json.Unmarshal([]byte(first), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("len(decoded) = %d, want 2", len(decoded))
	}
	if decoded[0]["sport_key"] != "badminton" || decoded[1]["sport_key"] != "basketball" {
		t.Fatalf("decoded sport order = %#v, want badminton then basketball", decoded)
	}
}

func TestSportShowCommandIncludesFacilityCapabilityText(t *testing.T) {
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

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	output := runRootCommand(t, "sport", "show", "--sport-key", "badminton", "--format", "text")
	if !strings.Contains(output, `key=badminton`) {
		t.Fatalf("output = %q, want sport key", output)
	}
	if !strings.Contains(output, `sport=badminton facility=ashtonbee zones=gym-floor`) {
		t.Fatalf("output = %q, want facility capability line", output)
	}
}

func TestSportCapabilityListCommandReturnsFacilityNotFoundForUnknownFacility(t *testing.T) {
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

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	_, stderr, err := runRootCommandWithResult(t, "sport", "capability", "list", "--facility-key", "unknown-facility", "--format", "text")
	if err == nil {
		t.Fatal("Execute() error = nil, want facility not found")
	}
	if err.Error() != `facility "unknown-facility" not found` {
		t.Fatalf("err = %q, want facility not found message", err.Error())
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty stderr because Execute returns the error", stderr)
	}
}

func TestSportCapabilityListCommandHonorsKnownFacilityFilter(t *testing.T) {
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

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	output := runRootCommand(t, "sport", "capability", "list", "--facility-key", "ashtonbee", "--format", "text")
	if !strings.Contains(output, `sport=badminton facility=ashtonbee zones=gym-floor`) {
		t.Fatalf("output = %q, want badminton capability line", output)
	}
	if !strings.Contains(output, `sport=basketball facility=ashtonbee zones=gym-floor`) {
		t.Fatalf("output = %q, want basketball capability line", output)
	}
}

func TestSportShowCommandReturnsSportNotFoundForUnknownSport(t *testing.T) {
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

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	_, _, err = runRootCommandWithResult(t, "sport", "show", "--sport-key", "pickleball", "--format", "text")
	if err == nil {
		t.Fatal("Execute() error = nil, want sport not found")
	}
	if err.Error() != `sport "pickleball" not found` {
		t.Fatalf("err = %q, want sport not found message", err.Error())
	}
}

func TestServeCommandShutsDownCleanlyWhenContextIsCanceled(t *testing.T) {
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

	addr := reserveListenAddress(t)
	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)
	t.Setenv("APOLLO_HTTP_ADDR", addr)
	t.Setenv("APOLLO_SESSION_COOKIE_SECRET", "0123456789abcdef0123456789abcdef")

	serveCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	command := newServeCmd()
	command.SetContext(serveCtx)

	done := make(chan error, 1)
	go func() {
		done <- command.Execute()
	}()

	waitForHealthyHTTP(t, fmt.Sprintf("http://%s/api/v1/health", addr))
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve command did not shut down within 10s after context cancellation")
	}
}

func runRootCommand(t *testing.T, args ...string) string {
	t.Helper()

	stdout, stderr, err := runRootCommandWithResult(t, args...)
	if err != nil {
		t.Fatalf("Execute(%v) error = %v stderr=%s", args, err, stderr)
	}

	return stdout
}

func runCompetitionResultCLICommand(t *testing.T, name string, sessionID uuid.UUID, matchID uuid.UUID, expectedVersion int, inputJSON string, dryRun bool, actorUserID uuid.UUID, actorSessionID uuid.UUID) competition.CompetitionCommandOutcome {
	t.Helper()

	args := []string{
		"competition", "command", "run",
		"--name", name,
		"--session-id", sessionID.String(),
		"--match-id", matchID.String(),
		"--expected-version", fmt.Sprint(expectedVersion),
		"--actor-role", "manager",
		"--input-json", inputJSON,
		"--format", "json",
	}
	if dryRun {
		args = append(args, "--dry-run")
	} else {
		args = append(args,
			"--actor-user-id", actorUserID.String(),
			"--actor-session-id", actorSessionID.String(),
			"--trusted-surface-key", "staff-console",
		)
	}

	output := runRootCommand(t, args...)
	var outcome competition.CompetitionCommandOutcome
	if err := json.Unmarshal([]byte(output), &outcome); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v output=%s", name, err, output)
	}
	return outcome
}

func runRootCommandWithResult(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	rootCmd := newRootCmd()
	rootCmd.SetArgs(args)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)

	err := rootCmd.Execute()
	return stdout.String(), stderr.String(), err
}

func reserveListenAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()

	return listener.Addr().String()
}

func waitForHealthyHTTP(t *testing.T, url string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(url) //nolint:gosec // local integration probe
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("health endpoint %s did not become ready", url)
}
