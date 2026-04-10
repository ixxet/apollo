package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ixxet/apollo/internal/exercises"
	"github.com/ixxet/apollo/internal/planner"
	"github.com/ixxet/apollo/internal/profile"
	"github.com/ixxet/apollo/internal/recommendations"
)

func TestPlannerRuntimeSupportsCatalogTemplatesWeeksAndTypedProfileRoundTrip(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, _ := createVerifiedSessionViaHTTP(t, env, "student-planner-001", "planner-001@example.com")

	initialRecommendationResponse := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/workout", nil, cookie)
	if initialRecommendationResponse.Code != http.StatusOK {
		t.Fatalf("initialRecommendationResponse.Code = %d, want %d", initialRecommendationResponse.Code, http.StatusOK)
	}
	initialRecommendation := decodeWorkoutRecommendationResponse(t, initialRecommendationResponse)

	equipmentResponse := env.doRequest(t, http.MethodGet, "/api/v1/planner/equipment", nil, cookie)
	if equipmentResponse.Code != http.StatusOK {
		t.Fatalf("equipmentResponse.Code = %d, want %d", equipmentResponse.Code, http.StatusOK)
	}
	equipment := decodeEquipmentCatalogResponse(t, equipmentResponse)
	if len(equipment) != 5 {
		t.Fatalf("len(equipment) = %d, want 5", len(equipment))
	}

	exerciseResponse := env.doRequest(t, http.MethodGet, "/api/v1/planner/exercises", nil, cookie)
	if exerciseResponse.Code != http.StatusOK {
		t.Fatalf("exerciseResponse.Code = %d, want %d", exerciseResponse.Code, http.StatusOK)
	}
	exerciseCatalog := decodeExerciseCatalogResponse(t, exerciseResponse)
	if len(exerciseCatalog) != 6 {
		t.Fatalf("len(exerciseCatalog) = %d, want 6", len(exerciseCatalog))
	}

	profilePatchResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"coaching_profile":{"goal_key":"build-strength","days_per_week":4,"session_minutes":60,"experience_level":"intermediate","preferred_equipment_keys":["barbell","dumbbell"]}}`, cookie)
	if profilePatchResponse.Code != http.StatusOK {
		t.Fatalf("profilePatchResponse.Code = %d, want %d", profilePatchResponse.Code, http.StatusOK)
	}
	patchedProfile := decodeProfileResponse(t, profilePatchResponse)
	if patchedProfile.CoachingProfile.GoalKey == nil || *patchedProfile.CoachingProfile.GoalKey != "build-strength" {
		t.Fatalf("GoalKey = %#v, want build-strength", patchedProfile.CoachingProfile.GoalKey)
	}
	if patchedProfile.CoachingProfile.DaysPerWeek == nil || *patchedProfile.CoachingProfile.DaysPerWeek != 4 {
		t.Fatalf("DaysPerWeek = %#v, want 4", patchedProfile.CoachingProfile.DaysPerWeek)
	}
	if len(patchedProfile.CoachingProfile.PreferredEquipmentKeys) != 2 {
		t.Fatalf("PreferredEquipmentKeys = %#v, want 2 keys", patchedProfile.CoachingProfile.PreferredEquipmentKeys)
	}
	if patchedProfile.CoachingProfile.ExperienceLevel == nil || *patchedProfile.CoachingProfile.ExperienceLevel != "intermediate" {
		t.Fatalf("ExperienceLevel = %#v, want intermediate", patchedProfile.CoachingProfile.ExperienceLevel)
	}

	createTemplateResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/planner/templates", `{"name":"Strength Base","items":[{"exercise_key":"barbell-back-squat","equipment_key":"barbell","sets":5,"reps":5,"weight_kg":100},{"exercise_key":"push-up","sets":3,"reps":12}]}`, cookie)
	if createTemplateResponse.Code != http.StatusCreated {
		t.Fatalf("createTemplateResponse.Code = %d, want %d", createTemplateResponse.Code, http.StatusCreated)
	}
	createdTemplate := decodeTemplateResponse(t, createTemplateResponse)
	if createdTemplate.Name != "Strength Base" {
		t.Fatalf("createdTemplate.Name = %q, want Strength Base", createdTemplate.Name)
	}
	if len(createdTemplate.Items) != 2 {
		t.Fatalf("len(createdTemplate.Items) = %d, want 2", len(createdTemplate.Items))
	}

	updateTemplateResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/planner/templates/"+createdTemplate.ID.String(), `{"name":"Strength Base","items":[{"exercise_key":"leg-press","equipment_key":"leg-press","sets":4,"reps":10},{"exercise_key":"cable-seated-row","equipment_key":"cable-stack","sets":4,"reps":12}]}`, cookie)
	if updateTemplateResponse.Code != http.StatusOK {
		t.Fatalf("updateTemplateResponse.Code = %d, want %d", updateTemplateResponse.Code, http.StatusOK)
	}
	updatedTemplate := decodeTemplateResponse(t, updateTemplateResponse)
	if updatedTemplate.Items[0].ExerciseKey != "leg-press" {
		t.Fatalf("updatedTemplate.Items[0].ExerciseKey = %q, want leg-press", updatedTemplate.Items[0].ExerciseKey)
	}

	templateDetailResponse := env.doRequest(t, http.MethodGet, "/api/v1/planner/templates/"+createdTemplate.ID.String(), nil, cookie)
	if templateDetailResponse.Code != http.StatusOK {
		t.Fatalf("templateDetailResponse.Code = %d, want %d", templateDetailResponse.Code, http.StatusOK)
	}
	templateDetail := decodeTemplateResponse(t, templateDetailResponse)
	if templateDetail.ID != createdTemplate.ID {
		t.Fatalf("templateDetail.ID = %s, want %s", templateDetail.ID, createdTemplate.ID)
	}

	templateListResponse := env.doRequest(t, http.MethodGet, "/api/v1/planner/templates", nil, cookie)
	if templateListResponse.Code != http.StatusOK {
		t.Fatalf("templateListResponse.Code = %d, want %d", templateListResponse.Code, http.StatusOK)
	}
	templateList := decodeTemplateListResponse(t, templateListResponse)
	if len(templateList) != 1 {
		t.Fatalf("len(templateList) = %d, want 1", len(templateList))
	}

	putWeekResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/planner/weeks/2026-04-06", `{"sessions":[{"day_index":0,"items":[{"exercise_key":"dumbbell-bench-press","equipment_key":"dumbbell","sets":4,"reps":8,"weight_kg":32.5},{"exercise_key":"push-up","sets":3,"reps":15}]},{"day_index":2,"template_id":"`+createdTemplate.ID.String()+`"}]}`, cookie)
	if putWeekResponse.Code != http.StatusOK {
		t.Fatalf("putWeekResponse.Code = %d, want %d", putWeekResponse.Code, http.StatusOK)
	}
	week := decodeWeekResponse(t, putWeekResponse)
	if week.WeekStart != "2026-04-06" {
		t.Fatalf("week.WeekStart = %q, want 2026-04-06", week.WeekStart)
	}
	if len(week.Sessions) != 2 {
		t.Fatalf("len(week.Sessions) = %d, want 2", len(week.Sessions))
	}
	if week.Sessions[1].TemplateID == nil || *week.Sessions[1].TemplateID != createdTemplate.ID {
		t.Fatalf("week.Sessions[1].TemplateID = %#v, want %s", week.Sessions[1].TemplateID, createdTemplate.ID)
	}
	if week.Sessions[1].Items[0].ExerciseKey != "leg-press" {
		t.Fatalf("week.Sessions[1].Items[0].ExerciseKey = %q, want leg-press", week.Sessions[1].Items[0].ExerciseKey)
	}

	getWeekResponse := env.doRequest(t, http.MethodGet, "/api/v1/planner/weeks/2026-04-06", nil, cookie)
	if getWeekResponse.Code != http.StatusOK {
		t.Fatalf("getWeekResponse.Code = %d, want %d", getWeekResponse.Code, http.StatusOK)
	}
	roundTripWeek := decodeWeekResponse(t, getWeekResponse)
	if len(roundTripWeek.Sessions) != 2 {
		t.Fatalf("len(roundTripWeek.Sessions) = %d, want 2", len(roundTripWeek.Sessions))
	}

	secondRecommendationResponse := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/workout", nil, cookie)
	if secondRecommendationResponse.Code != http.StatusOK {
		t.Fatalf("secondRecommendationResponse.Code = %d, want %d", secondRecommendationResponse.Code, http.StatusOK)
	}
	secondRecommendation := decodeWorkoutRecommendationResponse(t, secondRecommendationResponse)
	assertRecommendationShapeStable(t, initialRecommendation, secondRecommendation)
}

func decodeEquipmentCatalogResponse(t *testing.T, response *httptest.ResponseRecorder) []exercises.EquipmentDefinition {
	t.Helper()

	var payload []exercises.EquipmentDefinition
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(equipment catalog) error = %v", err)
	}
	return payload
}

func decodeExerciseCatalogResponse(t *testing.T, response *httptest.ResponseRecorder) []exercises.ExerciseDefinition {
	t.Helper()

	var payload []exercises.ExerciseDefinition
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(exercise catalog) error = %v", err)
	}
	return payload
}

func decodeTemplateResponse(t *testing.T, response *httptest.ResponseRecorder) planner.Template {
	t.Helper()

	var payload planner.Template
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(template) error = %v", err)
	}
	return payload
}

func decodeTemplateListResponse(t *testing.T, response *httptest.ResponseRecorder) []planner.Template {
	t.Helper()

	var payload []planner.Template
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(template list) error = %v", err)
	}
	return payload
}

func decodeWeekResponse(t *testing.T, response *httptest.ResponseRecorder) planner.Week {
	t.Helper()

	var payload planner.Week
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(week) error = %v", err)
	}
	return payload
}

func assertRecommendationShapeStable(t *testing.T, before recommendations.WorkoutRecommendation, after recommendations.WorkoutRecommendation) {
	t.Helper()

	if before.Type != after.Type {
		t.Fatalf("recommendation type changed from %q to %q", before.Type, after.Type)
	}
	if before.Reason != after.Reason {
		t.Fatalf("recommendation reason changed from %q to %q", before.Reason, after.Reason)
	}
	if (before.WorkoutID == nil) != (after.WorkoutID == nil) {
		t.Fatalf("recommendation workout id nil state changed: before=%#v after=%#v", before.WorkoutID, after.WorkoutID)
	}
	if before.WorkoutID != nil && after.WorkoutID != nil && *before.WorkoutID != *after.WorkoutID {
		t.Fatalf("recommendation workout id changed from %s to %s", *before.WorkoutID, *after.WorkoutID)
	}
	if before.Evidence.RecoveryWindowHours != after.Evidence.RecoveryWindowHours {
		t.Fatalf("recovery window changed from %d to %d", before.Evidence.RecoveryWindowHours, after.Evidence.RecoveryWindowHours)
	}
}

var _ = profile.MemberProfile{}
