package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/athena"
	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/ops"
)

type stubOpsOverviewReader struct {
	response ops.FacilityOverview
	err      error
}

func (s stubOpsOverviewReader) GetFacilityOverview(context.Context, ops.FacilityOverviewInput) (ops.FacilityOverview, error) {
	if s.err != nil {
		return ops.FacilityOverview{}, s.err
	}
	return s.response, nil
}

func TestOpsOverviewAuthzEnforcesStaffReadOnlyBoundary(t *testing.T) {
	response := testOpsOverviewResponse()

	t.Run("unauthenticated", func(t *testing.T) {
		handler := NewHandler(Dependencies{
			Auth: stubAuthenticator{cookieName: "apollo_session"},
			Ops:  stubOpsOverviewReader{response: response},
		})
		request := httptest.NewRequest(http.MethodGet, "/api/v1/ops/facilities/ashtonbee/overview?from=2026-04-15T13:00:00Z&until=2026-04-15T14:00:00Z", nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
		}
	})

	t.Run("member denied", func(t *testing.T) {
		handler := newOpsOverviewHandler(auth.Principal{
			UserID:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			SessionID:    uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			Role:         authz.RoleMember,
			Capabilities: authz.OpsCapabilitiesForRole(authz.RoleMember),
		}, nil)
		recorder := doOpsOverviewRequest(handler, "/api/v1/ops/facilities/ashtonbee/overview?from=2026-04-15T13:00:00Z&until=2026-04-15T14:00:00Z")
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
		}
	})

	for _, role := range []authz.Role{authz.RoleSupervisor, authz.RoleManager, authz.RoleOwner} {
		t.Run(string(role), func(t *testing.T) {
			handler := newOpsOverviewHandler(auth.Principal{
				UserID:       uuid.MustParse("33333333-3333-3333-3333-333333333333"),
				SessionID:    uuid.MustParse("44444444-4444-4444-4444-444444444444"),
				Role:         role,
				Capabilities: authz.OpsCapabilitiesForRole(role),
			}, nil)
			recorder := doOpsOverviewRequest(handler, "/api/v1/ops/facilities/ashtonbee/overview?from=2026-04-15T13:00:00Z&until=2026-04-15T14:00:00Z")
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
		})
	}
}

func TestOpsOverviewRejectsInvalidWindowsBeforeComposition(t *testing.T) {
	handler := newOpsOverviewHandler(auth.Principal{
		UserID:       uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		SessionID:    uuid.MustParse("66666666-6666-6666-6666-666666666666"),
		Role:         authz.RoleOwner,
		Capabilities: authz.OpsCapabilitiesForRole(authz.RoleOwner),
	}, nil)

	for _, path := range []string{
		"/api/v1/ops/facilities/ashtonbee/overview",
		"/api/v1/ops/facilities/ashtonbee/overview?from=2026-04-15&until=2026-04-16",
		"/api/v1/ops/facilities/ashtonbee/overview?from=2026-04-15T13:00:00Z&until=2026-04-15T14:00:00Z&bucket_minutes=0",
	} {
		recorder := doOpsOverviewRequest(handler, path)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want %d body=%s", path, recorder.Code, http.StatusBadRequest, recorder.Body.String())
		}
	}
}

func TestOpsOverviewMapsWindowAndUpstreamFailuresClearly(t *testing.T) {
	testCases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "reversed window",
			err:        ops.ErrWindowInvalid,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "oversized window",
			err:        ops.ErrWindowTooLarge,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "timeout",
			err:        fmt.Errorf("%w: test deadline", athena.ErrRequestTimeout),
			wantStatus: http.StatusGatewayTimeout,
		},
		{
			name:       "upstream status",
			err:        &athena.UpstreamStatusError{StatusCode: http.StatusServiceUnavailable, Message: "edge analytics are not configured"},
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "malformed upstream",
			err:        fmt.Errorf("%w: missing bucket_minutes", athena.ErrAnalyticsMalformed),
			wantStatus: http.StatusBadGateway,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			handler := newOpsOverviewHandler(auth.Principal{
				UserID:       uuid.MustParse("77777777-7777-7777-7777-777777777777"),
				SessionID:    uuid.MustParse("88888888-8888-8888-8888-888888888888"),
				Role:         authz.RoleManager,
				Capabilities: authz.OpsCapabilitiesForRole(authz.RoleManager),
			}, testCase.err)
			recorder := doOpsOverviewRequest(handler, "/api/v1/ops/facilities/ashtonbee/overview?from=2026-04-15T13:00:00Z&until=2026-04-15T14:00:00Z")
			if recorder.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d body=%s", recorder.Code, testCase.wantStatus, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "tag_hash") || strings.Contains(recorder.Body.String(), "account_raw") {
				t.Fatalf("error body leaked identity field: %s", recorder.Body.String())
			}
		})
	}
}

func TestOpsOverviewRouteDoesNotExposeWriteVerbs(t *testing.T) {
	handler := newOpsOverviewHandler(auth.Principal{
		UserID:       uuid.MustParse("99999999-9999-9999-9999-999999999999"),
		SessionID:    uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		Role:         authz.RoleOwner,
		Capabilities: authz.OpsCapabilitiesForRole(authz.RoleOwner),
	}, nil)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/ops/facilities/ashtonbee/overview?from=2026-04-15T13:00:00Z&until=2026-04-15T14:00:00Z", nil)
	request.AddCookie(&http.Cookie{Name: "apollo_session", Value: "signed"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
}

func newOpsOverviewHandler(principal auth.Principal, err error) http.Handler {
	return NewHandler(Dependencies{
		Auth: stubAuthenticator{
			cookieName: "apollo_session",
			principal:  principal,
		},
		Ops: stubOpsOverviewReader{
			response: testOpsOverviewResponse(),
			err:      err,
		},
	})
}

func doOpsOverviewRequest(handler http.Handler, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.AddCookie(&http.Cookie{Name: "apollo_session", Value: "signed"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func testOpsOverviewResponse() ops.FacilityOverview {
	from := time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC)
	until := from.Add(time.Hour)
	return ops.FacilityOverview{
		FacilityKey: "ashtonbee",
		Status:      "complete",
		Window: ops.OverviewWindow{
			From:  from.Format(time.RFC3339),
			Until: until.Format(time.RFC3339),
		},
		CurrentOccupancy: ops.CurrentOccupancy{
			FacilityID:    "ashtonbee",
			CurrentCount:  7,
			ObservedAt:    until.Format(time.RFC3339),
			SourceService: "athena",
		},
		ScheduleSummary: ops.ScheduleSummary{
			OccurrenceCount:         0,
			ReturnedOccurrenceCount: 0,
			ByKind:                  map[string]int{},
			ByEffect:                map[string]int{},
			ByVisibility:            map[string]int{},
			SourceService:           "apollo",
		},
		SourceServices: ops.SourceServices{
			Schedule:  "apollo",
			Occupancy: "athena",
			Analytics: "athena",
		},
	}
}
