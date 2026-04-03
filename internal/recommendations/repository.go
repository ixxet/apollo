package recommendations

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ixxet/apollo/internal/store"
)

var ErrInvalidWorkoutTimestamp = errors.New("workout timestamp is invalid")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetInProgressWorkoutByUserID(ctx context.Context, userID uuid.UUID) (*WorkoutSnapshot, error) {
	workout, err := store.New(r.db).GetInProgressWorkoutByUserID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return workoutSnapshotFromStore(workout)
}

func (r *Repository) GetLatestFinishedWorkoutByUserID(ctx context.Context, userID uuid.UUID) (*WorkoutSnapshot, error) {
	workout, err := store.New(r.db).GetLatestFinishedWorkoutByUserID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return workoutSnapshotFromStore(workout)
}

func workoutSnapshotFromStore(workout store.ApolloWorkout) (*WorkoutSnapshot, error) {
	if !workout.StartedAt.Valid {
		return nil, ErrInvalidWorkoutTimestamp
	}

	snapshot := &WorkoutSnapshot{
		ID:        workout.ID,
		StartedAt: workout.StartedAt.Time.UTC(),
	}
	if workout.FinishedAt.Valid {
		finishedAt := workout.FinishedAt.Time.UTC()
		snapshot.FinishedAt = &finishedAt
	}

	return snapshot, nil
}

func timeValue(value time.Time) time.Time {
	return value.UTC()
}
