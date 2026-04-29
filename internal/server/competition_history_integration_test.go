package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/competition"
	"github.com/ixxet/apollo/internal/rating"
)

func TestCompetitionHistoryRuntimeRecordsResultsCompletesSessionsAndExposesDerivedReads(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-history-001", "competition-history-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-competition-history-002", "competition-history-002@example.com")

	for _, cookie := range []*http.Cookie{ownerCookie, memberTwoCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	session := createStartedCompetitionSession(t, env, ownerCookie, "Tracer 22 Singles Result", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, memberTwo.ID})
	if session.Status != competition.SessionStatusInProgress {
		t.Fatalf("session.Status = %q, want %q", session.Status, competition.SessionStatusInProgress)
	}

	completedSession := recordCompetitionResult(t, env, ownerCookie, session.ID.String(), session.Matches[0].ID.String(), session.Matches[0].SideSlots, []string{"win", "loss"})
	if completedSession.Status != competition.SessionStatusCompleted {
		t.Fatalf("completedSession.Status = %q, want %q", completedSession.Status, competition.SessionStatusCompleted)
	}
	if completedSession.Matches[0].Status != competition.MatchStatusCompleted {
		t.Fatalf("completedSession.Matches[0].Status = %q, want %q", completedSession.Matches[0].Status, competition.MatchStatusCompleted)
	}
	if completedSession.Matches[0].Result == nil {
		t.Fatalf("completedSession.Matches[0].Result = nil, want result")
	}
	if got, want := len(completedSession.Matches[0].Result.Sides), 2; got != want {
		t.Fatalf("len(completedSession.Matches[0].Result.Sides) = %d, want %d", got, want)
	}
	if got, want := len(completedSession.Standings), 2; got != want {
		t.Fatalf("len(completedSession.Standings) = %d, want %d", got, want)
	}
	if completedSession.Standings[0].Rank != 1 || completedSession.Standings[0].Wins != 1 {
		t.Fatalf("completedSession.Standings[0] = %+v, want first-place winner", completedSession.Standings[0])
	}
	if completedSession.Standings[1].Rank != 2 || completedSession.Standings[1].Losses != 1 {
		t.Fatalf("completedSession.Standings[1] = %+v, want second-place loser", completedSession.Standings[1])
	}

	firstRead := env.doRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/competition/sessions/%s", completedSession.ID), nil, ownerCookie)
	if firstRead.Code != http.StatusOK {
		t.Fatalf("firstRead.Code = %d, want %d", firstRead.Code, http.StatusOK)
	}
	secondRead := env.doRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/competition/sessions/%s", completedSession.ID), nil, ownerCookie)
	if secondRead.Code != http.StatusOK {
		t.Fatalf("secondRead.Code = %d, want %d", secondRead.Code, http.StatusOK)
	}
	if firstRead.Body.String() != secondRead.Body.String() {
		t.Fatalf("completed session detail changed between repeated reads\nfirst=%s\nsecond=%s", firstRead.Body.String(), secondRead.Body.String())
	}

	ownerStatsResponse := env.doRequest(t, http.MethodGet, "/api/v1/competition/member-stats", nil, ownerCookie)
	if ownerStatsResponse.Code != http.StatusOK {
		t.Fatalf("ownerStatsResponse.Code = %d, want %d", ownerStatsResponse.Code, http.StatusOK)
	}
	ownerStats := decodeCompetitionMemberStats(t, ownerStatsResponse)
	if got, want := len(ownerStats), 1; got != want {
		t.Fatalf("len(ownerStats) = %d, want %d", got, want)
	}
	if ownerStats[0].ModeKey != "head_to_head:s2-p1" {
		t.Fatalf("ownerStats[0].ModeKey = %q, want %q", ownerStats[0].ModeKey, "head_to_head:s2-p1")
	}
	if ownerStats[0].Wins != 1 || ownerStats[0].Losses != 0 {
		t.Fatalf("ownerStats[0] = %+v, want one recorded win", ownerStats[0])
	}
	if ownerStats[0].CurrentRatingMu <= 0 || ownerStats[0].CurrentRatingSigma <= 0 {
		t.Fatalf("ownerStats[0] = %+v, want positive rating values", ownerStats[0])
	}
	assertLegacyRatingProjection(t, env, owner.ID, "badminton", "head_to_head:s2-p1", completedSession.Matches[0].Result.ID)
	assertOpenSkillComparison(t, env, owner.ID, "badminton", "head_to_head:s2-p1", completedSession.Matches[0].Result.ID, false)
	assertRatingEventCount(t, env, rating.EventLegacyComputed, completedSession.Matches[0].Result.ID, 2)
	assertRatingEventCount(t, env, rating.EventOpenSkillComputed, completedSession.Matches[0].Result.ID, 2)
	assertRatingEventCount(t, env, rating.EventProjectionRebuilt, completedSession.Matches[0].Result.ID, 1)
	assertRatingPolicySelectedEventCount(t, env, "badminton", 1)

	memberTwoStatsResponse := env.doRequest(t, http.MethodGet, "/api/v1/competition/member-stats", nil, memberTwoCookie)
	if memberTwoStatsResponse.Code != http.StatusOK {
		t.Fatalf("memberTwoStatsResponse.Code = %d, want %d", memberTwoStatsResponse.Code, http.StatusOK)
	}
	memberTwoStats := decodeCompetitionMemberStats(t, memberTwoStatsResponse)
	if got, want := len(memberTwoStats), 1; got != want {
		t.Fatalf("len(memberTwoStats) = %d, want %d", got, want)
	}
	if memberTwoStats[0].Losses != 1 {
		t.Fatalf("memberTwoStats[0] = %+v, want one loss", memberTwoStats[0])
	}

	ownerHistoryResponse := env.doRequest(t, http.MethodGet, "/api/v1/competition/history", nil, ownerCookie)
	if ownerHistoryResponse.Code != http.StatusOK {
		t.Fatalf("ownerHistoryResponse.Code = %d, want %d", ownerHistoryResponse.Code, http.StatusOK)
	}
	if strings.Contains(ownerHistoryResponse.Body.String(), "recorded_by_user_id") || strings.Contains(ownerHistoryResponse.Body.String(), "side_slots") {
		t.Fatalf("owner history response leaked staff or session detail: %s", ownerHistoryResponse.Body.String())
	}
	ownerHistory := decodeCompetitionMemberHistory(t, ownerHistoryResponse)
	if got, want := len(ownerHistory), 1; got != want {
		t.Fatalf("len(ownerHistory) = %d, want %d", got, want)
	}
	if ownerHistory[0].Outcome != "win" {
		t.Fatalf("ownerHistory[0].Outcome = %q, want win", ownerHistory[0].Outcome)
	}
	if ownerHistory[0].ModeKey != "head_to_head:s2-p1" {
		t.Fatalf("ownerHistory[0].ModeKey = %q, want %q", ownerHistory[0].ModeKey, "head_to_head:s2-p1")
	}
	if ownerHistory[0].DisplayName != "Tracer 22 Singles Result" {
		t.Fatalf("ownerHistory[0].DisplayName = %q, want %q", ownerHistory[0].DisplayName, "Tracer 22 Singles Result")
	}

	memberTwoHistoryResponse := env.doRequest(t, http.MethodGet, "/api/v1/competition/history", nil, memberTwoCookie)
	if memberTwoHistoryResponse.Code != http.StatusOK {
		t.Fatalf("memberTwoHistoryResponse.Code = %d, want %d", memberTwoHistoryResponse.Code, http.StatusOK)
	}
	memberTwoHistory := decodeCompetitionMemberHistory(t, memberTwoHistoryResponse)
	if got, want := len(memberTwoHistory), 1; got != want {
		t.Fatalf("len(memberTwoHistory) = %d, want %d", got, want)
	}
	if memberTwoHistory[0].Outcome != "loss" {
		t.Fatalf("memberTwoHistory[0].Outcome = %q, want loss", memberTwoHistory[0].Outcome)
	}

	publicReadinessResponse := env.doRequest(t, http.MethodGet, "/api/v1/public/competition/readiness", nil)
	if publicReadinessResponse.Code != http.StatusOK {
		t.Fatalf("publicReadinessResponse.Code = %d, want %d body=%s", publicReadinessResponse.Code, http.StatusOK, publicReadinessResponse.Body.String())
	}
	assertPublicCompetitionBodySafe(t, publicReadinessResponse.Body.String(), owner.ID, memberTwo.ID)
	if !strings.Contains(publicReadinessResponse.Body.String(), `"result_source":"finalized_or_corrected_canonical_results"`) {
		t.Fatalf("public readiness missing canonical result source: %s", publicReadinessResponse.Body.String())
	}
	if !strings.Contains(publicReadinessResponse.Body.String(), `"rating_source":"legacy_elo_like_active_projection"`) {
		t.Fatalf("public readiness missing legacy rating source: %s", publicReadinessResponse.Body.String())
	}

	publicLeaderboardResponse := env.doRequest(t, http.MethodGet, "/api/v1/public/competition/leaderboards?sport_key=badminton&mode_key=head_to_head:s2-p1&stat_type=wins", nil)
	if publicLeaderboardResponse.Code != http.StatusOK {
		t.Fatalf("publicLeaderboardResponse.Code = %d, want %d body=%s", publicLeaderboardResponse.Code, http.StatusOK, publicLeaderboardResponse.Body.String())
	}
	publicLeaderboardBody := publicLeaderboardResponse.Body.String()
	assertPublicCompetitionBodySafe(t, publicLeaderboardBody, owner.ID, memberTwo.ID)
	if !strings.Contains(publicLeaderboardBody, `"participant":"participant_1"`) {
		t.Fatalf("public leaderboard missing redacted participant label: %s", publicLeaderboardBody)
	}
	if strings.Contains(publicLeaderboardBody, "source_result_id") || strings.Contains(publicLeaderboardBody, "canonical_result_id") || strings.Contains(publicLeaderboardBody, "competition_match_id") {
		t.Fatalf("public leaderboard leaked internal result identity: %s", publicLeaderboardBody)
	}
}

