package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/store"
)

type stubStore struct {
	startVerificationFunc       func(ctx context.Context, studentID string, email string, displayName string, tokenHash string, expiresAt time.Time) (*store.ApolloUser, *store.ApolloEmailVerificationToken, error)
	verifyTokenAndCreateSession func(ctx context.Context, tokenHash string, now time.Time, sessionExpiresAt time.Time) (*store.ApolloUser, *store.ApolloSession, error)
	lookupSessionFunc           func(ctx context.Context, sessionID uuid.UUID, now time.Time) (*AuthenticatedSession, error)
	revokeSessionFunc           func(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error
}

func (s stubStore) StartVerification(ctx context.Context, studentID string, email string, displayName string, tokenHash string, expiresAt time.Time) (*store.ApolloUser, *store.ApolloEmailVerificationToken, error) {
	return s.startVerificationFunc(ctx, studentID, email, displayName, tokenHash, expiresAt)
}

func (s stubStore) VerifyTokenAndCreateSession(ctx context.Context, tokenHash string, now time.Time, sessionExpiresAt time.Time) (*store.ApolloUser, *store.ApolloSession, error) {
	return s.verifyTokenAndCreateSession(ctx, tokenHash, now, sessionExpiresAt)
}

func (s stubStore) LookupSession(ctx context.Context, sessionID uuid.UUID, now time.Time) (*AuthenticatedSession, error) {
	return s.lookupSessionFunc(ctx, sessionID, now)
}

func (s stubStore) RevokeSession(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error {
	return s.revokeSessionFunc(ctx, sessionID, revokedAt)
}

type fakeEmailSender struct {
	deliveries []VerificationDelivery
}

func (s *fakeEmailSender) SendVerification(_ context.Context, delivery VerificationDelivery) error {
	s.deliveries = append(s.deliveries, delivery)
	return nil
}

func TestStartVerificationAcceptsValidInputAndUsesFakeSender(t *testing.T) {
	now := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	sender := &fakeEmailSender{}
	service := NewService(stubStore{
		startVerificationFunc: func(_ context.Context, studentID string, email string, displayName string, tokenHash string, expiresAt time.Time) (*store.ApolloUser, *store.ApolloEmailVerificationToken, error) {
			if studentID != "student-001" {
				t.Fatalf("studentID = %q, want student-001", studentID)
			}
			if email != "member@example.com" {
				t.Fatalf("email = %q, want member@example.com", email)
			}
			if displayName != "student-001" {
				t.Fatalf("displayName = %q, want student-001", displayName)
			}
			if tokenHash == "" {
				t.Fatal("tokenHash = empty, want populated token hash")
			}
			if !expiresAt.Equal(now.Add(15 * time.Minute)) {
				t.Fatalf("expiresAt = %s, want %s", expiresAt, now.Add(15*time.Minute))
			}
			return &store.ApolloUser{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111")}, &store.ApolloEmailVerificationToken{}, nil
		},
		verifyTokenAndCreateSession: func(context.Context, string, time.Time, time.Time) (*store.ApolloUser, *store.ApolloSession, error) {
			panic("VerifyTokenAndCreateSession should not be called")
		},
		lookupSessionFunc: func(context.Context, uuid.UUID, time.Time) (*AuthenticatedSession, error) {
			panic("LookupSession should not be called")
		},
		revokeSessionFunc: func(context.Context, uuid.UUID, time.Time) error {
			panic("RevokeSession should not be called")
		},
	}, mustCookieManager(t), sender, 15*time.Minute, 24*time.Hour)
	service.now = func() time.Time { return now }

	if err := service.StartVerification(context.Background(), StartVerificationInput{
		StudentID: " Student-001 ",
		Email:     " MEMBER@example.com ",
	}); err != nil {
		t.Fatalf("StartVerification() error = %v", err)
	}
	if len(sender.deliveries) != 1 {
		t.Fatalf("len(sender.deliveries) = %d, want 1", len(sender.deliveries))
	}
	if sender.deliveries[0].StudentID != "student-001" {
		t.Fatalf("delivery.StudentID = %q, want student-001", sender.deliveries[0].StudentID)
	}
	if sender.deliveries[0].Email != "member@example.com" {
		t.Fatalf("delivery.Email = %q, want member@example.com", sender.deliveries[0].Email)
	}
	if sender.deliveries[0].Token == "" {
		t.Fatal("delivery.Token = empty, want generated verification token")
	}
}

func TestStartVerificationRejectsInvalidInputWithTableDrivenCoverage(t *testing.T) {
	testCases := []struct {
		name        string
		input       StartVerificationInput
		expectedErr error
	}{
		{
			name: "invalid student id",
			input: StartVerificationInput{
				StudentID: "bad id",
				Email:     "member@example.com",
			},
			expectedErr: ErrInvalidStudentID,
		},
		{
			name: "invalid email",
			input: StartVerificationInput{
				StudentID: "student-001",
				Email:     "not-an-email",
			},
			expectedErr: ErrInvalidEmail,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := NewService(stubStore{
				startVerificationFunc: func(context.Context, string, string, string, string, time.Time) (*store.ApolloUser, *store.ApolloEmailVerificationToken, error) {
					t.Fatal("StartVerification store call should not happen for invalid input")
					return nil, nil, nil
				},
				verifyTokenAndCreateSession: func(context.Context, string, time.Time, time.Time) (*store.ApolloUser, *store.ApolloSession, error) {
					panic("VerifyTokenAndCreateSession should not be called")
				},
				lookupSessionFunc: func(context.Context, uuid.UUID, time.Time) (*AuthenticatedSession, error) {
					panic("LookupSession should not be called")
				},
				revokeSessionFunc: func(context.Context, uuid.UUID, time.Time) error {
					panic("RevokeSession should not be called")
				},
			}, mustCookieManager(t), &fakeEmailSender{}, 15*time.Minute, 24*time.Hour)

			err := service.StartVerification(context.Background(), testCase.input)
			if err != testCase.expectedErr {
				t.Fatalf("StartVerification() error = %v, want %v", err, testCase.expectedErr)
			}
		})
	}
}

func TestVerifyTokenReturnsUnknownUserErrorClearly(t *testing.T) {
	service := NewService(stubStore{
		startVerificationFunc: func(context.Context, string, string, string, string, time.Time) (*store.ApolloUser, *store.ApolloEmailVerificationToken, error) {
			panic("StartVerification should not be called")
		},
		verifyTokenAndCreateSession: func(context.Context, string, time.Time, time.Time) (*store.ApolloUser, *store.ApolloSession, error) {
			return nil, nil, ErrVerificationUnknownUser
		},
		lookupSessionFunc: func(context.Context, uuid.UUID, time.Time) (*AuthenticatedSession, error) {
			panic("LookupSession should not be called")
		},
		revokeSessionFunc: func(context.Context, uuid.UUID, time.Time) error {
			panic("RevokeSession should not be called")
		},
	}, mustCookieManager(t), &fakeEmailSender{}, 15*time.Minute, 24*time.Hour)

	_, err := service.VerifyToken(context.Background(), "valid-looking-token")
	if err != ErrVerificationUnknownUser {
		t.Fatalf("VerifyToken() error = %v, want %v", err, ErrVerificationUnknownUser)
	}
}

func TestAuthenticateSessionRejectsExpiredSession(t *testing.T) {
	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	service := NewService(stubStore{
		startVerificationFunc: func(context.Context, string, string, string, string, time.Time) (*store.ApolloUser, *store.ApolloEmailVerificationToken, error) {
			panic("StartVerification should not be called")
		},
		verifyTokenAndCreateSession: func(context.Context, string, time.Time, time.Time) (*store.ApolloUser, *store.ApolloSession, error) {
			panic("VerifyTokenAndCreateSession should not be called")
		},
		lookupSessionFunc: func(_ context.Context, actualSessionID uuid.UUID, _ time.Time) (*AuthenticatedSession, error) {
			if actualSessionID != sessionID {
				t.Fatalf("actualSessionID = %s, want %s", actualSessionID, sessionID)
			}
			return nil, ErrSessionExpired
		},
		revokeSessionFunc: func(context.Context, uuid.UUID, time.Time) error {
			panic("RevokeSession should not be called")
		},
	}, mustCookieManager(t), &fakeEmailSender{}, 15*time.Minute, 24*time.Hour)

	_, err := service.AuthenticateSession(context.Background(), service.SessionCookie(sessionID, time.Now().Add(time.Hour)).Value)
	if err != ErrSessionExpired {
		t.Fatalf("AuthenticateSession() error = %v, want %v", err, ErrSessionExpired)
	}
}

func TestAuthenticateSessionReturnsDeterministicRoleAndCapabilities(t *testing.T) {
	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	service := NewService(stubStore{
		startVerificationFunc: func(context.Context, string, string, string, string, time.Time) (*store.ApolloUser, *store.ApolloEmailVerificationToken, error) {
			panic("StartVerification should not be called")
		},
		verifyTokenAndCreateSession: func(context.Context, string, time.Time, time.Time) (*store.ApolloUser, *store.ApolloSession, error) {
			panic("VerifyTokenAndCreateSession should not be called")
		},
		lookupSessionFunc: func(_ context.Context, actualSessionID uuid.UUID, _ time.Time) (*AuthenticatedSession, error) {
			if actualSessionID != sessionID {
				t.Fatalf("actualSessionID = %s, want %s", actualSessionID, sessionID)
			}
			return &AuthenticatedSession{
				SessionID: sessionID,
				UserID:    userID,
				Role:      authz.RoleManager,
			}, nil
		},
		revokeSessionFunc: func(context.Context, uuid.UUID, time.Time) error {
			panic("RevokeSession should not be called")
		},
	}, mustCookieManager(t), &fakeEmailSender{}, 15*time.Minute, 24*time.Hour)

	principal, err := service.AuthenticateSession(context.Background(), service.SessionCookie(sessionID, time.Now().Add(time.Hour)).Value)
	if err != nil {
		t.Fatalf("AuthenticateSession() error = %v", err)
	}
	if principal.Role != authz.RoleManager {
		t.Fatalf("principal.Role = %q, want %q", principal.Role, authz.RoleManager)
	}
	expected := []authz.Capability{
		authz.CapabilityCompetitionLiveManage,
		authz.CapabilityCompetitionRead,
		authz.CapabilityCompetitionStructureManage,
		authz.CapabilityScheduleManage,
		authz.CapabilityScheduleRead,
	}
	if len(principal.Capabilities) != len(expected) {
		t.Fatalf("len(principal.Capabilities) = %d, want %d", len(principal.Capabilities), len(expected))
	}
	for index, capability := range expected {
		if principal.Capabilities[index] != capability {
			t.Fatalf("principal.Capabilities[%d] = %q, want %q", index, principal.Capabilities[index], capability)
		}
	}
}

func mustCookieManager(t *testing.T) *SessionCookieManager {
	t.Helper()

	manager, err := NewSessionCookieManager("apollo_session", "0123456789abcdef0123456789abcdef", true)
	if err != nil {
		t.Fatalf("NewSessionCookieManager() error = %v", err)
	}

	return manager
}
