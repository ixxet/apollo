package profile

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ixxet/apollo/internal/store"
)

type Repository struct {
	queries *store.Queries
}

func NewRepository(db store.DBTX) *Repository {
	return &Repository{
		queries: store.New(db),
	}
}

func (r *Repository) GetUserByID(ctx context.Context, userID uuid.UUID) (*store.ApolloUser, error) {
	user, err := r.queries.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *Repository) UpdatePreferences(ctx context.Context, userID uuid.UUID, preferences []byte) (*store.ApolloUser, error) {
	user, err := r.queries.UpdateUserPreferences(ctx, store.UpdateUserPreferencesParams{
		ID:          userID,
		Preferences: preferences,
	})
	if err != nil {
		return nil, err
	}

	return &user, nil
}
