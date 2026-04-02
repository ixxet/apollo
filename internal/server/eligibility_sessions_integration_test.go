package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/store"
)

func TestLobbyEligibilityEndpointRejectsMissingTamperedExpiredAndRevokedSessions(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	missingCookieResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil)
	if missingCookieResponse.Code != http.StatusUnauthorized {
		t.Fatalf("missingCookieResponse.Code = %d, want %d", missingCookieResponse.Code, http.StatusUnauthorized)
	}

	validCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-eligibility-003", "eligibility-003@example.com")
	tamperedCookie := *validCookie
	tamperedCookie.Value = tamperSignedCookieValue(t, validCookie.Value)
	tamperedCookieResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, &tamperedCookie)
	if tamperedCookieResponse.Code != http.StatusUnauthorized {
		t.Fatalf("tamperedCookieResponse.Code = %d, want %d", tamperedCookieResponse.Code, http.StatusUnauthorized)
	}

	expiredUser := createVerifiedUser(t, env, "student-eligibility-expired", "eligibility-expired@example.com")
	expiredSession, err := env.queries.CreateSession(context.Background(), store.CreateSessionParams{
		UserID:    expiredUser.ID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateSession(expired) error = %v", err)
	}
	expiredCookie := env.cookies.SessionCookie(expiredSession.ID, expiredSession.ExpiresAt.Time)
	expiredCookieResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, expiredCookie)
	if expiredCookieResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expiredCookieResponse.Code = %d, want %d", expiredCookieResponse.Code, http.StatusUnauthorized)
	}

	logoutResponse := env.doRequest(t, http.MethodPost, "/api/v1/auth/logout", nil, validCookie)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logoutResponse.Code = %d, want %d", logoutResponse.Code, http.StatusNoContent)
	}
	postLogoutResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/eligibility", nil, validCookie)
	if postLogoutResponse.Code != http.StatusUnauthorized {
		t.Fatalf("postLogoutResponse.Code = %d, want %d", postLogoutResponse.Code, http.StatusUnauthorized)
	}
}