func TestCompetitionHistoryRuntimeRejectsInvalidResultWrites(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-history-neg-001", "competition-history-neg-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	strangerCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-competition-history-neg-002", "competition-history-neg-002@example.com")
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-competition-history-neg-003", "competition-history-neg-003@example.com")

	for _, cookie := range []*http.Cookie{ownerCookie, memberTwoCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	queuedSession := createAssignedCompetitionSession(t, env, ownerCookie, "Tracer 22 Result Negatives", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, memberTwo.ID})

	beforeStartResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", queuedSession.ID, queuedSession.Matches[0].ID), buildResultRequestBody(queuedSession.Matches[0].SideSlots, []string{"win", "loss"}), ownerCookie)
	if beforeStartResponse.Code != http.StatusConflict {
		t.Fatalf("beforeStartResponse.Code = %d, want %d", beforeStartResponse.Code, http.StatusConflict)
	}

	startedSession := startCompetitionSession(t, env, ownerCookie, queuedSession.ID.String())

	wrongOwnerResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", startedSession.ID, startedSession.Matches[0].ID), buildResultRequestBody(startedSession.Matches[0].SideSlots, []string{"win", "loss"}), strangerCookie)
	if wrongOwnerResponse.Code != http.StatusForbidden {
		t.Fatalf("wrongOwnerResponse.Code = %d, want %d", wrongOwnerResponse.Code, http.StatusForbidden)
	}

	invalidCountResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", startedSession.ID, startedSession.Matches[0].ID), `{"expected_result_version":0,"sides":[{"side_index":1,"competition_session_team_id":"`+startedSession.Matches[0].SideSlots[0].TeamID.String()+`","outcome":"win"}]}`, ownerCookie)
	if invalidCountResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidCountResponse.Code = %d, want %d", invalidCountResponse.Code, http.StatusBadRequest)
	}

	mismatchedTeamResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", startedSession.ID, startedSession.Matches[0].ID), fmt.Sprintf(`{"expected_result_version":0,"sides":[
			{"side_index":1,"competition_session_team_id":"%s","outcome":"win"},
			{"side_index":2,"competition_session_team_id":"%s","outcome":"loss"}
		]}`, uuid.New(), startedSession.Matches[0].SideSlots[1].TeamID), ownerCookie)
	if mismatchedTeamResponse.Code != http.StatusConflict {
		t.Fatalf("mismatchedTeamResponse.Code = %d, want %d", mismatchedTeamResponse.Code, http.StatusConflict)
	}

	garbageOutcomeResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", startedSession.ID, startedSession.Matches[0].ID), fmt.Sprintf(`{"expected_result_version":0,"sides":[
			{"side_index":1,"competition_session_team_id":"%s","outcome":"forfeit"},
			{"side_index":2,"competition_session_team_id":"%s","outcome":"loss"}
		]}`, startedSession.Matches[0].SideSlots[0].TeamID, startedSession.Matches[0].SideSlots[1].TeamID), ownerCookie)
	if garbageOutcomeResponse.Code != http.StatusBadRequest {
		t.Fatalf("garbageOutcomeResponse.Code = %d, want %d", garbageOutcomeResponse.Code, http.StatusBadRequest)
	}

	completedSession := recordCompetitionResult(t, env, ownerCookie, startedSession.ID.String(), startedSession.Matches[0].ID.String(), startedSession.Matches[0].SideSlots, []string{"win", "loss"})
	if completedSession.Status != competition.SessionStatusCompleted {
		t.Fatalf("completedSession.Status = %q, want %q", completedSession.Status, competition.SessionStatusCompleted)
	}

	duplicateResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", startedSession.ID, startedSession.Matches[0].ID), buildResultRequestBody(startedSession.Matches[0].SideSlots, []string{"win", "loss"}), ownerCookie)
	if duplicateResponse.Code != http.StatusConflict {
		t.Fatalf("duplicateResponse.Code = %d, want %d", duplicateResponse.Code, http.StatusConflict)
	}

	archivedSession := createStartedCompetitionSession(t, env, ownerCookie, "Tracer 22 Result Archived", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, memberTwo.ID})
	archiveResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/archive", archivedSession.ID), nil, ownerCookie)
	if archiveResponse.Code != http.StatusOK {
		t.Fatalf("archiveResponse.Code = %d, want %d body=%s", archiveResponse.Code, http.StatusOK, archiveResponse.Body.String())
	}
	afterArchiveResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", archivedSession.ID, archivedSession.Matches[0].ID), buildResultRequestBody(archivedSession.Matches[0].SideSlots, []string{"win", "loss"}), ownerCookie)
	if afterArchiveResponse.Code != http.StatusConflict {
		t.Fatalf("afterArchiveResponse.Code = %d, want %d", afterArchiveResponse.Code, http.StatusConflict)
	}
}

func TestPublicCompetitionReadinessExcludesRecordedOnlyResults(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-public-readiness-recorded-001", "public-readiness-recorded-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-public-readiness-recorded-002", "public-readiness-recorded-002@example.com")

	for _, cookie := range []*http.Cookie{ownerCookie, memberTwoCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	session := createStartedCompetitionSession(t, env, ownerCookie, "Recorded Only Public Guard", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, memberTwo.ID})
	recordedResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", session.ID, session.Matches[0].ID), buildResultRequestBody(session.Matches[0].SideSlots, []string{"win", "loss"}), ownerCookie)
	if recordedResponse.Code != http.StatusOK {
		t.Fatalf("recordedResponse.Code = %d, want %d body=%s", recordedResponse.Code, http.StatusOK, recordedResponse.Body.String())
	}

	publicReadinessResponse := env.doRequest(t, http.MethodGet, "/api/v1/public/competition/readiness", nil)
	if publicReadinessResponse.Code != http.StatusOK {
		t.Fatalf("publicReadinessResponse.Code = %d, want %d body=%s", publicReadinessResponse.Code, http.StatusOK, publicReadinessResponse.Body.String())
	}
	publicReadinessBody := publicReadinessResponse.Body.String()
	assertPublicCompetitionBodySafe(t, publicReadinessBody, owner.ID, memberTwo.ID)
	if !strings.Contains(publicReadinessBody, `"status":"unavailable"`) || !strings.Contains(publicReadinessBody, `"available_canonical_results":0`) {
		t.Fatalf("public readiness included non-final result truth: %s", publicReadinessBody)
	}

	publicLeaderboardResponse := env.doRequest(t, http.MethodGet, "/api/v1/public/competition/leaderboards?sport_key=badminton&mode_key=head_to_head:s2-p1&stat_type=wins", nil)
	if publicLeaderboardResponse.Code != http.StatusOK {
		t.Fatalf("publicLeaderboardResponse.Code = %d, want %d body=%s", publicLeaderboardResponse.Code, http.StatusOK, publicLeaderboardResponse.Body.String())
	}
	publicLeaderboardBody := publicLeaderboardResponse.Body.String()
	assertPublicCompetitionBodySafe(t, publicLeaderboardBody, owner.ID, memberTwo.ID)
	if !strings.Contains(publicLeaderboardBody, `"leaderboard":[]`) {
		t.Fatalf("public leaderboard included non-final result truth: %s", publicLeaderboardBody)
	}
}

func TestCompetitionHistoryRuntimeSeparatesRatingsBySportAndMode(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-history-mode-001", "competition-history-mode-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-competition-history-mode-002", "competition-history-mode-002@example.com")
	memberThreeCookie, memberThree := createVerifiedSessionViaHTTP(t, env, "student-competition-history-mode-003", "competition-history-mode-003@example.com")
	memberFourCookie, memberFour := createVerifiedSessionViaHTTP(t, env, "student-competition-history-mode-004", "competition-history-mode-004@example.com")
	memberFiveCookie, memberFive := createVerifiedSessionViaHTTP(t, env, "student-competition-history-mode-005", "competition-history-mode-005@example.com")
	memberSixCookie, memberSix := createVerifiedSessionViaHTTP(t, env, "student-competition-history-mode-006", "competition-history-mode-006@example.com")

	for _, cookie := range []*http.Cookie{ownerCookie, memberTwoCookie, memberThreeCookie, memberFourCookie, memberFiveCookie, memberSixCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	singles := createStartedCompetitionSession(t, env, ownerCookie, "Tracer 22 Singles", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, memberTwo.ID})
	recordCompetitionResult(t, env, ownerCookie, singles.ID.String(), singles.Matches[0].ID.String(), singles.Matches[0].SideSlots, []string{"win", "loss"})

	doubles := createStartedCompetitionSession(t, env, ownerCookie, "Tracer 22 Doubles", "badminton", "gym-floor", 2, []uuid.UUID{owner.ID, memberTwo.ID, memberThree.ID, memberFour.ID})
	recordCompetitionResult(t, env, ownerCookie, doubles.ID.String(), doubles.Matches[0].ID.String(), doubles.Matches[0].SideSlots, []string{"loss", "win"})

	basketball := createStartedCompetitionSession(t, env, ownerCookie, "Tracer 22 Basketball", "basketball", "gym-floor", 3, []uuid.UUID{owner.ID, memberTwo.ID, memberThree.ID, memberFour.ID, memberFive.ID, memberSix.ID})
	recordCompetitionResult(t, env, ownerCookie, basketball.ID.String(), basketball.Matches[0].ID.String(), basketball.Matches[0].SideSlots, []string{"draw", "draw"})

	statsResponse := env.doRequest(t, http.MethodGet, "/api/v1/competition/member-stats", nil, ownerCookie)
	if statsResponse.Code != http.StatusOK {
		t.Fatalf("statsResponse.Code = %d, want %d", statsResponse.Code, http.StatusOK)
	}
	stats := decodeCompetitionMemberStats(t, statsResponse)
	if got, want := len(stats), 3; got != want {
		t.Fatalf("len(stats) = %d, want %d", got, want)
	}

	statsByKey := make(map[string]competition.MemberStat, len(stats))
	for _, stat := range stats {
		statsByKey[stat.SportKey+"|"+stat.ModeKey] = stat
	}

	if stat, ok := statsByKey["badminton|head_to_head:s2-p1"]; !ok || stat.Wins != 1 || stat.MatchesPlayed != 1 {
		t.Fatalf("singles stat = %+v, want one badminton singles win", stat)
	}
	if stat, ok := statsByKey["badminton|head_to_head:s2-p2"]; !ok || stat.Losses != 1 || stat.MatchesPlayed != 1 {
		t.Fatalf("doubles stat = %+v, want one badminton doubles loss", stat)
	}
	if stat, ok := statsByKey["basketball|head_to_head:s2-p3"]; !ok || stat.Draws != 1 || stat.MatchesPlayed != 1 {
		t.Fatalf("basketball stat = %+v, want one basketball draw", stat)
	}

	var ratingCount int
	if err := env.db.DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM apollo.competition_member_ratings WHERE user_id = $1", owner.ID).Scan(&ratingCount); err != nil {
		t.Fatalf("QueryRow(rating count) error = %v", err)
	}
	if ratingCount != 3 {
		t.Fatalf("ratingCount = %d, want 3", ratingCount)
	}
}

func TestCompetitionHistoryRuntimeFlagsOpenSkillComparisonDeltas(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-openskill-delta-001", "openskill-delta-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-openskill-delta-002", "openskill-delta-002@example.com")

	for _, cookie := range []*http.Cookie{ownerCookie, memberTwoCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	first := createStartedCompetitionSession(t, env, ownerCookie, "3B.14 OpenSkill Delta First", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, memberTwo.ID})
	first = recordCompetitionResult(t, env, ownerCookie, first.ID.String(), first.Matches[0].ID.String(), first.Matches[0].SideSlots, []string{"win", "loss"})
	assertOpenSkillComparison(t, env, owner.ID, "badminton", "head_to_head:s2-p1", first.Matches[0].Result.ID, false)
	assertRatingEventCount(t, env, rating.EventDeltaFlagged, first.Matches[0].Result.ID, 0)

	second := createStartedCompetitionSession(t, env, ownerCookie, "3B.14 OpenSkill Delta Second", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, memberTwo.ID})
	second = recordCompetitionResult(t, env, ownerCookie, second.ID.String(), second.Matches[0].ID.String(), second.Matches[0].SideSlots, []string{"win", "loss"})
	assertOpenSkillComparison(t, env, owner.ID, "badminton", "head_to_head:s2-p1", second.Matches[0].Result.ID, true)
	assertRatingEventCount(t, env, rating.EventDeltaFlagged, second.Matches[0].Result.ID, 2)

	stats := readCompetitionStats(t, env, ownerCookie)
	if len(stats) != 1 || stats[0].CurrentRatingMu <= 0 || stats[0].CurrentRatingSigma <= 0 {
		t.Fatalf("stats = %+v, want legacy rating read path", stats)
	}

	var ratingEngine string
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT rating_engine
FROM apollo.competition_member_ratings
WHERE user_id = $1
  AND sport_key = 'badminton'
  AND mode_key = 'head_to_head:s2-p1'
`, owner.ID).Scan(&ratingEngine); err != nil {
		t.Fatalf("read rating engine error = %v", err)
	}
	if ratingEngine != rating.EngineLegacyEloLike {
		t.Fatalf("active rating engine = %q, want legacy read path", ratingEngine)
	}
}

