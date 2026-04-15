package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/presence"
)

func TestMemberFacilityCalendarRejectsUnauthenticatedInvalidAndOversizedRequests(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	memberCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-member-calendar-001", "member-calendar-001@example.com")
	base := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

	validPath := fmt.Sprintf(
		"/api/v1/presence/facilities/ashtonbee/calendar?from=%s&until=%s",
		base.Format(time.RFC3339),
		base.Add(7*24*time.Hour).Format(time.RFC3339),
	)
	if response := env.doRequest(t, http.MethodGet, validPath, nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated member facility calendar response = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	testCases := []struct {
		name   string
		path   string
		status int
	}{
		{
			name:   "missing boundaries",
			path:   "/api/v1/presence/facilities/ashtonbee/calendar",
			status: http.StatusBadRequest,
		},
		{
			name:   "date-only boundaries",
			path:   "/api/v1/presence/facilities/ashtonbee/calendar?from=2026-04-15&until=2026-04-16",
			status: http.StatusBadRequest,
		},
		{
			name: "equal boundaries",
			path: fmt.Sprintf(
				"/api/v1/presence/facilities/ashtonbee/calendar?from=%s&until=%s",
				base.Format(time.RFC3339),
				base.Format(time.RFC3339),
			),
			status: http.StatusBadRequest,
		},
		{
			name: "reversed boundaries",
			path: fmt.Sprintf(
				"/api/v1/presence/facilities/ashtonbee/calendar?from=%s&until=%s",
				base.Add(48*time.Hour).Format(time.RFC3339),
				base.Add(24*time.Hour).Format(time.RFC3339),
			),
			status: http.StatusBadRequest,
		},
		{
			name: "oversized boundary",
			path: fmt.Sprintf(
				"/api/v1/presence/facilities/ashtonbee/calendar?from=%s&until=%s",
				base.Format(time.RFC3339),
				base.Add(15*24*time.Hour).Format(time.RFC3339),
			),
			status: http.StatusBadRequest,
		},
		{
			name: "unknown facility",
			path: fmt.Sprintf(
				"/api/v1/presence/facilities/not-real/calendar?from=%s&until=%s",
				base.Format(time.RFC3339),
				base.Add(24*time.Hour).Format(time.RFC3339),
			),
			status: http.StatusNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response := env.doRequest(t, http.MethodGet, testCase.path, nil, memberCookie)
			if response.Code != testCase.status {
				t.Fatalf("%s response = %d, want %d body=%s", testCase.name, response.Code, testCase.status, response.Body.String())
			}
		})
	}
}

