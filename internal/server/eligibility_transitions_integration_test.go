package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/eligibility"
	"github.com/ixxet/apollo/internal/profile"
)

func TestLobbyEligibilityRoundTripFollowsExplicitProfileStateInsteadOfVisitHistory(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-eligibility-001", "eligibility-001@example.com")

	initialResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, cookie)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("initialResponse.Code = %d, want %d", initialResponse.Code, http.StatusOK)
	}
	initialEligibility := decodeEligibilityResponse(t, initialResponse)
	assertEligibility(t, initialEligibility, false, eligibility.ReasonAvailabilityUnavailable, profile.VisibilityModeGhost, profile.AvailabilityModeUnavailable)

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.visits (user_id, facility_key, source_event_id, arrived_at) VALUES ($1, $2, $3, $4)", user.ID, "ashtonbee", "visit-eligibility-001", time.Date(2026, 4, 2, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(insert visit) error = %v", err)
	}

	postVisitResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, cookie)
	if postVisitResponse.Code != http.StatusOK {
		t.Fatalf("postVisitResponse.Code = %d, want %d", postVisitResponse.Code, http.StatusOK)
	}
	postVisitEligibility := decodeEligibilityResponse(t, postVisitResponse)
	assertEligibility(t, postVisitEligibility, false, eligibility.ReasonAvailabilityUnavailable, profile.VisibilityModeGhost, profile.AvailabilityModeUnavailable)

	availabilityPatchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"availability_mode":"available_now"}`, cookie)
	if availabilityPatchResponse.Code != http.StatusOK {
		t.Fatalf("availabilityPatchResponse.Code = %d, want %d", availabilityPatchResponse.Code, http.StatusOK)
	}
	postAvailabilityResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, cookie)
	if postAvailabilityResponse.Code != http.StatusOK {
		t.Fatalf("postAvailabilityResponse.Code = %d, want %d", postAvailabilityResponse.Code, http.StatusOK)
	}
	postAvailabilityEligibility := decodeEligibilityResponse(t, postAvailabilityResponse)
	assertEligibility(t, postAvailabilityEligibility, false, eligibility.ReasonVisibilityGhost, profile.VisibilityModeGhost, profile.AvailabilityModeAvailableNow)

	visibilityPatchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"discoverable"}`, cookie)
	if visibilityPatchResponse.Code != http.StatusOK {
		t.Fatalf("visibilityPatchResponse.Code = %d, want %d", visibilityPatchResponse.Code, http.StatusOK)
	}
	postVisibilityResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, cookie)
	if postVisibilityResponse.Code != http.StatusOK {
		t.Fatalf("postVisibilityResponse.Code = %d, want %d", postVisibilityResponse.Code, http.StatusOK)
	}
	postVisibilityEligibility := decodeEligibilityResponse(t, postVisibilityResponse)
	assertEligibility(t, postVisibilityEligibility, true, eligibility.ReasonEligible, profile.VisibilityModeDiscoverable, profile.AvailabilityModeAvailableNow)

	withTeamPatchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"availability_mode":"with_team"}`, cookie)
	if withTeamPatchResponse.Code != http.StatusOK {
		t.Fatalf("withTeamPatchResponse.Code = %d, want %d", withTeamPatchResponse.Code, http.StatusOK)
	}
	postWithTeamResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, cookie)
	if postWithTeamResponse.Code != http.StatusOK {
		t.Fatalf("postWithTeamResponse.Code = %d, want %d", postWithTeamResponse.Code, http.StatusOK)
	}
	postWithTeamEligibility := decodeEligibilityResponse(t, postWithTeamResponse)
	assertEligibility(t, postWithTeamEligibility, false, eligibility.ReasonAvailabilityWithTeam, profile.VisibilityModeDiscoverable, profile.AvailabilityModeWithTeam)

	ghostPatchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"ghost","availability_mode":"available_now"}`, cookie)
	if ghostPatchResponse.Code != http.StatusOK {
		t.Fatalf("ghostPatchResponse.Code = %d, want %d", ghostPatchResponse.Code, http.StatusOK)
	}
	postGhostResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, cookie)
	if postGhostResponse.Code != http.StatusOK {
		t.Fatalf("postGhostResponse.Code = %d, want %d", postGhostResponse.Code, http.StatusOK)
	}
	postGhostEligibility := decodeEligibilityResponse(t, postGhostResponse)
	assertEligibility(t, postGhostEligibility, false, eligibility.ReasonVisibilityGhost, profile.VisibilityModeGhost, profile.AvailabilityModeAvailableNow)
}

func TestLobbyEligibilityEndpointDoesNotRequireVisitHistoryWhenExplicitStateMakesMemberEligible(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-eligibility-002", "eligibility-002@example.com")
	if visits := countRows(t, env, "apollo.visits", user.ID); visits != 0 {
		t.Fatalf("visit count = %d, want 0", visits)
	}

	patchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"discoverable","availability_mode":"available_now"}`, cookie)
	if patchResponse.Code != http.StatusOK {
		t.Fatalf("patchResponse.Code = %d, want %d", patchResponse.Code, http.StatusOK)
	}

	eligibilityResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, cookie)
	if eligibilityResponse.Code != http.StatusOK {
		t.Fatalf("eligibilityResponse.Code = %d, want %d", eligibilityResponse.Code, http.StatusOK)
	}

	memberEligibility := decodeEligibilityResponse(t, eligibilityResponse)
	assertEligibility(t, memberEligibility, true, eligibility.ReasonEligible, profile.VisibilityModeDiscoverable, profile.AvailabilityModeAvailableNow)
}
