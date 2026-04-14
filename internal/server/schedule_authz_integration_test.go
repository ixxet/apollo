package server

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/ixxet/apollo/internal/authz"
)

func TestScheduleAuthzEnforcesReadWriteMatrixAndTrustedSurface(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-schedule-authz-001", "schedule-authz-001@example.com")
	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-schedule-authz-002", "schedule-authz-002@example.com")
	supervisorCookie, supervisor := createVerifiedSessionViaHTTP(t, env, "student-schedule-authz-003", "schedule-authz-003@example.com")
	memberCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-schedule-authz-004", "schedule-authz-004@example.com")

	setUserRole(t, env, owner.ID, authz.RoleOwner)
	setUserRole(t, env, manager.ID, authz.RoleManager)
	setUserRole(t, env, supervisor.ID, authz.RoleSupervisor)

	if response := env.doRequest(t, http.MethodGet, "/api/v1/schedule/blocks?facility_key=ashtonbee", nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated read code = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if response := env.doRequest(t, http.MethodGet, "/api/v1/schedule/blocks?facility_key=ashtonbee", nil, memberCookie); response.Code != http.StatusForbidden {
		t.Fatalf("member read code = %d, want %d", response.Code, http.StatusForbidden)
	}
	if response := env.doRequest(t, http.MethodGet, "/api/v1/schedule/blocks?facility_key=ashtonbee", nil, supervisorCookie); response.Code != http.StatusOK {
		t.Fatalf("supervisor read code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	writeBody := `{
		"facility_key":"ashtonbee",
		"scope":"facility",
		"kind":"closure",
		"effect":"closed",
		"visibility":"internal",
		"one_off":{"starts_at":"2026-04-06T10:00:00Z","ends_at":"2026-04-06T11:00:00Z"}
	}`
	if response := env.doRequest(t, http.MethodPost, "/api/v1/schedule/blocks", bytes.NewBufferString(writeBody), supervisorCookie); response.Code != http.StatusForbidden {
		t.Fatalf("supervisor write code = %d, want %d", response.Code, http.StatusForbidden)
	}
	if response := env.doRequestWithoutTrustedSurface(t, http.MethodPost, "/api/v1/schedule/blocks", bytes.NewBufferString(writeBody), managerCookie); response.Code != http.StatusForbidden {
		t.Fatalf("manager missing trusted surface code = %d, want %d", response.Code, http.StatusForbidden)
	}
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/schedule/blocks", writeBody, ownerCookie); response.Code != http.StatusCreated {
		t.Fatalf("owner write code = %d, want %d body=%s", response.Code, http.StatusCreated, response.Body.String())
	}
}
