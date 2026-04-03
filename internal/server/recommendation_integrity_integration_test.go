package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/eligibility"
	"github.com/ixxet/apollo/internal/profile"
	"github.com/ixxet/apollo/internal/recommendations"
)

func TestWorkoutRecommendationEndpointStaysSideEffectFreeAcrossVisitsEligibilityAndClaimedTags(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-recommendation-020", "recommendation-020@example.com")

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", user.ID, "tag-recommendation-020", "locker tag"); err != nil {
		t.Fatalf("Exec(insert claimed tag) error = %v", err)
	}
	arrivedAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.visits (user_id, facility_key, source_event_id, arrived_at) VALUES ($1, $2, $3, $4)", user.ID, "ashtonbee", "visit-recommendation-020", arrivedAt); err != nil {
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

	recommendationResponse := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/workout", nil, cookie)
	if recommendationResponse.Code != http.StatusOK {
		t.Fatalf("recommendationResponse.Code = %d, want %d", recommendationResponse.Code, http.StatusOK)
	}
	recommendation := decodeWorkoutRecommendationResponse(t, recommendationResponse)
	assertWorkoutRecommendation(t, recommendation, recommendations.TypeStartFirstWorkout, recommendations.ReasonNoFinishedWorkouts, nil)

	if afterVisits := countRows(t, env, "apollo.visits", user.ID); afterVisits != beforeVisits {
		t.Fatalf("visit count changed from %d to %d after recommendation read", beforeVisits, afterVisits)
	}
	if afterWorkouts := countRows(t, env, "apollo.workouts", user.ID); afterWorkouts != beforeWorkouts {
		t.Fatalf("workout count changed from %d to %d after recommendation read", beforeWorkouts, afterWorkouts)
	}
	if afterClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID); afterClaimedTags != beforeClaimedTags {
		t.Fatalf("claimed tag count changed from %d to %d after recommendation read", beforeClaimedTags, afterClaimedTags)
	}
	if tagHashAfter := lookupTagHash(t, env, user.ID); tagHashAfter != tagHashBefore {
		t.Fatalf("tag hash changed from %q to %q after recommendation read", tagHashBefore, tagHashAfter)
	}

	var departedAt *time.Time
	if err := env.db.DB.QueryRow(context.Background(), "SELECT departed_at FROM apollo.visits WHERE user_id = $1 AND source_event_id = $2", user.ID, "visit-recommendation-020").Scan(&departedAt); err != nil {
		t.Fatalf("QueryRow(departed_at) error = %v", err)
	}
	if departedAt != nil {
		t.Fatalf("departedAt = %v, want nil after recommendation read", departedAt)
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
