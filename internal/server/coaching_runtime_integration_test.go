package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/coaching"
	"github.com/ixxet/apollo/internal/planner"
)

func TestCoachingRuntimeColdStartIsConservativeDeterministicAndReadOnly(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, _ := createVerifiedSessionViaHTTP(t, env, "student-coaching-010", "coaching-010@example.com")

	profilePatchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"coaching_profile":{"goal_key":"build-strength","days_per_week":4,"session_minutes":300}}`, cookie)
	if profilePatchResponse.Code != http.StatusOK {
		t.Fatalf("profilePatchResponse.Code = %d, want %d", profilePatchResponse.Code, http.StatusOK)
	}

	putWeekResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/planner/weeks/2026-04-06", `{"sessions":[{"day_index":0,"items":[{"exercise_key":"barbell-back-squat","equipment_key":"barbell","sets":5,"reps":5,"weight_kg":100},{"exercise_key":"push-up","sets":3,"reps":12}]}]}`, cookie)
	if putWeekResponse.Code != http.StatusOK {
		t.Fatalf("putWeekResponse.Code = %d, want %d", putWeekResponse.Code, http.StatusOK)
	}
	weekBefore := decodeWeekResponse(t, putWeekResponse)

	firstRecommendationResponse := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/coaching?week_start=2026-04-06", nil, cookie)
	if firstRecommendationResponse.Code != http.StatusOK {
		t.Fatalf("firstRecommendationResponse.Code = %d, want %d", firstRecommendationResponse.Code, http.StatusOK)
	}
	firstRecommendation := decodeCoachingRecommendationResponse(t, firstRecommendationResponse)
	if firstRecommendation.Kind != coaching.KindStartConservative {
		t.Fatalf("firstRecommendation.Kind = %q, want %q", firstRecommendation.Kind, coaching.KindStartConservative)
	}
	if firstRecommendation.TrainingGoal.TargetSessionMinutes != 120 {
		t.Fatalf("firstRecommendation.TrainingGoal.TargetSessionMinutes = %d, want 120", firstRecommendation.TrainingGoal.TargetSessionMinutes)
	}
	if !contains(firstRecommendation.Explanation.ReasonCodes, "experience_level_defaulted_beginner") {
		t.Fatalf("ReasonCodes = %#v, want experience_level_defaulted_beginner", firstRecommendation.Explanation.ReasonCodes)
	}
	if !contains(firstRecommendation.Explanation.ReasonCodes, "no_finished_workout") {
		t.Fatalf("ReasonCodes = %#v, want no_finished_workout", firstRecommendation.Explanation.ReasonCodes)
	}
	if len(firstRecommendation.Proposal.Changes) == 0 {
		t.Fatalf("len(firstRecommendation.Proposal.Changes) = %d, want > 0", len(firstRecommendation.Proposal.Changes))
	}

	secondRecommendationResponse := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/coaching?week_start=2026-04-06", nil, cookie)
	if secondRecommendationResponse.Code != http.StatusOK {
		t.Fatalf("secondRecommendationResponse.Code = %d, want %d", secondRecommendationResponse.Code, http.StatusOK)
	}
	secondRecommendation := decodeCoachingRecommendationResponse(t, secondRecommendationResponse)

	if !reflect.DeepEqual(normalizeRecommendation(firstRecommendation), normalizeRecommendation(secondRecommendation)) {
		t.Fatalf("recommendations changed across rerun:\nfirst=%#v\nsecond=%#v", firstRecommendation, secondRecommendation)
	}

	weekAfterResponse := env.doRequest(t, http.MethodGet, "/api/v1/planner/weeks/2026-04-06", nil, cookie)
	if weekAfterResponse.Code != http.StatusOK {
		t.Fatalf("weekAfterResponse.Code = %d, want %d", weekAfterResponse.Code, http.StatusOK)
	}
	weekAfter := decodeWeekResponse(t, weekAfterResponse)
	if !reflect.DeepEqual(weekBefore, weekAfter) {
		t.Fatalf("planner week mutated by coaching read:\nbefore=%#v\nafter=%#v", weekBefore, weekAfter)
	}
}

func TestCoachingRuntimeFeedbackDrivesBoundedProgressAndRegression(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, _ := createVerifiedSessionViaHTTP(t, env, "student-coaching-011", "coaching-011@example.com")

	profilePatchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"coaching_profile":{"goal_key":"build-strength","days_per_week":4,"session_minutes":60,"experience_level":"intermediate"}}`, cookie)
	if profilePatchResponse.Code != http.StatusOK {
		t.Fatalf("profilePatchResponse.Code = %d, want %d", profilePatchResponse.Code, http.StatusOK)
	}

	putWeekResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/planner/weeks/2026-04-06", `{"sessions":[{"day_index":1,"items":[{"exercise_key":"barbell-back-squat","equipment_key":"barbell","sets":5,"reps":5,"weight_kg":100},{"exercise_key":"push-up","sets":3,"reps":12}]}]}`, cookie)
	if putWeekResponse.Code != http.StatusOK {
		t.Fatalf("putWeekResponse.Code = %d, want %d", putWeekResponse.Code, http.StatusOK)
	}

	createWorkoutResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/workouts", `{"notes":"coaching feedback runtime"}`, cookie)
	if createWorkoutResponse.Code != http.StatusCreated {
		t.Fatalf("createWorkoutResponse.Code = %d, want %d", createWorkoutResponse.Code, http.StatusCreated)
	}
	createdWorkout := decodeWorkoutResponse(t, createWorkoutResponse)

	updateWorkoutResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+createdWorkout.ID.String(), `{"exercises":[{"name":"back squat","sets":5,"reps":5,"weight_kg":100}]}`, cookie)
	if updateWorkoutResponse.Code != http.StatusOK {
		t.Fatalf("updateWorkoutResponse.Code = %d, want %d", updateWorkoutResponse.Code, http.StatusOK)
	}
	finishWorkoutResponse := env.doRequest(t, http.MethodPost, "/api/v1/workouts/"+createdWorkout.ID.String()+"/finish", nil, cookie)
	if finishWorkoutResponse.Code != http.StatusOK {
		t.Fatalf("finishWorkoutResponse.Code = %d, want %d", finishWorkoutResponse.Code, http.StatusOK)
	}

	effortResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+createdWorkout.ID.String()+"/effort-feedback", `{"effort_level":"easy"}`, cookie)
	if effortResponse.Code != http.StatusOK {
		t.Fatalf("effortResponse.Code = %d, want %d", effortResponse.Code, http.StatusOK)
	}
	recoveryResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+createdWorkout.ID.String()+"/recovery-feedback", `{"recovery_level":"recovered"}`, cookie)
	if recoveryResponse.Code != http.StatusOK {
		t.Fatalf("recoveryResponse.Code = %d, want %d", recoveryResponse.Code, http.StatusOK)
	}

	progressRecommendationResponse := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/coaching?week_start=2026-04-06", nil, cookie)
	if progressRecommendationResponse.Code != http.StatusOK {
		t.Fatalf("progressRecommendationResponse.Code = %d, want %d", progressRecommendationResponse.Code, http.StatusOK)
	}
	progressRecommendation := decodeCoachingRecommendationResponse(t, progressRecommendationResponse)
	if progressRecommendation.Kind != coaching.KindProgress {
		t.Fatalf("progressRecommendation.Kind = %q, want %q", progressRecommendation.Kind, coaching.KindProgress)
	}
	if progressRecommendation.SourceWorkoutID == nil || *progressRecommendation.SourceWorkoutID != createdWorkout.ID {
		t.Fatalf("progressRecommendation.SourceWorkoutID = %#v, want %s", progressRecommendation.SourceWorkoutID, createdWorkout.ID)
	}

	effortRegressResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+createdWorkout.ID.String()+"/effort-feedback", `{"effort_level":"maxed"}`, cookie)
	if effortRegressResponse.Code != http.StatusOK {
		t.Fatalf("effortRegressResponse.Code = %d, want %d", effortRegressResponse.Code, http.StatusOK)
	}
	regressRecommendationResponse := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/coaching?week_start=2026-04-06", nil, cookie)
	if regressRecommendationResponse.Code != http.StatusOK {
		t.Fatalf("regressRecommendationResponse.Code = %d, want %d", regressRecommendationResponse.Code, http.StatusOK)
	}
	regressRecommendation := decodeCoachingRecommendationResponse(t, regressRecommendationResponse)
	if regressRecommendation.Kind != coaching.KindRegress {
		t.Fatalf("regressRecommendation.Kind = %q, want %q", regressRecommendation.Kind, coaching.KindRegress)
	}
}

func decodeCoachingRecommendationResponse(t *testing.T, response *httptest.ResponseRecorder) coaching.CoachingRecommendation {
	t.Helper()

	var recommendation coaching.CoachingRecommendation
	if err := json.Unmarshal(response.Body.Bytes(), &recommendation); err != nil {
		t.Fatalf("json.Unmarshal(coaching recommendation) error = %v", err)
	}
	return recommendation
}

func normalizeRecommendation(input coaching.CoachingRecommendation) coaching.CoachingRecommendation {
	result := input
	result.GeneratedAt = time.Time{}
	return result
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

var _ = planner.Week{}
