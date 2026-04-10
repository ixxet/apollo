package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestNutritionRuntimeRejectsInvalidPayloadsAndWrongOwnerMutations(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	firstCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-nutrition-int-001", "nutrition-int-001@example.com")
	secondCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-nutrition-int-002", "nutrition-int-002@example.com")
	base := time.Now().UTC().Truncate(time.Second)

	conflictingProfileResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{
		"nutrition_profile":{"dietary_restrictions":["vegetarian","vegan"]}
	}`, firstCookie)
	if conflictingProfileResponse.Code != http.StatusBadRequest {
		t.Fatalf("conflictingProfileResponse.Code = %d, want %d", conflictingProfileResponse.Code, http.StatusBadRequest)
	}

	invalidTemplateResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/nutrition/meal-templates", `{
		"name":"Broken Template",
		"meal_type":"breakfast"
	}`, firstCookie)
	if invalidTemplateResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidTemplateResponse.Code = %d, want %d", invalidTemplateResponse.Code, http.StatusBadRequest)
	}

	validTemplateResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/nutrition/meal-templates", `{
		"name":"Integrity Oats",
		"meal_type":"breakfast",
		"calories":520,
		"protein_grams":28
	}`, firstCookie)
	if validTemplateResponse.Code != http.StatusCreated {
		t.Fatalf("validTemplateResponse.Code = %d, want %d", validTemplateResponse.Code, http.StatusCreated)
	}
	template := decodeMealTemplateResponse(t, validTemplateResponse)

	duplicateTemplateResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/nutrition/meal-templates", `{
		"name":"Integrity Oats",
		"meal_type":"breakfast",
		"calories":520
	}`, firstCookie)
	if duplicateTemplateResponse.Code != http.StatusConflict {
		t.Fatalf("duplicateTemplateResponse.Code = %d, want %d", duplicateTemplateResponse.Code, http.StatusConflict)
	}

	wrongOwnerTemplateResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/nutrition/meal-templates/"+template.ID.String(), `{
		"name":"Integrity Oats",
		"meal_type":"breakfast",
		"calories":540
	}`, secondCookie)
	if wrongOwnerTemplateResponse.Code != http.StatusNotFound {
		t.Fatalf("wrongOwnerTemplateResponse.Code = %d, want %d", wrongOwnerTemplateResponse.Code, http.StatusNotFound)
	}

	invalidMealLogResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/nutrition/meal-logs", `{
		"name":"Broken Log",
		"meal_type":"lunch"
	}`, firstCookie)
	if invalidMealLogResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidMealLogResponse.Code = %d, want %d", invalidMealLogResponse.Code, http.StatusBadRequest)
	}

	validMealLogResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/nutrition/meal-logs", fmt.Sprintf(`{
		"source_template_id":"%s",
		"logged_at":"%s"
	}`, template.ID, base.Add(-time.Hour).Format(time.RFC3339)), firstCookie)
	if validMealLogResponse.Code != http.StatusCreated {
		t.Fatalf("validMealLogResponse.Code = %d, want %d", validMealLogResponse.Code, http.StatusCreated)
	}
	mealLog := decodeMealLogResponse(t, validMealLogResponse)

	wrongOwnerMealLogResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/nutrition/meal-logs/"+mealLog.ID.String(), fmt.Sprintf(`{
		"source_template_id":"%s",
		"logged_at":"%s",
		"notes":"should fail"
	}`, template.ID, base.Add(-time.Hour).Format(time.RFC3339)), secondCookie)
	if wrongOwnerMealLogResponse.Code != http.StatusNotFound {
		t.Fatalf("wrongOwnerMealLogResponse.Code = %d, want %d", wrongOwnerMealLogResponse.Code, http.StatusNotFound)
	}

	missingLoggedAtResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/nutrition/meal-logs/"+mealLog.ID.String(), fmt.Sprintf(`{
		"source_template_id":"%s"
	}`, template.ID), firstCookie)
	if missingLoggedAtResponse.Code != http.StatusBadRequest {
		t.Fatalf("missingLoggedAtResponse.Code = %d, want %d", missingLoggedAtResponse.Code, http.StatusBadRequest)
	}
}

func TestNutritionRuntimeDoesNotMutateUnrelatedStateDomains(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-nutrition-int-003", "nutrition-int-003@example.com")
	base := time.Now().UTC().Truncate(time.Second)

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", user.ID, "tag-nutrition-001", "nutrition tag"); err != nil {
		t.Fatalf("Exec(insert claimed tag) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.visits (user_id, facility_key, source_event_id, arrived_at) VALUES ($1, $2, $3, $4)", user.ID, "ashtonbee", "visit-nutrition-001", base.Add(-2*time.Hour)); err != nil {
		t.Fatalf("Exec(insert visit) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.workouts (user_id, started_at, status, finished_at) VALUES ($1, $2, $3, $4)", user.ID, base.Add(-90*time.Minute), "finished", base.Add(-30*time.Minute)); err != nil {
		t.Fatalf("Exec(insert workout) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.recommendations (user_id, content, model_used) VALUES ($1, '{}'::jsonb, 'deterministic-fixture')", user.ID); err != nil {
		t.Fatalf("Exec(insert recommendation row) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.lobby_memberships (user_id, status, joined_at) VALUES ($1, 'joined', $2)", user.ID, base.Add(-time.Hour)); err != nil {
		t.Fatalf("Exec(insert membership row) error = %v", err)
	}

	beforeClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID)
	beforeVisits := countRows(t, env, "apollo.visits", user.ID)
	beforeWorkouts := countRows(t, env, "apollo.workouts", user.ID)
	beforeRecommendations := countTotalRows(t, env, "apollo.recommendations")
	beforeMembershipRows := countTotalRows(t, env, "apollo.lobby_memberships")
	beforePlannerWeeks := countTotalRows(t, env, "apollo.planner_weeks")
	beforePlannerSessions := countTotalRows(t, env, "apollo.planner_sessions")
	beforePlannerItems := countTotalRows(t, env, "apollo.planner_session_items")
	beforeEffortRows := countTotalRows(t, env, "apollo.workout_effort_feedback")
	beforeRecoveryRows := countTotalRows(t, env, "apollo.workout_recovery_feedback")
	beforeCompetitionRows := countTotalRows(t, env, "apollo.competition_sessions")
	beforeAresMatchRows := countTotalRows(t, env, "apollo.ares_matches")
	beforeAresRatingRows := countTotalRows(t, env, "apollo.ares_ratings")
	tagHashBefore := lookupTagHash(t, env, user.ID)

	patchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{
		"nutrition_profile":{
			"dietary_restrictions":["vegetarian","dairy_free"],
			"meal_preference":{"cuisine_preferences":["mediterranean"]},
			"budget_preference":"budget_constrained",
			"cooking_capability":"microwave_only"
		}
	}`, cookie)
	if patchResponse.Code != http.StatusOK {
		t.Fatalf("patchResponse.Code = %d, want %d", patchResponse.Code, http.StatusOK)
	}

	templateResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/nutrition/meal-templates", `{
		"name":"Integrity Bowl",
		"meal_type":"dinner",
		"calories":700,
		"protein_grams":40,
		"carbs_grams":65,
		"fat_grams":20
	}`, cookie)
	if templateResponse.Code != http.StatusCreated {
		t.Fatalf("templateResponse.Code = %d, want %d", templateResponse.Code, http.StatusCreated)
	}
	template := decodeMealTemplateResponse(t, templateResponse)

	updateTemplateResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/nutrition/meal-templates/"+template.ID.String(), `{
		"name":"Integrity Bowl",
		"meal_type":"dinner",
		"calories":720,
		"protein_grams":42,
		"carbs_grams":66,
		"fat_grams":21
	}`, cookie)
	if updateTemplateResponse.Code != http.StatusOK {
		t.Fatalf("updateTemplateResponse.Code = %d, want %d", updateTemplateResponse.Code, http.StatusOK)
	}

	mealLogResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/nutrition/meal-logs", fmt.Sprintf(`{
		"source_template_id":"%s",
		"logged_at":"%s",
		"notes":"integrity pass"
	}`, template.ID, base.Add(-time.Hour).Format(time.RFC3339)), cookie)
	if mealLogResponse.Code != http.StatusCreated {
		t.Fatalf("mealLogResponse.Code = %d, want %d", mealLogResponse.Code, http.StatusCreated)
	}
	mealLog := decodeMealLogResponse(t, mealLogResponse)

	updateMealLogResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/nutrition/meal-logs/"+mealLog.ID.String(), fmt.Sprintf(`{
		"source_template_id":"%s",
		"logged_at":"%s",
		"notes":"updated integrity pass"
	}`, template.ID, base.Add(-time.Hour).Format(time.RFC3339)), cookie)
	if updateMealLogResponse.Code != http.StatusOK {
		t.Fatalf("updateMealLogResponse.Code = %d, want %d", updateMealLogResponse.Code, http.StatusOK)
	}

	recommendationResponse := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/nutrition", nil, cookie)
	if recommendationResponse.Code != http.StatusOK {
		t.Fatalf("recommendationResponse.Code = %d, want %d", recommendationResponse.Code, http.StatusOK)
	}

	if afterClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID); afterClaimedTags != beforeClaimedTags {
		t.Fatalf("claimed tag count changed from %d to %d", beforeClaimedTags, afterClaimedTags)
	}
	if afterVisits := countRows(t, env, "apollo.visits", user.ID); afterVisits != beforeVisits {
		t.Fatalf("visit count changed from %d to %d", beforeVisits, afterVisits)
	}
	if afterWorkouts := countRows(t, env, "apollo.workouts", user.ID); afterWorkouts != beforeWorkouts {
		t.Fatalf("workout count changed from %d to %d", beforeWorkouts, afterWorkouts)
	}
	if afterRecommendations := countTotalRows(t, env, "apollo.recommendations"); afterRecommendations != beforeRecommendations {
		t.Fatalf("recommendation row count changed from %d to %d", beforeRecommendations, afterRecommendations)
	}
	if afterMembershipRows := countTotalRows(t, env, "apollo.lobby_memberships"); afterMembershipRows != beforeMembershipRows {
		t.Fatalf("membership row count changed from %d to %d", beforeMembershipRows, afterMembershipRows)
	}
	if afterPlannerWeeks := countTotalRows(t, env, "apollo.planner_weeks"); afterPlannerWeeks != beforePlannerWeeks {
		t.Fatalf("planner week row count changed from %d to %d", beforePlannerWeeks, afterPlannerWeeks)
	}
	if afterPlannerSessions := countTotalRows(t, env, "apollo.planner_sessions"); afterPlannerSessions != beforePlannerSessions {
		t.Fatalf("planner session row count changed from %d to %d", beforePlannerSessions, afterPlannerSessions)
	}
	if afterPlannerItems := countTotalRows(t, env, "apollo.planner_session_items"); afterPlannerItems != beforePlannerItems {
		t.Fatalf("planner item row count changed from %d to %d", beforePlannerItems, afterPlannerItems)
	}
	if afterEffortRows := countTotalRows(t, env, "apollo.workout_effort_feedback"); afterEffortRows != beforeEffortRows {
		t.Fatalf("workout effort feedback row count changed from %d to %d", beforeEffortRows, afterEffortRows)
	}
	if afterRecoveryRows := countTotalRows(t, env, "apollo.workout_recovery_feedback"); afterRecoveryRows != beforeRecoveryRows {
		t.Fatalf("workout recovery feedback row count changed from %d to %d", beforeRecoveryRows, afterRecoveryRows)
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
	if tagHashAfter := lookupTagHash(t, env, user.ID); tagHashAfter != tagHashBefore {
		t.Fatalf("tag hash changed from %q to %q", tagHashBefore, tagHashAfter)
	}
}
