package auth

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSessionCookieManagerRoundTripAndSecurityAttributes(t *testing.T) {
	manager, err := NewSessionCookieManager("apollo_session", "0123456789abcdef0123456789abcdef", true)
	if err != nil {
		t.Fatalf("NewSessionCookieManager() error = %v", err)
	}

	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	expiresAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	cookie := manager.SessionCookie(sessionID, expiresAt)

	decodedSessionID, err := manager.Decode(cookie.Value)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decodedSessionID != sessionID {
		t.Fatalf("decodedSessionID = %s, want %s", decodedSessionID, sessionID)
	}
	if !cookie.HttpOnly {
		t.Fatal("cookie.HttpOnly = false, want true")
	}
	if !cookie.Secure {
		t.Fatal("cookie.Secure = false, want true")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie.SameSite = %v, want %v", cookie.SameSite, http.SameSiteStrictMode)
	}
	if cookie.Path != "/" {
		t.Fatalf("cookie.Path = %q, want /", cookie.Path)
	}
}

func TestSessionCookieManagerRejectsTamperedCookie(t *testing.T) {
	manager, err := NewSessionCookieManager("apollo_session", "0123456789abcdef0123456789abcdef", true)
	if err != nil {
		t.Fatalf("NewSessionCookieManager() error = %v", err)
	}

	value := manager.Encode(uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	tampered := tamperSignedCookieValue(t, value)

	if _, err := manager.Decode(tampered); err == nil {
		t.Fatal("Decode(tampered) error = nil, want tamper rejection")
	}
}

func tamperSignedCookieValue(t *testing.T, value string) string {
	t.Helper()

	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		t.Fatalf("cookie format = %q, want two parts", value)
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	if len(signature) == 0 {
		t.Fatal("signature length = 0, want non-empty signature")
	}

	signature[0] ^= 0xFF

	return parts[0] + "." + base64.RawURLEncoding.EncodeToString(signature)
}