func TestMemberFacilityCalendarRuntimeStaysMemberSafeWithoutScheduleCapability(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-member-calendar-002", "member-calendar-002@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-member-calendar-003", "member-calendar-003@example.com")

	base := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	createBlock := func(payload string) {
		t.Helper()

		response := env.doJSONRequest(t, http.MethodPost, "/api/v1/schedule/blocks", payload, ownerCookie)
		if response.Code != http.StatusCreated {
			t.Fatalf("schedule block create response = %d, want %d body=%s", response.Code, http.StatusCreated, response.Body.String())
		}
	}

	createBlock(fmt.Sprintf(`{
		"facility_key":"ashtonbee",
		"scope":"facility",
		"kind":"operating_hours",
		"effect":"informational",
		"visibility":"public_labeled",
		"one_off":{
			"starts_at":"%s",
			"ends_at":"%s"
		}
	}`, base.Add(time.Hour).Format(time.RFC3339), base.Add(9*time.Hour).Format(time.RFC3339)))
	createBlock(fmt.Sprintf(`{
		"facility_key":"ashtonbee",
		"scope":"facility",
		"kind":"closure",
		"effect":"closed",
		"visibility":"public_busy",
		"one_off":{
			"starts_at":"%s",
			"ends_at":"%s"
		}
	}`, base.Add(24*time.Hour).Format(time.RFC3339), base.Add(26*time.Hour).Format(time.RFC3339)))
	createBlock(fmt.Sprintf(`{
		"facility_key":"ashtonbee",
		"scope":"facility",
		"kind":"closure",
		"effect":"closed",
		"visibility":"internal",
		"one_off":{
			"starts_at":"%s",
			"ends_at":"%s"
		}
	}`, base.Add(48*time.Hour).Format(time.RFC3339), base.Add(49*time.Hour).Format(time.RFC3339)))
	createBlock(fmt.Sprintf(`{
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"scope":"zone",
		"kind":"operating_hours",
		"effect":"informational",
		"visibility":"public_labeled",
		"one_off":{
			"starts_at":"%s",
			"ends_at":"%s"
		}
	}`, base.Add(2*time.Hour).Format(time.RFC3339), base.Add(3*time.Hour).Format(time.RFC3339)))
	createBlock(fmt.Sprintf(`{
		"facility_key":"ashtonbee",
		"scope":"facility",
		"kind":"event",
		"effect":"informational",
		"visibility":"public_busy",
		"one_off":{
			"starts_at":"%s",
			"ends_at":"%s"
		}
	}`, base.Add(4*time.Hour).Format(time.RFC3339), base.Add(5*time.Hour).Format(time.RFC3339)))
	createBlock(fmt.Sprintf(`{
		"facility_key":"morningside",
		"scope":"facility",
		"kind":"operating_hours",
		"effect":"informational",
		"visibility":"public_labeled",
		"one_off":{
			"starts_at":"%s",
			"ends_at":"%s"
		}
	}`, base.Add(6*time.Hour).Format(time.RFC3339), base.Add(7*time.Hour).Format(time.RFC3339)))

	staffRoute := fmt.Sprintf(
		"/api/v1/schedule/calendar?facility_key=ashtonbee&from=%s&until=%s",
		base.Format(time.RFC3339),
		base.Add(7*24*time.Hour).Format(time.RFC3339),
	)
	if response := env.doRequest(t, http.MethodGet, staffRoute, nil, memberCookie); response.Code != http.StatusForbidden {
		t.Fatalf("member staff calendar response = %d, want %d body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}

	memberRoute := fmt.Sprintf(
		"/api/v1/presence/facilities/ashtonbee/calendar?from=%s&until=%s",
		base.Format(time.RFC3339),
		base.Add(7*24*time.Hour).Format(time.RFC3339),
	)
	response := env.doRequest(t, http.MethodGet, memberRoute, nil, memberCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("member facility calendar response = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	for _, leaked := range []string{
		"created_by_user_id",
		"updated_by_user_id",
		"block_id",
		"conflicts",
		"visibility",
		"kind",
		"effect",
		"scope",
		"zone_key",
		"resource_key",
	} {
		if strings.Contains(response.Body.String(), leaked) {
			t.Fatalf("member facility calendar leaked %q: %s", leaked, response.Body.String())
		}
	}

	calendar := decodeMemberFacilityCalendarResponse(t, response)
	if calendar.FacilityKey != "ashtonbee" {
		t.Fatalf("calendar.FacilityKey = %q, want ashtonbee", calendar.FacilityKey)
	}
	if got, want := len(calendar.Hours), 1; got != want {
		t.Fatalf("len(calendar.Hours) = %d, want %d", got, want)
	}
	if got, want := len(calendar.Closures), 1; got != want {
		t.Fatalf("len(calendar.Closures) = %d, want %d", got, want)
	}
	if calendar.Hours[0].OccurrenceDate != "2026-04-15" {
		t.Fatalf("calendar.Hours[0].OccurrenceDate = %q, want 2026-04-15", calendar.Hours[0].OccurrenceDate)
	}
	if calendar.Closures[0].OccurrenceDate != "2026-04-16" {
		t.Fatalf("calendar.Closures[0].OccurrenceDate = %q, want 2026-04-16", calendar.Closures[0].OccurrenceDate)
	}
}

func decodeMemberFacilityCalendarResponse(t *testing.T, response *httptest.ResponseRecorder) presence.MemberFacilityCalendar {
	t.Helper()

	var payload presence.MemberFacilityCalendar
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(member facility calendar) error = %v", err)
	}
	return payload
}
