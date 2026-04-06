package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ixxet/apollo/internal/store"
)

var (
	ErrDuplicateStudentID       = errors.New("student id already belongs to another account")
	ErrDuplicateEmail           = errors.New("email already belongs to another account")
	ErrRegistrationConflict     = errors.New("student id and email belong to different accounts")
	ErrVerificationTokenInvalid = errors.New("verification token is invalid")
	ErrVerificationTokenExpired = errors.New("verification token is expired")
	ErrVerificationTokenUsed    = errors.New("verification token has already been used")
	ErrVerificationUnknownUser  = errors.New("verification token does not map to a user")
	ErrSessionNotFound          = errors.New("session not found")
	ErrSessionExpired           = errors.New("session expired")
	ErrSessionRevoked           = errors.New("session revoked")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) StartVerification(ctx context.Context, studentID string, email string, displayName string, tokenHash string, expiresAt time.Time) (*store.ApolloUser, *store.ApolloEmailVerificationToken, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := store.New(tx)
	studentUser, err := optionalUserByStudentID(ctx, queries, studentID)
	if err != nil {
		return nil, nil, err
	}
	emailUser, err := optionalUserByEmail(ctx, queries, email)
	if err != nil {
		return nil, nil, err
	}

	user, err := resolveRegistrationUser(studentID, email, studentUser, emailUser)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		createdUser, createErr := queries.CreateUser(ctx, store.CreateUserParams{
			StudentID:   studentID,
			DisplayName: displayName,
			Email:       email,
		})
		if createErr != nil {
			return nil, nil, createErr
		}
		converted := store.ApolloUserFromCreateUserRow(createdUser)
		user = &converted
	}

	if err := queries.DeletePendingEmailVerificationTokensByUserID(ctx, user.ID); err != nil {
		return nil, nil, err
	}

	token, err := queries.CreateEmailVerificationToken(ctx, store.CreateEmailVerificationTokenParams{
		UserID:    user.ID,
		Email:     email,
		TokenHash: tokenHash,
		ExpiresAt: timestamptz(expiresAt),
	})
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return user, &token, nil
}

func (r *Repository) VerifyTokenAndCreateSession(ctx context.Context, tokenHash string, now time.Time, sessionExpiresAt time.Time) (*store.ApolloUser, *store.ApolloSession, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := store.New(tx)
	token, err := queries.MarkEmailVerificationTokenUsed(ctx, store.MarkEmailVerificationTokenUsedParams{
		TokenHash: tokenHash,
		UsedAt:    timestamptz(now),
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, err
		}

		existingToken, lookupErr := optionalVerificationToken(ctx, queries.GetEmailVerificationTokenByHash, tokenHash)
		if lookupErr != nil {
			return nil, nil, lookupErr
		}
		switch {
		case existingToken == nil:
			return nil, nil, ErrVerificationTokenInvalid
		case existingToken.UsedAt.Valid:
			return nil, nil, ErrVerificationTokenUsed
		case !existingToken.ExpiresAt.Time.After(now.UTC()):
			return nil, nil, ErrVerificationTokenExpired
		default:
			return nil, nil, ErrVerificationTokenInvalid
		}
	}

	user, err := optionalUserByID(ctx, queries, token.UserID)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, ErrVerificationUnknownUser
	}

	verifiedUser, err := queries.MarkUserEmailVerified(ctx, store.MarkUserEmailVerifiedParams{
		ID:        token.UserID,
		UpdatedAt: timestamptz(now),
	})
	if err != nil {
		return nil, nil, err
	}

	session, err := queries.CreateSession(ctx, store.CreateSessionParams{
		UserID:    token.UserID,
		ExpiresAt: timestamptz(sessionExpiresAt),
	})
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	convertedUser := store.ApolloUserFromMarkUserEmailVerifiedRow(verifiedUser)
	return &convertedUser, &session, nil
}

func (r *Repository) LookupSession(ctx context.Context, sessionID uuid.UUID, now time.Time) (*store.ApolloSession, error) {
	session, err := store.New(r.db).GetSessionByID(ctx, sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	if session.RevokedAt.Valid {
		return nil, ErrSessionRevoked
	}
	if !session.ExpiresAt.Time.After(now.UTC()) {
		return nil, ErrSessionExpired
	}

	return &session, nil
}

func (r *Repository) RevokeSession(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error {
	return store.New(r.db).RevokeSession(ctx, store.RevokeSessionParams{
		ID:        sessionID,
		RevokedAt: timestamptz(revokedAt),
	})
}

func resolveRegistrationUser(studentID string, email string, studentUser *store.ApolloUser, emailUser *store.ApolloUser) (*store.ApolloUser, error) {
	switch {
	case studentUser != nil && studentUser.Email != email:
		return nil, ErrDuplicateStudentID
	case emailUser != nil && emailUser.StudentID != studentID:
		return nil, ErrDuplicateEmail
	case studentUser != nil && emailUser != nil && studentUser.ID != emailUser.ID:
		return nil, ErrRegistrationConflict
	case studentUser != nil:
		return studentUser, nil
	case emailUser != nil:
		return emailUser, nil
	default:
		return nil, nil
	}
}

func optionalUserByEmail(ctx context.Context, queries *store.Queries, email string) (*store.ApolloUser, error) {
	user, err := queries.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	converted := store.ApolloUserFromGetUserByEmailRow(user)
	return &converted, nil
}

func optionalUserByStudentID(ctx context.Context, queries *store.Queries, studentID string) (*store.ApolloUser, error) {
	user, err := queries.GetUserByStudentID(ctx, studentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	converted := store.ApolloUserFromGetUserByStudentIDRow(user)
	return &converted, nil
}

func optionalVerificationToken(ctx context.Context, lookup func(context.Context, string) (store.ApolloEmailVerificationToken, error), tokenHash string) (*store.ApolloEmailVerificationToken, error) {
	token, err := lookup(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func optionalUserByID(ctx context.Context, queries *store.Queries, userID uuid.UUID) (*store.ApolloUser, error) {
	user, err := queries.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	converted := store.ApolloUserFromGetUserByIDRow(user)
	return &converted, nil
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}
