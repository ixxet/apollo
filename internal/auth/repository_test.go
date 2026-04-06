package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/store"
	"github.com/ixxet/apollo/internal/testutil"
)

func TestRepositoryStartVerificationCreatesUserAndReplacesPendingTokens(t *testing.T) {
	ctx := context.Background()
	env := newAuthPostgresEnv(t, ctx)
	defer closePostgresEnv(t, env)

	repository := NewRepository(env.DB)
	queries := store.New(env.DB)
	expiresAt := time.Date(2026, 4, 2, 12, 15, 0, 0, time.UTC)

	firstUser, firstToken, err := repository.StartVerification(ctx, "student-001", "member@example.com", "student-001", "hash-001", expiresAt)
	if err != nil {
		t.Fatalf("StartVerification(first) error = %v", err)
	}
	secondUser, secondToken, err := repository.StartVerification(ctx, "student-001", "member@example.com", "student-001", "hash-002", expiresAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("StartVerification(second) error = %v", err)
	}
	if firstUser.ID != secondUser.ID {
		t.Fatalf("user ID changed across duplicate registration attempt: %s != %s", firstUser.ID, secondUser.ID)
	}
	if firstToken.ID == secondToken.ID {
		t.Fatal("expected second verification start to replace the first pending token")
	}

	var tokenCount int
	if err := env.DB.QueryRow(ctx, "SELECT count(*) FROM apollo.email_verification_tokens WHERE user_id = $1", firstUser.ID).Scan(&tokenCount); err != nil {
		t.Fatalf("QueryRow(token count) error = %v", err)
	}
	if tokenCount != 1 {
		t.Fatalf("tokenCount = %d, want 1", tokenCount)
	}

	storedUser, err := queries.GetUserByStudentID(ctx, "student-001")
	if err != nil {
		t.Fatalf("GetUserByStudentID() error = %v", err)
	}
	if storedUser.EmailVerifiedAt.Valid {
		t.Fatal("EmailVerifiedAt.Valid = true before verification, want false")
	}

	storedToken, err := queries.GetEmailVerificationTokenByHash(ctx, "hash-002")
	if err != nil {
		t.Fatalf("GetEmailVerificationTokenByHash() error = %v", err)
	}
	if storedToken.UserID != firstUser.ID {
		t.Fatalf("storedToken.UserID = %s, want %s", storedToken.UserID, firstUser.ID)
	}
}

func TestRepositoryStartVerificationRejectsConflictingRegistrations(t *testing.T) {
	testCases := []struct {
		name        string
		prepare     func(t *testing.T, ctx context.Context, queries *store.Queries)
		studentID   string
		email       string
		expectedErr error
	}{
		{
			name: "duplicate student id",
			prepare: func(t *testing.T, ctx context.Context, queries *store.Queries) {
				t.Helper()
				createUser(t, ctx, queries, "student-001", "member@example.com")
			},
			studentID:   "student-001",
			email:       "other@example.com",
			expectedErr: ErrDuplicateStudentID,
		},
		{
			name: "duplicate email",
			prepare: func(t *testing.T, ctx context.Context, queries *store.Queries) {
				t.Helper()
				createUser(t, ctx, queries, "student-001", "member@example.com")
			},
			studentID:   "student-002",
			email:       "member@example.com",
			expectedErr: ErrDuplicateEmail,
		},
		{
			name: "student and email belong to different accounts",
			prepare: func(t *testing.T, ctx context.Context, queries *store.Queries) {
				t.Helper()
				createUser(t, ctx, queries, "student-001", "one@example.com")
				createUser(t, ctx, queries, "student-002", "member@example.com")
			},
			studentID:   "student-001",
			email:       "member@example.com",
			expectedErr: ErrDuplicateStudentID,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			env := newAuthPostgresEnv(t, ctx)
			defer closePostgresEnv(t, env)

			queries := store.New(env.DB)
			testCase.prepare(t, ctx, queries)

			_, _, err := NewRepository(env.DB).StartVerification(ctx, testCase.studentID, testCase.email, testCase.studentID, "hash", time.Date(2026, 4, 2, 12, 15, 0, 0, time.UTC))
			if !errors.Is(err, testCase.expectedErr) {
				t.Fatalf("StartVerification() error = %v, want %v", err, testCase.expectedErr)
			}
		})
	}
}