func TestCompetitionAnalyticsFoundationProjectsTrustedStats(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-analytics-001", "analytics-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-analytics-002", "analytics-002@example.com")
	memberThreeCookie, memberThree := createVerifiedSessionViaHTTP(t, env, "student-analytics-003", "analytics-003@example.com")
	memberFourCookie, memberFour := createVerifiedSessionViaHTTP(t, env, "student-analytics-004", "analytics-004@example.com")

	for _, cookie := range []*http.Cookie{ownerCookie, memberTwoCookie, memberThreeCookie, memberFourCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	singlesWin := createStartedCompetitionSession(t, env, ownerCookie, "3B.16 Singles Win", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, memberTwo.ID})
	singlesLoss := createStartedCompetitionSession(t, env, ownerCookie, "3B.16 Singles Loss", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, memberTwo.ID})
	doublesWin := createStartedCompetitionSession(t, env, ownerCookie, "3B.16 Doubles Win", "badminton", "gym-floor", 2, []uuid.UUID{owner.ID, memberTwo.ID, memberThree.ID, memberFour.ID})

	beforeQueueIntents := countTableRows(t, env, "apollo.competition_queue_intents")
	beforeAresMatches := countTableRows(t, env, "apollo.ares_matches")
	beforeAresPlayers := countTableRows(t, env, "apollo.ares_match_players")
	beforePreviews := countTableRows(t, env, "apollo.competition_match_previews")
	beforePreviewMembers := countTableRows(t, env, "apollo.competition_match_preview_members")
	beforePreviewEvents := countTableRows(t, env, "apollo.competition_match_preview_events")

	singlesWin = recordCompetitionResult(t, env, ownerCookie, singlesWin.ID.String(), singlesWin.Matches[0].ID.String(), singlesWin.Matches[0].SideSlots, competitionOutcomesForUser(t, singlesWin, owner.ID, "win"))
	singlesLoss = recordCompetitionResult(t, env, ownerCookie, singlesLoss.ID.String(), singlesLoss.Matches[0].ID.String(), singlesLoss.Matches[0].SideSlots, competitionOutcomesForUser(t, singlesLoss, owner.ID, "loss"))
	doublesWin = recordCompetitionResult(t, env, ownerCookie, doublesWin.ID.String(), doublesWin.Matches[0].ID.String(), doublesWin.Matches[0].SideSlots, competitionOutcomesForUser(t, doublesWin, owner.ID, "win"))

	if afterQueueIntents := countTableRows(t, env, "apollo.competition_queue_intents"); afterQueueIntents != beforeQueueIntents {
		t.Fatalf("queue intent rows changed from %d to %d during analytics rebuild", beforeQueueIntents, afterQueueIntents)
	}
	if afterAresMatches := countTableRows(t, env, "apollo.ares_matches"); afterAresMatches != beforeAresMatches {
		t.Fatalf("ares_matches changed from %d to %d during analytics rebuild", beforeAresMatches, afterAresMatches)
	}
	if afterAresPlayers := countTableRows(t, env, "apollo.ares_match_players"); afterAresPlayers != beforeAresPlayers {
		t.Fatalf("ares_match_players changed from %d to %d during analytics rebuild", beforeAresPlayers, afterAresPlayers)
	}
	if afterPreviews := countTableRows(t, env, "apollo.competition_match_previews"); afterPreviews != beforePreviews {
		t.Fatalf("competition_match_previews changed from %d to %d during analytics rebuild", beforePreviews, afterPreviews)
	}
	if afterPreviewMembers := countTableRows(t, env, "apollo.competition_match_preview_members"); afterPreviewMembers != beforePreviewMembers {
		t.Fatalf("competition_match_preview_members changed from %d to %d during analytics rebuild", beforePreviewMembers, afterPreviewMembers)
	}
	if afterPreviewEvents := countTableRows(t, env, "apollo.competition_match_preview_events"); afterPreviewEvents != beforePreviewEvents {
		t.Fatalf("competition_match_preview_events changed from %d to %d during analytics rebuild", beforePreviewEvents, afterPreviewEvents)
	}

	matchesPlayed := readAnalyticsProjection(t, env, owner.ID, "badminton", "ashtonbee", "head_to_head:s2-p1", "all", "matches_played")
	if matchesPlayed.value != 2 || matchesPlayed.sampleSize != 2 {
		t.Fatalf("singles matches projection = %+v, want value/sample 2", matchesPlayed)
	}
	if matchesPlayed.projectionVersion != "competition_analytics_v1" || matchesPlayed.confidence != 0.2 {
		t.Fatalf("singles matches metadata = %+v, want explicit v1 confidence 0.2", matchesPlayed)
	}
	if matchesPlayed.sourceResultID != singlesLoss.Matches[0].Result.ID {
		t.Fatalf("singles matches source_result_id = %s, want latest singles result %s", matchesPlayed.sourceResultID, singlesLoss.Matches[0].Result.ID)
	}
	if !strings.Contains(matchesPlayed.projectionWatermark, singlesLoss.Matches[0].Result.ID.String()) && !strings.Contains(matchesPlayed.projectionWatermark, doublesWin.Matches[0].Result.ID.String()) {
		t.Fatalf("projection_watermark = %q, want canonical source result id", matchesPlayed.projectionWatermark)
	}

	currentStreak := readAnalyticsProjection(t, env, owner.ID, "badminton", "ashtonbee", "head_to_head:s2-p1", "all", "current_streak")
	if currentStreak.value != -1 || currentStreak.sampleSize != 2 {
		t.Fatalf("current streak projection = %+v, want one-match loss streak over singles sample", currentStreak)
	}
	assertAnalyticsStatEvent(t, env, owner.ID, currentStreak.sourceResultID, "badminton", "ashtonbee", "head_to_head:s2-p1", "all", "current_streak", -1, 2, 0.2)

	ratingMovement := readAnalyticsProjection(t, env, owner.ID, "badminton", "ashtonbee", "head_to_head:s2-p1", "all", "rating_movement")
	if ratingMovement.sampleSize != 2 || ratingMovement.value == 0 {
		t.Fatalf("rating movement projection = %+v, want two legacy rating movement samples", ratingMovement)
	}

	opponentStrength := readAnalyticsProjection(t, env, owner.ID, "badminton", "ashtonbee", "head_to_head:s2-p1", "all", "opponent_strength")
	if opponentStrength.sampleSize != 2 || opponentStrength.value <= 0 {
		t.Fatalf("opponent strength projection = %+v, want positive legacy rating-derived opponent strength", opponentStrength)
	}

	teamVsSolo := readAnalyticsProjection(t, env, owner.ID, "badminton", "all", "all", "all", "team_vs_solo_delta")
	if teamVsSolo.value != 0.5 || teamVsSolo.sampleSize != 3 || teamVsSolo.confidence != 0.3 {
		t.Fatalf("team vs solo projection = %+v, want team win-rate delta 0.5 over three trusted results", teamVsSolo)
	}
	if teamVsSolo.sourceResultID != doublesWin.Matches[0].Result.ID {
		t.Fatalf("team vs solo source_result_id = %s, want latest trusted result %s", teamVsSolo.sourceResultID, doublesWin.Matches[0].Result.ID)
	}
	assertAnalyticsStatEvent(t, env, owner.ID, teamVsSolo.sourceResultID, "badminton", "all", "all", "all", "team_vs_solo_delta", 0.5, 3, 0.3)

	sportSplit := readAnalyticsProjection(t, env, owner.ID, "badminton", "all", "all", "all", "matches_played")
	if sportSplit.value != 3 || sportSplit.sampleSize != 3 {
		t.Fatalf("sport split projection = %+v, want all badminton matches", sportSplit)
	}
	modeSplit := readAnalyticsProjection(t, env, owner.ID, "badminton", "all", "head_to_head:s2-p2", "team", "wins")
	if modeSplit.value != 1 || modeSplit.sampleSize != 1 {
		t.Fatalf("team mode split projection = %+v, want one doubles win", modeSplit)
	}

	assertAnalyticsStatEventSourceCount(t, env, owner.ID, singlesLoss.Matches[0].Result.ID, "rating_movement", 1)
	assertAnalyticsStatEventSourceCount(t, env, owner.ID, doublesWin.Matches[0].Result.ID, "opponent_strength", 1)
	assertAnalyticsProjectionRebuiltEvent(t, env, "badminton", doublesWin.Matches[0].Result.ID, 3)

	assertLegacyRatingProjection(t, env, owner.ID, "badminton", "head_to_head:s2-p2", doublesWin.Matches[0].Result.ID)
}

