package coaching

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

func (r *Repository) GetWorkoutByIDForUser(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID) (*WorkoutSnapshot, error) {
	workout, err := store.New(r.db).GetWorkoutByIDForUser(ctx, store.GetWorkoutByIDForUserParams{
		ID:     workoutID,
		UserID: userID,
	})
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

func (r *Repository) UpsertEffortFeedbackForFinishedWorkout(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID, effortLevel string) (*EffortFeedbackRecord, error) {
	row, err := store.New(r.db).UpsertEffortFeedbackForFinishedWorkout(ctx, store.UpsertEffortFeedbackForFinishedWorkoutParams{
		ID:          workoutID,
		UserID:      userID,
		EffortLevel: effortLevel,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &EffortFeedbackRecord{
		WorkoutID:   row.WorkoutID,
		EffortLevel: row.EffortLevel,
		CreatedAt:   row.CreatedAt.Time.UTC(),
	}, nil
}

func (r *Repository) UpsertRecoveryFeedbackForFinishedWorkout(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID, recoveryLevel string) (*RecoveryFeedbackRecord, error) {
	row, err := store.New(r.db).UpsertRecoveryFeedbackForFinishedWorkout(ctx, store.UpsertRecoveryFeedbackForFinishedWorkoutParams{
		ID:            workoutID,
		UserID:        userID,
		RecoveryLevel: recoveryLevel,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &RecoveryFeedbackRecord{
		WorkoutID:     row.WorkoutID,
		RecoveryLevel: row.RecoveryLevel,
		CreatedAt:     row.CreatedAt.Time.UTC(),
	}, nil
}

func (r *Repository) GetEffortFeedbackByWorkoutID(ctx context.Context, workoutID uuid.UUID) (*EffortFeedbackRecord, error) {
	row, err := store.New(r.db).GetEffortFeedbackByWorkoutID(ctx, workoutID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &EffortFeedbackRecord{
		WorkoutID:   row.WorkoutID,
		EffortLevel: row.EffortLevel,
		CreatedAt:   row.CreatedAt.Time.UTC(),
	}, nil
}

func (r *Repository) GetRecoveryFeedbackByWorkoutID(ctx context.Context, workoutID uuid.UUID) (*RecoveryFeedbackRecord, error) {
	row, err := store.New(r.db).GetRecoveryFeedbackByWorkoutID(ctx, workoutID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &RecoveryFeedbackRecord{
		WorkoutID:     row.WorkoutID,
		RecoveryLevel: row.RecoveryLevel,
		CreatedAt:     row.CreatedAt.Time.UTC(),
	}, nil
}

func workoutSnapshotFromStore(workout store.ApolloWorkout) (*WorkoutSnapshot, error) {
	if !workout.StartedAt.Valid {
		return nil, ErrInvalidWorkoutTimestamp
	}

	snapshot := &WorkoutSnapshot{
		ID:     workout.ID,
		Status: workout.Status,
	}
	if workout.FinishedAt.Valid {
		finishedAt := timeValue(workout.FinishedAt.Time)
		snapshot.FinishedAt = &finishedAt
	}

	return snapshot, nil
}

func timeValue(value time.Time) time.Time {
	return value.UTC()
}
