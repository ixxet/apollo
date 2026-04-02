package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/workouts"
)

type stubWorkoutManager struct {
	createResponse workouts.Workout
	createErr      error
	createInput    workouts.CreateInput

	listResponse []workouts.Workout
	listErr      error

	getResponse workouts.Workout
	getErr      error

	updateResponse workouts.Workout
	updateErr      error
	updateInput    workouts.UpdateInput

	finishResponse workouts.Workout
	finishErr      error
}

func (s *stubWorkoutManager) CreateWorkout(_ context.Context, _ uuid.UUID, input workouts.CreateInput) (workouts.Workout, error) {
	s.createInput = input
	if s.createErr != nil {
		return workouts.Workout{}, s.createErr
	}
	return s.createResponse, nil
}

func (s *stubWorkoutManager) ListWorkouts(context.Context, uuid.UUID) ([]workouts.Workout, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.listResponse, nil
}

func (s *stubWorkoutManager) GetWorkout(context.Context, uuid.UUID, uuid.UUID) (workouts.Workout, error) {
	if s.getErr != nil {
		return workouts.Workout{}, s.getErr
	}
	return s.getResponse, nil
}

func (s *stubWorkoutManager) UpdateWorkout(_ context.Context, _ uuid.UUID, _ uuid.UUID, input workouts.UpdateInput) (workouts.Workout, error) {
	s.updateInput = input
	if s.updateErr != nil {
		return workouts.Workout{}, s.updateErr
	}
	return s.updateResponse, nil
}

func (s *stubWorkoutManager) FinishWorkout(context.Context, uuid.UUID, uuid.UUID) (workouts.Workout, error) {
	if s.finishErr != nil {
		return workouts.Workout{}, s.finishErr
	}
	return s.finishResponse, nil
}

func TestWorkoutEndpointsRequireAuthentication(t *testing.T) {
	handler := NewHandler(Dependencies{
		Auth:     stubAuthenticator{cookieName: "apollo_session"},
		Workouts: &stubWorkoutManager{},
	})

	for _, testCase := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/workouts"},
		{method: http.MethodGet, path: "/api/v1/workouts/11111111-1111-1111-1111-111111111111"},
		{method: http.MethodPost, path: "/api/v1/workouts/11111111-1111-1111-1111-111111111111/finish"},
	} {
		request := httptest.NewRequest(testCase.method, testCase.path, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", testCase.method, testCase.path, recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestWorkoutEndpointsReturnStableWorkoutShapes(t *testing.T) {
	workoutID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	startedAt := time.Date(2026, 4, 2, 20, 0, 0, 0, time.UTC)
	finishedAt := time.Date(2026, 4, 2, 21, 15, 0, 0, time.UTC)
	manager := &stubWorkoutManager{
		createResponse: workouts.Workout{
			ID:         workoutID,
			Status:     workouts.StatusInProgress,
			StartedAt:  startedAt,
			FinishedAt: nil,
			Notes:      stringPtr("push day"),
		},
		listResponse: []workouts.Workout{
			{
				ID:         workoutID,
				Status:     workouts.StatusFinished,
				StartedAt:  startedAt,
				FinishedAt: &finishedAt,
				Notes:      stringPtr("push day"),
				Exercises: []workouts.Exercise{
					{Position: 1, Name: "bench press", Sets: 3, Reps: 8},
				},
			},
		},
		getResponse: workouts.Workout{
			ID:         workoutID,
			Status:     workouts.StatusFinished,
			StartedAt:  startedAt,
			FinishedAt: &finishedAt,
			Notes:      stringPtr("push day"),
			Exercises: []workouts.Exercise{
				{Position: 1, Name: "bench press", Sets: 3, Reps: 8},
			},
		},
		updateResponse: workouts.Workout{
			ID:        workoutID,
			Status:    workouts.StatusInProgress,
			StartedAt: startedAt,
			Notes:     stringPtr("updated"),
			Exercises: []workouts.Exercise{
				{Position: 1, Name: "front squat", Sets: 4, Reps: 5},
			},
		},
		finishResponse: workouts.Workout{
			ID:         workoutID,
			Status:     workouts.StatusFinished,
			StartedAt:  startedAt,
			FinishedAt: &finishedAt,
			Exercises: []workouts.Exercise{
				{Position: 1, Name: "front squat", Sets: 4, Reps: 5},
			},
		},
	}
	handler := NewHandler(Dependencies{
		Auth: stubAuthenticator{
			cookieName: "apollo_session",
			principal:  auth.Principal{UserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
		},
		Workouts: manager,
	})

	cookie := &http.Cookie{Name: "apollo_session", Value: "signed"}

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/workouts", bytes.NewBufferString(`{"notes":"push day"}`))
	createRequest.AddCookie(cookie)
	createRecorder := httptest.NewRecorder()
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRecorder.Code, http.StatusCreated)
	}
	if manager.createInput.Notes == nil || *manager.createInput.Notes != "push day" {
		t.Fatalf("manager.createInput.Notes = %#v, want push day", manager.createInput.Notes)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/workouts", nil)
	listRequest.AddCookie(cookie)
	listRecorder := httptest.NewRecorder()
	handler.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRecorder.Code, http.StatusOK)
	}
	var listPayload []map[string]any
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("json.Unmarshal(list) error = %v", err)
	}
	if len(listPayload) != 1 || listPayload[0]["status"] != workouts.StatusFinished {
		t.Fatalf("listPayload = %#v, want one finished workout", listPayload)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/workouts/"+workoutID.String(), nil)
	getRequest.AddCookie(cookie)
	getRecorder := httptest.NewRecorder()
	handler.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", getRecorder.Code, http.StatusOK)
	}

	updateRequest := httptest.NewRequest(http.MethodPut, "/api/v1/workouts/"+workoutID.String(), bytes.NewBufferString(`{"notes":"updated","exercises":[{"name":"front squat","sets":4,"reps":5}]}`))
	updateRequest.AddCookie(cookie)
	updateRecorder := httptest.NewRecorder()
	handler.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d", updateRecorder.Code, http.StatusOK)
	}
	if manager.updateInput.Exercises == nil || len(*manager.updateInput.Exercises) != 1 {
		t.Fatalf("manager.updateInput.Exercises = %#v, want one exercise", manager.updateInput.Exercises)
	}

	finishRequest := httptest.NewRequest(http.MethodPost, "/api/v1/workouts/"+workoutID.String()+"/finish", nil)
	finishRequest.AddCookie(cookie)
	finishRecorder := httptest.NewRecorder()
	handler.ServeHTTP(finishRecorder, finishRequest)
	if finishRecorder.Code != http.StatusOK {
		t.Fatalf("finish status = %d, want %d", finishRecorder.Code, http.StatusOK)
	}
}