func TestCompetitionResultTrustLifecycleGuardsRatingConsumption(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-result-trust-001", "result-trust-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-result-trust-002", "result-trust-002@example.com")

	for _, cookie := range []*http.Cookie{ownerCookie, memberTwoCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	session := createStartedCompetitionSession(t, env, ownerCookie, "3B.12 Result Trust", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, memberTwo.ID})
	match := session.Matches[0]

	recordResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", session.ID, match.ID), buildResultRequestBodyWithVersion(match.SideSlots, []string{"win", "loss"}, match.ResultVersion), ownerCookie)
	if recordResponse.Code != http.StatusOK {
		t.Fatalf("recordResponse.Code = %d, want %d body=%s", recordResponse.Code, http.StatusOK, recordResponse.Body.String())
	}
	recorded := decodeCompetitionSession(t, recordResponse)
	recordedMatch := recorded.Matches[0]
	if recordedMatch.Result == nil || recordedMatch.Result.ResultStatus != competition.ResultStatusRecorded {
		t.Fatalf("recorded result = %#v, want recorded status", recordedMatch.Result)
	}
	assertCompetitionStatsCount(t, env, ownerCookie, 0)
	assertAnalyticsProjectionCount(t, env, owner.ID, 0)
	assertAnalyticsStatEventSourceCount(t, env, owner.ID, recordedMatch.Result.ID, "matches_played", 0)
	assertRatingProjectionCount(t, env, owner.ID, 0)
	assertOpenSkillComparisonSourceCount(t, env, recordedMatch.Result.ID, 0)
	assertRatingEventCount(t, env, rating.EventLegacyComputed, recordedMatch.Result.ID, 0)
	assertRatingEventCount(t, env, rating.EventOpenSkillComputed, recordedMatch.Result.ID, 0)

	finalizeResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result/finalize", session.ID, match.ID), fmt.Sprintf(`{"expected_result_version":%d}`, recordedMatch.ResultVersion), ownerCookie)
	if finalizeResponse.Code != http.StatusOK {
		t.Fatalf("finalizeResponse.Code = %d, want %d body=%s", finalizeResponse.Code, http.StatusOK, finalizeResponse.Body.String())
	}
	finalized := decodeCompetitionSession(t, finalizeResponse)
	finalizedMatch := finalized.Matches[0]
	if finalizedMatch.Result == nil || finalizedMatch.Result.ResultStatus != competition.ResultStatusFinalized || finalizedMatch.Result.FinalizedAt == nil {
		t.Fatalf("finalized result = %#v, want finalized status and timestamp", finalizedMatch.Result)
	}
	assertCompetitionStatsCount(t, env, ownerCookie, 1)
	assertAnalyticsProjectionMinCount(t, env, owner.ID, 1)
	assertAnalyticsStatEventSourceCount(t, env, owner.ID, finalizedMatch.Result.ID, "matches_played", 1)
	assertLegacyRatingProjection(t, env, owner.ID, "badminton", "head_to_head:s2-p1", finalizedMatch.Result.ID)
	assertOpenSkillComparison(t, env, owner.ID, "badminton", "head_to_head:s2-p1", finalizedMatch.Result.ID, false)

	disputeResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result/dispute", session.ID, match.ID), fmt.Sprintf(`{"expected_result_version":%d}`, finalizedMatch.ResultVersion), ownerCookie)
	if disputeResponse.Code != http.StatusOK {
		t.Fatalf("disputeResponse.Code = %d, want %d body=%s", disputeResponse.Code, http.StatusOK, disputeResponse.Body.String())
	}
	disputed := decodeCompetitionSession(t, disputeResponse)
	disputedMatch := disputed.Matches[0]
	if disputedMatch.Result == nil || disputedMatch.Result.ResultStatus != competition.ResultStatusDisputed || disputedMatch.Result.DisputeStatus != competition.DisputeStatusDisputed {
		t.Fatalf("disputed result = %#v, want disputed status", disputedMatch.Result)
	}
	assertCompetitionStatsCount(t, env, ownerCookie, 0)
	assertAnalyticsProjectionCount(t, env, owner.ID, 0)
	assertAnalyticsStatEventSourceCount(t, env, owner.ID, finalizedMatch.Result.ID, "matches_played", 0)
	assertRatingProjectionCount(t, env, owner.ID, 0)
	assertOpenSkillComparisonSourceCount(t, env, finalizedMatch.Result.ID, 0)

	correctResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result/correct", session.ID, match.ID), buildResultRequestBodyWithVersion(disputedMatch.SideSlots, []string{"loss", "win"}, disputedMatch.ResultVersion), ownerCookie)
	if correctResponse.Code != http.StatusOK {
		t.Fatalf("correctResponse.Code = %d, want %d body=%s", correctResponse.Code, http.StatusOK, correctResponse.Body.String())
	}
	corrected := decodeCompetitionSession(t, correctResponse)
	correctedMatch := corrected.Matches[0]
	if correctedMatch.Result == nil || correctedMatch.Result.ResultStatus != competition.ResultStatusCorrected || correctedMatch.Result.CorrectionID == nil || correctedMatch.Result.SupersedesResultID == nil || correctedMatch.Result.CorrectedAt == nil {
		t.Fatalf("corrected result = %#v, want correction linkage and timestamp", correctedMatch.Result)
	}
	if *correctedMatch.Result.SupersedesResultID != finalizedMatch.Result.ID {
		t.Fatalf("supersedes_result_id = %s, want original result %s", correctedMatch.Result.SupersedesResultID, finalizedMatch.Result.ID)
	}
	stats := readCompetitionStats(t, env, ownerCookie)
	if len(stats) != 1 || stats[0].Losses != 1 || stats[0].Wins != 0 {
		t.Fatalf("corrected stats = %+v, want corrected loss only", stats)
	}
	assertLegacyRatingProjection(t, env, owner.ID, "badminton", "head_to_head:s2-p1", correctedMatch.Result.ID)
	assertAnalyticsProjectionMinCount(t, env, owner.ID, 1)
	assertAnalyticsStatEventSourceCount(t, env, owner.ID, finalizedMatch.Result.ID, "matches_played", 0)
	assertAnalyticsStatEventSourceCount(t, env, owner.ID, correctedMatch.Result.ID, "matches_played", 1)
	assertOpenSkillComparison(t, env, owner.ID, "badminton", "head_to_head:s2-p1", correctedMatch.Result.ID, false)
	assertRatingProjectionSourceCount(t, env, finalizedMatch.Result.ID, 0)
	assertOpenSkillComparisonSourceCount(t, env, finalizedMatch.Result.ID, 0)
	assertRatingEventCount(t, env, rating.EventLegacyComputed, finalizedMatch.Result.ID, 2)
	assertRatingEventCount(t, env, rating.EventLegacyComputed, correctedMatch.Result.ID, 2)
	assertRatingEventCount(t, env, rating.EventOpenSkillComputed, finalizedMatch.Result.ID, 2)
	assertRatingEventCount(t, env, rating.EventOpenSkillComputed, correctedMatch.Result.ID, 2)

	voidResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result/void", session.ID, match.ID), fmt.Sprintf(`{"expected_result_version":%d}`, correctedMatch.ResultVersion), ownerCookie)
	if voidResponse.Code != http.StatusOK {
		t.Fatalf("voidResponse.Code = %d, want %d body=%s", voidResponse.Code, http.StatusOK, voidResponse.Body.String())
	}
	voided := decodeCompetitionSession(t, voidResponse)
	voidedMatch := voided.Matches[0]
	if voidedMatch.Result == nil || voidedMatch.Result.ResultStatus != competition.ResultStatusVoided {
		t.Fatalf("voided result = %#v, want voided status", voidedMatch.Result)
	}
	assertCompetitionStatsCount(t, env, ownerCookie, 0)
	assertAnalyticsProjectionCount(t, env, owner.ID, 0)
	assertAnalyticsStatEventSourceCount(t, env, owner.ID, correctedMatch.Result.ID, "matches_played", 0)
	assertRatingProjectionCount(t, env, owner.ID, 0)
	assertRatingProjectionSourceCount(t, env, correctedMatch.Result.ID, 0)
	assertOpenSkillComparisonSourceCount(t, env, correctedMatch.Result.ID, 0)

	var resultCount int
	if err := env.db.DB.QueryRow(context.Background(), `SELECT count(*) FROM apollo.competition_match_results WHERE competition_match_id = $1`, match.ID).Scan(&resultCount); err != nil {
		t.Fatalf("count match results error = %v", err)
	}
	if resultCount != 2 {
		t.Fatalf("resultCount = %d, want original plus corrected result", resultCount)
	}

	for _, eventType := range []string{
		"competition.match.started",
		"competition.result.recorded",
		"competition.result.finalized",
		"competition.result.disputed",
		"competition.result.corrected",
		"competition.result.voided",
	} {
		var eventCount int
		if err := env.db.DB.QueryRow(context.Background(), `SELECT count(*) FROM apollo.competition_lifecycle_events WHERE event_type = $1 AND competition_match_id = $2`, eventType, match.ID).Scan(&eventCount); err != nil {
			t.Fatalf("count lifecycle event %s error = %v", eventType, err)
		}
		if eventCount == 0 {
			t.Fatalf("event %s count = 0, want at least one", eventType)
		}
	}
}

