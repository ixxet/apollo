package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLobbyMembershipRuntimeSupportsExplicitJoinLeaveRoundTrip(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-membership-012", "membership-012@example.com")

	initialMembershipResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/membership", nil, cookie)
	if initialMembershipResponse.Code != http.StatusOK {
		t.Fatalf("initialMembershipResponse.Code = %d, want %d", initialMembershipResponse.Code, http.StatusOK)
	}
	initialMembership := decodeMembershipResponse(t, initialMembershipResponse)
	if initialMembership.Status != "not_joined" {
		t.Fatalf("initialMembership.Status = %q, want %q", initialMembership.Status, "not_joined")
	}

	patchEligibilityResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"discoverable","availability_mode":"available_now"}`, cookie)
	if patchEligibilityResponse.Code != http.StatusOK {
		t.Fatalf("patchEligibilityResponse.Code = %d, want %d", patchEligibilityResponse.Code, http.StatusOK)
	}

	joinResponse := env.doRequest(t, http.MethodPost, "/api/v1/lobby/membership/join", nil, cookie)
	if joinResponse.Code != http.StatusOK {
		t.Fatalf("joinResponse.Code = %d, want %d", joinResponse.Code, http.StatusOK)
	}
	joinedMembership := decodeMembershipResponse(t, joinResponse)
	if joinedMembership.Status != "joined" {
		t.Fatalf("joinedMembership.Status = %q, want %q", joinedMembership.Status, "joined")
	}
	if joinedMembership.JoinedAt == nil {
		t.Fatal("joinedMembership.JoinedAt = nil, want timestamp")
	}
	if joinedMembership.LeftAt != nil {
		t.Fatalf("joinedMembership.LeftAt = %#v, want nil", joinedMembership.LeftAt)
	}

	readJoinedResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/membership", nil, cookie)
	if readJoinedResponse.Code != http.StatusOK {
		t.Fatalf("readJoinedResponse.Code = %d, want %d", readJoinedResponse.Code, http.StatusOK)
	}
	readJoinedMembership := decodeMembershipResponse(t, readJoinedResponse)
	if readJoinedMembership.Status != "joined" {
		t.Fatalf("readJoinedMembership.Status = %q, want %q", readJoinedMembership.Status, "joined")
	}

	leaveResponse := env.doRequest(t, http.MethodPost, "/api/v1/lobby/membership/leave", nil, cookie)
	if leaveResponse.Code != http.StatusOK {
		t.Fatalf("leaveResponse.Code = %d, want %d", leaveResponse.Code, http.StatusOK)
	}
	leftMembership := decodeMembershipResponse(t, leaveResponse)
	if leftMembership.Status != "not_joined" {
		t.Fatalf("leftMembership.Status = %q, want %q", leftMembership.Status, "not_joined")
	}
	if leftMembership.JoinedAt == nil {
		t.Fatal("leftMembership.JoinedAt = nil, want original joined timestamp")
	}
	if leftMembership.LeftAt == nil {
		t.Fatal("leftMembership.LeftAt = nil, want leave timestamp")
	}

	finalReadResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/membership", nil, cookie)
	if finalReadResponse.Code != http.StatusOK {
		t.Fatalf("finalReadResponse.Code = %d, want %d", finalReadResponse.Code, http.StatusOK)
	}
	finalMembership := decodeMembershipResponse(t, finalReadResponse)
	if finalMembership.Status != "not_joined" {
		t.Fatalf("finalMembership.Status = %q, want %q", finalMembership.Status, "not_joined")
	}
	if finalMembership.LeftAt == nil {
		t.Fatal("finalMembership.LeftAt = nil, want persisted leave timestamp")
	}

	var persistedStatus string
	var persistedJoinedAt time.Time
	var persistedLeftAt time.Time
	if err := env.db.DB.QueryRow(
		context.Background(),
		"SELECT status, joined_at, left_at FROM apollo.lobby_memberships WHERE user_id = $1",
		user.ID,
	).Scan(&persistedStatus, &persistedJoinedAt, &persistedLeftAt); err != nil {
		t.Fatalf("QueryRow(lobby_memberships) error = %v", err)
	}
	if persistedStatus != "not_joined" {
		t.Fatalf("persistedStatus = %q, want %q", persistedStatus, "not_joined")
	}
	if !persistedJoinedAt.Equal(joinedMembership.JoinedAt.UTC()) {
		t.Fatalf("persistedJoinedAt = %s, want %s", persistedJoinedAt, joinedMembership.JoinedAt.UTC())
	}
	if !persistedLeftAt.Equal(leftMembership.LeftAt.UTC()) {
		t.Fatalf("persistedLeftAt = %s, want %s", persistedLeftAt, leftMembership.LeftAt.UTC())
	}
}

func TestLobbyMembershipRuntimeRejectsIneligibleAndRepeatedTransitionsDeterministically(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-membership-013", "membership-013@example.com")

	ineligibleJoinResponse := env.doRequest(t, http.MethodPost, "/api/v1/lobby/membership/join", nil, cookie)
	if ineligibleJoinResponse.Code != http.StatusConflict {
		t.Fatalf("ineligibleJoinResponse.Code = %d, want %d", ineligibleJoinResponse.Code, http.StatusConflict)
	}

	if membershipRows := countRows(t, env, "apollo.lobby_memberships", user.ID); membershipRows != 0 {
		t.Fatalf("membershipRows = %d, want 0 before explicit eligible join", membershipRows)
	}

	patchEligibilityResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"discoverable","availability_mode":"available_now"}`, cookie)
	if patchEligibilityResponse.Code != http.StatusOK {
		t.Fatalf("patchEligibilityResponse.Code = %d, want %d", patchEligibilityResponse.Code, http.StatusOK)
	}

	firstJoinResponse := env.doRequest(t, http.MethodPost, "/api/v1/lobby/membership/join", nil, cookie)
	if firstJoinResponse.Code != http.StatusOK {
		t.Fatalf("firstJoinResponse.Code = %d, want %d", firstJoinResponse.Code, http.StatusOK)
	}

	repeatedJoinResponse := env.doRequest(t, http.MethodPost, "/api/v1/lobby/membership/join", nil, cookie)
	if repeatedJoinResponse.Code != http.StatusConflict {
		t.Fatalf("repeatedJoinResponse.Code = %d, want %d", repeatedJoinResponse.Code, http.StatusConflict)
	}

	firstLeaveResponse := env.doRequest(t, http.MethodPost, "/api/v1/lobby/membership/leave", nil, cookie)
	if firstLeaveResponse.Code != http.StatusOK {
		t.Fatalf("firstLeaveResponse.Code = %d, want %d", firstLeaveResponse.Code, http.StatusOK)
	}

	repeatedLeaveResponse := env.doRequest(t, http.MethodPost, "/api/v1/lobby/membership/leave", nil, cookie)
	if repeatedLeaveResponse.Code != http.StatusConflict {
		t.Fatalf("repeatedLeaveResponse.Code = %d, want %d", repeatedLeaveResponse.Code, http.StatusConflict)
	}

	if membershipRows := countRows(t, env, "apollo.lobby_memberships", user.ID); membershipRows != 1 {
		t.Fatalf("membershipRows = %d, want 1 after deterministic join/leave cycle", membershipRows)
	}
}

func decodeMembershipResponse(t *testing.T, response *httptest.ResponseRecorder) membershipPayload {
	t.Helper()

	var payload membershipPayload
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(membership) error = %v", err)
	}

	return payload
}

type membershipPayload struct {
	Status   string     `json:"status"`
	JoinedAt *time.Time `json:"joined_at"`
	LeftAt   *time.Time `json:"left_at"`
}
