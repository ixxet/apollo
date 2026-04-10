package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/nutrition"
)

func TestNutritionRuntimeRoundTripAndRecommendationStayDeterministic(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, _ := createVerifiedSessionViaHTTP(t, env, "student-nutrition-rt-001", "nutrition-rt-001@example.com")
	base := time.Now().UTC().Truncate(time.Second)

	patchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{
		"coaching_profile":{"goal_key":"build-strength","days_per_week":4,"session_minutes":60},
		"nutrition_profile":{
			"dietary_restrictions":["vegetarian","dairy_free"],
			"meal_preference":{"cuisine_preferences":["mediterranean","south-asian"]},
			"budget_preference":"budget_constrained",
			"cooking_capability":"microwave_only"
		}
	}`, cookie)
	if patchResponse.Code != http.StatusOK {
		t.Fatalf("patchResponse.Code = %d, want %d", patchResponse.Code, http.StatusOK)
	}
	memberProfile := decodeProfileResponse(t, patchResponse)
	if len(memberProfile.NutritionProfile.DietaryRestrictions) != 2 {
		t.Fatalf("DietaryRestrictions = %#v, want 2 entries", memberProfile.NutritionProfile.DietaryRestrictions)
	}
	if memberProfile.NutritionProfile.BudgetPreference == nil || *memberProfile.NutritionProfile.BudgetPreference != "budget_constrained" {
		t.Fatalf("BudgetPreference = %#v, want budget_constrained", memberProfile.NutritionProfile.BudgetPreference)
	}
	if memberProfile.NutritionProfile.CookingCapability == nil || *memberProfile.NutritionProfile.CookingCapability != "microwave_only" {
		t.Fatalf("CookingCapability = %#v, want microwave_only", memberProfile.NutritionProfile.CookingCapability)
	}

	templateOneResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/nutrition/meal-templates", `{
		"name":"Overnight Oats",
		"meal_type":"breakfast",
		"calories":3600,
		"protein_grams":210,
		"carbs_grams":380,
		"fat_grams":120,
		"notes":"default breakfast"
	}`, cookie)
	if templateOneResponse.Code != http.StatusCreated {
		t.Fatalf("templateOneResponse.Code = %d, want %d", templateOneResponse.Code, http.StatusCreated)
	}
	templateOne := decodeMealTemplateResponse(t, templateOneResponse)

	templateTwoResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/nutrition/meal-templates", `{
		"name":"Rice Bowl",
		"meal_type":"dinner",
		"calories":3400,
		"protein_grams":205,
		"carbs_grams":370,
		"fat_grams":115,
		"notes":"default dinner"
	}`, cookie)
	if templateTwoResponse.Code != http.StatusCreated {
		t.Fatalf("templateTwoResponse.Code = %d, want %d", templateTwoResponse.Code, http.StatusCreated)
	}
	templateTwo := decodeMealTemplateResponse(t, templateTwoResponse)

	updateTemplateResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/nutrition/meal-templates/"+templateTwo.ID.String(), `{
		"name":"Rice Bowl",
		"meal_type":"dinner",
		"calories":3500,
		"protein_grams":210,
		"carbs_grams":375,
		"fat_grams":118,
		"notes":"updated dinner"
	}`, cookie)
	if updateTemplateResponse.Code != http.StatusOK {
		t.Fatalf("updateTemplateResponse.Code = %d, want %d", updateTemplateResponse.Code, http.StatusOK)
	}
	updatedTemplate := decodeMealTemplateResponse(t, updateTemplateResponse)
	if updatedTemplate.Calories == nil || *updatedTemplate.Calories != 3500 {
		t.Fatalf("updatedTemplate.Calories = %#v, want 3500", updatedTemplate.Calories)
	}

	templateListResponse := env.doRequest(t, http.MethodGet, "/api/v1/nutrition/meal-templates", nil, cookie)
	if templateListResponse.Code != http.StatusOK {
		t.Fatalf("templateListResponse.Code = %d, want %d", templateListResponse.Code, http.StatusOK)
	}
	templateList := decodeMealTemplateListResponse(t, templateListResponse)
	if len(templateList) != 2 {
		t.Fatalf("len(templateList) = %d, want 2", len(templateList))
	}
	if templateList[0].Name != "Overnight Oats" || templateList[1].Name != "Rice Bowl" {
		t.Fatalf("template order = %#v, want alphabetical order", []string{templateList[0].Name, templateList[1].Name})
	}

	firstLogResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/nutrition/meal-logs", fmt.Sprintf(`{
		"source_template_id":"%s",
		"logged_at":"%s",
		"notes":"first pass"
	}`, templateOne.ID, base.AddDate(0, 0, -1).Format(time.RFC3339)), cookie)
	if firstLogResponse.Code != http.StatusCreated {
		t.Fatalf("firstLogResponse.Code = %d, want %d", firstLogResponse.Code, http.StatusCreated)
	}
	firstLog := decodeMealLogResponse(t, firstLogResponse)

	updateLogResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/nutrition/meal-logs/"+firstLog.ID.String(), fmt.Sprintf(`{
		"source_template_id":"%s",
		"logged_at":"%s",
		"notes":"after training"
	}`, templateOne.ID, base.AddDate(0, 0, -1).Format(time.RFC3339)), cookie)
	if updateLogResponse.Code != http.StatusOK {
		t.Fatalf("updateLogResponse.Code = %d, want %d", updateLogResponse.Code, http.StatusOK)
	}

	for _, testCase := range []struct {
		name string
		body string
	}{
		{
			name: "manual day 2",
			body: fmt.Sprintf(`{"name":"Lunch Plate","meal_type":"lunch","logged_at":"%s","calories":3550,"protein_grams":212,"carbs_grams":378,"fat_grams":119}`, base.AddDate(0, 0, -2).Format(time.RFC3339)),
		},
		{
			name: "manual day 3",
			body: fmt.Sprintf(`{"name":"Dinner Plate","meal_type":"dinner","logged_at":"%s","calories":3500,"protein_grams":208,"carbs_grams":370,"fat_grams":118}`, base.AddDate(0, 0, -3).Format(time.RFC3339)),
		},
		{
			name: "template day 4",
			body: fmt.Sprintf(`{"source_template_id":"%s","logged_at":"%s","notes":"reuse template"}`, templateTwo.ID, base.AddDate(0, 0, -4).Format(time.RFC3339)),
		},
		{
			name: "manual day 5",
			body: fmt.Sprintf(`{"name":"Snack Stack","meal_type":"snack","logged_at":"%s","calories":3600,"protein_grams":214,"carbs_grams":382,"fat_grams":120}`, base.AddDate(0, 0, -5).Format(time.RFC3339)),
		},
	} {
		response := env.doJSONRequest(t, http.MethodPost, "/api/v1/nutrition/meal-logs", testCase.body, cookie)
		if response.Code != http.StatusCreated {
			t.Fatalf("%s response.Code = %d, want %d", testCase.name, response.Code, http.StatusCreated)
		}
	}

	mealLogListResponse := env.doRequest(t, http.MethodGet, "/api/v1/nutrition/meal-logs", nil, cookie)
	if mealLogListResponse.Code != http.StatusOK {
		t.Fatalf("mealLogListResponse.Code = %d, want %d", mealLogListResponse.Code, http.StatusOK)
	}
	mealLogs := decodeMealLogListResponse(t, mealLogListResponse)
	if len(mealLogs) != 5 {
		t.Fatalf("len(mealLogs) = %d, want 5", len(mealLogs))
	}
	if mealLogs[0].ID != firstLog.ID {
		t.Fatalf("mealLogs[0].ID = %s, want %s", mealLogs[0].ID, firstLog.ID)
	}
	if mealLogs[0].SourceTemplateID == nil || *mealLogs[0].SourceTemplateID != templateOne.ID {
		t.Fatalf("mealLogs[0].SourceTemplateID = %#v, want %s", mealLogs[0].SourceTemplateID, templateOne.ID)
	}

	firstRecommendationResponse := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/nutrition", nil, cookie)
	if firstRecommendationResponse.Code != http.StatusOK {
		t.Fatalf("firstRecommendationResponse.Code = %d, want %d", firstRecommendationResponse.Code, http.StatusOK)
	}
	firstRecommendation := decodeNutritionRecommendationResponse(t, firstRecommendationResponse)

	secondRecommendationResponse := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/nutrition", nil, cookie)
	if secondRecommendationResponse.Code != http.StatusOK {
		t.Fatalf("secondRecommendationResponse.Code = %d, want %d", secondRecommendationResponse.Code, http.StatusOK)
	}
	secondRecommendation := decodeNutritionRecommendationResponse(t, secondRecommendationResponse)

	if !reflect.DeepEqual(normalizeNutritionRecommendation(firstRecommendation), normalizeNutritionRecommendation(secondRecommendation)) {
		t.Fatalf("recommendations changed across rerun:\nfirst=%#v\nsecond=%#v", firstRecommendation, secondRecommendation)
	}
	if firstRecommendation.Kind != nutrition.KindTighten {
		t.Fatalf("Kind = %q, want %q", firstRecommendation.Kind, nutrition.KindTighten)
	}
	if !reflect.DeepEqual(firstRecommendation.StrategyFlags, []string{"assembly_first", "budget_first", "template_reuse_first"}) {
		t.Fatalf("StrategyFlags = %#v, want conservative strategy flags", firstRecommendation.StrategyFlags)
	}
	if !containsNutritionString(firstRecommendation.Explanation.ReasonCodes, "recent_history_stabilized") {
		t.Fatalf("ReasonCodes = %#v, want recent_history_stabilized", firstRecommendation.Explanation.ReasonCodes)
	}
	if !containsNutritionString(firstRecommendation.Explanation.ReasonCodes, "recent_intake_above_range") {
		t.Fatalf("ReasonCodes = %#v, want recent_intake_above_range", firstRecommendation.Explanation.ReasonCodes)
	}
	if !containsNutritionString(firstRecommendation.Explanation.Limitations, "non-clinical guidance only; not medical advice or diagnosis") {
		t.Fatalf("Limitations = %#v, want non-clinical limitation", firstRecommendation.Explanation.Limitations)
	}
}

func TestNutritionRecommendationRuntimeStaysConservativeWithSparseHistory(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, _ := createVerifiedSessionViaHTTP(t, env, "student-nutrition-rt-002", "nutrition-rt-002@example.com")

	patchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{
		"nutrition_profile":{
			"budget_preference":"budget_constrained",
			"cooking_capability":"microwave_only"
		}
	}`, cookie)
	if patchResponse.Code != http.StatusOK {
		t.Fatalf("patchResponse.Code = %d, want %d", patchResponse.Code, http.StatusOK)
	}

	recommendationResponse := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/nutrition", nil, cookie)
	if recommendationResponse.Code != http.StatusOK {
		t.Fatalf("recommendationResponse.Code = %d, want %d", recommendationResponse.Code, http.StatusOK)
	}
	recommendation := decodeNutritionRecommendationResponse(t, recommendationResponse)
	if recommendation.Kind != nutrition.KindStartConservative {
		t.Fatalf("Kind = %q, want %q", recommendation.Kind, nutrition.KindStartConservative)
	}
	if !containsNutritionString(recommendation.Explanation.ReasonCodes, "default_goal_fallback") {
		t.Fatalf("ReasonCodes = %#v, want default_goal_fallback", recommendation.Explanation.ReasonCodes)
	}
	if !containsNutritionString(recommendation.Explanation.ReasonCodes, "sparse_recent_history") {
		t.Fatalf("ReasonCodes = %#v, want sparse_recent_history", recommendation.Explanation.ReasonCodes)
	}
	if recommendation.Explanation.Evidence.RecentMealLogCount != 0 {
		t.Fatalf("RecentMealLogCount = %d, want 0", recommendation.Explanation.Evidence.RecentMealLogCount)
	}
}