func TestCompetitionHistoryRouteRequiresAuthenticatedSession(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	response := env.doRequest(t, http.MethodGet, "/api/v1/competition/history", nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("response.Code = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func createAssignedCompetitionSession(t *testing.T, env *authProfileServerEnv, ownerCookie *http.Cookie, displayName string, sportKey string, zoneKey string, participantsPerSide int, userIDs []uuid.UUID) competition.Session {
	t.Helper()

	createSessionResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", fmt.Sprintf(`{
		"display_name":%q,
		"sport_key":%q,
		"facility_key":"ashtonbee",
		"zone_key":%q,
		"participants_per_side":%d
	}`, displayName, sportKey, zoneKey, participantsPerSide), ownerCookie)
	if createSessionResponse.Code != http.StatusCreated {
		t.Fatalf("createSessionResponse.Code = %d, want %d body=%s", createSessionResponse.Code, http.StatusCreated, createSessionResponse.Body.String())
	}
	session := decodeCompetitionSession(t, createSessionResponse)
	session = openCompetitionQueue(t, env, ownerCookie, session.ID.String())
	for _, userID := range userIDs {
		session = queueCompetitionMember(t, env, ownerCookie, session.ID.String(), userID)
	}

	assignResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/assignment", session.ID), fmt.Sprintf(`{"expected_queue_version":%d}`, session.QueueVersion), ownerCookie)
	if assignResponse.Code != http.StatusOK {
		t.Fatalf("assignResponse.Code = %d, want %d body=%s", assignResponse.Code, http.StatusOK, assignResponse.Body.String())
	}

	return decodeCompetitionSession(t, assignResponse)
}

func createStartedCompetitionSession(t *testing.T, env *authProfileServerEnv, ownerCookie *http.Cookie, displayName string, sportKey string, zoneKey string, participantsPerSide int, userIDs []uuid.UUID) competition.Session {
	t.Helper()

	return startCompetitionSession(t, env, ownerCookie, createAssignedCompetitionSession(t, env, ownerCookie, displayName, sportKey, zoneKey, participantsPerSide, userIDs).ID.String())
}

func startCompetitionSession(t *testing.T, env *authProfileServerEnv, ownerCookie *http.Cookie, sessionID string) competition.Session {
	t.Helper()

	response := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/start", sessionID), nil, ownerCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("start response code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	return decodeCompetitionSession(t, response)
}

func recordCompetitionResult(t *testing.T, env *authProfileServerEnv, ownerCookie *http.Cookie, sessionID string, matchID string, sideSlots []competition.MatchSideRef, outcomes []string) competition.Session {
	t.Helper()

	response := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", sessionID, matchID), buildResultRequestBody(sideSlots, outcomes), ownerCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("result response code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	recorded := decodeCompetitionSession(t, response)
	recordedMatch := recorded.Matches[0]
	for _, match := range recorded.Matches {
		if match.ID.String() == matchID {
			recordedMatch = match
			break
		}
	}

	finalizeResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result/finalize", sessionID, matchID), fmt.Sprintf(`{"expected_result_version":%d}`, recordedMatch.ResultVersion), ownerCookie)
	if finalizeResponse.Code != http.StatusOK {
		t.Fatalf("finalize response code = %d, want %d body=%s", finalizeResponse.Code, http.StatusOK, finalizeResponse.Body.String())
	}

	return decodeCompetitionSession(t, finalizeResponse)
}

func buildResultRequestBody(sideSlots []competition.MatchSideRef, outcomes []string) string {
	return buildResultRequestBodyWithVersion(sideSlots, outcomes, 0)
}

func buildResultRequestBodyWithVersion(sideSlots []competition.MatchSideRef, outcomes []string, expectedVersion int) string {
	body := fmt.Sprintf(`{"expected_result_version":%d,"sides":[`, expectedVersion)
	for index, sideSlot := range sideSlots {
		if index > 0 {
			body += ","
		}
		body += fmt.Sprintf(`{"side_index":%d,"competition_session_team_id":"%s","outcome":"%s"}`, sideSlot.SideIndex, sideSlot.TeamID, outcomes[index])
	}
	body += `]}`
	return body
}

func assertCompetitionStatsCount(t *testing.T, env *authProfileServerEnv, cookie *http.Cookie, want int) {
	t.Helper()

	stats := readCompetitionStats(t, env, cookie)
	if len(stats) != want {
		t.Fatalf("len(stats) = %d, want %d; stats=%+v", len(stats), want, stats)
	}
}

func assertRatingProjectionCount(t *testing.T, env *authProfileServerEnv, userID uuid.UUID, want int) {
	t.Helper()

	var count int
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT count(*)
FROM apollo.competition_member_ratings
WHERE user_id = $1
`, userID).Scan(&count); err != nil {
		t.Fatalf("count competition_member_ratings error = %v", err)
	}
	if count != want {
		t.Fatalf("rating projection count = %d, want %d", count, want)
	}
}

