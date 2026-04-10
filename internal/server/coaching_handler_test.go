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
	"github.com/ixxet/apollo/internal/coaching"
	"github.com/ixxet/apollo/internal/helper"
	"github.com/ixxet/apollo/internal/planner"
)

type stubCoachingManager struct {
	recommendation      coaching.CoachingRecommendation
	recommendationErr   error
	helperRead          coaching.CoachingHelperRead
	helperReadErr       error
	helperWhy           coaching.CoachingHelperWhy
	helperWhyErr        error
	variationPreview    coaching.CoachingVariationPreview
	variationPreviewErr error
	effortResponse      coaching.EffortFeedback
	effortErr           error
	recoveryResponse    coaching.RecoveryFeedback
	recoveryErr         error
}

func (s stubCoachingManager) GetCoachingRecommendation(context.Context, uuid.UUID, string) (coaching.CoachingRecommendation, error) {
	if s.recommendationErr != nil {
		return coaching.CoachingRecommendation{}, s.recommendationErr
	}
	return s.recommendation, nil
}

func (s stubCoachingManager) GetHelperRead(context.Context, uuid.UUID, string) (coaching.CoachingHelperRead, error) {
	if s.helperReadErr != nil {
		return coaching.CoachingHelperRead{}, s.helperReadErr
	}
	return s.helperRead, nil
}

func (s stubCoachingManager) AskWhy(context.Context, uuid.UUID, string, string) (coaching.CoachingHelperWhy, error) {
	if s.helperWhyErr != nil {
		return coaching.CoachingHelperWhy{}, s.helperWhyErr
	}
	return s.helperWhy, nil
}

func (s stubCoachingManager) PreviewVariation(context.Context, uuid.UUID, string, string) (coaching.CoachingVariationPreview, error) {
	if s.variationPreviewErr != nil {
		return coaching.CoachingVariationPreview{}, s.variationPreviewErr
	}
	return s.variationPreview, nil
}

func (s stubCoachingManager) PutEffortFeedback(context.Context, uuid.UUID, uuid.UUID, coaching.EffortFeedbackInput) (coaching.EffortFeedback, error) {
	if s.effortErr != nil {
		return coaching.EffortFeedback{}, s.effortErr
	}
	return s.effortResponse, nil
}

func (s stubCoachingManager) PutRecoveryFeedback(context.Context, uuid.UUID, uuid.UUID, coaching.RecoveryFeedbackInput) (coaching.RecoveryFeedback, error) {
	if s.recoveryErr != nil {
		return coaching.RecoveryFeedback{}, s.recoveryErr
	}
	return s.recoveryResponse, nil
}

