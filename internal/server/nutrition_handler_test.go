package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/nutrition"
)

type stubNutritionManager struct {
	listMealLogsResponse       []nutrition.MealLog
	listMealLogsErr            error
	createMealLogResponse      nutrition.MealLog
	createMealLogErr           error
	updateMealLogResponse      nutrition.MealLog
	updateMealLogErr           error
	listMealTemplatesResponse  []nutrition.MealTemplate
	listMealTemplatesErr       error
	createMealTemplateResponse nutrition.MealTemplate
	createMealTemplateErr      error
	updateMealTemplateResponse nutrition.MealTemplate
	updateMealTemplateErr      error
	recommendationResponse     nutrition.Recommendation
	recommendationErr          error
}

func (s stubNutritionManager) ListMealLogs(context.Context, uuid.UUID) ([]nutrition.MealLog, error) {
	if s.listMealLogsErr != nil {
		return nil, s.listMealLogsErr
	}
	return s.listMealLogsResponse, nil
}

func (s stubNutritionManager) CreateMealLog(context.Context, uuid.UUID, nutrition.MealLogInput) (nutrition.MealLog, error) {
	if s.createMealLogErr != nil {
		return nutrition.MealLog{}, s.createMealLogErr
	}
	return s.createMealLogResponse, nil
}

func (s stubNutritionManager) UpdateMealLog(context.Context, uuid.UUID, uuid.UUID, nutrition.MealLogInput) (nutrition.MealLog, error) {
	if s.updateMealLogErr != nil {
		return nutrition.MealLog{}, s.updateMealLogErr
	}
	return s.updateMealLogResponse, nil
}

func (s stubNutritionManager) ListMealTemplates(context.Context, uuid.UUID) ([]nutrition.MealTemplate, error) {
	if s.listMealTemplatesErr != nil {
		return nil, s.listMealTemplatesErr
	}
	return s.listMealTemplatesResponse, nil
}

func (s stubNutritionManager) CreateMealTemplate(context.Context, uuid.UUID, nutrition.MealTemplateInput) (nutrition.MealTemplate, error) {
	if s.createMealTemplateErr != nil {
		return nutrition.MealTemplate{}, s.createMealTemplateErr
	}
	return s.createMealTemplateResponse, nil
}

func (s stubNutritionManager) UpdateMealTemplate(context.Context, uuid.UUID, uuid.UUID, nutrition.MealTemplateInput) (nutrition.MealTemplate, error) {
	if s.updateMealTemplateErr != nil {
		return nutrition.MealTemplate{}, s.updateMealTemplateErr
	}
	return s.updateMealTemplateResponse, nil
}

func (s stubNutritionManager) GetRecommendation(context.Context, uuid.UUID) (nutrition.Recommendation, error) {
	if s.recommendationErr != nil {
		return nutrition.Recommendation{}, s.recommendationErr
	}
	return s.recommendationResponse, nil
}