func TestWorkoutEndpointsMapErrorsClearly(t *testing.T) {
	workoutID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testCases := []struct {
		name       string
		method     string
		path       string
		body       string
		manager    *stubWorkoutManager
		wantStatus int
	}{
		{
			name:       "invalid workout id returns bad request",
			method:     http.MethodGet,
			path:       "/api/v1/workouts/not-a-uuid",
			manager:    &stubWorkoutManager{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "duplicate in-progress create returns conflict",
			method:     http.MethodPost,
			path:       "/api/v1/workouts",
			body:       `{}`,
			manager:    &stubWorkoutManager{createErr: workouts.ErrWorkoutAlreadyInProgress},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "missing exercise payload returns bad request",
			method:     http.MethodPut,
			path:       "/api/v1/workouts/" + workoutID.String(),
			body:       `{"notes":"updated"}`,
			manager:    &stubWorkoutManager{updateErr: workouts.ErrExercisePayloadRequired},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "finished workout update returns conflict",
			method:     http.MethodPut,
			path:       "/api/v1/workouts/" + workoutID.String(),
			body:       `{"exercises":[]}`,
			manager:    &stubWorkoutManager{updateErr: workouts.ErrWorkoutFinished},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "finish missing workout returns not found",
			method:     http.MethodPost,
			path:       "/api/v1/workouts/" + workoutID.String() + "/finish",
			manager:    &stubWorkoutManager{finishErr: workouts.ErrNotFound},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "finish empty workout returns bad request",
			method:     http.MethodPost,
			path:       "/api/v1/workouts/" + workoutID.String() + "/finish",
			manager:    &stubWorkoutManager{finishErr: workouts.ErrCannotFinishEmptyWorkout},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unexpected list failure returns internal server error",
			method:     http.MethodGet,
			path:       "/api/v1/workouts",
			manager:    &stubWorkoutManager{listErr: errors.New("boom")},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			handler := NewHandler(Dependencies{
				Auth: stubAuthenticator{
					cookieName: "apollo_session",
					principal:  auth.Principal{UserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
				},
				Workouts: testCase.manager,
			})

			request := httptest.NewRequest(testCase.method, testCase.path, bytes.NewBufferString(testCase.body))
			request.AddCookie(&http.Cookie{Name: "apollo_session", Value: "signed"})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, testCase.wantStatus, recorder.Body.String())
			}
		})
	}
}

func stringPtr(value string) *string {
	return &value
}
