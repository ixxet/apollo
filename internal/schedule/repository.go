package schedule

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ixxet/apollo/internal/store"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListResourcesByFacilityKey(ctx context.Context, facilityKey string) ([]store.ApolloScheduleResource, error) {
	return store.New(r.db).ListScheduleResourcesByFacilityKey(ctx, facilityKey)
}

func (r *Repository) GetResourceByKey(ctx context.Context, resourceKey string) (*store.ApolloScheduleResource, error) {
	row, err := store.New(r.db).GetScheduleResourceByKey(ctx, resourceKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repository) UpsertResource(ctx context.Context, params store.UpsertScheduleResourceParams) (store.ApolloScheduleResource, error) {
	return store.New(r.db).UpsertScheduleResource(ctx, params)
}

func (r *Repository) ListResourceEdgesByFacilityKey(ctx context.Context, facilityKey string) ([]store.ApolloScheduleResourceEdge, error) {
	return store.New(r.db).ListScheduleResourceEdgesByFacilityKey(ctx, facilityKey)
}

func (r *Repository) UpsertResourceEdge(ctx context.Context, params store.UpsertScheduleResourceEdgeParams) (store.ApolloScheduleResourceEdge, error) {
	return store.New(r.db).UpsertScheduleResourceEdge(ctx, params)
}

func (r *Repository) ListBlocksByFacilityKey(ctx context.Context, facilityKey string) ([]store.ApolloScheduleBlock, error) {
	return store.New(r.db).ListScheduleBlocksByFacilityKey(ctx, facilityKey)
}

func (r *Repository) GetBlockByID(ctx context.Context, blockID uuid.UUID) (*store.ApolloScheduleBlock, error) {
	row, err := store.New(r.db).GetScheduleBlockByID(ctx, blockID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repository) ListExceptionsByBlockIDs(ctx context.Context, blockIDs []uuid.UUID) ([]store.ApolloScheduleBlockException, error) {
	if len(blockIDs) == 0 {
		return []store.ApolloScheduleBlockException{}, nil
	}

	return store.New(r.db).ListScheduleBlockExceptionsByBlockIDs(ctx, blockIDs)
}

func (r *Repository) CreateBlock(ctx context.Context, params store.CreateScheduleBlockParams) (store.ApolloScheduleBlock, error) {
	return store.New(r.db).CreateScheduleBlock(ctx, params)
}

func (r *Repository) BumpBlockVersion(ctx context.Context, params store.BumpScheduleBlockVersionParams) (store.ApolloScheduleBlock, error) {
	return store.New(r.db).BumpScheduleBlockVersion(ctx, params)
}

func (r *Repository) CancelBlock(ctx context.Context, params store.CancelScheduleBlockParams) (store.ApolloScheduleBlock, error) {
	return store.New(r.db).CancelScheduleBlock(ctx, params)
}

func (r *Repository) InsertBlockException(ctx context.Context, params store.InsertScheduleBlockExceptionParams) (store.ApolloScheduleBlockException, error) {
	return store.New(r.db).InsertScheduleBlockException(ctx, params)
}

func withScheduleQueriesTx[T any](ctx context.Context, db *pgxpool.Pool, fn func(*store.Queries) (T, error)) (T, error) {
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

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == "23505"
}

func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == "23514"
}
