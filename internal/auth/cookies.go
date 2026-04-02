package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidSessionCookie = errors.New("invalid session cookie")

type SessionCookieManager struct {
	name   string
	secret []byte
	secure bool
}

func NewSessionCookieManager(name string, secret string, secure bool) (*SessionCookieManager, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("session cookie name is required")
	}
	if len(strings.TrimSpace(secret)) < 32 {
		return nil, fmt.Errorf("session cookie secret must be at least 32 characters")
	}

	return &SessionCookieManager{
		name:   name,
		secret: []byte(secret),
		secure: secure,
	}, nil
}

func (m *SessionCookieManager) Name() string {
	return m.name
}

func (m *SessionCookieManager) Encode(sessionID uuid.UUID) string {
	signature := m.sign(sessionID.String())
	return sessionID.String() + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func (m *SessionCookieManager) Decode(value string) (uuid.UUID, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return uuid.Nil, ErrInvalidSessionCookie
	}

	sessionID, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, ErrInvalidSessionCookie
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return uuid.Nil, ErrInvalidSessionCookie
	}

	expected := m.sign(sessionID.String())
	if !hmac.Equal(signature, expected) {
		return uuid.Nil, ErrInvalidSessionCookie
	}

	return sessionID, nil
}

func (m *SessionCookieManager) SessionCookie(sessionID uuid.UUID, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     m.name,
		Value:    m.Encode(sessionID),
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt.UTC(),
		MaxAge:   int(time.Until(expiresAt.UTC()).Seconds()),
	}
}

func (m *SessionCookieManager) ExpiredCookie() *http.Cookie {
	return &http.Cookie{
		Name:     m.name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
	}
}

func (m *SessionCookieManager) sign(value string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