func assertLegacyRatingProjection(t *testing.T, env *authProfileServerEnv, userID uuid.UUID, sportKey string, modeKey string, sourceResultID uuid.UUID) {
	t.Helper()

	var ratingEngine string
	var engineVersion string
	var policyVersion string
	var projectedSourceResultID uuid.UUID
	var ratingEventID uuid.UUID
	var projectionWatermark string
	var mu float64
	var sigma float64
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT rating_engine,
       engine_version,
       policy_version,
       source_result_id,
       rating_event_id,
       projection_watermark,
       mu::double precision,
       sigma::double precision
FROM apollo.competition_member_ratings
WHERE user_id = $1
  AND sport_key = $2
  AND mode_key = $3
`, userID, sportKey, modeKey).Scan(&ratingEngine, &engineVersion, &policyVersion, &projectedSourceResultID, &ratingEventID, &projectionWatermark, &mu, &sigma); err != nil {
		t.Fatalf("read legacy rating projection error = %v", err)
	}

	if ratingEngine != rating.EngineLegacyEloLike || engineVersion != rating.EngineVersionLegacy || policyVersion != rating.PolicyVersionLegacy {
		t.Fatalf("rating policy = %s/%s/%s, want %s/%s/%s", ratingEngine, engineVersion, policyVersion, rating.EngineLegacyEloLike, rating.EngineVersionLegacy, rating.PolicyVersionLegacy)
	}
	if projectedSourceResultID != sourceResultID {
		t.Fatalf("projection source_result_id = %s, want %s", projectedSourceResultID, sourceResultID)
	}
	if ratingEventID == uuid.Nil {
		t.Fatal("rating_event_id is nil, want auditable event id")
	}
	if !strings.Contains(projectionWatermark, sourceResultID.String()) {
		t.Fatalf("projection_watermark = %q, want source result id %s", projectionWatermark, sourceResultID)
	}
	if mu <= 0 || sigma <= 0 {
		t.Fatalf("rating projection mu/sigma = %.4f/%.4f, want positive values", mu, sigma)
	}

	var eventType string
	var eventEngine string
	var eventPolicy string
	var eventSourceResultID uuid.UUID
	var deltaMu float64
	var deltaSigma float64
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT event_type,
       rating_engine,
       policy_version,
       source_result_id,
       delta_mu::double precision,
       delta_sigma::double precision
FROM apollo.competition_rating_events
WHERE id = $1
`, ratingEventID).Scan(&eventType, &eventEngine, &eventPolicy, &eventSourceResultID, &deltaMu, &deltaSigma); err != nil {
		t.Fatalf("read rating event error = %v", err)
	}
	if eventType != rating.EventLegacyComputed || eventEngine != rating.EngineLegacyEloLike || eventPolicy != rating.PolicyVersionLegacy {
		t.Fatalf("rating event = %s/%s/%s, want legacy computed policy", eventType, eventEngine, eventPolicy)
	}
	if eventSourceResultID != sourceResultID {
		t.Fatalf("event source_result_id = %s, want %s", eventSourceResultID, sourceResultID)
	}
	if deltaMu == 0 && deltaSigma == 0 {
		t.Fatalf("rating event deltas = %.4f/%.4f, want computed audit deltas", deltaMu, deltaSigma)
	}
}

