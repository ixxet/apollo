package booking

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ixxet/apollo/internal/store"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, facilityKey string) ([]store.ApolloBookingRequest, error) {
	queries := store.New(r.db)
	if facilityKey != "" {
		return queries.ListBookingRequestsByFacilityKey(ctx, facilityKey)
	}
	return queries.ListBookingRequests(ctx)
}

func (r *Repository) Get(ctx context.Context, requestID uuid.UUID) (*store.ApolloBookingRequest, error) {
	row, err := store.New(r.db).GetBookingRequestByID(ctx, requestID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func withBookingQueriesTx[T any](ctx context.Context, db *pgxpool.Pool, fn func(*store.Queries) (T, error)) (T, error) {
	var zero T

	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return zero, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	result, err := fn(store.New(tx))
	if err != nil {
		return zero, err
	}

	if err := tx.Commit(ctx); err != nil {
		return zero, err
	}

	return result, nil
}
