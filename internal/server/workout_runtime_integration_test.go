package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ixxet/apollo/internal/workouts"
)

func TestWorkoutEndpointsRejectUnauthorizedAccess(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	for _, testCase := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/v1/workouts", body: `{}`},
		{method: http.MethodGet, path: "/api/v1/workouts"},
		{method: http.MethodGet, path: "/api/v1/workouts/11111111-1111-1111-1111-111111111111"},
		{method: http.MethodPut, path: "/api/v1/workouts/11111111-1111-1111-1111-111111111111", body: `{"exercises":[]}`},
		{method: http.MethodPost, path: "/api/v1/workouts/11111111-1111-1111-1111-111111111111/finish"},
	} {
		response := env.doJSONRequest(t, testCase.method, testCase.path, testCase.body)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", testCase.method, testCase.path, response.Code, http.StatusUnauthorized)
		}
	}
}

func TestWorkoutRuntimeRoundTripThroughAuthenticatedSession(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, _ := createVerifiedSessionViaHTTP(t, env, "student-workout-010", "workout-010@example.com")

	createResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/workouts", `{"notes":"  push day  "}`, cookie)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("createResponse.Code = %d, want %d", createResponse.Code, http.StatusCreated)
	}
	createdWorkout := decodeWorkoutResponse(t, createResponse)
	if createdWorkout.Status != workouts.StatusInProgress {
		t.Fatalf("createdWorkout.Status = %q, want %q", createdWorkout.Status, workouts.StatusInProgress)
	}
	if createdWorkout.Notes == nil || *createdWorkout.Notes != "push day" {
		t.Fatalf("createdWorkout.Notes = %#v, want push day", createdWorkout.Notes)
	}
	if len(createdWorkout.Exercises) != 0 {
		t.Fatalf("len(createdWorkout.Exercises) = %d, want 0", len(createdWorkout.Exercises))
	}

	listResponse := env.doRequest(t, http.MethodGet, "/api/v1/workouts", nil, cookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("listResponse.Code = %d, want %d", listResponse.Code, http.StatusOK)
	}
	workoutsList := decodeWorkoutListResponse(t, listResponse)
	if len(workoutsList) != 1 || workoutsList[0].ID != createdWorkout.ID {
		t.Fatalf("workoutsList = %#v, want one workout %s", workoutsList, createdWorkout.ID)
	}

	updateResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+createdWorkout.ID.String(), `{"notes":" upper body ","exercises":[{"name":" bench press ","sets":3,"reps":8,"weight_kg":84.5,"rpe":8.5},{"name":"row","sets":3,"reps":10}]}`, cookie)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("updateResponse.Code = %d, want %d", updateResponse.Code, http.StatusOK)
	}
	updatedWorkout := decodeWorkoutResponse(t, updateResponse)
	if updatedWorkout.Notes == nil || *updatedWorkout.Notes != "upper body" {
		t.Fatalf("updatedWorkout.Notes = %#v, want upper body", updatedWorkout.Notes)
	}
	if len(updatedWorkout.Exercises) != 2 {
		t.Fatalf("len(updatedWorkout.Exercises) = %d, want 2", len(updatedWorkout.Exercises))
	}
	if updatedWorkout.Exercises[0].Name != "bench press" || updatedWorkout.Exercises[1].Name != "row" {
		t.Fatalf("updatedWorkout.Exercises names = [%q %q], want [bench press row]", updatedWorkout.Exercises[0].Name, updatedWorkout.Exercises[1].Name)
	}

	getResponse := env.doRequest(t, http.MethodGet, "/api/v1/workouts/"+createdWorkout.ID.String(), nil, cookie)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("getResponse.Code = %d, want %d", getResponse.Code, http.StatusOK)
	}
	gotWorkout := decodeWorkoutResponse(t, getResponse)
	if gotWorkout.ID != createdWorkout.ID {
		t.Fatalf("gotWorkout.ID = %s, want %s", gotWorkout.ID, createdWorkout.ID)
	}
	if len(gotWorkout.Exercises) != 2 {
		t.Fatalf("len(gotWorkout.Exercises) = %d, want 2", len(gotWorkout.Exercises))
	}

	finishResponse := env.doRequest(t, http.MethodPost, "/api/v1/workouts/"+createdWorkout.ID.String()+"/finish", nil, cookie)
	if finishResponse.Code != http.StatusOK {
		t.Fatalf("finishResponse.Code = %d, want %d", finishResponse.Code, http.StatusOK)
	}
	finishedWorkout := decodeWorkoutResponse(t, finishResponse)
	if finishedWorkout.Status != workouts.StatusFinished {
		t.Fatalf("finishedWorkout.Status = %q, want %q", finishedWorkout.Status, workouts.StatusFinished)
	}
	if finishedWorkout.FinishedAt == nil {
		t.Fatal("finishedWorkout.FinishedAt = nil, want timestamp")
	}

	secondFinishResponse := env.doRequest(t, http.MethodPost, "/api/v1/workouts/"+createdWorkout.ID.String()+"/finish", nil, cookie)
	if secondFinishResponse.Code != http.StatusConflict {
		t.Fatalf("secondFinishResponse.Code = %d, want %d", secondFinishResponse.Code, http.StatusConflict)
	}
}