func assertOpenSkillComparison(t *testing.T, env *authProfileServerEnv, userID uuid.UUID, sportKey string, modeKey string, sourceResultID uuid.UUID, wantFlagged bool) {
	t.Helper()

	var legacyEngine string
	var openSkillEngine string
	var legacyMu float64
	var legacySigma float64
	var openSkillMu float64
	var openSkillSigma float64
	var deltaFromLegacy float64
	var acceptedDeltaBudget float64
	var comparisonScenario string
	var deltaFlagged bool
	var projectionWatermark string
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT legacy_rating_engine,
       openskill_rating_engine,
       legacy_mu::double precision,
       legacy_sigma::double precision,
       openskill_mu::double precision,
       openskill_sigma::double precision,
       delta_from_legacy::double precision,
       accepted_delta_budget::double precision,
       comparison_scenario,
       delta_flagged,
       projection_watermark
FROM apollo.competition_rating_comparisons
WHERE user_id = $1
  AND sport_key = $2
  AND mode_key = $3
  AND source_result_id = $4
`, userID, sportKey, modeKey, sourceResultID).Scan(&legacyEngine, &openSkillEngine, &legacyMu, &legacySigma, &openSkillMu, &openSkillSigma, &deltaFromLegacy, &acceptedDeltaBudget, &comparisonScenario, &deltaFlagged, &projectionWatermark); err != nil {
		t.Fatalf("read OpenSkill comparison error = %v", err)
	}

	if legacyEngine != rating.EngineLegacyEloLike || openSkillEngine != rating.EngineOpenSkill {
		t.Fatalf("comparison engines = %s/%s, want legacy/OpenSkill", legacyEngine, openSkillEngine)
	}
	if legacyMu <= 0 || legacySigma <= 0 || openSkillMu <= 0 || openSkillSigma <= 0 {
		t.Fatalf("comparison values = legacy %.4f/%.4f openskill %.4f/%.4f, want positive values", legacyMu, legacySigma, openSkillMu, openSkillSigma)
	}
	if acceptedDeltaBudget != rating.AcceptedOpenSkillDeltaBudget {
		t.Fatalf("accepted_delta_budget = %.4f, want %.4f", acceptedDeltaBudget, rating.AcceptedOpenSkillDeltaBudget)
	}
	if comparisonScenario == "" {
		t.Fatal("comparison_scenario is empty")
	}
	if deltaFlagged != wantFlagged {
		t.Fatalf("delta_flagged = %t, want %t (delta_from_legacy %.4f)", deltaFlagged, wantFlagged, deltaFromLegacy)
	}
	if !strings.Contains(projectionWatermark, sourceResultID.String()) {
		t.Fatalf("projection_watermark = %q, want source result id %s", projectionWatermark, sourceResultID)
	}
}

func assertRatingEventCount(t *testing.T, env *authProfileServerEnv, eventType string, sourceResultID uuid.UUID, want int) {
	t.Helper()

	var count int
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT count(*)
FROM apollo.competition_rating_events
WHERE event_type = $1
  AND source_result_id = $2
`, eventType, sourceResultID).Scan(&count); err != nil {
		t.Fatalf("count rating events error = %v", err)
	}
	if count != want {
		t.Fatalf("rating event %s count = %d, want %d for source result %s", eventType, count, want, sourceResultID)
	}
}

func assertOpenSkillComparisonSourceCount(t *testing.T, env *authProfileServerEnv, sourceResultID uuid.UUID, want int) {
	t.Helper()

	var count int
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT count(*)
FROM apollo.competition_rating_comparisons
WHERE source_result_id = $1
`, sourceResultID).Scan(&count); err != nil {
		t.Fatalf("count OpenSkill comparison source error = %v", err)
	}
	if count != want {
		t.Fatalf("OpenSkill comparison source count = %d, want %d for source result %s", count, want, sourceResultID)
	}
}

func assertRatingPolicySelectedEventCount(t *testing.T, env *authProfileServerEnv, sportKey string, want int) {
	t.Helper()

	var count int
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT count(*)
FROM apollo.competition_rating_events
WHERE event_type = $1
  AND sport_key = $2
  AND rating_engine = $3
  AND engine_version = $4
  AND policy_version = $5
`, rating.EventPolicySelected, sportKey, rating.EngineLegacyEloLike, rating.EngineVersionLegacy, rating.PolicyVersionLegacy).Scan(&count); err != nil {
		t.Fatalf("count policy selected events error = %v", err)
	}
	if count != want {
		t.Fatalf("policy selected event count = %d, want %d", count, want)
	}
}

func assertRatingProjectionSourceCount(t *testing.T, env *authProfileServerEnv, sourceResultID uuid.UUID, want int) {
	t.Helper()

	var count int
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT count(*)
FROM apollo.competition_member_ratings
WHERE source_result_id = $1
`, sourceResultID).Scan(&count); err != nil {
		t.Fatalf("count rating projection source error = %v", err)
	}
	if count != want {
		t.Fatalf("rating projection source count = %d, want %d for source result %s", count, want, sourceResultID)
	}
}

type analyticsProjectionRead struct {
	value               float64
	sampleSize          int
	confidence          float64
	projectionVersion   string
	projectionWatermark string
	sourceResultID      uuid.UUID
}

func readAnalyticsProjection(t *testing.T, env *authProfileServerEnv, userID uuid.UUID, sportKey string, facilityKey string, modeKey string, teamScope string, statType string) analyticsProjectionRead {
	t.Helper()

	var projection analyticsProjectionRead
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT stat_value::double precision,
       sample_size,
       confidence::double precision,
       projection_version,
       projection_watermark,
       source_result_id
FROM apollo.competition_analytics_projections
WHERE user_id = $1
  AND sport_key = $2
  AND facility_key = $3
  AND mode_key = $4
  AND team_scope = $5
  AND stat_type = $6
`, userID, sportKey, facilityKey, modeKey, teamScope, statType).Scan(&projection.value, &projection.sampleSize, &projection.confidence, &projection.projectionVersion, &projection.projectionWatermark, &projection.sourceResultID); err != nil {
		t.Fatalf("read analytics projection %s/%s/%s/%s/%s error = %v", sportKey, facilityKey, modeKey, teamScope, statType, err)
	}

	return projection
}

