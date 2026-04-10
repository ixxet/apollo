package server

import (
	"context"
	"net/http"
	"testing"
)

func TestCoachingRuntimeRejectsInvalidPayloadsAndWrongOwnerMutations(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	firstCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-coaching-int-001", "coaching-int-001@example.com")
	secondCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-coaching-int-002", "coaching-int-002@example.com")

	firstCreateResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/workouts", `{"notes":"integrity workout"}`, firstCookie)
	if firstCreateResponse.Code != http.StatusCreated {
		t.Fatalf("firstCreateResponse.Code = %d, want %d", firstCreateResponse.Code, http.StatusCreated)
	}
	firstWorkout := decodeWorkoutResponse(t, firstCreateResponse)

	firstUpdateResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+firstWorkout.ID.String(), `{"exercises":[{"name":"bench","sets":3,"reps":8}]}`, firstCookie)
	if firstUpdateResponse.Code != http.StatusOK {
		t.Fatalf("firstUpdateResponse.Code = %d, want %d", firstUpdateResponse.Code, http.StatusOK)
	}
	firstFinishResponse := env.doRequest(t, http.MethodPost, "/api/v1/workouts/"+firstWorkout.ID.String()+"/finish", nil, firstCookie)
	if firstFinishResponse.Code != http.StatusOK {
		t.Fatalf("firstFinishResponse.Code = %d, want %d", firstFinishResponse.Code, http.StatusOK)
	}

	wrongOwnerFeedbackResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+firstWorkout.ID.String()+"/effort-feedback", `{"effort_level":"easy"}`, secondCookie)
	if wrongOwnerFeedbackResponse.Code != http.StatusNotFound {
		t.Fatalf("wrongOwnerFeedbackResponse.Code = %d, want %d", wrongOwnerFeedbackResponse.Code, http.StatusNotFound)
	}

	secondCreateResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/workouts", `{"notes":"still in progress"}`, secondCookie)
	if secondCreateResponse.Code != http.StatusCreated {
		t.Fatalf("secondCreateResponse.Code = %d, want %d", secondCreateResponse.Code, http.StatusCreated)
	}
	secondWorkout := decodeWorkoutResponse(t, secondCreateResponse)

	inProgressFeedbackResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+secondWorkout.ID.String()+"/recovery-feedback", `{"recovery_level":"recovered"}`, secondCookie)
	if inProgressFeedbackResponse.Code != http.StatusConflict {
		t.Fatalf("inProgressFeedbackResponse.Code = %d, want %d", inProgressFeedbackResponse.Code, http.StatusConflict)
	}

	invalidEffortPayloadResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+firstWorkout.ID.String()+"/effort-feedback", `{"effort_level":"elite"}`, firstCookie)
	if invalidEffortPayloadResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidEffortPayloadResponse.Code = %d, want %d", invalidEffortPayloadResponse.Code, http.StatusBadRequest)
	}
	invalidRecoveryPayloadResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+firstWorkout.ID.String()+"/recovery-feedback", `{"recovery_level":"fresh"}`, firstCookie)
	if invalidRecoveryPayloadResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidRecoveryPayloadResponse.Code = %d, want %d", invalidRecoveryPayloadResponse.Code, http.StatusBadRequest)
	}

	missingTopicResponse := env.doRequest(t, http.MethodGet, "/api/v1/helpers/coaching/why?week_start=2026-04-06", nil, firstCookie)
	if missingTopicResponse.Code != http.StatusBadRequest {
		t.Fatalf("missingTopicResponse.Code = %d, want %d", missingTopicResponse.Code, http.StatusBadRequest)
	}
	unsupportedTopicResponse := env.doRequest(t, http.MethodGet, "/api/v1/helpers/coaching/why?week_start=2026-04-06&topic=timeline", nil, firstCookie)
	if unsupportedTopicResponse.Code != http.StatusBadRequest {
		t.Fatalf("unsupportedTopicResponse.Code = %d, want %d", unsupportedTopicResponse.Code, http.StatusBadRequest)
	}
	unsupportedVariationResponse := env.doRequest(t, http.MethodGet, "/api/v1/helpers/coaching/variation?week_start=2026-04-06&variation=longer", nil, firstCookie)
	if unsupportedVariationResponse.Code != http.StatusBadRequest {
		t.Fatalf("unsupportedVariationResponse.Code = %d, want %d", unsupportedVariationResponse.Code, http.StatusBadRequest)
	}
}

