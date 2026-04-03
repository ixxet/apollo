package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/recommendations"
	"github.com/ixxet/apollo/internal/workouts"
)

func TestWorkoutRecommendationRuntimeFollowsDeterministicPrecedence(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	now := time.Now().UTC()
	repository := workouts.NewRepository(env.db.DB)

	t.Run("cold start member gets start first workout", func(t *testing.T) {
		cookie, _ := createVerifiedSessionViaHTTP(t, env, "student-recommendation-010", "recommendation-010@example.com")

		response := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/workout", nil, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("response.Code = %d, want %d", response.Code, http.StatusOK)
		}

		recommendation := decodeWorkoutRecommendationResponse(t, response)
		assertWorkoutRecommendation(t, recommendation, recommendations.TypeStartFirstWorkout, recommendations.ReasonNoFinishedWorkouts, nil)
	})

	t.Run("in progress workout beats older finished workouts", func(t *testing.T) {
		cookie, user := createVerifiedSessionViaHTTP(t, env, "student-recommendation-011", "recommendation-011@example.com")

		finishedWorkoutID := seedFinishedWorkoutAt(t, env, repository, user.ID, "finished", now.Add(-72*time.Hour))
		inProgressWorkoutID := seedInProgressWorkout(t, repository, user.ID, "resume me")

		response := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/workout", nil, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("response.Code = %d, want %d", response.Code, http.StatusOK)
		}

		recommendation := decodeWorkoutRecommendationResponse(t, response)
		assertWorkoutRecommendation(t, recommendation, recommendations.TypeResumeInProgressWorkout, recommendations.ReasonInProgressWorkoutExists, &inProgressWorkoutID)
		if recommendation.Evidence.InProgressWorkoutID == nil || *recommendation.Evidence.InProgressWorkoutID != inProgressWorkoutID {
			t.Fatalf("recommendation.Evidence.InProgressWorkoutID = %#v, want %s", recommendation.Evidence.InProgressWorkoutID, inProgressWorkoutID)
		}
		if recommendation.Evidence.LastFinishedWorkoutID != nil {
			t.Fatalf("recommendation.Evidence.LastFinishedWorkoutID = %#v, want nil when in-progress wins", recommendation.Evidence.LastFinishedWorkoutID)
		}
		if finishedWorkoutID == inProgressWorkoutID {
			t.Fatal("test setup produced identical workout ids")
		}
	})

	t.Run("most recent finished workout inside recovery window returns recovery day", func(t *testing.T) {
		cookie, user := createVerifiedSessionViaHTTP(t, env, "student-recommendation-012", "recommendation-012@example.com")

		_ = seedFinishedWorkoutAt(t, env, repository, user.ID, "older", now.Add(-72*time.Hour))
		recentWorkoutID := seedFinishedWorkoutAt(t, env, repository, user.ID, "recent", now.Add(-23*time.Hour))

		response := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/workout", nil, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("response.Code = %d, want %d", response.Code, http.StatusOK)
		}

		recommendation := decodeWorkoutRecommendationResponse(t, response)
		assertWorkoutRecommendation(t, recommendation, recommendations.TypeRecoveryDay, recommendations.ReasonLastFinishedWithinRecoveryWindow, &recentWorkoutID)
		if recommendation.Evidence.LastFinishedWorkoutID == nil || *recommendation.Evidence.LastFinishedWorkoutID != recentWorkoutID {
			t.Fatalf("recommendation.Evidence.LastFinishedWorkoutID = %#v, want %s", recommendation.Evidence.LastFinishedWorkoutID, recentWorkoutID)
		}
		if recommendation.Evidence.RecoveryWindowHours != 24 {
			t.Fatalf("recommendation.Evidence.RecoveryWindowHours = %d, want 24", recommendation.Evidence.RecoveryWindowHours)
		}
	})

	t.Run("older finished workout returns repeat recommendation", func(t *testing.T) {
		cookie, user := createVerifiedSessionViaHTTP(t, env, "student-recommendation-013", "recommendation-013@example.com")

		finishedWorkoutID := seedFinishedWorkoutAt(t, env, repository, user.ID, "older", now.Add(-25*time.Hour))

		response := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/workout", nil, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("response.Code = %d, want %d", response.Code, http.StatusOK)
		}

		recommendation := decodeWorkoutRecommendationResponse(t, response)
		assertWorkoutRecommendation(t, recommendation, recommendations.TypeRepeatLastFinishedWorkout, recommendations.ReasonLastFinishedOutsideRecoveryWindow, &finishedWorkoutID)
	})
}

