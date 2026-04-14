package server

import (
	"net/http"
	"testing"

	"github.com/ixxet/apollo/internal/authz"
)

func TestScheduleNegativeHTTPRejectsInvalidScopeMissingAuthAndDateOnlyCalendarWindow(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-schedule-negative-001", "schedule-negative-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)

	invalidScopeResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/schedule/blocks", `{
		"facility_key":"ashtonbee",
		"scope":"resource",
		"kind":"hold",
		"effect":"soft_hold",
		"visibility":"public_busy",
		"one_off":{"starts_at":"2026-04-06T10:00:00Z","ends_at":"2026-04-06T11:00:00Z"}
	}`, ownerCookie)
	if invalidScopeResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalidScopeResponse.Code = %d, want %d body=%s", invalidScopeResponse.Code, http.StatusBadRequest, invalidScopeResponse.Body.String())
	}

	missingAuthResponse := env.doRequest(t, http.MethodGet, "/api/v1/schedule/calendar?facility_key=ashtonbee&from=2026-04-06T00:00:00Z&until=2026-04-27T00:00:00Z", nil)
	if missingAuthResponse.Code != http.StatusUnauthorized {
		t.Fatalf("missingAuthResponse.Code = %d, want %d", missingAuthResponse.Code, http.StatusUnauthorized)
	}

	dateOnlyResponse := env.doRequest(t, http.MethodGet, "/api/v1/schedule/calendar?facility_key=ashtonbee&from=2026-04-06&until=2026-04-27", nil, ownerCookie)
	if dateOnlyResponse.Code != http.StatusBadRequest {
		t.Fatalf("dateOnlyResponse.Code = %d, want %d body=%s", dateOnlyResponse.Code, http.StatusBadRequest, dateOnlyResponse.Body.String())
	}
}
