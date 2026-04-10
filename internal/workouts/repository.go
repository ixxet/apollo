package workouts

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ixxet/apollo/internal/store"
)

var emptyMetadata = json.RawMessage(`{}`)

type ExerciseDraft struct {
	Name     string
	Sets     int32
	Reps     int32
	WeightKg pgtype.Numeric
	RPE      pgtype.Numeric
	Notes    *string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateWorkout(ctx context.Context, userID uuid.UUID, notes *string) (*store.ApolloWorkout, error) {
	workout, err := store.New(r.db).CreateWorkout(ctx, store.CreateWorkoutParams{
		UserID:   userID,
		Notes:    notes,
		Metadata: emptyMetadata,
	})
	if err != nil {
		return nil, err
	}

	return workoutFromCreateRow(workout), nil
}

func (r *Repository) GetWorkoutByIDForUser(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID) (*store.ApolloWorkout, error) {
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

	return &workout, nil
}

func (r *Repository) GetInProgressWorkoutByUserID(ctx context.Context, userID uuid.UUID) (*store.ApolloWorkout, error) {
	workout, err := store.New(r.db).GetInProgressWorkoutByUserID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &workout, nil
}

func (r *Repository) ListWorkoutsByUserID(ctx context.Context, userID uuid.UUID) ([]store.ApolloWorkout, error) {
	return store.New(r.db).ListWorkoutsByUserID(ctx, userID)
}

func (r *Repository) ListExercisesByWorkoutID(ctx context.Context, workoutID uuid.UUID) ([]store.ApolloExercise, error) {
	return store.New(r.db).ListExercisesByWorkoutID(ctx, workoutID)
}

func (r *Repository) ListExercisesByWorkoutIDs(ctx context.Context, workoutIDs []uuid.UUID) (map[uuid.UUID][]store.ApolloExercise, error) {
	grouped := make(map[uuid.UUID][]store.ApolloExercise, len(workoutIDs))
	if len(workoutIDs) == 0 {
		return grouped, nil
	}

	rows, err := r.db.Query(ctx, `
SELECT id, workout_id, name, sets, reps, weight_kg, rpe, notes, position
FROM apollo.exercises
WHERE workout_id = ANY($1::uuid[])
ORDER BY workout_id ASC, position ASC, id ASC
`, workoutIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var exercise store.ApolloExercise
		if err := rows.Scan(
			&exercise.ID,
			&exercise.WorkoutID,
			&exercise.Name,
			&exercise.Sets,
			&exercise.Reps,
			&exercise.WeightKg,
			&exercise.Rpe,
			&exercise.Notes,
			&exercise.Position,
		); err != nil {
			return nil, err
		}
		grouped[exercise.WorkoutID] = append(grouped[exercise.WorkoutID], exercise)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return grouped, nil
}

func (r *Repository) ReplaceWorkoutDraft(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID, notes *string, exercises []ExerciseDraft) (*store.ApolloWorkout, []store.ApolloExercise, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := store.New(tx)
	updatedWorkout, err := queries.UpdateWorkoutDraft(ctx, store.UpdateWorkoutDraftParams{
		ID:       workoutID,
		UserID:   userID,
		Notes:    notes,
		Metadata: emptyMetadata,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	if err := queries.DeleteExercisesByWorkoutID(ctx, workoutID); err != nil {
		return nil, nil, err
	}

	storedExercises := make([]store.ApolloExercise, 0, len(exercises))
	for index, exercise := range exercises {
		row, err := queries.CreateExercise(ctx, store.CreateExerciseParams{
			WorkoutID: workoutID,
			Position:  int32(index + 1),
			Name:      exercise.Name,
			Sets:      exercise.Sets,
			Reps:      exercise.Reps,
			WeightKg:  exercise.WeightKg,
			Rpe:       exercise.RPE,
			Notes:     exercise.Notes,
		})
		if err != nil {
			return nil, nil, err
		}

		storedExercises = append(storedExercises, exerciseFromCreateRow(row))
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return workoutFromUpdateRow(updatedWorkout), storedExercises, nil
}

func (r *Repository) CountExercisesByWorkoutID(ctx context.Context, workoutID uuid.UUID) (int64, error) {
	return store.New(r.db).CountExercisesByWorkoutID(ctx, workoutID)
}

func (r *Repository) FinishWorkout(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID, finishedAt time.Time) (*store.ApolloWorkout, error) {
	workout, err := store.New(r.db).FinishWorkout(ctx, store.FinishWorkoutParams{
		ID:         workoutID,
		UserID:     userID,
		FinishedAt: timestamptz(finishedAt),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return workoutFromFinishRow(workout), nil
}

func workoutFromCreateRow(row store.CreateWorkoutRow) *store.ApolloWorkout {
	return &store.ApolloWorkout{
		ID:         row.ID,
		UserID:     row.UserID,
		StartedAt:  row.StartedAt,
		Notes:      row.Notes,
		Metadata:   row.Metadata,
		Status:     row.Status,
		FinishedAt: row.FinishedAt,
	}
}

func workoutFromUpdateRow(row store.UpdateWorkoutDraftRow) *store.ApolloWorkout {
	return &store.ApolloWorkout{
		ID:         row.ID,
		UserID:     row.UserID,
		StartedAt:  row.StartedAt,
		Notes:      row.Notes,
		Metadata:   row.Metadata,
		Status:     row.Status,
		FinishedAt: row.FinishedAt,
	}
}

func workoutFromFinishRow(row store.FinishWorkoutRow) *store.ApolloWorkout {
	return &store.ApolloWorkout{
		ID:         row.ID,
		UserID:     row.UserID,
		StartedAt:  row.StartedAt,
		Notes:      row.Notes,
		Metadata:   row.Metadata,
		Status:     row.Status,
		FinishedAt: row.FinishedAt,
	}
}

func exerciseFromCreateRow(row store.CreateExerciseRow) store.ApolloExercise {
	return store.ApolloExercise{
		ID:        row.ID,
		WorkoutID: row.WorkoutID,
		Name:      row.Name,
		Sets:      row.Sets,
		Reps:      row.Reps,
		WeightKg:  row.WeightKg,
		Rpe:       row.Rpe,
		Notes:     row.Notes,
		Position:  row.Position,
	}
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}