func TestWorkoutRuntimeEnforcesOwnershipAndStateConflicts(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	firstCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-workout-011", "workout-011@example.com")
	secondCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-workout-012", "workout-012@example.com")

	firstCreateResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/workouts", `{}`, firstCookie)
	if firstCreateResponse.Code != http.StatusCreated {
		t.Fatalf("firstCreateResponse.Code = %d, want %d", firstCreateResponse.Code, http.StatusCreated)
	}
	firstWorkout := decodeWorkoutResponse(t, firstCreateResponse)

	secondCreateForSameUser := env.doJSONRequest(t, http.MethodPost, "/api/v1/workouts", `{}`, firstCookie)
	if secondCreateForSameUser.Code != http.StatusConflict {
		t.Fatalf("secondCreateForSameUser.Code = %d, want %d", secondCreateForSameUser.Code, http.StatusConflict)
	}

	secondUserGetResponse := env.doRequest(t, http.MethodGet, "/api/v1/workouts/"+firstWorkout.ID.String(), nil, secondCookie)
	if secondUserGetResponse.Code != http.StatusNotFound {
		t.Fatalf("secondUserGetResponse.Code = %d, want %d", secondUserGetResponse.Code, http.StatusNotFound)
	}
	secondUserUpdateResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+firstWorkout.ID.String(), `{"exercises":[{"name":"row","sets":3,"reps":10}]}`, secondCookie)
	if secondUserUpdateResponse.Code != http.StatusNotFound {
		t.Fatalf("secondUserUpdateResponse.Code = %d, want %d", secondUserUpdateResponse.Code, http.StatusNotFound)
	}
	secondUserFinishResponse := env.doRequest(t, http.MethodPost, "/api/v1/workouts/"+firstWorkout.ID.String()+"/finish", nil, secondCookie)
	if secondUserFinishResponse.Code != http.StatusNotFound {
		t.Fatalf("secondUserFinishResponse.Code = %d, want %d", secondUserFinishResponse.Code, http.StatusNotFound)
	}

	missingExercisesResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+firstWorkout.ID.String(), `{"notes":"updated"}`, firstCookie)
	if missingExercisesResponse.Code != http.StatusBadRequest {
		t.Fatalf("missingExercisesResponse.Code = %d, want %d", missingExercisesResponse.Code, http.StatusBadRequest)
	}

	emptyFinishResponse := env.doRequest(t, http.MethodPost, "/api/v1/workouts/"+firstWorkout.ID.String()+"/finish", nil, firstCookie)
	if emptyFinishResponse.Code != http.StatusBadRequest {
		t.Fatalf("emptyFinishResponse.Code = %d, want %d", emptyFinishResponse.Code, http.StatusBadRequest)
	}

	secondUserCreateResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/workouts", `{}`, secondCookie)
	if secondUserCreateResponse.Code != http.StatusCreated {
		t.Fatalf("secondUserCreateResponse.Code = %d, want %d", secondUserCreateResponse.Code, http.StatusCreated)
	}
}

func TestWorkoutRuntimeListsNewestWorkoutFirst(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, _ := createVerifiedSessionViaHTTP(t, env, "student-workout-013", "workout-013@example.com")

	firstCreateResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/workouts", `{"notes":"first"}`, cookie)
	if firstCreateResponse.Code != http.StatusCreated {
		t.Fatalf("firstCreateResponse.Code = %d, want %d", firstCreateResponse.Code, http.StatusCreated)
	}
	firstWorkout := decodeWorkoutResponse(t, firstCreateResponse)

	firstUpdateResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+firstWorkout.ID.String(), `{"exercises":[{"name":"bike","sets":5,"reps":60}]}`, cookie)
	if firstUpdateResponse.Code != http.StatusOK {
		t.Fatalf("firstUpdateResponse.Code = %d, want %d", firstUpdateResponse.Code, http.StatusOK)
	}
	firstFinishResponse := env.doRequest(t, http.MethodPost, "/api/v1/workouts/"+firstWorkout.ID.String()+"/finish", nil, cookie)
	if firstFinishResponse.Code != http.StatusOK {
		t.Fatalf("firstFinishResponse.Code = %d, want %d", firstFinishResponse.Code, http.StatusOK)
	}

	secondCreateResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/workouts", `{"notes":"second"}`, cookie)
	if secondCreateResponse.Code != http.StatusCreated {
		t.Fatalf("secondCreateResponse.Code = %d, want %d", secondCreateResponse.Code, http.StatusCreated)
	}
	secondWorkout := decodeWorkoutResponse(t, secondCreateResponse)

	listResponse := env.doRequest(t, http.MethodGet, "/api/v1/workouts", nil, cookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("listResponse.Code = %d, want %d", listResponse.Code, http.StatusOK)
	}
	workoutsList := decodeWorkoutListResponse(t, listResponse)
	if len(workoutsList) != 2 {
		t.Fatalf("len(workoutsList) = %d, want 2", len(workoutsList))
	}
	if workoutsList[0].ID != secondWorkout.ID || workoutsList[1].ID != firstWorkout.ID {
		t.Fatalf("workoutsList order = [%s %s], want [%s %s]", workoutsList[0].ID, workoutsList[1].ID, secondWorkout.ID, firstWorkout.ID)
	}
}

func decodeWorkoutResponse(t *testing.T, response *httptest.ResponseRecorder) workouts.Workout {
	t.Helper()

	var workout workouts.Workout
	if err := json.Unmarshal(response.Body.Bytes(), &workout); err != nil {
		t.Fatalf("json.Unmarshal(workout) error = %v", err)
	}

	return workout
}

func decodeWorkoutListResponse(t *testing.T, response *httptest.ResponseRecorder) []workouts.Workout {
	t.Helper()

	var workoutsList []workouts.Workout
	if err := json.Unmarshal(response.Body.Bytes(), &workoutsList); err != nil {
		t.Fatalf("json.Unmarshal(workouts list) error = %v", err)
	}

	return workoutsList
}
