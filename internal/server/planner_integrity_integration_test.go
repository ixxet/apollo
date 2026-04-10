package server

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestPlannerIntegrityRejectsInvalidInputAndDoesNotTouchUnrelatedDomains(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	firstCookie, firstUser := createVerifiedSessionViaHTTP(t, env, "student-planner-010", "planner-010@example.com")
	secondCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-planner-011", "planner-011@example.com")

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", firstUser.ID, "tag-planner-010", "planner tag"); err != nil {
		t.Fatalf("Exec(insert claimed tag) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.visits (user_id, facility_key, source_event_id, arrived_at) VALUES ($1, $2, $3, $4)", firstUser.ID, "ashtonbee", "planner-visit-001", time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(insert visit) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.workouts (user_id, started_at, status, finished_at) VALUES ($1, $2, $3, $4)", firstUser.ID, time.Date(2026, 4, 3, 12, 5, 0, 0, time.UTC), "finished", time.Date(2026, 4, 3, 13, 5, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(insert workout) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.recommendations (user_id, content, model_used) VALUES ($1, $2, $3)", firstUser.ID, `{"type":"seeded"}`, "test"); err != nil {
		t.Fatalf("Exec(insert recommendation) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.lobby_memberships (user_id, status, joined_at) VALUES ($1, $2, $3)", firstUser.ID, "joined", time.Date(2026, 4, 3, 12, 10, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(insert lobby membership) error = %v", err)
	}

	beforeVisits := countRows(t, env, "apollo.visits", firstUser.ID)
	beforeWorkouts := countRows(t, env, "apollo.workouts", firstUser.ID)
	beforeClaimedTags := countRows(t, env, "apollo.claimed_tags", firstUser.ID)
	beforeRecommendations := countRows(t, env, "apollo.recommendations", firstUser.ID)
	beforeMemberships := countRows(t, env, "apollo.lobby_memberships", firstUser.ID)
	tagHashBefore := lookupTagHash(t, env, firstUser.ID)

	createTemplateResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/planner/templates", `{"name":"Integrity Template","items":[{"exercise_key":"barbell-back-squat","equipment_key":"barbell","sets":5,"reps":5}]}`, firstCookie)
	if createTemplateResponse.Code != http.StatusCreated {
		t.Fatalf("createTemplateResponse.Code = %d, want %d", createTemplateResponse.Code, http.StatusCreated)
	}
	template := decodeTemplateResponse(t, createTemplateResponse)

	duplicateTemplateResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/planner/templates", `{"name":"Integrity Template","items":[{"exercise_key":"push-up","sets":3,"reps":12}]}`, firstCookie)
	if duplicateTemplateResponse.Code != http.StatusConflict {
		t.Fatalf("duplicateTemplateResponse.Code = %d, want %d", duplicateTemplateResponse.Code, http.StatusConflict)
	}

	otherUserTemplateRead := env.doRequest(t, http.MethodGet, "/api/v1/planner/templates/"+template.ID.String(), nil, secondCookie)
	if otherUserTemplateRead.Code != http.StatusNotFound {
		t.Fatalf("otherUserTemplateRead.Code = %d, want %d", otherUserTemplateRead.Code, http.StatusNotFound)
	}
	otherUserTemplateUpdate := env.doJSONRequest(t, http.MethodPut, "/api/v1/planner/templates/"+template.ID.String(), `{"name":"Should Fail","items":[{"exercise_key":"push-up","sets":3,"reps":10}]}`, secondCookie)
	if otherUserTemplateUpdate.Code != http.StatusNotFound {
		t.Fatalf("otherUserTemplateUpdate.Code = %d, want %d", otherUserTemplateUpdate.Code, http.StatusNotFound)
	}

	missingCookieResponse := env.doRequest(t, http.MethodGet, "/api/v1/planner/templates", nil)
	if missingCookieResponse.Code != http.StatusUnauthorized {
		t.Fatalf("missingCookieResponse.Code = %d, want %d", missingCookieResponse.Code, http.StatusUnauthorized)
	}

	invalidExerciseResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/planner/templates", `{"name":"Invalid Exercise","items":[{"exercise_key":"missing-exercise","sets":3,"reps":10}]}`, firstCookie)
	if invalidExerciseResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidExerciseResponse.Code = %d, want %d", invalidExerciseResponse.Code, http.StatusBadRequest)
	}
	invalidEquipmentResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/planner/templates", `{"name":"Invalid Equipment","items":[{"exercise_key":"push-up","equipment_key":"missing-equipment","sets":3,"reps":10}]}`, firstCookie)
	if invalidEquipmentResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidEquipmentResponse.Code = %d, want %d", invalidEquipmentResponse.Code, http.StatusBadRequest)
	}
	disallowedEquipmentResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/planner/templates", `{"name":"Disallowed Equipment","items":[{"exercise_key":"push-up","equipment_key":"barbell","sets":3,"reps":10}]}`, firstCookie)
	if disallowedEquipmentResponse.Code != http.StatusBadRequest {
		t.Fatalf("disallowedEquipmentResponse.Code = %d, want %d", disallowedEquipmentResponse.Code, http.StatusBadRequest)
	}

	invalidWeekStartResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/planner/weeks/2026-04-07", `{"sessions":[]}`, firstCookie)
	if invalidWeekStartResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidWeekStartResponse.Code = %d, want %d", invalidWeekStartResponse.Code, http.StatusBadRequest)
	}
	invalidDayIndexResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/planner/weeks/2026-04-06", `{"sessions":[{"day_index":7,"items":[{"exercise_key":"push-up","sets":3,"reps":12}]}]}`, firstCookie)
	if invalidDayIndexResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidDayIndexResponse.Code = %d, want %d", invalidDayIndexResponse.Code, http.StatusBadRequest)
	}
	invalidSessionShapeResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/planner/weeks/2026-04-06", `{"sessions":[{"day_index":1,"template_id":"`+template.ID.String()+`","items":[{"exercise_key":"push-up","sets":3,"reps":12}]}]}`, firstCookie)
	if invalidSessionShapeResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidSessionShapeResponse.Code = %d, want %d", invalidSessionShapeResponse.Code, http.StatusBadRequest)
	}
	emptyItemsResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/planner/weeks/2026-04-06", `{"sessions":[{"day_index":1,"items":[]} ]}`, firstCookie)
	if emptyItemsResponse.Code != http.StatusBadRequest {
		t.Fatalf("emptyItemsResponse.Code = %d, want %d", emptyItemsResponse.Code, http.StatusBadRequest)
	}

	profileInvalidEquipmentResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"coaching_profile":{"preferred_equipment_keys":["missing-equipment"]}}`, firstCookie)
	if profileInvalidEquipmentResponse.Code != http.StatusBadRequest {
		t.Fatalf("profileInvalidEquipmentResponse.Code = %d, want %d", profileInvalidEquipmentResponse.Code, http.StatusBadRequest)
	}

	putWeekResponse := env.doJSONRequest(t, http.MethodPut, "/api/v1/planner/weeks/2026-04-06", `{"sessions":[{"day_index":0,"template_id":"`+template.ID.String()+`"}]}`, firstCookie)
	if putWeekResponse.Code != http.StatusOK {
		t.Fatalf("putWeekResponse.Code = %d, want %d", putWeekResponse.Code, http.StatusOK)
	}

	if afterVisits := countRows(t, env, "apollo.visits", firstUser.ID); afterVisits != beforeVisits {
		t.Fatalf("visit count changed from %d to %d", beforeVisits, afterVisits)
	}
	if afterWorkouts := countRows(t, env, "apollo.workouts", firstUser.ID); afterWorkouts != beforeWorkouts {
		t.Fatalf("workout count changed from %d to %d", beforeWorkouts, afterWorkouts)
	}
	if afterClaimedTags := countRows(t, env, "apollo.claimed_tags", firstUser.ID); afterClaimedTags != beforeClaimedTags {
		t.Fatalf("claimed tag count changed from %d to %d", beforeClaimedTags, afterClaimedTags)
	}
	if afterRecommendations := countRows(t, env, "apollo.recommendations", firstUser.ID); afterRecommendations != beforeRecommendations {
		t.Fatalf("recommendation count changed from %d to %d", beforeRecommendations, afterRecommendations)
	}
	if afterMemberships := countRows(t, env, "apollo.lobby_memberships", firstUser.ID); afterMemberships != beforeMemberships {
		t.Fatalf("membership count changed from %d to %d", beforeMemberships, afterMemberships)
	}
	if tagHashAfter := lookupTagHash(t, env, firstUser.ID); tagHashAfter != tagHashBefore {
		t.Fatalf("tag hash changed from %q to %q", tagHashBefore, tagHashAfter)
	}
}
