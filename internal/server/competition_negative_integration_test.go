package server

import (
	"fmt"
	"net/http"
	"testing"
)

func TestCompetitionRuntimeRejectsInvalidBindingsOwnershipAndInvalidTransitions(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-neg-001", "competition-neg-001@example.com")
	strangerCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-competition-neg-002", "competition-neg-002@example.com")
	_, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-competition-neg-003", "competition-neg-003@example.com")

	missingCookieResponse := env.doRequest(t, http.MethodGet, "/api/v1/competition/sessions", nil)
	if missingCookieResponse.Code != http.StatusUnauthorized {
		t.Fatalf("missingCookieResponse.Code = %d, want %d", missingCookieResponse.Code, http.StatusUnauthorized)
	}

	invalidSportResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Unknown Sport Session",
		"sport_key":"pickleball",
		"facility_key":"ashtonbee",
		"participants_per_side":1
	}`, ownerCookie)
	if invalidSportResponse.Code != http.StatusNotFound {
		t.Fatalf("invalidSportResponse.Code = %d, want %d", invalidSportResponse.Code, http.StatusNotFound)
	}

	invalidFacilityResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Badminton Wrong Facility",
		"sport_key":"badminton",
		"facility_key":"morningside",
		"participants_per_side":1
	}`, ownerCookie)
	if invalidFacilityResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidFacilityResponse.Code = %d, want %d", invalidFacilityResponse.Code, http.StatusBadRequest)
	}

	createSessionResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Tracer 20 Singles",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, ownerCookie)
	if createSessionResponse.Code != http.StatusCreated {
		t.Fatalf("createSessionResponse.Code = %d, want %d", createSessionResponse.Code, http.StatusCreated)
	}
	session := decodeCompetitionSession(t, createSessionResponse)

	duplicateSessionResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Tracer 20 Singles",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, ownerCookie)
	if duplicateSessionResponse.Code != http.StatusConflict {
		t.Fatalf("duplicateSessionResponse.Code = %d, want %d", duplicateSessionResponse.Code, http.StatusConflict)
	}

	teamOneResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/teams", session.ID), `{"side_index":1}`, ownerCookie)
	if teamOneResponse.Code != http.StatusCreated {
		t.Fatalf("teamOneResponse.Code = %d, want %d", teamOneResponse.Code, http.StatusCreated)
	}
	teamOne := decodeCompetitionTeam(t, teamOneResponse)

	duplicateTeamResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/teams", session.ID), `{"side_index":1}`, ownerCookie)
	if duplicateTeamResponse.Code != http.StatusConflict {
		t.Fatalf("duplicateTeamResponse.Code = %d, want %d", duplicateTeamResponse.Code, http.StatusConflict)
	}

	teamTwoResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/teams", session.ID), `{"side_index":2}`, ownerCookie)
	if teamTwoResponse.Code != http.StatusCreated {
		t.Fatalf("teamTwoResponse.Code = %d, want %d", teamTwoResponse.Code, http.StatusCreated)
	}
	teamTwo := decodeCompetitionTeam(t, teamTwoResponse)

	addRosterMember(t, env, ownerCookie, session.ID.String(), teamOne.ID.String(), owner.ID.String(), 1)

	rosterConflictResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/teams/%s/members", session.ID, teamTwo.ID), fmt.Sprintf(`{
		"user_id":"%s",
		"slot_index":1
	}`, owner.ID), ownerCookie)
	if rosterConflictResponse.Code != http.StatusConflict {
		t.Fatalf("rosterConflictResponse.Code = %d, want %d", rosterConflictResponse.Code, http.StatusConflict)
	}

	slotOutOfRangeResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/teams/%s/members", session.ID, teamTwo.ID), fmt.Sprintf(`{
		"user_id":"%s",
		"slot_index":2
	}`, memberTwo.ID), ownerCookie)
	if slotOutOfRangeResponse.Code != http.StatusBadRequest {
		t.Fatalf("slotOutOfRangeResponse.Code = %d, want %d", slotOutOfRangeResponse.Code, http.StatusBadRequest)
	}

	teamSizeMismatchResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches", session.ID), fmt.Sprintf(`{
		"match_index":1,
		"side_slots":[
			{"team_id":"%s","side_index":1},
			{"team_id":"%s","side_index":2}
		]
	}`, teamOne.ID, teamTwo.ID), ownerCookie)
	if teamSizeMismatchResponse.Code != http.StatusConflict {
		t.Fatalf("teamSizeMismatchResponse.Code = %d, want %d", teamSizeMismatchResponse.Code, http.StatusConflict)
	}

	addRosterMember(t, env, ownerCookie, session.ID.String(), teamTwo.ID.String(), memberTwo.ID.String(), 1)

	unauthorizedSessionRead := env.doRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/competition/sessions/%s", session.ID), nil, strangerCookie)
	if unauthorizedSessionRead.Code != http.StatusNotFound {
		t.Fatalf("unauthorizedSessionRead.Code = %d, want %d", unauthorizedSessionRead.Code, http.StatusNotFound)
	}

	createMatchResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches", session.ID), fmt.Sprintf(`{
		"match_index":1,
		"side_slots":[
			{"team_id":"%s","side_index":1},
			{"team_id":"%s","side_index":2}
		]
	}`, teamOne.ID, teamTwo.ID), ownerCookie)
	if createMatchResponse.Code != http.StatusCreated {
		t.Fatalf("createMatchResponse.Code = %d, want %d body=%s", createMatchResponse.Code, http.StatusCreated, createMatchResponse.Body.String())
	}
	match := decodeCompetitionMatch(t, createMatchResponse)

	removeRosterMemberResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/teams/%s/members/%s/remove", session.ID, teamOne.ID, owner.ID), nil, ownerCookie)
	if removeRosterMemberResponse.Code != http.StatusConflict {
		t.Fatalf("removeRosterMemberResponse.Code = %d, want %d", removeRosterMemberResponse.Code, http.StatusConflict)
	}

	archiveSessionWhileDraftMatchResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/archive", session.ID), nil, ownerCookie)
	if archiveSessionWhileDraftMatchResponse.Code != http.StatusConflict {
		t.Fatalf("archiveSessionWhileDraftMatchResponse.Code = %d, want %d", archiveSessionWhileDraftMatchResponse.Code, http.StatusConflict)
	}

	archiveMatchResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/archive", session.ID, match.ID), nil, ownerCookie)
	if archiveMatchResponse.Code != http.StatusOK {
		t.Fatalf("archiveMatchResponse.Code = %d, want %d", archiveMatchResponse.Code, http.StatusOK)
	}

	repeatedArchiveMatchResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/archive", session.ID, match.ID), nil, ownerCookie)
	if repeatedArchiveMatchResponse.Code != http.StatusConflict {
		t.Fatalf("repeatedArchiveMatchResponse.Code = %d, want %d", repeatedArchiveMatchResponse.Code, http.StatusConflict)
	}

	archiveSessionResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/archive", session.ID), nil, ownerCookie)
	if archiveSessionResponse.Code != http.StatusOK {
		t.Fatalf("archiveSessionResponse.Code = %d, want %d", archiveSessionResponse.Code, http.StatusOK)
	}

	createTeamOnArchivedSessionResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/teams", session.ID), `{"side_index":3}`, ownerCookie)
	if createTeamOnArchivedSessionResponse.Code != http.StatusConflict {
		t.Fatalf("createTeamOnArchivedSessionResponse.Code = %d, want %d", createTeamOnArchivedSessionResponse.Code, http.StatusConflict)
	}
}
