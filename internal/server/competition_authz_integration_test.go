package server

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
)

func TestCompetitionAuthzEnforcesRoleCapabilityMatrix(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-competition-authz-001", "competition-authz-001@example.com")
	supervisorCookie, supervisor := createVerifiedSessionViaHTTP(t, env, "student-competition-authz-002", "competition-authz-002@example.com")
	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-authz-003", "competition-authz-003@example.com")
	memberCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-competition-authz-004", "competition-authz-004@example.com")

	setUserRole(t, env, manager.ID, authz.RoleManager)
	setUserRole(t, env, supervisor.ID, authz.RoleSupervisor)
	setUserRole(t, env, owner.ID, authz.RoleOwner)

	managerCreate := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Tracer 28 Manager Session",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, managerCookie)
	if managerCreate.Code != http.StatusCreated {
		t.Fatalf("managerCreate.Code = %d, want %d body=%s", managerCreate.Code, http.StatusCreated, managerCreate.Body.String())
	}
	session := decodeCompetitionSession(t, managerCreate)

	supervisorRead := env.doRequest(t, http.MethodGet, "/api/v1/competition/sessions", nil, supervisorCookie)
	if supervisorRead.Code != http.StatusOK {
		t.Fatalf("supervisorRead.Code = %d, want %d", supervisorRead.Code, http.StatusOK)
	}

	supervisorCreate := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Tracer 28 Supervisor Denied",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, supervisorCookie)
	if supervisorCreate.Code != http.StatusForbidden {
		t.Fatalf("supervisorCreate.Code = %d, want %d", supervisorCreate.Code, http.StatusForbidden)
	}

	supervisorOpenQueue := env.doRequest(t, http.MethodPost, "/api/v1/competition/sessions/"+session.ID.String()+"/queue/open", nil, supervisorCookie)
	if supervisorOpenQueue.Code != http.StatusOK {
		t.Fatalf("supervisorOpenQueue.Code = %d, want %d body=%s", supervisorOpenQueue.Code, http.StatusOK, supervisorOpenQueue.Body.String())
	}

	ownerCreate := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Tracer 28 Owner Session",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, ownerCookie)
	if ownerCreate.Code != http.StatusCreated {
		t.Fatalf("ownerCreate.Code = %d, want %d body=%s", ownerCreate.Code, http.StatusCreated, ownerCreate.Body.String())
	}

	memberRead := env.doRequest(t, http.MethodGet, "/api/v1/competition/sessions", nil, memberCookie)
	if memberRead.Code != http.StatusForbidden {
		t.Fatalf("memberRead.Code = %d, want %d", memberRead.Code, http.StatusForbidden)
	}

	memberCreate := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Tracer 28 Member Denied",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, memberCookie)
	if memberCreate.Code != http.StatusForbidden {
		t.Fatalf("memberCreate.Code = %d, want %d", memberCreate.Code, http.StatusForbidden)
	}
}

func TestCompetitionAuthzRejectsMissingAndInvalidTrustedSurface(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-authz-011", "competition-authz-011@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)

	body := bytes.NewBufferString(`{
		"display_name":"Tracer 28 Trusted Surface",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`)

	missingSurface := env.doRequestWithoutTrustedSurface(t, http.MethodPost, "/api/v1/competition/sessions", bytes.NewBuffer(body.Bytes()), ownerCookie)
	if missingSurface.Code != http.StatusForbidden {
		t.Fatalf("missingSurface.Code = %d, want %d", missingSurface.Code, http.StatusForbidden)
	}

	invalidSurface := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/competition/sessions", bytes.NewBuffer(body.Bytes()), map[string]string{
		authz.TrustedSurfaceHeader:      env.trustedSurfaceKey,
		authz.TrustedSurfaceTokenHeader: "wrong-secret",
	}, ownerCookie)
	if invalidSurface.Code != http.StatusForbidden {
		t.Fatalf("invalidSurface.Code = %d, want %d", invalidSurface.Code, http.StatusForbidden)
	}
}

