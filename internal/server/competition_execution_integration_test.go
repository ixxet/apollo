package server

import (
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/competition"
)

func TestCompetitionExecutionRuntimeSupportsDeterministicQueueAssignmentAndLifecycle(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-exec-001", "competition-exec-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-competition-exec-002", "competition-exec-002@example.com")
	memberThreeCookie, memberThree := createVerifiedSessionViaHTTP(t, env, "student-competition-exec-003", "competition-exec-003@example.com")
	memberFourCookie, memberFour := createVerifiedSessionViaHTTP(t, env, "student-competition-exec-004", "competition-exec-004@example.com")

	for _, cookie := range []*http.Cookie{ownerCookie, memberTwoCookie, memberThreeCookie, memberFourCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	createSessionResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Tracer 21 Badminton Queue",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":2
	}`, ownerCookie)
	if createSessionResponse.Code != http.StatusCreated {
		t.Fatalf("createSessionResponse.Code = %d, want %d", createSessionResponse.Code, http.StatusCreated)
	}
	session := decodeCompetitionSession(t, createSessionResponse)

	session = openCompetitionQueue(t, env, ownerCookie, session.ID.String())
	if session.Status != competition.SessionStatusQueueOpen {
		t.Fatalf("session.Status = %q, want %q", session.Status, competition.SessionStatusQueueOpen)
	}
	if session.QueueVersion != 1 {
		t.Fatalf("session.QueueVersion = %d, want 1", session.QueueVersion)
	}

	session = queueCompetitionMember(t, env, ownerCookie, session.ID.String(), owner.ID)
	session = queueCompetitionMember(t, env, ownerCookie, session.ID.String(), memberTwo.ID)
	session = queueCompetitionMember(t, env, ownerCookie, session.ID.String(), memberThree.ID)
	session = queueCompetitionMember(t, env, ownerCookie, session.ID.String(), memberFour.ID)

	firstRead := env.doRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/competition/sessions/%s", session.ID), nil, ownerCookie)
	if firstRead.Code != http.StatusOK {
		t.Fatalf("firstRead.Code = %d, want %d", firstRead.Code, http.StatusOK)
	}
	secondRead := env.doRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/competition/sessions/%s", session.ID), nil, ownerCookie)
	if secondRead.Code != http.StatusOK {
		t.Fatalf("secondRead.Code = %d, want %d", secondRead.Code, http.StatusOK)
	}
	if firstRead.Body.String() != secondRead.Body.String() {
		t.Fatalf("queue-open session detail changed between repeated reads\nfirst=%s\nsecond=%s", firstRead.Body.String(), secondRead.Body.String())
	}

	assignResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/assignment", session.ID), fmt.Sprintf(`{
		"expected_queue_version":%d
	}`, session.QueueVersion), ownerCookie)
	if assignResponse.Code != http.StatusOK {
		t.Fatalf("assignResponse.Code = %d, want %d body=%s", assignResponse.Code, http.StatusOK, assignResponse.Body.String())
	}
	assignedSession := decodeCompetitionSession(t, assignResponse)
	if assignedSession.Status != competition.SessionStatusAssigned {
		t.Fatalf("assignedSession.Status = %q, want %q", assignedSession.Status, competition.SessionStatusAssigned)
	}
	if len(assignedSession.Queue) != 0 {
		t.Fatalf("len(assignedSession.Queue) = %d, want 0", len(assignedSession.Queue))
	}
	if got, want := len(assignedSession.Teams), 2; got != want {
		t.Fatalf("len(assignedSession.Teams) = %d, want %d", got, want)
	}
	if got, want := len(assignedSession.Matches), 1; got != want {
		t.Fatalf("len(assignedSession.Matches) = %d, want %d", got, want)
	}
	if assignedSession.Matches[0].Status != competition.MatchStatusAssigned {
		t.Fatalf("assignedSession.Matches[0].Status = %q, want %q", assignedSession.Matches[0].Status, competition.MatchStatusAssigned)
	}

	expectedTeamOne := []uuid.UUID{owner.ID, memberTwo.ID}
	expectedTeamTwo := []uuid.UUID{memberThree.ID, memberFour.ID}
	if got := rosterUserIDs(assignedSession.Teams[0]); !slices.Equal(got, expectedTeamOne) {
		t.Fatalf("rosterUserIDs(assignedSession.Teams[0]) = %v, want %v", got, expectedTeamOne)
	}
	if got := rosterUserIDs(assignedSession.Teams[1]); !slices.Equal(got, expectedTeamTwo) {
		t.Fatalf("rosterUserIDs(assignedSession.Teams[1]) = %v, want %v", got, expectedTeamTwo)
	}

	startResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/start", assignedSession.ID), nil, ownerCookie)
	if startResponse.Code != http.StatusOK {
		t.Fatalf("startResponse.Code = %d, want %d body=%s", startResponse.Code, http.StatusOK, startResponse.Body.String())
	}
	startedSession := decodeCompetitionSession(t, startResponse)
	if startedSession.Status != competition.SessionStatusInProgress {
		t.Fatalf("startedSession.Status = %q, want %q", startedSession.Status, competition.SessionStatusInProgress)
	}
	if startedSession.Matches[0].Status != competition.MatchStatusInProgress {
		t.Fatalf("startedSession.Matches[0].Status = %q, want %q", startedSession.Matches[0].Status, competition.MatchStatusInProgress)
	}

	startedReadOne := env.doRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/competition/sessions/%s", startedSession.ID), nil, ownerCookie)
	if startedReadOne.Code != http.StatusOK {
		t.Fatalf("startedReadOne.Code = %d, want %d", startedReadOne.Code, http.StatusOK)
	}
	startedReadTwo := env.doRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/competition/sessions/%s", startedSession.ID), nil, ownerCookie)
	if startedReadTwo.Code != http.StatusOK {
		t.Fatalf("startedReadTwo.Code = %d, want %d", startedReadTwo.Code, http.StatusOK)
	}
	if startedReadOne.Body.String() != startedReadTwo.Body.String() {
		t.Fatalf("in-progress session detail changed between repeated reads\nfirst=%s\nsecond=%s", startedReadOne.Body.String(), startedReadTwo.Body.String())
	}

	archiveResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/archive", startedSession.ID), nil, ownerCookie)
	if archiveResponse.Code != http.StatusOK {
		t.Fatalf("archiveResponse.Code = %d, want %d body=%s", archiveResponse.Code, http.StatusOK, archiveResponse.Body.String())
	}
	archivedSession := decodeCompetitionSession(t, archiveResponse)
	if archivedSession.Status != competition.SessionStatusArchived {
		t.Fatalf("archivedSession.Status = %q, want %q", archivedSession.Status, competition.SessionStatusArchived)
	}
	if archivedSession.Matches[0].Status != competition.MatchStatusArchived {
		t.Fatalf("archivedSession.Matches[0].Status = %q, want %q", archivedSession.Matches[0].Status, competition.MatchStatusArchived)
	}
}

func TestCompetitionExecutionRuntimeRejectsStaleQueueStateReplayAndOwnershipFailures(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-neg-021", "competition-neg-021@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	strangerCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-competition-neg-022", "competition-neg-022@example.com")
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-competition-neg-023", "competition-neg-023@example.com")
	memberThreeCookie, memberThree := createVerifiedSessionViaHTTP(t, env, "student-competition-neg-024", "competition-neg-024@example.com")

	for _, cookie := range []*http.Cookie{ownerCookie, memberTwoCookie, memberThreeCookie} {
		makeEligibleForLobby(t, env, cookie)
	}
	joinLobbyMembership(t, env, ownerCookie)
	joinLobbyMembership(t, env, memberTwoCookie)

	createSessionResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Tracer 21 Singles Queue",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, ownerCookie)
	if createSessionResponse.Code != http.StatusCreated {
		t.Fatalf("createSessionResponse.Code = %d, want %d", createSessionResponse.Code, http.StatusCreated)
	}
	session := decodeCompetitionSession(t, createSessionResponse)
	session = openCompetitionQueue(t, env, ownerCookie, session.ID.String())

	unauthorizedQueueResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/queue/members", session.ID), fmt.Sprintf(`{"user_id":"%s"}`, owner.ID), strangerCookie)
	if unauthorizedQueueResponse.Code != http.StatusForbidden {
		t.Fatalf("unauthorizedQueueResponse.Code = %d, want %d", unauthorizedQueueResponse.Code, http.StatusForbidden)
	}

	startBeforeAssignResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/start", session.ID), nil, ownerCookie)
	if startBeforeAssignResponse.Code != http.StatusConflict {
		t.Fatalf("startBeforeAssignResponse.Code = %d, want %d", startBeforeAssignResponse.Code, http.StatusConflict)
	}

	notJoinedQueueResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/queue/members", session.ID), fmt.Sprintf(`{"user_id":"%s"}`, memberThree.ID), ownerCookie)
	if notJoinedQueueResponse.Code != http.StatusConflict {
		t.Fatalf("notJoinedQueueResponse.Code = %d, want %d", notJoinedQueueResponse.Code, http.StatusConflict)
	}

	session = queueCompetitionMember(t, env, ownerCookie, session.ID.String(), owner.ID)
	staleQueueVersion := session.QueueVersion

	duplicateQueueResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/queue/members", session.ID), fmt.Sprintf(`{"user_id":"%s"}`, owner.ID), ownerCookie)
	if duplicateQueueResponse.Code != http.StatusConflict {
		t.Fatalf("duplicateQueueResponse.Code = %d, want %d", duplicateQueueResponse.Code, http.StatusConflict)
	}

	session = queueCompetitionMember(t, env, ownerCookie, session.ID.String(), memberTwo.ID)

	joinLobbyMembership(t, env, memberThreeCookie)
	capacityResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/queue/members", session.ID), fmt.Sprintf(`{"user_id":"%s"}`, memberThree.ID), ownerCookie)
	if capacityResponse.Code != http.StatusConflict {
		t.Fatalf("capacityResponse.Code = %d, want %d", capacityResponse.Code, http.StatusConflict)
	}

	staleAssignResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/assignment", session.ID), fmt.Sprintf(`{
		"expected_queue_version":%d
	}`, staleQueueVersion), ownerCookie)
	if staleAssignResponse.Code != http.StatusConflict {
		t.Fatalf("staleAssignResponse.Code = %d, want %d", staleAssignResponse.Code, http.StatusConflict)
	}

	memberTwoLeaveResponse := env.doRequest(t, http.MethodPost, "/api/v1/lobby/membership/leave", nil, memberTwoCookie)
	if memberTwoLeaveResponse.Code != http.StatusOK {
		t.Fatalf("memberTwoLeaveResponse.Code = %d, want %d", memberTwoLeaveResponse.Code, http.StatusOK)
	}

	staleMemberAssignResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/assignment", session.ID), fmt.Sprintf(`{
		"expected_queue_version":%d
	}`, session.QueueVersion), ownerCookie)
	if staleMemberAssignResponse.Code != http.StatusConflict {
		t.Fatalf("staleMemberAssignResponse.Code = %d, want %d", staleMemberAssignResponse.Code, http.StatusConflict)
	}

	joinLobbyMembership(t, env, memberTwoCookie)
	assignResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/assignment", session.ID), fmt.Sprintf(`{
		"expected_queue_version":%d
	}`, session.QueueVersion), ownerCookie)
	if assignResponse.Code != http.StatusOK {
		t.Fatalf("assignResponse.Code = %d, want %d body=%s", assignResponse.Code, http.StatusOK, assignResponse.Body.String())
	}
	assignedSession := decodeCompetitionSession(t, assignResponse)

	replayAssignResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/assignment", session.ID), fmt.Sprintf(`{
		"expected_queue_version":%d
	}`, assignedSession.QueueVersion), ownerCookie)
	if replayAssignResponse.Code != http.StatusConflict {
		t.Fatalf("replayAssignResponse.Code = %d, want %d", replayAssignResponse.Code, http.StatusConflict)
	}

	startResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/start", assignedSession.ID), nil, ownerCookie)
	if startResponse.Code != http.StatusOK {
		t.Fatalf("startResponse.Code = %d, want %d", startResponse.Code, http.StatusOK)
	}
	replayStartResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/start", assignedSession.ID), nil, ownerCookie)
	if replayStartResponse.Code != http.StatusConflict {
		t.Fatalf("replayStartResponse.Code = %d, want %d", replayStartResponse.Code, http.StatusConflict)
	}

	strangerArchiveResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/archive", assignedSession.ID), nil, strangerCookie)
	if strangerArchiveResponse.Code != http.StatusForbidden {
		t.Fatalf("strangerArchiveResponse.Code = %d, want %d", strangerArchiveResponse.Code, http.StatusForbidden)
	}
}

func TestCompetitionExecutionRuntimeSupportsQueueRemoveOverHTTP(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-remove-031", "competition-remove-031@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-competition-remove-032", "competition-remove-032@example.com")

	for _, cookie := range []*http.Cookie{ownerCookie, memberTwoCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	createSessionResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Tracer 21 Queue Remove",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, ownerCookie)
	if createSessionResponse.Code != http.StatusCreated {
		t.Fatalf("createSessionResponse.Code = %d, want %d", createSessionResponse.Code, http.StatusCreated)
	}
	session := decodeCompetitionSession(t, createSessionResponse)

	session = openCompetitionQueue(t, env, ownerCookie, session.ID.String())
	session = queueCompetitionMember(t, env, ownerCookie, session.ID.String(), owner.ID)
	session = queueCompetitionMember(t, env, ownerCookie, session.ID.String(), memberTwo.ID)

	removeResponse := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/queue/members/%s/remove", session.ID, memberTwo.ID), nil, ownerCookie)
	if removeResponse.Code != http.StatusOK {
		t.Fatalf("removeResponse.Code = %d, want %d body=%s", removeResponse.Code, http.StatusOK, removeResponse.Body.String())
	}
	updatedSession := decodeCompetitionSession(t, removeResponse)
	if got, want := len(updatedSession.Queue), 1; got != want {
		t.Fatalf("len(updatedSession.Queue) = %d, want %d", got, want)
	}
	if updatedSession.Queue[0].UserID != owner.ID {
		t.Fatalf("updatedSession.Queue[0].UserID = %s, want %s", updatedSession.Queue[0].UserID, owner.ID)
	}
}

func openCompetitionQueue(t *testing.T, env *authProfileServerEnv, cookie *http.Cookie, sessionID string) competition.Session {
	t.Helper()

	response := env.doRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/queue/open", sessionID), nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("queue open response code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	return decodeCompetitionSession(t, response)
}

func queueCompetitionMember(t *testing.T, env *authProfileServerEnv, cookie *http.Cookie, sessionID string, userID uuid.UUID) competition.Session {
	t.Helper()

	response := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/queue/members", sessionID), fmt.Sprintf(`{
		"user_id":"%s"
	}`, userID), cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("queue member response code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	return decodeCompetitionSession(t, response)
}

func rosterUserIDs(team competition.Team) []uuid.UUID {
	memberIDs := make([]uuid.UUID, 0, len(team.Roster))
	for _, member := range team.Roster {
		memberIDs = append(memberIDs, member.UserID)
	}

	return memberIDs
}
