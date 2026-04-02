package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/eligibility"
	"github.com/ixxet/apollo/internal/profile"
)

func TestLobbyEligibilityEndpointIsSideEffectFreeAcrossVisitsWorkoutsAndClaimedTags(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-eligibility-004", "eligibility-004@example.com")

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", user.ID, "tag-eligibility-001", "locker tag"); err != nil {
		t.Fatalf("Exec(insert claimed tag) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.visits (user_id, facility_key, source_event_id, arrived_at) VALUES ($1, $2, $3, $4)", user.ID, "ashtonbee", "visit-eligibility-002", time.Date(2026, 4, 2, 15, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(insert visit) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.workouts (user_id, logged_at) VALUES ($1, $2)", user.ID, time.Date(2026, 4, 2, 15, 5, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(insert workout) error = %v", err)
	}

	patchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"discoverable","availability_mode":"available_now"}`, cookie)
	if patchResponse.Code != http.StatusOK {
		t.Fatalf("patchResponse.Code = %d, want %d", patchResponse.Code, http.StatusOK)
	}

	beforeVisits := countRows(t, env, "apollo.visits", user.ID)
	beforeWorkouts := countRows(t, env, "apollo.workouts", user.ID)
	beforeClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID)
	tagHashBefore := lookupTagHash(t, env, user.ID)

	firstEligibilityResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, cookie)
	if firstEligibilityResponse.Code != http.StatusOK {
		t.Fatalf("firstEligibilityResponse.Code = %d, want %d", firstEligibilityResponse.Code, http.StatusOK)
	}
	secondEligibilityResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, cookie)
	if secondEligibilityResponse.Code != http.StatusOK {
		t.Fatalf("secondEligibilityResponse.Code = %d, want %d", secondEligibilityResponse.Code, http.StatusOK)
	}

	firstEligibility := decodeEligibilityResponse(t, firstEligibilityResponse)
	secondEligibility := decodeEligibilityResponse(t, secondEligibilityResponse)
	assertEligibility(t, firstEligibility, true, eligibility.ReasonEligible, profile.VisibilityModeDiscoverable, profile.AvailabilityModeAvailableNow)
	assertEligibility(t, secondEligibility, true, eligibility.ReasonEligible, profile.VisibilityModeDiscoverable, profile.AvailabilityModeAvailableNow)

	if afterVisits := countRows(t, env, "apollo.visits", user.ID); afterVisits != beforeVisits {
		t.Fatalf("visit count changed from %d to %d after eligibility reads", beforeVisits, afterVisits)
	}
	if afterWorkouts := countRows(t, env, "apollo.workouts", user.ID); afterWorkouts != beforeWorkouts {
		t.Fatalf("workout count changed from %d to %d after eligibility reads", beforeWorkouts, afterWorkouts)
	}
	if afterClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID); afterClaimedTags != beforeClaimedTags {
		t.Fatalf("claimed tag count changed from %d to %d after eligibility reads", beforeClaimedTags, afterClaimedTags)
	}
	if tagHashAfter := lookupTagHash(t, env, user.ID); tagHashAfter != tagHashBefore {
		t.Fatalf("tag hash changed from %q to %q after eligibility reads", tagHashBefore, tagHashAfter)
	}
}

func TestLobbyEligibilityEndpointSurfacesInvalidPersistedStatesDeterministically(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	testCases := []struct {
		name                 string
		studentID            string
		email                string
		preferences          string
		wantReason           string
		wantVisibilityMode   string
		wantAvailabilityMode string
	}{
		{
			name:                 "invalid visibility value stays deterministic",
			studentID:            "student-eligibility-invalid-visibility",
			email:                "eligibility-invalid-visibility@example.com",
			preferences:          `{"visibility_mode":"hidden","availability_mode":"available_now"}`,
			wantReason:           eligibility.ReasonInvalidVisibilityMode,
			wantVisibilityMode:   profile.VisibilityModeGhost,
			wantAvailabilityMode: profile.AvailabilityModeAvailableNow,
		},
		{
			name:                 "invalid availability value stays deterministic",
			studentID:            "student-eligibility-invalid-availability",
			email:                "eligibility-invalid-availability@example.com",
			preferences:          `{"visibility_mode":"discoverable","availability_mode":"queued"}`,
			wantReason:           eligibility.ReasonInvalidAvailabilityMode,
			wantVisibilityMode:   profile.VisibilityModeDiscoverable,
			wantAvailabilityMode: profile.AvailabilityModeUnavailable,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cookie, user := createVerifiedSessionViaHTTP(t, env, testCase.studentID, testCase.email)
			if _, err := env.db.DB.Exec(context.Background(), "UPDATE apollo.users SET preferences = $2 WHERE id = $1", user.ID, testCase.preferences); err != nil {
				t.Fatalf("Exec(update preferences) error = %v", err)
			}

			response := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, cookie)
			if response.Code != http.StatusOK {
				t.Fatalf("response.Code = %d, want %d", response.Code, http.StatusOK)
			}

			memberEligibility := decodeEligibilityResponse(t, response)
			assertEligibility(t, memberEligibility, false, testCase.wantReason, testCase.wantVisibilityMode, testCase.wantAvailabilityMode)
		})
	}
}
