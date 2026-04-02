package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/store"
)

var (
	ErrInvalidStudentID = errors.New("invalid student id")
	ErrInvalidEmail     = errors.New("invalid email")
	ErrMissingToken     = errors.New("missing verification token")
)

var studentIDPattern = regexp.MustCompile(`^[a-z0-9-]{4,64}$`)

type EmailSender interface {
	SendVerification(ctx context.Context, delivery VerificationDelivery) error
}

type VerificationDelivery struct {
	StudentID string
	Email     string
	Token     string
}

type NoopEmailSender struct{}

func (s NoopEmailSender) SendVerification(context.Context, VerificationDelivery) error {
	return nil
}

type LogEmailSender struct{}

func (s LogEmailSender) SendVerification(_ context.Context, delivery VerificationDelivery) error {
	slog.Info(
		"member verification token generated",
		"student_id", delivery.StudentID,
		"email", delivery.Email,
		"token", delivery.Token,
		"verify_path", "/api/v1/auth/verify?token="+delivery.Token,
	)
	return nil
}

type Clock func() time.Time

type Store interface {
	StartVerification(ctx context.Context, studentID string, email string, displayName string, tokenHash string, expiresAt time.Time) (*store.ApolloUser, *store.ApolloEmailVerificationToken, error)
	VerifyTokenAndCreateSession(ctx context.Context, tokenHash string, now time.Time, sessionExpiresAt time.Time) (*store.ApolloUser, *store.ApolloSession, error)
	LookupSession(ctx context.Context, sessionID uuid.UUID, now time.Time) (*store.ApolloSession, error)
	RevokeSession(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error
}

type Service struct {
	repository           Store
	cookies              *SessionCookieManager
	sender               EmailSender
	now                  Clock
	verificationTokenTTL time.Duration
	sessionTTL           time.Duration
}

type StartVerificationInput struct {
	StudentID string
	Email     string
}

type VerifiedSession struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
}

type Principal struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
}

func NewService(repository Store, cookies *SessionCookieManager, sender EmailSender, verificationTokenTTL time.Duration, sessionTTL time.Duration) *Service {
	if sender == nil {
		sender = NoopEmailSender{}
	}

	return &Service{
		repository:           repository,
		cookies:              cookies,
		sender:               sender,
		now:                  time.Now,
		verificationTokenTTL: verificationTokenTTL,
		sessionTTL:           sessionTTL,
	}
}

func (s *Service) StartVerification(ctx context.Context, input StartVerificationInput) error {
	studentID, err := normalizeStudentID(input.StudentID)
	if err != nil {
		return err
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return err
	}

	token, tokenHash, err := generateVerificationToken()
	if err != nil {
		return err
	}

	now := s.now().UTC()
	_, _, err = s.repository.StartVerification(ctx, studentID, email, studentID, tokenHash, now.Add(s.verificationTokenTTL))
	if err != nil {
		return err
	}

	return s.sender.SendVerification(ctx, VerificationDelivery{
		StudentID: studentID,
		Email:     email,
		Token:     token,
	})
}

func (s *Service) VerifyToken(ctx context.Context, rawToken string) (VerifiedSession, error) {
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return VerifiedSession{}, ErrMissingToken
	}

	now := s.now().UTC()
	user, session, err := s.repository.VerifyTokenAndCreateSession(ctx, hashVerificationToken(token), now, now.Add(s.sessionTTL))
	if err != nil {
		return VerifiedSession{}, err
	}

	return VerifiedSession{
		SessionID: session.ID,
		UserID:    user.ID,
		ExpiresAt: session.ExpiresAt.Time.UTC(),
	}, nil
}

func (s *Service) AuthenticateSession(ctx context.Context, cookieValue string) (Principal, error) {
	sessionID, err := s.cookies.Decode(cookieValue)
	if err != nil {
		return Principal{}, ErrInvalidSessionCookie
	}

	session, err := s.repository.LookupSession(ctx, sessionID, s.now().UTC())
	if err != nil {
		return Principal{}, err
	}

	return Principal{
		SessionID: session.ID,
		UserID:    session.UserID,
	}, nil
}

func (s *Service) LogoutSession(ctx context.Context, cookieValue string) error {
	sessionID, err := s.cookies.Decode(cookieValue)
	if err != nil {
		return ErrInvalidSessionCookie
	}

	return s.repository.RevokeSession(ctx, sessionID, s.now().UTC())
}

func (s *Service) SessionCookie(sessionID uuid.UUID, expiresAt time.Time) *http.Cookie {
	return s.cookies.SessionCookie(sessionID, expiresAt)
}

func (s *Service) ExpiredSessionCookie() *http.Cookie {
	return s.cookies.ExpiredCookie()
}

func (s *Service) SessionCookieName() string {
	return s.cookies.Name()
}

func normalizeStudentID(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if !studentIDPattern.MatchString(normalized) {
		return "", ErrInvalidStudentID
	}
	return normalized, nil
}

func normalizeEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized {
		return "", ErrInvalidEmail
	}
	return normalized, nil
}

func generateVerificationToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate verification token: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashVerificationToken(token), nil
}

func hashVerificationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
