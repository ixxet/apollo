package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/recommendations"
)

type stubRecommendationReader struct {
	response recommendations.WorkoutRecommendation
	err      error
}

func (s stubRecommendationReader) GetWorkoutRecommendation(context.Context, uuid.UUID) (recommendations.WorkoutRecommendation, error) {
	if s.err != nil {
		return recommendations.WorkoutRecommendation{}, s.err
	}

	return s.response, nil
}

func TestWorkoutRecommendationEndpointRequiresAuthentication(t *testing.T) {
	handler := NewHandler(Dependencies{
		Auth:            stubAuthenticator{cookieName: "apollo_session"},
		Recommendations: stubRecommendationReader{},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/recommendations/workout", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestWorkoutRecommendationEndpointReturnsTheStableRecommendationShape(t *testing.T) {
	workoutID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	generatedAt := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	finishedAt := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)
	handler := NewHandler(Dependencies{
		Auth: stubAuthenticator{
			cookieName: "apollo_session",
			principal:  auth.Principal{UserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
		},
		Recommendations: stubRecommendationReader{
			response: recommendations.WorkoutRecommendation{
				Type:        recommendations.TypeRepeatLastFinishedWorkout,
				Reason:      recommendations.ReasonLastFinishedOutsideRecoveryWindow,
				WorkoutID:   &workoutID,
				GeneratedAt: generatedAt,
				Evidence: recommendations.Evidence{
					LastFinishedWorkoutID: &workoutID,
					LastFinishedAt:        &finishedAt,
					RecoveryWindowHours:   24,
				},
			},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/recommendations/workout", nil)
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
	if payload["type"] != string(recommendations.TypeRepeatLastFinishedWorkout) {
		t.Fatalf("type = %#v, want %q", payload["type"], recommendations.TypeRepeatLastFinishedWorkout)
	}
	if payload["reason"] != string(recommendations.ReasonLastFinishedOutsideRecoveryWindow) {
		t.Fatalf("reason = %#v, want %q", payload["reason"], recommendations.ReasonLastFinishedOutsideRecoveryWindow)
	}
	if payload["workout_id"] != workoutID.String() {
		t.Fatalf("workout_id = %#v, want %q", payload["workout_id"], workoutID.String())
	}
	evidence, ok := payload["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence = %#v, want object", payload["evidence"])
	}
	if evidence["last_finished_workout_id"] != workoutID.String() {
		t.Fatalf("evidence.last_finished_workout_id = %#v, want %q", evidence["last_finished_workout_id"], workoutID.String())
	}
	if evidence["recovery_window_hours"] != float64(24) {
		t.Fatalf("evidence.recovery_window_hours = %#v, want 24", evidence["recovery_window_hours"])
	}
}

func TestWorkoutRecommendationEndpointMapsLookupFailuresClearly(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "invalid finished workout state returns internal server error",
			err:        recommendations.ErrInvalidFinishedWorkoutState,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "unexpected failure returns internal server error",
			err:        errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			handler := NewHandler(Dependencies{
				Auth: stubAuthenticator{
					cookieName: "apollo_session",
					principal:  auth.Principal{UserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
				},
				Recommendations: stubRecommendationReader{err: testCase.err},
			})

			request := httptest.NewRequest(http.MethodGet, "/api/v1/recommendations/workout", nil)
			request.AddCookie(&http.Cookie{Name: "apollo_session", Value: "signed"})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, testCase.wantStatus)
			}
		})
	}
}