func TestNutritionEndpointsRequireAuthentication(t *testing.T) {
	handler := NewHandler(Dependencies{
		Auth:      stubAuthenticator{cookieName: "apollo_session"},
		Nutrition: stubNutritionManager{},
	})

	for _, testCase := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/v1/recommendations/nutrition"},
		{method: http.MethodGet, path: "/api/v1/nutrition/meal-logs"},
		{method: http.MethodPost, path: "/api/v1/nutrition/meal-logs", body: `{}`},
		{method: http.MethodPut, path: "/api/v1/nutrition/meal-logs/11111111-1111-1111-1111-111111111111", body: `{}`},
		{method: http.MethodGet, path: "/api/v1/nutrition/meal-templates"},
		{method: http.MethodPost, path: "/api/v1/nutrition/meal-templates", body: `{}`},
		{method: http.MethodPut, path: "/api/v1/nutrition/meal-templates/11111111-1111-1111-1111-111111111111", body: `{}`},
	} {
		request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", testCase.method, testCase.path, recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestNutritionRecommendationEndpointReturnsStableShape(t *testing.T) {
	generatedAt := time.Date(2026, 4, 10, 17, 0, 0, 0, time.UTC)
	handler := NewHandler(Dependencies{
		Auth: stubAuthenticator{
			cookieName: "apollo_session",
			principal:  auth.Principal{UserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
		},
		Nutrition: stubNutritionManager{
			recommendationResponse: nutrition.Recommendation{
				Kind:              nutrition.KindHold,
				GoalKey:           "build-strength",
				DailyCalories:     nutrition.Range{Min: 2100, Max: 2600},
				DailyProteinGrams: nutrition.Range{Min: 120, Max: 175},
				DailyCarbsGrams:   nutrition.Range{Min: 220, Max: 320},
				DailyFatGrams:     nutrition.Range{Min: 60, Max: 90},
				StrategyFlags:     []string{"budget_first", "template_reuse_first"},
				Explanation: nutrition.RecommendationExplanation{
					ReasonCodes: []string{"budget_constrained", "recent_history_stabilized"},
					Evidence: nutrition.RecommendationEvidence{
						RecentMealLogCount:   5,
						RecentLoggedDayCount: 5,
					},
					Limitations: []string{"non-clinical guidance only; not medical advice or diagnosis"},
				},
				PolicyVersion: nutrition.PolicyVersion,
				GeneratedAt:   generatedAt,
			},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/recommendations/nutrition", nil)
	request.AddCookie(&http.Cookie{Name: "apollo_session", Value: "signed"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["kind"] != string(nutrition.KindHold) {
		t.Fatalf("kind = %#v, want %q", payload["kind"], nutrition.KindHold)
	}
	if payload["goal_key"] != "build-strength" {
		t.Fatalf("goal_key = %#v, want build-strength", payload["goal_key"])
	}
	dailyCalories, ok := payload["daily_calories"].(map[string]any)
	if !ok {
		t.Fatalf("daily_calories = %#v, want object", payload["daily_calories"])
	}
	if dailyCalories["min"] != float64(2100) || dailyCalories["max"] != float64(2600) {
		t.Fatalf("daily_calories = %#v, want min/max 2100/2600", dailyCalories)
	}
	if payload["policy_version"] != nutrition.PolicyVersion {
		t.Fatalf("policy_version = %#v, want %q", payload["policy_version"], nutrition.PolicyVersion)
	}
}

func TestNutritionEndpointsMapValidationOwnershipAndConflictErrors(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		method     string
		path       string
		body       string
		manager    stubNutritionManager
		wantStatus int
	}{
		{
			name:       "meal log validation maps to bad request",
			method:     http.MethodPost,
			path:       "/api/v1/nutrition/meal-logs",
			body:       `{"name":"Lunch","meal_type":"lunch"}`,
			manager:    stubNutritionManager{createMealLogErr: nutrition.ErrMealLogNutritionRequired},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong owner meal log update maps to not found",
			method:     http.MethodPut,
			path:       "/api/v1/nutrition/meal-logs/11111111-1111-1111-1111-111111111111",
			body:       `{"name":"Lunch","meal_type":"lunch","logged_at":"2026-04-10T12:00:00Z","calories":600}`,
			manager:    stubNutritionManager{updateMealLogErr: nutrition.ErrMealLogNotFound},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "duplicate template name maps to conflict",
			method:     http.MethodPost,
			path:       "/api/v1/nutrition/meal-templates",
			body:       `{"name":"Overnight Oats","meal_type":"breakfast","calories":520}`,
			manager:    stubNutritionManager{createMealTemplateErr: nutrition.ErrDuplicateMealTemplateName},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "invalid template payload maps to bad request",
			method:     http.MethodPut,
			path:       "/api/v1/nutrition/meal-templates/11111111-1111-1111-1111-111111111111",
			body:       `{"name":"Overnight Oats","meal_type":"breakfast"}`,
			manager:    stubNutritionManager{updateMealTemplateErr: nutrition.ErrMealTemplateNutritionRequired},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unexpected recommendation failure maps to internal error",
			method:     http.MethodGet,
			path:       "/api/v1/recommendations/nutrition",
			manager:    stubNutritionManager{recommendationErr: errors.New("boom")},
			wantStatus: http.StatusInternalServerError,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			handler := NewHandler(Dependencies{
				Auth: stubAuthenticator{
					cookieName: "apollo_session",
					principal:  auth.Principal{UserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
				},
				Nutrition: testCase.manager,
			})

			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			request.AddCookie(&http.Cookie{Name: "apollo_session", Value: "signed"})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, testCase.wantStatus)
			}
		})
	}
}
