package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ixxet/apollo/internal/competition"
)

func TestCompetitionRuntimeSupportsDeterministicSessionTeamRosterMatchRoundTrip(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-001", "competition-001@example.com")
	_, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-competition-002", "competition-002@example.com")
	_, memberThree := createVerifiedSessionViaHTTP(t, env, "student-competition-003", "competition-003@example.com")
	_, memberFour := createVerifiedSessionViaHTTP(t, env, "student-competition-004", "competition-004@example.com")

	createSessionResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Tracer 20 Badminton Doubles",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":2
	}`, ownerCookie)
	if createSessionResponse.Code != http.StatusCreated {
		t.Fatalf("createSessionResponse.Code = %d, want %d", createSessionResponse.Code, http.StatusCreated)
	}
	session := decodeCompetitionSession(t, createSessionResponse)
	if session.DisplayName != "Tracer 20 Badminton Doubles" {
		t.Fatalf("session.DisplayName = %q, want Tracer 20 Badminton Doubles", session.DisplayName)
	}

	firstTeamResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/teams", session.ID), `{"side_index":1}`, ownerCookie)
	if firstTeamResponse.Code != http.StatusCreated {
		t.Fatalf("firstTeamResponse.Code = %d, want %d", firstTeamResponse.Code, http.StatusCreated)
	}
	firstTeam := decodeCompetitionTeam(t, firstTeamResponse)

	secondTeamResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/teams", session.ID), `{"side_index":2}`, ownerCookie)
	if secondTeamResponse.Code != http.StatusCreated {
		t.Fatalf("secondTeamResponse.Code = %d, want %d", secondTeamResponse.Code, http.StatusCreated)
	}
	secondTeam := decodeCompetitionTeam(t, secondTeamResponse)

	addRosterMember(t, env, ownerCookie, session.ID.String(), firstTeam.ID.String(), owner.ID.String(), 1)
	addRosterMember(t, env, ownerCookie, session.ID.String(), firstTeam.ID.String(), memberTwo.ID.String(), 2)
	addRosterMember(t, env, ownerCookie, session.ID.String(), secondTeam.ID.String(), memberThree.ID.String(), 1)
	addRosterMember(t, env, ownerCookie, session.ID.String(), secondTeam.ID.String(), memberFour.ID.String(), 2)

	createMatchResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches", session.ID), fmt.Sprintf(`{
		"match_index":1,
		"side_slots":[
			{"team_id":"%s","side_index":1},
			{"team_id":"%s","side_index":2}
		]
	}`, firstTeam.ID, secondTeam.ID), ownerCookie)
	if createMatchResponse.Code != http.StatusCreated {
		t.Fatalf("createMatchResponse.Code = %d, want %d", createMatchResponse.Code, http.StatusCreated)
	}
	match := decodeCompetitionMatch(t, createMatchResponse)
	if match.MatchIndex != 1 {
		t.Fatalf("match.MatchIndex = %d, want 1", match.MatchIndex)
	}

	firstRead := env.doRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/competition/sessions/%s", session.ID), nil, ownerCookie)
	if firstRead.Code != http.StatusOK {
		t.Fatalf("firstRead.Code = %d, want %d", firstRead.Code, http.StatusOK)
	}
	secondRead := env.doRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/competition/sessions/%s", session.ID), nil, ownerCookie)
	if secondRead.Code != http.StatusOK {
		t.Fatalf("secondRead.Code = %d, want %d", secondRead.Code, http.StatusOK)
	}
	if firstRead.Body.String() != secondRead.Body.String() {
		t.Fatalf("session detail changed between repeated reads\nfirst=%s\nsecond=%s", firstRead.Body.String(), secondRead.Body.String())
	}

	detail := decodeCompetitionSession(t, firstRead)
	if got, want := len(detail.Teams), 2; got != want {
		t.Fatalf("len(detail.Teams) = %d, want %d", got, want)
	}
	if got, want := len(detail.Matches), 1; got != want {
		t.Fatalf("len(detail.Matches) = %d, want %d", got, want)
	}
	if got, want := len(detail.Teams[0].Roster), 2; got != want {
		t.Fatalf("len(detail.Teams[0].Roster) = %d, want %d", got, want)
	}
	if got, want := detail.Teams[0].Roster[0].UserID, owner.ID; got != want {
		t.Fatalf("detail.Teams[0].Roster[0].UserID = %s, want %s", got, want)
	}
	if got, want := detail.Matches[0].SideSlots[0].TeamID, firstTeam.ID; got != want {
		t.Fatalf("detail.Matches[0].SideSlots[0].TeamID = %s, want %s", got, want)
	}

	archiveMatchResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/archive", session.ID, match.ID), nil, ownerCookie)
	if archiveMatchResponse.Code != http.StatusOK {
		t.Fatalf("archiveMatchResponse.Code = %d, want %d", archiveMatchResponse.Code, http.StatusOK)
	}
	archivedMatch := decodeCompetitionMatch(t, archiveMatchResponse)
	if archivedMatch.Status != competition.MatchStatusArchived {
		t.Fatalf("archivedMatch.Status = %q, want %q", archivedMatch.Status, competition.MatchStatusArchived)
	}

	archiveSessionResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/archive", session.ID), nil, ownerCookie)
	if archiveSessionResponse.Code != http.StatusOK {
		t.Fatalf("archiveSessionResponse.Code = %d, want %d", archiveSessionResponse.Code, http.StatusOK)
	}
	archivedSession := decodeCompetitionSession(t, archiveSessionResponse)
	if archivedSession.Status != competition.SessionStatusArchived {
		t.Fatalf("archivedSession.Status = %q, want %q", archivedSession.Status, competition.SessionStatusArchived)
	}
}

func TestCompetitionRuntimeAllowsSameUserAcrossDifferentSessions(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-competition-cross-001", "competition-cross-001@example.com")
	_, member := createVerifiedSessionViaHTTP(t, env, "student-competition-cross-002", "competition-cross-002@example.com")

	firstSessionResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Tracer 20 Cross Session One",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, ownerCookie)
	if firstSessionResponse.Code != http.StatusCreated {
		t.Fatalf("firstSessionResponse.Code = %d, want %d", firstSessionResponse.Code, http.StatusCreated)
	}
	firstSession := decodeCompetitionSession(t, firstSessionResponse)

	secondSessionResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Tracer 20 Cross Session Two",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, ownerCookie)
	if secondSessionResponse.Code != http.StatusCreated {
		t.Fatalf("secondSessionResponse.Code = %d, want %d", secondSessionResponse.Code, http.StatusCreated)
	}
	secondSession := decodeCompetitionSession(t, secondSessionResponse)

	firstTeamResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/teams", firstSession.ID), `{"side_index":1}`, ownerCookie)
	if firstTeamResponse.Code != http.StatusCreated {
		t.Fatalf("firstTeamResponse.Code = %d, want %d", firstTeamResponse.Code, http.StatusCreated)
	}
	firstTeam := decodeCompetitionTeam(t, firstTeamResponse)

	secondTeamResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/teams", secondSession.ID), `{"side_index":1}`, ownerCookie)
	if secondTeamResponse.Code != http.StatusCreated {
		t.Fatalf("secondTeamResponse.Code = %d, want %d", secondTeamResponse.Code, http.StatusCreated)
	}
	secondTeam := decodeCompetitionTeam(t, secondTeamResponse)

	addRosterMember(t, env, ownerCookie, firstSession.ID.String(), firstTeam.ID.String(), member.ID.String(), 1)
	addRosterMember(t, env, ownerCookie, secondSession.ID.String(), secondTeam.ID.String(), member.ID.String(), 1)
}

func addRosterMember(t *testing.T, env *authProfileServerEnv, cookie *http.Cookie, sessionID string, teamID string, userID string, slotIndex int) {
	t.Helper()

	response := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/teams/%s/members", sessionID, teamID), fmt.Sprintf(`{
		"user_id":"%s",
		"slot_index":%d
	}`, userID, slotIndex), cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("add roster member response code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}
}

func decodeCompetitionSession(t *testing.T, response *httptest.ResponseRecorder) competition.Session {
	t.Helper()

	var payload competition.Session
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(competition session) error = %v", err)
	}

	return payload
}

func decodeCompetitionTeam(t *testing.T, response *httptest.ResponseRecorder) competition.Team {
	t.Helper()

	var payload competition.Team
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(competition team) error = %v", err)
	}

	return payload
}

func decodeCompetitionMatch(t *testing.T, response *httptest.ResponseRecorder) competition.Match {
	t.Helper()

	var payload competition.Match
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(competition match) error = %v", err)
	}

	return payload
}