func TestCompetitionAuthzWritesActorAttributionForStaffMutations(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-authz-021", "competition-authz-021@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberCookie, member := createVerifiedSessionViaHTTP(t, env, "student-competition-authz-022", "competition-authz-022@example.com")

	makeEligibleForLobby(t, env, ownerCookie)
	joinLobbyMembership(t, env, ownerCookie)
	makeEligibleForLobby(t, env, memberCookie)
	joinLobbyMembership(t, env, memberCookie)

	createSessionResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Tracer 28 Attribution Session",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, ownerCookie)
	if createSessionResponse.Code != http.StatusCreated {
		t.Fatalf("createSessionResponse.Code = %d, want %d body=%s", createSessionResponse.Code, http.StatusCreated, createSessionResponse.Body.String())
	}
	session := decodeCompetitionSession(t, createSessionResponse)

	openQueueResponse := env.doRequest(t, http.MethodPost, "/api/v1/competition/sessions/"+session.ID.String()+"/queue/open", nil, ownerCookie)
	if openQueueResponse.Code != http.StatusOK {
		t.Fatalf("openQueueResponse.Code = %d, want %d body=%s", openQueueResponse.Code, http.StatusOK, openQueueResponse.Body.String())
	}

	queueMemberResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions/"+session.ID.String()+"/queue/members", `{"user_id":"`+member.ID.String()+`"}`, ownerCookie)
	if queueMemberResponse.Code != http.StatusOK {
		t.Fatalf("queueMemberResponse.Code = %d, want %d body=%s", queueMemberResponse.Code, http.StatusOK, queueMemberResponse.Body.String())
	}

	actorSessionID, err := env.cookies.Decode(ownerCookie.Value)
	if err != nil {
		t.Fatalf("Decode(ownerCookie.Value) error = %v", err)
	}

	rows, err := env.db.DB.Query(context.Background(), `
		SELECT actor_user_id,
		       actor_role,
		       session_id,
		       capability,
		       trusted_surface_key,
		       action,
		       competition_session_id,
		       subject_user_id
		FROM apollo.competition_staff_action_attributions
		WHERE actor_user_id = $1
		ORDER BY occurred_at ASC, id ASC
	`, owner.ID)
	if err != nil {
		t.Fatalf("Query(attributions) error = %v", err)
	}
	defer rows.Close()

	type attributionRow struct {
		ActorUserID          uuid.UUID
		ActorRole            string
		SessionID            uuid.UUID
		Capability           string
		TrustedSurfaceKey    string
		Action               string
		CompetitionSessionID *uuid.UUID
		SubjectUserID        *uuid.UUID
	}

	var attributions []attributionRow
	for rows.Next() {
		var row attributionRow
		if err := rows.Scan(
			&row.ActorUserID,
			&row.ActorRole,
			&row.SessionID,
			&row.Capability,
			&row.TrustedSurfaceKey,
			&row.Action,
			&row.CompetitionSessionID,
			&row.SubjectUserID,
		); err != nil {
			t.Fatalf("rows.Scan() error = %v", err)
		}
		attributions = append(attributions, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() = %v", err)
	}
	if len(attributions) < 2 {
		t.Fatalf("len(attributions) = %d, want at least 2", len(attributions))
	}
	if attributions[0].ActorUserID != owner.ID {
		t.Fatalf("attributions[0].ActorUserID = %s, want %s", attributions[0].ActorUserID, owner.ID)
	}
	if attributions[0].ActorRole != string(authz.RoleOwner) {
		t.Fatalf("attributions[0].ActorRole = %q, want %q", attributions[0].ActorRole, authz.RoleOwner)
	}
	if attributions[0].SessionID != actorSessionID {
		t.Fatalf("attributions[0].SessionID = %s, want %s", attributions[0].SessionID, actorSessionID)
	}
	if attributions[0].TrustedSurfaceKey != env.trustedSurfaceKey {
		t.Fatalf("attributions[0].TrustedSurfaceKey = %q, want %q", attributions[0].TrustedSurfaceKey, env.trustedSurfaceKey)
	}
	if attributions[0].CompetitionSessionID == nil || *attributions[0].CompetitionSessionID != session.ID {
		t.Fatalf("attributions[0].CompetitionSessionID = %v, want %s", attributions[0].CompetitionSessionID, session.ID)
	}
	if attributions[0].Action != "competition_session.create" {
		t.Fatalf("attributions[0].Action = %q, want %q", attributions[0].Action, "competition_session.create")
	}
	if attributions[1].Action != "competition_session.queue_open" {
		t.Fatalf("attributions[1].Action = %q, want %q", attributions[1].Action, "competition_session.queue_open")
	}
	if len(attributions) > 2 && attributions[2].Action == "competition_session.queue_member_add" {
		if attributions[2].SubjectUserID == nil || *attributions[2].SubjectUserID != member.ID {
			t.Fatalf("attributions[2].SubjectUserID = %v, want %s", attributions[2].SubjectUserID, member.ID)
		}
	}
}