func TestRepositoryVerifyTokenAndCreateSessionLifecycle(t *testing.T) {
	testCases := []struct {
		name        string
		prepare     func(t *testing.T, ctx context.Context, env *testutil.PostgresEnv, user store.ApolloUser)
		expectedErr error
	}{
		{
			name: "invalid token",
			prepare: func(t *testing.T, ctx context.Context, env *testutil.PostgresEnv, user store.ApolloUser) {
				t.Helper()
			},
			expectedErr: ErrVerificationTokenInvalid,
		},
		{
			name: "expired token",
			prepare: func(t *testing.T, ctx context.Context, env *testutil.PostgresEnv, user store.ApolloUser) {
				t.Helper()
				insertToken(t, ctx, env.DB, user.ID, user.Email, "expired-hash", time.Date(2026, 4, 2, 11, 0, 0, 0, time.UTC), nil)
			},
			expectedErr: ErrVerificationTokenExpired,
		},
		{
			name: "used token",
			prepare: func(t *testing.T, ctx context.Context, env *testutil.PostgresEnv, user store.ApolloUser) {
				t.Helper()
				usedAt := time.Date(2026, 4, 2, 11, 30, 0, 0, time.UTC)
				insertToken(t, ctx, env.DB, user.ID, user.Email, "used-hash", time.Date(2026, 4, 2, 13, 0, 0, 0, time.UTC), &usedAt)
			},
			expectedErr: ErrVerificationTokenUsed,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			env := newAuthPostgresEnv(t, ctx)
			defer closePostgresEnv(t, env)

			queries := store.New(env.DB)
			user := createUser(t, ctx, queries, "student-001", "member@example.com")
			testCase.prepare(t, ctx, env, user)

			_, _, err := NewRepository(env.DB).VerifyTokenAndCreateSession(ctx, tokenHashForTest(testCase.name), time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC), time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC))
			if !errors.Is(err, testCase.expectedErr) {
				t.Fatalf("VerifyTokenAndCreateSession() error = %v, want %v", err, testCase.expectedErr)
			}
		})
	}

	ctx := context.Background()
	env := newAuthPostgresEnv(t, ctx)
	defer closePostgresEnv(t, env)

	queries := store.New(env.DB)
	user := createUser(t, ctx, queries, "student-002", "verified@example.com")
	insertToken(t, ctx, env.DB, user.ID, user.Email, "valid-hash", time.Date(2026, 4, 2, 13, 0, 0, 0, time.UTC), nil)

	verifiedUser, session, err := NewRepository(env.DB).VerifyTokenAndCreateSession(ctx, "valid-hash", time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC), time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("VerifyTokenAndCreateSession(valid) error = %v", err)
	}
	if !verifiedUser.EmailVerifiedAt.Valid {
		t.Fatal("EmailVerifiedAt.Valid = false after verification, want true")
	}
	if session.ID == uuid.Nil {
		t.Fatal("session.ID = nil, want persisted session")
	}

	token, err := queries.GetEmailVerificationTokenByHash(ctx, "valid-hash")
	if err != nil {
		t.Fatalf("GetEmailVerificationTokenByHash(valid-hash) error = %v", err)
	}
	if !token.UsedAt.Valid {
		t.Fatal("UsedAt.Valid = false after verification, want true")
	}
}

func TestRepositoryVerifyTokenAllowsOnlyOneConcurrentWinner(t *testing.T) {
	ctx := context.Background()
	env := newAuthPostgresEnv(t, ctx)
	defer closePostgresEnv(t, env)

	queries := store.New(env.DB)
	user := createUser(t, ctx, queries, "student-001", "member@example.com")
	insertToken(t, ctx, env.DB, user.ID, user.Email, "concurrent-hash", time.Date(2026, 4, 2, 13, 0, 0, 0, time.UTC), nil)

	repository := NewRepository(env.DB)
	errorsCh := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for range 2 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, _, err := repository.VerifyTokenAndCreateSession(context.Background(), "concurrent-hash", time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC), time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC))
			errorsCh <- err
		}()
	}

	waitGroup.Wait()
	close(errorsCh)

	successCount := 0
	usedCount := 0
	for err := range errorsCh {
		switch {
		case err == nil:
			successCount++
		case errors.Is(err, ErrVerificationTokenUsed):
			usedCount++
		default:
			t.Fatalf("unexpected concurrent verification error: %v", err)
		}
	}
	if successCount != 1 {
		t.Fatalf("successCount = %d, want 1", successCount)
	}
	if usedCount != 1 {
		t.Fatalf("usedCount = %d, want 1", usedCount)
	}
}

