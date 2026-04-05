package membership

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

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserByID(ctx context.Context, userID uuid.UUID) (*store.ApolloUser, error) {
	user, err := store.New(r.db).GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *Repository) GetLobbyMembershipByUserID(ctx context.Context, userID uuid.UUID) (*store.ApolloLobbyMembership, error) {
	record, err := store.New(r.db).GetLobbyMembershipByUserID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func (r *Repository) UpsertLobbyMembershipJoin(ctx context.Context, userID uuid.UUID, joinedAt time.Time) (*store.ApolloLobbyMembership, error) {
	record, err := store.New(r.db).UpsertLobbyMembershipJoin(ctx, store.UpsertLobbyMembershipJoinParams{
		UserID:   userID,
		JoinedAt: pgtype.Timestamptz{Time: joinedAt.UTC(), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func (r *Repository) LeaveLobbyMembership(ctx context.Context, userID uuid.UUID, leftAt time.Time) (*store.ApolloLobbyMembership, error) {
	record, err := store.New(r.db).LeaveLobbyMembership(ctx, store.LeaveLobbyMembershipParams{
		UserID: userID,
		LeftAt: pgtype.Timestamptz{Time: leftAt.UTC(), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &record, nil
}
