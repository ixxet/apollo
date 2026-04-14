package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/profile"
	"github.com/ixxet/apollo/internal/recommendations"
	"github.com/ixxet/apollo/internal/workouts"
)

func TestMemberWebShellRoutesStayBoundedAndMemberSafe(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	loginPage := env.doRequest(t, http.MethodGet, "/app/login", nil)
	if loginPage.Code != http.StatusOK {
		t.Fatalf("loginPage.Code = %d, want %d", loginPage.Code, http.StatusOK)
	}

	shellBlocked := env.doRequest(t, http.MethodGet, "/app/home", nil)
	if shellBlocked.Code != http.StatusSeeOther {
		t.Fatalf("shellBlocked.Code = %d, want %d", shellBlocked.Code, http.StatusSeeOther)
	}
	if location := shellBlocked.Header().Get("Location"); location != "/app/login" {
		t.Fatalf("shellBlocked Location = %q, want %q", location, "/app/login")
	}

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-web-011", "web-011@example.com")

	homeRedirect := env.doRequest(t, http.MethodGet, "/", nil, cookie)
	if homeRedirect.Code != http.StatusSeeOther {
		t.Fatalf("homeRedirect.Code = %d, want %d", homeRedirect.Code, http.StatusSeeOther)
	}
	if location := homeRedirect.Header().Get("Location"); location != "/app/home" {
		t.Fatalf("homeRedirect Location = %q, want %q", location, "/app/home")
	}

	shellEntry := env.doRequest(t, http.MethodGet, "/app", nil, cookie)
	if shellEntry.Code != http.StatusSeeOther {
		t.Fatalf("shellEntry.Code = %d, want %d", shellEntry.Code, http.StatusSeeOther)
	}
	if location := shellEntry.Header().Get("Location"); location != "/app/home" {
		t.Fatalf("shellEntry Location = %q, want %q", location, "/app/home")
	}

	for _, path := range []string{"/app/home", "/app/workouts", "/app/meals", "/app/tournaments", "/app/settings"} {
		response := env.doRequest(t, http.MethodGet, path, nil, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("%s Code = %d, want %d", path, response.Code, http.StatusOK)
		}
		body := response.Body.String()
		if !strings.Contains(body, `data-apollo-view="shell"`) {
			t.Fatalf("%s did not render shell frame", path)
		}
		if !strings.Contains(body, `class="member-nav"`) {
			t.Fatalf("%s did not render member nav", path)
		}
		if strings.Contains(body, "booking") {
			t.Fatalf("%s leaked booking copy into the member shell", path)
		}
	}

	unknownSection := env.doRequest(t, http.MethodGet, "/app/not-real", nil, cookie)
	if unknownSection.Code != http.StatusSeeOther {
		t.Fatalf("unknownSection.Code = %d, want %d", unknownSection.Code, http.StatusSeeOther)
	}
	if location := unknownSection.Header().Get("Location"); location != "/app/home" {
		t.Fatalf("unknownSection Location = %q, want %q", location, "/app/home")
	}

	loginRedirect := env.doRequest(t, http.MethodGet, "/app/login", nil, cookie)
	if loginRedirect.Code != http.StatusSeeOther {
		t.Fatalf("loginRedirect.Code = %d, want %d", loginRedirect.Code, http.StatusSeeOther)
	}
	if location := loginRedirect.Header().Get("Location"); location != "/app/home" {
		t.Fatalf("loginRedirect Location = %q, want %q", location, "/app/home")
	}

	_ = user
}

func TestMemberWebShellAllowsAnyAuthenticatedRoleIntoTheSameBoundedShell(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	for _, testCase := range []struct {
		name string
		role authz.Role
	}{
		{name: "member", role: authz.RoleMember},
		{name: "supervisor", role: authz.RoleSupervisor},
		{name: "manager", role: authz.RoleManager},
		{name: "owner", role: authz.RoleOwner},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			cookie, user := createVerifiedSessionViaHTTP(t, env, "role-"+string(testCase.role), "role-"+string(testCase.role)+"@example.com")
			setUserRole(t, env, user.ID, testCase.role)

			response := env.doRequest(t, http.MethodGet, "/app/home", nil, cookie)
			if response.Code != http.StatusOK {
				t.Fatalf("response.Code = %d, want %d", response.Code, http.StatusOK)
			}

			body := response.Body.String()
			if !strings.Contains(body, `data-apollo-section="home"`) {
				t.Fatalf("body did not render home section for %s", testCase.role)
			}
			if strings.Contains(body, "trusted-surface") {
				t.Fatalf("body leaked trusted-surface posture for %s", testCase.role)
			}
			if strings.Contains(body, "manager shell") {
				t.Fatalf("body leaked staff shell posture for %s", testCase.role)
			}
		})
	}
}