func TestRepositoryLookupSessionRejectsExpiredAndRevokedSessions(t *testing.T) {
	testCases := []struct {
		name        string
		expiresAt   time.Time
		revokedAt   *time.Time
		expectedErr error
	}{
		{
			name:        "expired",
			expiresAt:   time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
			expectedErr: ErrSessionExpired,
		},
		{
			name:        "revoked",
			expiresAt:   time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
			revokedAt:   timePtr(time.Date(2026, 4, 2, 11, 0, 0, 0, time.UTC)),
			expectedErr: ErrSessionRevoked,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			env := newAuthPostgresEnv(t, ctx)
			defer closePostgresEnv(t, env)

			queries := store.New(env.DB)
			user := createUser(t, ctx, queries, "student-001", "member@example.com")
			sessionID := insertSession(t, ctx, env.DB, user.ID, testCase.expiresAt, testCase.revokedAt)

			_, err := NewRepository(env.DB).LookupSession(ctx, sessionID, time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC))
			if !errors.Is(err, testCase.expectedErr) {
				t.Fatalf("LookupSession() error = %v, want %v", err, testCase.expectedErr)
			}
		})
	}
}

func newAuthPostgresEnv(t *testing.T, ctx context.Context) *testutil.PostgresEnv {
	t.Helper()

	env, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	if err := testutil.ApplyApolloSchema(ctx, env.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	return env
}

func closePostgresEnv(t *testing.T, env *testutil.PostgresEnv) {
	t.Helper()

	if err := env.Close(); err != nil {
		t.Fatalf("env.Close() error = %v", err)
	}
}

func createUser(t *testing.T, ctx context.Context, queries *store.Queries, studentID string, email string) store.ApolloUser {
	t.Helper()

	user, err := queries.CreateUser(ctx, store.CreateUserParams{
		StudentID:   studentID,
		DisplayName: studentID,
		Email:       email,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	return store.ApolloUserFromCreateUserRow(user)
}

func insertToken(t *testing.T, ctx context.Context, db store.DBTX, userID uuid.UUID, email string, tokenHash string, expiresAt time.Time, usedAt *time.Time) {
	t.Helper()

	queries := store.New(db)
	if _, err := queries.CreateEmailVerificationToken(ctx, store.CreateEmailVerificationTokenParams{
		UserID:    userID,
		Email:     email,
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}); err != nil {
		t.Fatalf("CreateEmailVerificationToken() error = %v", err)
	}
	if usedAt != nil {
		if _, err := db.Exec(ctx, "UPDATE apollo.email_verification_tokens SET used_at = $2 WHERE token_hash = $1", tokenHash, *usedAt); err != nil {
			t.Fatalf("Exec(update used_at) error = %v", err)
		}
	}
}

func insertSession(t *testing.T, ctx context.Context, db store.DBTX, userID uuid.UUID, expiresAt time.Time, revokedAt *time.Time) uuid.UUID {
	t.Helper()

	queries := store.New(db)
	session, err := queries.CreateSession(ctx, store.CreateSessionParams{
		UserID:    userID,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if revokedAt != nil {
		if err := queries.RevokeSession(ctx, store.RevokeSessionParams{
			ID:        session.ID,
			RevokedAt: pgtype.Timestamptz{Time: *revokedAt, Valid: true},
		}); err != nil {
			t.Fatalf("RevokeSession() error = %v", err)
		}
	}

	return session.ID
}

func tokenHashForTest(name string) string {
	switch name {
	case "expired token":
		return "expired-hash"
	case "used token":
		return "used-hash"
	default:
		return "missing-hash"
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
