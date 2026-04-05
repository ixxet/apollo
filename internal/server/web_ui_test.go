package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/auth"
)

func TestWebUIRoutesRedirectAndRenderAgainstSessionState(t *testing.T) {
	handler := NewHandler(Dependencies{
		Auth: stubAuthenticator{
			cookieName: "apollo_session",
			principal:  auth.Principal{UserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
		},
	})

	for _, testCase := range []struct {
		name         string
		path         string
		cookies      []*http.Cookie
		wantStatus   int
		wantLocation string
		wantBody     string
	}{
		{
			name:         "root redirects unauthenticated member to login",
			path:         "/",
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/app/login",
		},
		{
			name:         "root redirects authenticated member to shell",
			path:         "/",
			cookies:      []*http.Cookie{{Name: "apollo_session", Value: "signed"}},
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/app",
		},
		{
			name:       "login page renders for unauthenticated member",
			path:       "/app/login",
			wantStatus: http.StatusOK,
			wantBody:   `data-apollo-view="login"`,
		},
		{
			name:         "login page redirects authenticated member to shell",
			path:         "/app/login",
			cookies:      []*http.Cookie{{Name: "apollo_session", Value: "signed"}},
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/app",
		},
		{
			name:         "shell redirects unauthenticated member to login",
			path:         "/app",
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/app/login",
		},
		{
			name:       "shell renders for authenticated member",
			path:       "/app",
			cookies:    []*http.Cookie{{Name: "apollo_session", Value: "signed"}},
			wantStatus: http.StatusOK,
			wantBody:   `id="membership-card"`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, testCase.path, nil)
			for _, cookie := range testCase.cookies {
				request.AddCookie(cookie)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, testCase.wantStatus)
			}
			if testCase.wantLocation != "" {
				if location := recorder.Header().Get("Location"); location != testCase.wantLocation {
					t.Fatalf("Location = %q, want %q", location, testCase.wantLocation)
				}
			}
			if testCase.wantBody != "" && !strings.Contains(recorder.Body.String(), testCase.wantBody) {
				t.Fatalf("body = %q, want substring %q", recorder.Body.String(), testCase.wantBody)
			}
		})
	}
}

func TestWebUIAssetsAreServedThroughTheEmbeddedShell(t *testing.T) {
	handler := NewHandler(Dependencies{
		Auth: stubAuthenticator{cookieName: "apollo_session"},
	})

	for _, testCase := range []struct {
		path         string
		wantStatus   int
		wantContains string
	}{
		{
			path:         "/app/assets/app.css",
			wantStatus:   http.StatusOK,
			wantContains: ".workout-shell-grid",
		},
		{
			path:         "/app/assets/app.mjs",
			wantStatus:   http.StatusOK,
			wantContains: "export function formatTimestamp",
		},
	} {
		request := httptest.NewRequest(http.MethodGet, testCase.path, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != testCase.wantStatus {
			t.Fatalf("%s status = %d, want %d", testCase.path, recorder.Code, testCase.wantStatus)
		}
		if !strings.Contains(recorder.Body.String(), testCase.wantContains) {
			t.Fatalf("%s body = %q, want substring %q", testCase.path, recorder.Body.String(), testCase.wantContains)
		}
	}
}