func decodeMealTemplateResponse(t *testing.T, response *httptest.ResponseRecorder) nutrition.MealTemplate {
	t.Helper()

	var template nutrition.MealTemplate
	if err := json.Unmarshal(response.Body.Bytes(), &template); err != nil {
		t.Fatalf("json.Unmarshal(template) error = %v", err)
	}
	return template
}

func decodeMealTemplateListResponse(t *testing.T, response *httptest.ResponseRecorder) []nutrition.MealTemplate {
	t.Helper()

	var templates []nutrition.MealTemplate
	if err := json.Unmarshal(response.Body.Bytes(), &templates); err != nil {
		t.Fatalf("json.Unmarshal(template list) error = %v", err)
	}
	return templates
}

func decodeMealLogResponse(t *testing.T, response *httptest.ResponseRecorder) nutrition.MealLog {
	t.Helper()

	var mealLog nutrition.MealLog
	if err := json.Unmarshal(response.Body.Bytes(), &mealLog); err != nil {
		t.Fatalf("json.Unmarshal(meal log) error = %v", err)
	}
	return mealLog
}

func decodeMealLogListResponse(t *testing.T, response *httptest.ResponseRecorder) []nutrition.MealLog {
	t.Helper()

	var mealLogs []nutrition.MealLog
	if err := json.Unmarshal(response.Body.Bytes(), &mealLogs); err != nil {
		t.Fatalf("json.Unmarshal(meal log list) error = %v", err)
	}
	return mealLogs
}

func decodeNutritionRecommendationResponse(t *testing.T, response *httptest.ResponseRecorder) nutrition.Recommendation {
	t.Helper()

	var recommendation nutrition.Recommendation
	if err := json.Unmarshal(response.Body.Bytes(), &recommendation); err != nil {
		t.Fatalf("json.Unmarshal(nutrition recommendation) error = %v", err)
	}
	return recommendation
}

func normalizeNutritionRecommendation(input nutrition.Recommendation) nutrition.Recommendation {
	input.GeneratedAt = time.Time{}
	return input
}

func containsNutritionString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