func TestCoachingRuntimeDoesNotMutateUnrelatedStateDomains(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-coaching-int-003", "coaching-int-003@example.com")

	if _, err := env.db.DB.Exec(context.Background(), `
INSERT INTO apollo.recommendations (user_id, content, model_used)
VALUES ($1, '{}'::jsonb, 'deterministic-fixture')
`, user.ID); err != nil {
		t.Fatalf("Exec(seed recommendation row) error = %v", err)
	}

	beforeRecommendationRows := countTotalRows(t, env, "apollo.recommendations")
	beforeMembershipRows := countTotalRows(t, env, "apollo.lobby_memberships")
	beforeCompetitionRows := countTotalRows(t, env, "apollo.competition_sessions")
	beforeAresMatchRows := countTotalRows(t, env, "apollo.ares_matches")
	beforeAresRatingRows := countTotalRows(t, env, "apollo.ares_ratings")

	putWeekResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/planner/weeks/2026-04-06", `{"sessions":[{"day_index":0,"items":[{"exercise_key":"push-up","sets":3,"reps":12}]}]}`, cookie)
	if putWeekResponse.Code != http.StatusOK {
		t.Fatalf("putWeekResponse.Code = %d, want %d", putWeekResponse.Code, http.StatusOK)
	}
	recommendationResponse := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/coaching?week_start=2026-04-06", nil, cookie)
	if recommendationResponse.Code != http.StatusOK {
		t.Fatalf("recommendationResponse.Code = %d, want %d", recommendationResponse.Code, http.StatusOK)
	}
	helperReadResponse := env.doRequest(t, http.MethodGet, "/api/v1/helpers/coaching?week_start=2026-04-06", nil, cookie)
	if helperReadResponse.Code != http.StatusOK {
		t.Fatalf("helperReadResponse.Code = %d, want %d", helperReadResponse.Code, http.StatusOK)
	}
	helperWhyResponse := env.doRequest(t, http.MethodGet, "/api/v1/helpers/coaching/why?week_start=2026-04-06&topic=proposal", nil, cookie)
	if helperWhyResponse.Code != http.StatusOK {
		t.Fatalf("helperWhyResponse.Code = %d, want %d", helperWhyResponse.Code, http.StatusOK)
	}
	helperVariationResponse := env.doRequest(t, http.MethodGet, "/api/v1/helpers/coaching/variation?week_start=2026-04-06&variation=easier", nil, cookie)
	if helperVariationResponse.Code != http.StatusOK {
		t.Fatalf("helperVariationResponse.Code = %d, want %d", helperVariationResponse.Code, http.StatusOK)
	}

	if afterRecommendationRows := countTotalRows(t, env, "apollo.recommendations"); afterRecommendationRows != beforeRecommendationRows {
		t.Fatalf("recommendation row count changed from %d to %d", beforeRecommendationRows, afterRecommendationRows)
	}
	if afterMembershipRows := countTotalRows(t, env, "apollo.lobby_memberships"); afterMembershipRows != beforeMembershipRows {
		t.Fatalf("membership row count changed from %d to %d", beforeMembershipRows, afterMembershipRows)
	}
	if afterCompetitionRows := countTotalRows(t, env, "apollo.competition_sessions"); afterCompetitionRows != beforeCompetitionRows {
		t.Fatalf("competition session row count changed from %d to %d", beforeCompetitionRows, afterCompetitionRows)
	}
	if afterAresMatchRows := countTotalRows(t, env, "apollo.ares_matches"); afterAresMatchRows != beforeAresMatchRows {
		t.Fatalf("ares match row count changed from %d to %d", beforeAresMatchRows, afterAresMatchRows)
	}
	if afterAresRatingRows := countTotalRows(t, env, "apollo.ares_ratings"); afterAresRatingRows != beforeAresRatingRows {
		t.Fatalf("ares rating row count changed from %d to %d", beforeAresRatingRows, afterAresRatingRows)
	}
}

func countTotalRows(t *testing.T, env *authProfileServerEnv, table string) int {
	t.Helper()

	var count int
	query := "SELECT count(*) FROM " + table
	if err := env.db.DB.QueryRow(context.Background(), query).Scan(&count); err != nil {
		t.Fatalf("QueryRow(%s) error = %v", table, err)
	}
	return count
}