func decodeWorkoutRecommendationResponse(t *testing.T, response *httptest.ResponseRecorder) recommendations.WorkoutRecommendation {
	t.Helper()

	var recommendation recommendations.WorkoutRecommendation
	if err := json.Unmarshal(response.Body.Bytes(), &recommendation); err != nil {
		t.Fatalf("json.Unmarshal(recommendation) error = %v", err)
	}

	return recommendation
}

func assertWorkoutRecommendation(t *testing.T, recommendation recommendations.WorkoutRecommendation, wantType recommendations.RecommendationType, wantReason recommendations.RecommendationReason, wantWorkoutID *uuid.UUID) {
	t.Helper()

	if recommendation.Type != wantType {
		t.Fatalf("recommendation.Type = %q, want %q", recommendation.Type, wantType)
	}
	if recommendation.Reason != wantReason {
		t.Fatalf("recommendation.Reason = %q, want %q", recommendation.Reason, wantReason)
	}
	if recommendation.GeneratedAt.IsZero() {
		t.Fatal("recommendation.GeneratedAt = zero, want timestamp")
	}
	switch {
	case wantWorkoutID == nil && recommendation.WorkoutID != nil:
		t.Fatalf("recommendation.WorkoutID = %s, want nil", *recommendation.WorkoutID)
	case wantWorkoutID != nil && (recommendation.WorkoutID == nil || *recommendation.WorkoutID != *wantWorkoutID):
		t.Fatalf("recommendation.WorkoutID = %#v, want %s", recommendation.WorkoutID, *wantWorkoutID)
	}
}

func seedInProgressWorkout(t *testing.T, repository *workouts.Repository, userID uuid.UUID, notes string) uuid.UUID {
	t.Helper()

	workout, err := repository.CreateWorkout(context.Background(), userID, stringPtr(notes))
	if err != nil {
		t.Fatalf("CreateWorkout(%q) error = %v", notes, err)
	}

	return workout.ID
}

func seedFinishedWorkoutAt(t *testing.T, env *authProfileServerEnv, repository *workouts.Repository, userID uuid.UUID, notes string, finishedAt time.Time) uuid.UUID {
	t.Helper()

	workout, err := repository.CreateWorkout(context.Background(), userID, stringPtr(notes))
	if err != nil {
		t.Fatalf("CreateWorkout(%q) error = %v", notes, err)
	}
	if _, _, err := repository.ReplaceWorkoutDraft(context.Background(), workout.ID, userID, stringPtr(notes), []workouts.ExerciseDraft{
		{Name: "squat", Sets: 3, Reps: 5},
	}); err != nil {
		t.Fatalf("ReplaceWorkoutDraft(%q) error = %v", notes, err)
	}
	if _, err := repository.FinishWorkout(context.Background(), workout.ID, userID, finishedAt); err != nil {
		t.Fatalf("FinishWorkout(%q) error = %v", notes, err)
	}

	startedAt := finishedAt.Add(-90 * time.Minute)
	if _, err := env.db.DB.Exec(context.Background(), "UPDATE apollo.workouts SET started_at = $2, finished_at = $3 WHERE id = $1", workout.ID, startedAt, finishedAt); err != nil {
		t.Fatalf("Exec(update workout timestamps %q) error = %v", notes, err)
	}

	return workout.ID
}
