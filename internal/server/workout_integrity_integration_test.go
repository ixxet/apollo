package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/eligibility"
	"github.com/ixxet/apollo/internal/profile"
	"github.com/ixxet/apollo/internal/workouts"
)

func TestWorkoutRuntimeEndpointsStaySideEffectFreeAcrossVisitsEligibilityAndClaimedTags(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-workout-020", "workout-020@example.com")

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", user.ID, "tag-workout-020", "locker tag"); err != nil {
		t.Fatalf("Exec(insert claimed tag) error = %v", err)
	}
	arrivedAt := time.Date(2026, 4, 2, 18, 0, 0, 0, time.UTC)
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.visits (user_id, facility_key, source_event_id, arrived_at) VALUES ($1, $2, $3, $4)", user.ID, "ashtonbee", "visit-workout-020", arrivedAt); err != nil {
		t.Fatalf("Exec(insert visit) error = %v", err)
	}

	profilePatchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"discoverable","availability_mode":"available_now"}`, cookie)
	if profilePatchResponse.Code != http.StatusOK {
		t.Fatalf("profilePatchResponse.Code = %d, want %d", profilePatchResponse.Code, http.StatusOK)
	}

	beforeVisits := countRows(t, env, "apollo.visits", user.ID)
	beforeWorkouts := countRows(t, env, "apollo.workouts", user.ID)
	beforeClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID)
	tagHashBefore := lookupTagHash(t, env, user.ID)
	initialEligibilityResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, cookie)
	if initialEligibilityResponse.Code != http.StatusOK {
		t.Fatalf("initialEligibilityResponse.Code = %d, want %d", initialEligibilityResponse.Code, http.StatusOK)
	}
	initialEligibility := decodeEligibilityResponse(t, initialEligibilityResponse)
	assertEligibility(t, initialEligibility, true, eligibility.ReasonEligible, profile.VisibilityModeDiscoverable, profile.AvailabilityModeAvailableNow)

	createResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/workouts", `{"notes":"conditioning"}`, cookie)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("createResponse.Code = %d, want %d", createResponse.Code, http.StatusCreated)
	}
	createdWorkout := decodeWorkoutResponse(t, createResponse)

	updateResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+createdWorkout.ID.String(), `{"notes":"conditioning","exercises":[{"name":"bike sprint","sets":6,"reps":30}]}`, cookie)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("updateResponse.Code = %d, want %d", updateResponse.Code, http.StatusOK)
	}

	finishResponse := env.doRequest(t, http.MethodPost, "/api/v1/workouts/"+createdWorkout.ID.String()+"/finish", nil, cookie)
	if finishResponse.Code != http.StatusOK {
		t.Fatalf("finishResponse.Code = %d, want %d", finishResponse.Code, http.StatusOK)
	}
	finishedWorkout := decodeWorkoutResponse(t, finishResponse)
	if finishedWorkout.Status != workouts.StatusFinished {
		t.Fatalf("finishedWorkout.Status = %q, want %q", finishedWorkout.Status, workouts.StatusFinished)
	}

	if afterVisits := countRows(t, env, "apollo.visits", user.ID); afterVisits != beforeVisits {
		t.Fatalf("visit count changed from %d to %d after workout runtime", beforeVisits, afterVisits)
	}
	if afterWorkouts := countRows(t, env, "apollo.workouts", user.ID); afterWorkouts != beforeWorkouts+1 {
		t.Fatalf("workout count changed from %d to %d after workout runtime, want %d", beforeWorkouts, afterWorkouts, beforeWorkouts+1)
	}
	if afterClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID); afterClaimedTags != beforeClaimedTags {
		t.Fatalf("claimed tag count changed from %d to %d after workout runtime", beforeClaimedTags, afterClaimedTags)
	}
	if tagHashAfter := lookupTagHash(t, env, user.ID); tagHashAfter != tagHashBefore {
		t.Fatalf("tag hash changed from %q to %q after workout runtime", tagHashBefore, tagHashAfter)
	}

	var departedAt *time.Time
	if err := env.db.DB.QueryRow(context.Background(), "SELECT departed_at FROM apollo.visits WHERE user_id = $1 AND source_event_id = $2", user.ID, "visit-workout-020").Scan(&departedAt); err != nil {
		t.Fatalf("QueryRow(departed_at) error = %v", err)
	}
	if departedAt != nil {
		t.Fatalf("departedAt = %v, want nil after workout runtime", departedAt)
	}

	profileResponse := env.doRequest(t, http.MethodGet, "/api/v1/profile", nil, cookie)
	if profileResponse.Code != http.StatusOK {
		t.Fatalf("profileResponse.Code = %d, want %d", profileResponse.Code, http.StatusOK)
	}
	memberProfile := decodeProfileResponse(t, profileResponse)
	if memberProfile.VisibilityMode != profile.VisibilityModeDiscoverable || memberProfile.AvailabilityMode != profile.AvailabilityModeAvailableNow {
		t.Fatalf("memberProfile = %#v, want discoverable + available_now", memberProfile)
	}

	finalEligibilityResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, cookie)
	if finalEligibilityResponse.Code != http.StatusOK {
		t.Fatalf("finalEligibilityResponse.Code = %d, want %d", finalEligibilityResponse.Code, http.StatusOK)
	}
	finalEligibility := decodeEligibilityResponse(t, finalEligibilityResponse)
	assertEligibility(t, finalEligibility, true, eligibility.ReasonEligible, profile.VisibilityModeDiscoverable, profile.AvailabilityModeAvailableNow)
}