func TestCoachingEndpointsRequireAuthentication(t *testing.T) {
	handler := NewHandler(Dependencies{
		Auth:     stubAuthenticator{cookieName: "apollo_session"},
		Coaching: stubCoachingManager{},
	})

	for _, testCase := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/v1/recommendations/coaching?week_start=2026-04-06"},
		{method: http.MethodGet, path: "/api/v1/helpers/coaching?week_start=2026-04-06"},
		{method: http.MethodGet, path: "/api/v1/helpers/coaching/why?week_start=2026-04-06&topic=proposal"},
		{method: http.MethodGet, path: "/api/v1/helpers/coaching/variation?week_start=2026-04-06&variation=easier"},
		{method: http.MethodPut, path: "/api/v1/workouts/11111111-1111-1111-1111-111111111111/effort-feedback", body: `{"effort_level":"easy"}`},
		{method: http.MethodPut, path: "/api/v1/workouts/11111111-1111-1111-1111-111111111111/recovery-feedback", body: `{"recovery_level":"recovered"}`},
	} {
		request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", testCase.method, testCase.path, recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestCoachingRecommendationEndpointReturnsStableShape(t *testing.T) {
	workoutID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	generatedAt := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	recommendation := coaching.CoachingRecommendation{
		Kind:            coaching.KindProgress,
		TargetWeekStart: "2026-04-06",
		SourceWorkoutID: &workoutID,
		TrainingGoal: coaching.TrainingGoal{
			GoalKey:              "build-strength",
			TargetDaysPerWeek:    4,
			TargetSessionMinutes: 60,
		},
		Proposal: coaching.PlanChangeProposal{
			WeekStart: "2026-04-06",
			Changes: []coaching.PlanChange{
				{
					DayIndex:        0,
					SessionPosition: 1,
					ItemPosition:    1,
					Before:          coaching.PlanItemValues{Sets: 5, Reps: 5, WeightKg: floatPtr(100)},
					After:           coaching.PlanItemValues{Sets: 5, Reps: 5, WeightKg: floatPtr(102.5)},
				},
			},
		},
		Explanation: coaching.CoachingExplanation{
			ReasonCodes: []string{"effort_easy_and_recovery_recovered"},
			Evidence: coaching.CoachingExplanationEvidence{
				GoalKey:              "build-strength",
				TargetDaysPerWeek:    4,
				TargetSessionMinutes: 60,
				SourceWorkoutID:      &workoutID,
				EffortLevel:          stringPtr("easy"),
				RecoveryLevel:        stringPtr("recovered"),
			},
			Limitations: []string{},
		},
		PolicyVersion: coaching.PolicyVersion,
		GeneratedAt:   generatedAt,
	}

	handler := NewHandler(Dependencies{
		Auth: stubAuthenticator{
			cookieName: "apollo_session",
			principal:  auth.Principal{UserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
		},
		Coaching: stubCoachingManager{recommendation: recommendation},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/recommendations/coaching?week_start=2026-04-06", nil)
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
	if payload["kind"] != string(coaching.KindProgress) {
		t.Fatalf("kind = %#v, want %q", payload["kind"], coaching.KindProgress)
	}
	if payload["target_week_start"] != "2026-04-06" {
		t.Fatalf("target_week_start = %#v, want 2026-04-06", payload["target_week_start"])
	}
	if payload["source_workout_id"] != workoutID.String() {
		t.Fatalf("source_workout_id = %#v, want %s", payload["source_workout_id"], workoutID)
	}
}

func TestCoachingHelperEndpointsReturnStableShapes(t *testing.T) {
	workoutID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	manager := stubCoachingManager{
		helperRead: coaching.CoachingHelperRead{
			PreviewMode: "read_only",
			Recommendation: coaching.CoachingRecommendation{
				Kind:            coaching.KindProgress,
				TargetWeekStart: "2026-04-06",
				SourceWorkoutID: &workoutID,
			},
			Summary: coachingHelperSummaryFixture(),
		},
		helperWhy: coaching.CoachingHelperWhy{
			PreviewMode: "read_only",
			Topic:       coaching.WhyTopicProposal,
			Recommendation: coaching.CoachingRecommendation{
				Kind: coaching.KindProgress,
			},
			Summary: coachingHelperSummaryFixture(),
		},
		variationPreview: coaching.CoachingVariationPreview{
			PreviewMode: "read_only",
			Variation:   coaching.VariationEasier,
			BaseKind:    coaching.KindProgress,
			Recommendation: coaching.CoachingRecommendation{
				Kind: coaching.KindHold,
			},
			Summary: coachingHelperSummaryFixture(),
		},
	}
	handler := NewHandler(Dependencies{
		Auth: stubAuthenticator{
			cookieName: "apollo_session",
			principal:  auth.Principal{UserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
		},
		Coaching: manager,
	})

	for _, testCase := range []struct {
		path      string
		wantField string
		wantValue string
	}{
		{path: "/api/v1/helpers/coaching?week_start=2026-04-06", wantField: "preview_mode", wantValue: "read_only"},
		{path: "/api/v1/helpers/coaching/why?week_start=2026-04-06&topic=proposal", wantField: "topic", wantValue: coaching.WhyTopicProposal},
		{path: "/api/v1/helpers/coaching/variation?week_start=2026-04-06&variation=easier", wantField: "variation", wantValue: coaching.VariationEasier},
	} {
		request := httptest.NewRequest(http.MethodGet, testCase.path, nil)
		request.AddCookie(&http.Cookie{Name: "apollo_session", Value: "signed"})
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", testCase.path, recorder.Code, http.StatusOK)
		}

		var payload map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(%s) error = %v", testCase.path, err)
		}
		if payload[testCase.wantField] != testCase.wantValue {
			t.Fatalf("%s %s = %#v, want %q", testCase.path, testCase.wantField, payload[testCase.wantField], testCase.wantValue)
		}
	}
}

func TestCoachingEndpointsMapValidationAndOwnershipErrors(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		method     string
		path       string
		body       string
		manager    stubCoachingManager
		wantStatus int
	}{
		{
			name:       "missing week_start fails cleanly",
			method:     http.MethodGet,
			path:       "/api/v1/recommendations/coaching",
			manager:    stubCoachingManager{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing helper topic fails cleanly",
			method:     http.MethodGet,
			path:       "/api/v1/helpers/coaching/why?week_start=2026-04-06",
			manager:    stubCoachingManager{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unsupported helper topic maps to bad request",
			method:     http.MethodGet,
			path:       "/api/v1/helpers/coaching/why?week_start=2026-04-06&topic=timeline",
			manager:    stubCoachingManager{helperWhyErr: helper.ErrUnsupportedWhyTopic},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unsupported helper variation maps to bad request",
			method:     http.MethodGet,
			path:       "/api/v1/helpers/coaching/variation?week_start=2026-04-06&variation=longer",
			manager:    stubCoachingManager{variationPreviewErr: helper.ErrUnsupportedVariation},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid week_start maps to bad request",
			method:     http.MethodGet,
			path:       "/api/v1/recommendations/coaching?week_start=2026-04-07",
			manager:    stubCoachingManager{recommendationErr: planner.ErrWeekStartInvalid},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad effort payload maps to bad request",
			method:     http.MethodPut,
			path:       "/api/v1/workouts/11111111-1111-1111-1111-111111111111/effort-feedback",
			body:       `{"effort_level":"elite"}`,
			manager:    stubCoachingManager{effortErr: coaching.ErrInvalidEffortLevel},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong owner effort write maps to not found",
			method:     http.MethodPut,
			path:       "/api/v1/workouts/11111111-1111-1111-1111-111111111111/effort-feedback",
			body:       `{"effort_level":"easy"}`,
			manager:    stubCoachingManager{effortErr: coaching.ErrWorkoutNotFound},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "in-progress recovery write maps to conflict",
			method:     http.MethodPut,
			path:       "/api/v1/workouts/11111111-1111-1111-1111-111111111111/recovery-feedback",
			body:       `{"recovery_level":"recovered"}`,
			manager:    stubCoachingManager{recoveryErr: coaching.ErrWorkoutNotFinished},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "unexpected coaching failure maps to internal server error",
			method:     http.MethodGet,
			path:       "/api/v1/recommendations/coaching?week_start=2026-04-06",
			manager:    stubCoachingManager{recommendationErr: errors.New("boom")},
			wantStatus: http.StatusInternalServerError,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			handler := NewHandler(Dependencies{
				Auth: stubAuthenticator{
					cookieName: "apollo_session",
					principal:  auth.Principal{UserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
				},
				Coaching: testCase.manager,
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

func coachingHelperSummaryFixture() helper.Summary {
	return helper.Summary{
		Headline: "Progress the next planned week slightly",
		Detail:   "The helper stays read-only and tied to deterministic coaching output.",
		Bullets:  []string{"planner preview changes: 1 item(s)"},
	}
}

func floatPtr(value float64) *float64 {
	return &value
}
