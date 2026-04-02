package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/eligibility"
)

type stubAuthenticator struct {
	principal         auth.Principal
	authenticateError error
	cookieName        string
}

func (s stubAuthenticator) StartVerification(context.Context, auth.StartVerificationInput) error {
	panic("unexpected StartVerification call")
}

func (s stubAuthenticator) VerifyToken(context.Context, string) (auth.VerifiedSession, error) {
	panic("unexpected VerifyToken call")
}

func (s stubAuthenticator) AuthenticateSession(context.Context, string) (auth.Principal, error) {
	if s.authenticateError != nil {
		return auth.Principal{}, s.authenticateError
	}

	return s.principal, nil
}

func (s stubAuthenticator) LogoutSession(context.Context, string) error {
	panic("unexpected LogoutSession call")
}

func (s stubAuthenticator) SessionCookie(uuid.UUID, time.Time) *http.Cookie {
	panic("unexpected SessionCookie call")
}

func (s stubAuthenticator) ExpiredSessionCookie() *http.Cookie {
	panic("unexpected ExpiredSessionCookie call")
}

func (s stubAuthenticator) SessionCookieName() string {
	if s.cookieName == "" {
		return "apollo_session"
	}

	return s.cookieName
}

type stubEligibilityReader struct {
	response eligibility.LobbyEligibility
	err      error
}

func (s stubEligibilityReader) GetLobbyEligibility(context.Context, uuid.UUID) (eligibility.LobbyEligibility, error) {
	if s.err != nil {
		return eligibility.LobbyEligibility{}, s.err
	}

	return s.response, nil
}

func TestLobbyEligibilityEndpointRequiresAuthentication(t *testing.T) {
	handler := NewHandler(Dependencies{
		Auth:        stubAuthenticator{cookieName: "apollo_session"},
		Eligibility: stubEligibilityReader{},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/lobby/eligibility", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestLobbyEligibilityEndpointReturnsTheStableEligibilityShape(t *testing.T) {
	handler := NewHandler(Dependencies{
		Auth: stubAuthenticator{
			cookieName: "apollo_session",
			principal:  auth.Principal{UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
		},
		Eligibility: stubEligibilityReader{
			response: eligibility.LobbyEligibility{
				Eligible:         true,
				Reason:           eligibility.ReasonEligible,
				VisibilityMode:   "discoverable",
				AvailabilityMode: "available_now",
			},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/lobby/eligibility", nil)
	request.AddCookie(&http.Cookie{Name: "apollo_session", Value: "signed"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["eligible"] != true {
		t.Fatalf("eligible = %#v, want true", payload["eligible"])
	}
	if payload["reason"] != eligibility.ReasonEligible {
		t.Fatalf("reason = %#v, want %q", payload["reason"], eligibility.ReasonEligible)
	}
	if payload["visibility_mode"] != "discoverable" {
		t.Fatalf("visibility_mode = %#v, want %q", payload["visibility_mode"], "discoverable")
	}
	if payload["availability_mode"] != "available_now" {
		t.Fatalf("availability_mode = %#v, want %q", payload["availability_mode"], "available_now")
	}
}

func TestLobbyEligibilityEndpointMapsLookupFailuresClearly(t *testing.T) {
	testCases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "missing member returns not found",
			err:        eligibility.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unexpected failure returns internal server error",
			err:        errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			handler := NewHandler(Dependencies{
				Auth: stubAuthenticator{
					cookieName: "apollo_session",
					principal:  auth.Principal{UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
				},
				Eligibility: stubEligibilityReader{err: testCase.err},
			})

			request := httptest.NewRequest(http.MethodGet, "/api/v1/lobby/eligibility", nil)
			request.AddCookie(&http.Cookie{Name: "apollo_session", Value: "signed"})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, testCase.wantStatus)
			}
		})
	}
}
