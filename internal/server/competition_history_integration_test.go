package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/competition"
)

func TestCompetitionHistoryRuntimeRecordsResultsCompletesSessionsAndExposesDerivedReads(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-history-001", "competition-history-001@example.com")
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
}

func TestCompetitionHistoryRuntimeRejectsInvalidResultWrites(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-history-neg-001", "competition-history-neg-001@example.com")
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
	if wrongOwnerResponse.Code != http.StatusNotFound {
		t.Fatalf("wrongOwnerResponse.Code = %d, want %d", wrongOwnerResponse.Code, http.StatusNotFound)
	}

	invalidCountResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", startedSession.ID, startedSession.Matches[0].ID), `{"sides":[{"side_index":1,"competition_session_team_id":"`+startedSession.Matches[0].SideSlots[0].TeamID.String()+`","outcome":"win"}]}`, ownerCookie)
	if invalidCountResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidCountResponse.Code = %d, want %d", invalidCountResponse.Code, http.StatusBadRequest)
	}

	mismatchedTeamResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", startedSession.ID, startedSession.Matches[0].ID), fmt.Sprintf(`{"sides":[
		{"side_index":1,"competition_session_team_id":"%s","outcome":"win"},
		{"side_index":2,"competition_session_team_id":"%s","outcome":"loss"}
	]}`, uuid.New(), startedSession.Matches[0].SideSlots[1].TeamID), ownerCookie)
	if mismatchedTeamResponse.Code != http.StatusConflict {
		t.Fatalf("mismatchedTeamResponse.Code = %d, want %d", mismatchedTeamResponse.Code, http.StatusConflict)
	}

	garbageOutcomeResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", startedSession.ID, startedSession.Matches[0].ID), fmt.Sprintf(`{"sides":[
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

func TestCompetitionHistoryRuntimeSeparatesRatingsBySportAndMode(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-history-mode-001", "competition-history-mode-001@example.com")
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

	return decodeCompetitionSession(t, response)
}

func buildResultRequestBody(sideSlots []competition.MatchSideRef, outcomes []string) string {
	body := `{"sides":[`
	for index, sideSlot := range sideSlots {
		if index > 0 {
			body += ","
		}
		body += fmt.Sprintf(`{"side_index":%d,"competition_session_team_id":"%s","outcome":"%s"}`, sideSlot.SideIndex, sideSlot.TeamID, outcomes[index])
	}
	body += `]}`
	return body
}

func decodeCompetitionMemberStats(t *testing.T, response *httptest.ResponseRecorder) []competition.MemberStat {
	t.Helper()

	var payload []competition.MemberStat
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(competition member stats) error = %v", err)
	}

	return payload
}