func assertAnalyticsProjectionCount(t *testing.T, env *authProfileServerEnv, userID uuid.UUID, want int) {
	t.Helper()

	var count int
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT count(*)
FROM apollo.competition_analytics_projections
WHERE user_id = $1
`, userID).Scan(&count); err != nil {
		t.Fatalf("count analytics projections error = %v", err)
	}
	if count != want {
		t.Fatalf("analytics projection count = %d, want %d", count, want)
	}
}

func assertAnalyticsProjectionMinCount(t *testing.T, env *authProfileServerEnv, userID uuid.UUID, minCount int) {
	t.Helper()

	var count int
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT count(*)
FROM apollo.competition_analytics_projections
WHERE user_id = $1
`, userID).Scan(&count); err != nil {
		t.Fatalf("count analytics projections error = %v", err)
	}
	if count < minCount {
		t.Fatalf("analytics projection count = %d, want at least %d", count, minCount)
	}
}

func assertAnalyticsStatEventSourceCount(t *testing.T, env *authProfileServerEnv, userID uuid.UUID, sourceResultID uuid.UUID, statType string, want int) {
	t.Helper()

	var count int
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT count(*)
FROM apollo.competition_analytics_events
WHERE event_type = 'competition.analytics.stat_computed'
  AND user_id = $1
  AND source_result_id = $2
  AND stat_type = $3
`, userID, sourceResultID, statType).Scan(&count); err != nil {
		t.Fatalf("count analytics stat events error = %v", err)
	}
	if count != want {
		t.Fatalf("analytics stat event count for %s/%s = %d, want %d", sourceResultID, statType, count, want)
	}
}

func assertAnalyticsStatEvent(t *testing.T, env *authProfileServerEnv, userID uuid.UUID, sourceResultID uuid.UUID, sportKey string, facilityKey string, modeKey string, teamScope string, statType string, wantValue float64, wantSampleSize int, wantConfidence float64) {
	t.Helper()

	var value float64
	var sampleSize int
	var confidence float64
	var projectionVersion string
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT stat_value::double precision,
       sample_size,
       confidence::double precision,
       projection_version
FROM apollo.competition_analytics_events
WHERE event_type = 'competition.analytics.stat_computed'
  AND user_id = $1
  AND source_result_id = $2
  AND sport_key = $3
  AND facility_key = $4
  AND mode_key = $5
  AND team_scope = $6
  AND stat_type = $7
ORDER BY computed_at DESC, id DESC
LIMIT 1
`, userID, sourceResultID, sportKey, facilityKey, modeKey, teamScope, statType).Scan(&value, &sampleSize, &confidence, &projectionVersion); err != nil {
		t.Fatalf("read analytics stat event %s/%s/%s/%s/%s error = %v", sportKey, facilityKey, modeKey, teamScope, statType, err)
	}
	if projectionVersion != "competition_analytics_v1" {
		t.Fatalf("analytics stat event projection_version = %q, want competition_analytics_v1", projectionVersion)
	}
	if value != wantValue || sampleSize != wantSampleSize || confidence != wantConfidence {
		t.Fatalf("analytics stat event %s = value %.4f sample %d confidence %.4f, want value %.4f sample %d confidence %.4f", statType, value, sampleSize, confidence, wantValue, wantSampleSize, wantConfidence)
	}
}

func assertAnalyticsProjectionRebuiltEvent(t *testing.T, env *authProfileServerEnv, sportKey string, sourceResultID uuid.UUID, sampleSize int) {
	t.Helper()

	var projectionVersion string
	var projectionWatermark string
	var confidence float64
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT projection_version,
       projection_watermark,
       confidence::double precision
FROM apollo.competition_analytics_events
WHERE event_type = 'competition.analytics.projection_rebuilt'
  AND sport_key = $1
  AND source_result_id = $2
  AND sample_size = $3
ORDER BY computed_at DESC, id DESC
LIMIT 1
`, sportKey, sourceResultID, sampleSize).Scan(&projectionVersion, &projectionWatermark, &confidence); err != nil {
		t.Fatalf("read analytics projection rebuilt event error = %v", err)
	}
	if projectionVersion != "competition_analytics_v1" {
		t.Fatalf("projection rebuilt version = %q, want competition_analytics_v1", projectionVersion)
	}
	if !strings.Contains(projectionWatermark, sourceResultID.String()) {
		t.Fatalf("projection rebuilt watermark = %q, want source result %s", projectionWatermark, sourceResultID)
	}
	if confidence != 0.3 {
		t.Fatalf("projection rebuilt confidence = %.4f, want 0.3", confidence)
	}
}

func competitionOutcomesForUser(t *testing.T, session competition.Session, userID uuid.UUID, outcome string) []string {
	t.Helper()

	var teamID uuid.UUID
	for _, team := range session.Teams {
		for _, member := range team.Roster {
			if member.UserID == userID {
				teamID = team.ID
				break
			}
		}
	}
	if teamID == uuid.Nil {
		t.Fatalf("user %s has no team in session %s", userID, session.ID)
	}

	opposite := "loss"
	if outcome == "loss" {
		opposite = "win"
	}
	if outcome == "draw" {
		opposite = "draw"
	}

	outcomes := make([]string, len(session.Matches[0].SideSlots))
	for index, sideSlot := range session.Matches[0].SideSlots {
		if sideSlot.TeamID == teamID {
			outcomes[index] = outcome
		} else {
			outcomes[index] = opposite
		}
	}
	return outcomes
}

func readCompetitionStats(t *testing.T, env *authProfileServerEnv, cookie *http.Cookie) []competition.MemberStat {
	t.Helper()

	response := env.doRequest(t, http.MethodGet, "/api/v1/competition/member-stats", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("stats response code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	return decodeCompetitionMemberStats(t, response)
}

func decodeCompetitionMemberStats(t *testing.T, response *httptest.ResponseRecorder) []competition.MemberStat {
	t.Helper()

	var payload []competition.MemberStat
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(competition member stats) error = %v", err)
	}

	return payload
}

func decodeCompetitionMemberHistory(t *testing.T, response *httptest.ResponseRecorder) []competition.MemberHistoryEntry {
	t.Helper()

	var payload []competition.MemberHistoryEntry
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(competition member history) error = %v", err)
	}

	return payload
}

func assertPublicCompetitionBodySafe(t *testing.T, body string, userIDs ...uuid.UUID) {
	t.Helper()

	for _, userID := range userIDs {
		if strings.Contains(body, userID.String()) {
			t.Fatalf("public competition body leaked user id %s: %s", userID, body)
		}
	}
	for _, forbidden := range []string{
		"openskill",
		"accepted_delta_budget",
		"safety",
		"reliability",
		"reporter_user_id",
		"subject_user_id",
		"blocked_user_id",
		"private_notes",
		"trusted_surface",
		"actor_user_id",
		"command",
		"readiness_message",
		"match_quality",
		"predicted_win_probability",
		"projection_watermark",
		"sample_size",
		"confidence",
		"rating_engine",
		"engine_version",
		"policy_version",
		"rating_mu",
		"rating_sigma",
		"current_rating",
	} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Fatalf("public competition body leaked %q: %s", forbidden, body)
		}
	}
}
