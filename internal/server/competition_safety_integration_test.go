package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/competition"
)

func TestCompetitionSafetyReliabilityRuntimeCapturesManagerOnlyFacts(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-safety-runtime-001", "safety-runtime-001@example.com")
	memberCookie, member := createVerifiedSessionViaHTTP(t, env, "student-safety-runtime-002", "safety-runtime-002@example.com")
	supervisorCookie, supervisor := createVerifiedSessionViaHTTP(t, env, "student-safety-runtime-003", "safety-runtime-003@example.com")
	_, outsider := createVerifiedSessionViaHTTP(t, env, "student-safety-runtime-004", "safety-runtime-004@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	setUserRole(t, env, supervisor.ID, authz.RoleSupervisor)

	for _, cookie := range []*http.Cookie{ownerCookie, memberCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}
	session := createStartedCompetitionSession(t, env, ownerCookie, "Safety Runtime Session", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, member.ID})
	match := session.Matches[0]

	beforeSnapshot := competitionTruthSnapshot(t, env, session.ID, match.ID)
	outOfScopeReport := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/safety/reports", fmt.Sprintf(`{
		"competition_session_id":"%s",
		"reporter_user_id":"%s",
		"subject_user_id":"%s",
		"target_type":"competition_member",
		"target_id":"%s",
		"reason_code":"conduct",
		"note":"out-of-scope report should not persist"
	}`, session.ID, outsider.ID, member.ID, member.ID), ownerCookie)
	if outOfScopeReport.Code != http.StatusBadRequest {
		t.Fatalf("outOfScopeReport.Code = %d, want %d body=%s", outOfScopeReport.Code, http.StatusBadRequest, outOfScopeReport.Body.String())
	}
	if !strings.Contains(outOfScopeReport.Body.String(), competition.ErrSafetyUserOutOfScope.Error()) {
		t.Fatalf("outOfScopeReport body = %s, want out-of-scope error", outOfScopeReport.Body.String())
	}
	assertTableRowCount(t, env, "apollo.competition_safety_reports", 0)

	reportResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/safety/reports", fmt.Sprintf(`{
		"competition_session_id":"%s",
		"reporter_user_id":"%s",
		"subject_user_id":"%s",
		"target_type":"competition_member",
		"target_id":"%s",
		"reason_code":"conduct",
		"note":"manager private report note"
	}`, session.ID, owner.ID, member.ID, member.ID), ownerCookie)
	if reportResponse.Code != http.StatusCreated {
		t.Fatalf("reportResponse.Code = %d, want %d body=%s", reportResponse.Code, http.StatusCreated, reportResponse.Body.String())
	}
	report := decodeSafetyReport(t, reportResponse.Body.Bytes())
	if report.PrivacyScope != "manager_internal" || report.ReporterUserID != owner.ID || report.SubjectUserID == nil || *report.SubjectUserID != member.ID {
		t.Fatalf("report = %#v, want manager-internal reporter/subject fact", report)
	}
	assertCompetitionTruthSnapshot(t, env, session.ID, match.ID, beforeSnapshot)
	assertTableRowCount(t, env, "apollo.competition_safety_reports", 1)
	assertTableRowCount(t, env, "apollo.competition_safety_events", 1)

	outOfScopeBlock := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/safety/blocks", fmt.Sprintf(`{
		"competition_session_id":"%s",
		"competition_match_id":"%s",
		"blocker_user_id":"%s",
		"blocked_user_id":"%s",
		"reason_code":"conduct"
	}`, session.ID, match.ID, owner.ID, outsider.ID), ownerCookie)
	if outOfScopeBlock.Code != http.StatusBadRequest {
		t.Fatalf("outOfScopeBlock.Code = %d, want %d body=%s", outOfScopeBlock.Code, http.StatusBadRequest, outOfScopeBlock.Body.String())
	}
	assertTableRowCount(t, env, "apollo.competition_safety_blocks", 0)

	blockResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/safety/blocks", fmt.Sprintf(`{
		"competition_session_id":"%s",
		"competition_match_id":"%s",
		"blocker_user_id":"%s",
		"blocked_user_id":"%s",
		"reason_code":"conduct"
	}`, session.ID, match.ID, owner.ID, member.ID), ownerCookie)
	if blockResponse.Code != http.StatusCreated {
		t.Fatalf("blockResponse.Code = %d, want %d body=%s", blockResponse.Code, http.StatusCreated, blockResponse.Body.String())
	}
	block := decodeSafetyBlock(t, blockResponse.Body.Bytes())
	if block.PrivacyScope != "manager_internal" || block.BlockerUserID != owner.ID || block.BlockedUserID != member.ID {
		t.Fatalf("block = %#v, want manager-internal block fact", block)
	}
	duplicateBlock := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/safety/blocks", fmt.Sprintf(`{
		"competition_session_id":"%s",
		"blocker_user_id":"%s",
		"blocked_user_id":"%s",
		"reason_code":"conduct"
	}`, session.ID, owner.ID, member.ID), ownerCookie)
	if duplicateBlock.Code != http.StatusConflict {
		t.Fatalf("duplicateBlock.Code = %d, want %d body=%s", duplicateBlock.Code, http.StatusConflict, duplicateBlock.Body.String())
	}
	assertTableRowCount(t, env, "apollo.competition_safety_blocks", 1)
	assertTableRowCount(t, env, "apollo.competition_safety_events", 2)

	outOfScopeReliability := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/reliability/events", fmt.Sprintf(`{
		"competition_session_id":"%s",
		"competition_match_id":"%s",
		"subject_user_id":"%s",
		"reliability_type":"no_show",
		"severity":"warning",
		"note":"out-of-scope reliability should not persist"
	}`, session.ID, match.ID, outsider.ID), ownerCookie)
	if outOfScopeReliability.Code != http.StatusBadRequest {
		t.Fatalf("outOfScopeReliability.Code = %d, want %d body=%s", outOfScopeReliability.Code, http.StatusBadRequest, outOfScopeReliability.Body.String())
	}
	assertTableRowCount(t, env, "apollo.competition_reliability_events", 0)

	reliabilityResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/reliability/events", fmt.Sprintf(`{
		"competition_session_id":"%s",
		"competition_match_id":"%s",
		"subject_user_id":"%s",
		"reliability_type":"no_show",
		"severity":"warning",
		"note":"manager private reliability note"
	}`, session.ID, match.ID, member.ID), ownerCookie)
	if reliabilityResponse.Code != http.StatusCreated {
		t.Fatalf("reliabilityResponse.Code = %d, want %d body=%s", reliabilityResponse.Code, http.StatusCreated, reliabilityResponse.Body.String())
	}
	reliabilityEvent := decodeReliabilityEvent(t, reliabilityResponse.Body.Bytes())
	if reliabilityEvent.PrivacyScope != "manager_internal" || reliabilityEvent.SubjectUserID == nil || *reliabilityEvent.SubjectUserID != member.ID {
		t.Fatalf("reliabilityEvent = %#v, want manager-internal reliability fact", reliabilityEvent)
	}
	assertCompetitionTruthSnapshot(t, env, session.ID, match.ID, beforeSnapshot)
	assertTableRowCount(t, env, "apollo.competition_reliability_events", 1)
	assertTableRowCount(t, env, "apollo.competition_safety_events", 3)

	readinessResponse := env.doRequestWithHeaders(t, http.MethodGet, "/api/v1/competition/safety/readiness", nil, trustedSurfaceHeaders(env), ownerCookie)
	if readinessResponse.Code != http.StatusOK {
		t.Fatalf("readinessResponse.Code = %d, want %d body=%s", readinessResponse.Code, http.StatusOK, readinessResponse.Body.String())
	}
	readiness := decodeSafetyReadiness(t, readinessResponse.Body.Bytes())
	if readiness.Summary.ReportCount != 1 || readiness.Summary.BlockCount != 1 || readiness.Summary.ReliabilityEventCount != 1 || readiness.Summary.AuditEventCount != 3 {
		t.Fatalf("readiness summary = %#v, want 1/1/1/3", readiness.Summary)
	}

	reviewResponse := env.doRequestWithHeaders(t, http.MethodGet, "/api/v1/competition/safety/review?limit=5", nil, trustedSurfaceHeaders(env), ownerCookie)
	if reviewResponse.Code != http.StatusOK {
		t.Fatalf("reviewResponse.Code = %d, want %d body=%s", reviewResponse.Code, http.StatusOK, reviewResponse.Body.String())
	}
	reviewBody := reviewResponse.Body.String()
	if !strings.Contains(reviewBody, "manager private report note") || !strings.Contains(reviewBody, "manager private reliability note") {
		t.Fatalf("manager safety review missing private notes: %s", reviewBody)
	}

	supervisorReview := env.doRequestWithHeaders(t, http.MethodGet, "/api/v1/competition/safety/review?limit=5", nil, trustedSurfaceHeaders(env), supervisorCookie)
	if supervisorReview.Code != http.StatusForbidden {
		t.Fatalf("supervisorReview.Code = %d, want %d body=%s", supervisorReview.Code, http.StatusForbidden, supervisorReview.Body.String())
	}
	memberHistory := env.doRequest(t, http.MethodGet, "/api/v1/competition/history", nil, memberCookie)
	if memberHistory.Code != http.StatusOK {
		t.Fatalf("memberHistory.Code = %d, want %d body=%s", memberHistory.Code, http.StatusOK, memberHistory.Body.String())
	}
	assertNoSafetyLeak(t, memberHistory.Body.String())
	memberStats := env.doRequest(t, http.MethodGet, "/api/v1/competition/member-stats", nil, memberCookie)
	if memberStats.Code != http.StatusOK {
		t.Fatalf("memberStats.Code = %d, want %d body=%s", memberStats.Code, http.StatusOK, memberStats.Body.String())
	}
	assertNoSafetyLeak(t, memberStats.Body.String())
	publicAttempt := env.doRequest(t, http.MethodGet, "/api/v1/public/competition/safety/review", nil)
	if publicAttempt.Code != http.StatusNotFound {
		t.Fatalf("publicAttempt.Code = %d, want %d body=%s", publicAttempt.Code, http.StatusNotFound, publicAttempt.Body.String())
	}
}

func TestCompetitionSafetyCommandsAreCapabilityGatedAndAuditable(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-safety-command-001", "safety-command-001@example.com")
	memberCookie, member := createVerifiedSessionViaHTTP(t, env, "student-safety-command-002", "safety-command-002@example.com")
	setUserRole(t, env, manager.ID, authz.RoleManager)

	for _, cookie := range []*http.Cookie{managerCookie, memberCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}
	session := createStartedCompetitionSession(t, env, managerCookie, "Safety Command Session", "badminton", "gym-floor", 1, []uuid.UUID{manager.ID, member.ID})

	deniedOutcome := executeCompetitionCommand(t, env, memberCookie, fmt.Sprintf(`{
		"name":"record_safety_report",
		"dry_run":true,
		"safety_report":{
			"competition_session_id":"%s",
			"reporter_user_id":"%s",
			"subject_user_id":"%s",
			"target_type":"competition_member",
			"target_id":"%s",
			"reason_code":"conduct"
		}
	}`, session.ID, manager.ID, member.ID, member.ID), http.StatusForbidden)
	if deniedOutcome.Status != competition.CommandStatusDenied {
		t.Fatalf("deniedOutcome.Status = %q, want %q", deniedOutcome.Status, competition.CommandStatusDenied)
	}

	outcome := executeCompetitionCommand(t, env, managerCookie, fmt.Sprintf(`{
		"name":"record_safety_report",
		"safety_report":{
			"competition_session_id":"%s",
			"reporter_user_id":"%s",
			"subject_user_id":"%s",
			"target_type":"competition_member",
			"target_id":"%s",
			"reason_code":"unsafe_play",
			"note":"command private report note"
		}
	}`, session.ID, manager.ID, member.ID, member.ID), http.StatusOK)
	if outcome.Status != competition.CommandStatusSucceeded || !outcome.Mutated {
		t.Fatalf("outcome = %#v, want succeeded mutating safety command", outcome)
	}
	assertTableRowCount(t, env, "apollo.competition_safety_reports", 1)
	assertTableRowCount(t, env, "apollo.competition_safety_events", 1)

	missingTrustedSurface := env.doRequestWithoutTrustedSurface(t, http.MethodPost, "/api/v1/competition/safety/reports", bytes.NewBufferString(fmt.Sprintf(`{
		"competition_session_id":"%s",
		"reporter_user_id":"%s",
		"target_type":"competition_session",
		"target_id":"%s",
		"reason_code":"other"
	}`, session.ID, manager.ID, session.ID)), managerCookie)
	if missingTrustedSurface.Code != http.StatusForbidden {
		t.Fatalf("missingTrustedSurface.Code = %d, want %d body=%s", missingTrustedSurface.Code, http.StatusForbidden, missingTrustedSurface.Body.String())
	}
	assertTableRowCount(t, env, "apollo.competition_safety_reports", 1)
	assertTableRowCount(t, env, "apollo.competition_safety_events", 1)
}

type competitionSafetyTruthSnapshot struct {
	SessionStatus             string
	SessionQueueVersion       int
	MatchStatus               string
	MatchResultVersion        int
	ResultRows                int
	RatingRows                int
	AnalyticsEventRows        int
	AnalyticsProjectionRows   int
	ARESPreviewRows           int
	TournamentRows            int
	TournamentEventRows       int
	TournamentAdvancementRows int
}

func competitionTruthSnapshot(t *testing.T, env *authProfileServerEnv, sessionID uuid.UUID, matchID uuid.UUID) competitionSafetyTruthSnapshot {
	t.Helper()

	var snapshot competitionSafetyTruthSnapshot
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT status, queue_version
FROM apollo.competition_sessions
WHERE id = $1
`, sessionID).Scan(&snapshot.SessionStatus, &snapshot.SessionQueueVersion); err != nil {
		t.Fatalf("QueryRow(session truth) error = %v", err)
	}
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT status, result_version
FROM apollo.competition_matches
WHERE id = $1
`, matchID).Scan(&snapshot.MatchStatus, &snapshot.MatchResultVersion); err != nil {
		t.Fatalf("QueryRow(match truth) error = %v", err)
	}
	snapshot.ResultRows = countTableRows(t, env, "apollo.competition_match_results")
	snapshot.RatingRows = countTableRows(t, env, "apollo.competition_member_ratings")
	snapshot.AnalyticsEventRows = countTableRows(t, env, "apollo.competition_analytics_events")
	snapshot.AnalyticsProjectionRows = countTableRows(t, env, "apollo.competition_analytics_projections")
	snapshot.ARESPreviewRows = countTableRows(t, env, "apollo.competition_match_previews")
	snapshot.TournamentRows = countTableRows(t, env, "apollo.competition_tournaments")
	snapshot.TournamentEventRows = countTableRows(t, env, "apollo.competition_tournament_events")
	snapshot.TournamentAdvancementRows = countTableRows(t, env, "apollo.competition_tournament_advancements")
	return snapshot
}

func assertCompetitionTruthSnapshot(t *testing.T, env *authProfileServerEnv, sessionID uuid.UUID, matchID uuid.UUID, want competitionSafetyTruthSnapshot) {
	t.Helper()

	got := competitionTruthSnapshot(t, env, sessionID, matchID)
	if got != want {
		t.Fatalf("competition truth snapshot changed\ngot=%#v\nwant=%#v", got, want)
	}
}

func assertTableRowCount(t *testing.T, env *authProfileServerEnv, table string, want int) {
	t.Helper()

	if got := countTableRows(t, env, table); got != want {
		t.Fatalf("%s row count = %d, want %d", table, got, want)
	}
}

func trustedSurfaceHeaders(env *authProfileServerEnv) map[string]string {
	return map[string]string{
		authz.TrustedSurfaceHeader:      env.trustedSurfaceKey,
		authz.TrustedSurfaceTokenHeader: env.trustedSurfaceToken,
	}
}

func assertNoSafetyLeak(t *testing.T, body string) {
	t.Helper()

	for _, forbidden := range []string{
		"manager private report note",
		"manager private reliability note",
		"blocked_user_id",
		"reporter_user_id",
		"privacy_scope",
		"competition_safety",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("body leaks safety/reliability detail %q: %s", forbidden, body)
		}
	}
}

func decodeSafetyReport(t *testing.T, raw []byte) competition.SafetyReport {
	t.Helper()

	var payload competition.SafetyReport
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal(safety report) error = %v raw=%s", err, string(raw))
	}
	return payload
}

func decodeSafetyBlock(t *testing.T, raw []byte) competition.SafetyBlock {
	t.Helper()

	var payload competition.SafetyBlock
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal(safety block) error = %v raw=%s", err, string(raw))
	}
	return payload
}

func decodeReliabilityEvent(t *testing.T, raw []byte) competition.ReliabilityEvent {
	t.Helper()

	var payload competition.ReliabilityEvent
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal(reliability event) error = %v raw=%s", err, string(raw))
	}
	return payload
}

func decodeSafetyReadiness(t *testing.T, raw []byte) competition.CompetitionSafetyReadiness {
	t.Helper()

	var payload competition.CompetitionSafetyReadiness
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal(safety readiness) error = %v raw=%s", err, string(raw))
	}
	return payload
}