func TestMemberWebShellRuntimeSupportsMemberAPIsWithoutBoundaryDrift(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-web-012", "web-012@example.com")

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", user.ID, "tag-web-012", "shell tag"); err != nil {
		t.Fatalf("Exec(insert claimed tag) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.visits (user_id, facility_key, source_event_id, arrived_at) VALUES ($1, $2, $3, $4)", user.ID, "ashtonbee", "visit-web-012", time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(insert visit) error = %v", err)
	}

	beforeVisits := countRows(t, env, "apollo.visits", user.ID)
	beforeClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID)
	beforePreferences := lookupPreferences(t, env, user.ID)

	profileResponse := env.doRequest(t, http.MethodGet, "/api/v1/profile", nil, cookie)
	if profileResponse.Code != http.StatusOK {
		t.Fatalf("profileResponse.Code = %d, want %d", profileResponse.Code, http.StatusOK)
	}
	memberProfile := decodeProfileResponse(t, profileResponse)
	if memberProfile.VisibilityMode != profile.VisibilityModeGhost {
		t.Fatalf("memberProfile.VisibilityMode = %q, want %q", memberProfile.VisibilityMode, profile.VisibilityModeGhost)
	}

	sectionPage := env.doRequest(t, http.MethodGet, "/app/tournaments", nil, cookie)
	if sectionPage.Code != http.StatusOK {
		t.Fatalf("sectionPage.Code = %d, want %d", sectionPage.Code, http.StatusOK)
	}
	if body := sectionPage.Body.String(); strings.Contains(body, "/api/v1/schedule/") || strings.Contains(body, "/api/v1/competition/sessions") {
		t.Fatalf("sectionPage body leaked staff or schedule routes")
	}

	initialRecommendation := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/workout", nil, cookie)
	if initialRecommendation.Code != http.StatusOK {
		t.Fatalf("initialRecommendation.Code = %d, want %d", initialRecommendation.Code, http.StatusOK)
	}

	initialList := env.doRequest(t, http.MethodGet, "/api/v1/workouts", nil, cookie)
	if initialList.Code != http.StatusOK {
		t.Fatalf("initialList.Code = %d, want %d", initialList.Code, http.StatusOK)
	}
	if workoutsList := decodeWorkoutListResponse(t, initialList); len(workoutsList) != 0 {
		t.Fatalf("len(workoutsList) = %d, want 0", len(workoutsList))
	}

	createResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/workouts", `{"notes":"starter"}`, cookie)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("createResponse.Code = %d, want %d", createResponse.Code, http.StatusCreated)
	}
	createdWorkout := decodeWorkoutResponse(t, createResponse)
	if createdWorkout.Status != workouts.StatusInProgress {
		t.Fatalf("createdWorkout.Status = %q, want %q", createdWorkout.Status, workouts.StatusInProgress)
	}

	updateResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/workouts/"+createdWorkout.ID.String(), `{"notes":"legs","exercises":[{"name":"front squat","sets":4,"reps":5,"weight_kg":102.5,"rpe":8.5},{"name":"split squat","sets":3,"reps":8}]}`, cookie)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("updateResponse.Code = %d, want %d", updateResponse.Code, http.StatusOK)
	}
	updatedWorkout := decodeWorkoutResponse(t, updateResponse)
	if len(updatedWorkout.Exercises) != 2 {
		t.Fatalf("len(updatedWorkout.Exercises) = %d, want 2", len(updatedWorkout.Exercises))
	}

	detailResponse := env.doRequest(t, http.MethodGet, "/api/v1/workouts/"+createdWorkout.ID.String(), nil, cookie)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detailResponse.Code = %d, want %d", detailResponse.Code, http.StatusOK)
	}
	detailedWorkout := decodeWorkoutResponse(t, detailResponse)
	if detailedWorkout.ID != createdWorkout.ID {
		t.Fatalf("detailedWorkout.ID = %s, want %s", detailedWorkout.ID, createdWorkout.ID)
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

	finalRecommendation := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/workout", nil, cookie)
	if finalRecommendation.Code != http.StatusOK {
		t.Fatalf("finalRecommendation.Code = %d, want %d", finalRecommendation.Code, http.StatusOK)
	}
	recommendation := decodeRecommendationResponse(t, finalRecommendation)
	if recommendation.Type != "recovery_day" {
		t.Fatalf("recommendation.Type = %q, want %q", recommendation.Type, "recovery_day")
	}
	if recommendation.WorkoutID == nil || *recommendation.WorkoutID != createdWorkout.ID {
		t.Fatalf("recommendation.WorkoutID = %#v, want %s", recommendation.WorkoutID, createdWorkout.ID)
	}

	settingsPatch := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"discoverable","availability_mode":"available_now"}`, cookie)
	if settingsPatch.Code != http.StatusOK {
		t.Fatalf("settingsPatch.Code = %d, want %d", settingsPatch.Code, http.StatusOK)
	}

	if afterVisits := countRows(t, env, "apollo.visits", user.ID); afterVisits != beforeVisits {
		t.Fatalf("visit count changed from %d to %d", beforeVisits, afterVisits)
	}
	if afterClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID); afterClaimedTags != beforeClaimedTags {
		t.Fatalf("claimed tag count changed from %d to %d", beforeClaimedTags, afterClaimedTags)
	}
	if afterPreferences := lookupPreferences(t, env, user.ID); afterPreferences == beforePreferences {
		t.Fatalf("preferences did not change after profile patch")
	}
}

func lookupPreferences(t *testing.T, env *authProfileServerEnv, userID interface{}) string {
	t.Helper()

	var preferences string
	if err := env.db.DB.QueryRow(context.Background(), "SELECT preferences::text FROM apollo.users WHERE id = $1", userID).Scan(&preferences); err != nil {
		t.Fatalf("QueryRow(preferences) error = %v", err)
	}

	return preferences
}

func decodeRecommendationResponse(t *testing.T, response *httptest.ResponseRecorder) recommendations.WorkoutRecommendation {
	t.Helper()

	var recommendation recommendations.WorkoutRecommendation
	if err := json.Unmarshal(response.Body.Bytes(), &recommendation); err != nil {
		t.Fatalf("json.Unmarshal(recommendation) error = %v", err)
	}

	return recommendation
}
