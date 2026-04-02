package workouts

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/store"
)

const (
	StatusInProgress = "in_progress"
	StatusFinished   = "finished"
)

var (
	ErrNotFound                 = errors.New("workout not found")
	ErrWorkoutAlreadyInProgress = errors.New("workout already in progress")
	ErrWorkoutFinished          = errors.New("workout is already finished")
	ErrExercisePayloadRequired  = errors.New("exercise payload is required")
	ErrExerciseNameRequired     = errors.New("exercise name is required")
	ErrExerciseSetsInvalid      = errors.New("exercise sets must be positive")
	ErrExerciseRepsInvalid      = errors.New("exercise reps must be positive")
	ErrExerciseWeightInvalid    = errors.New("exercise weight_kg must be between 0 and 9999.99")
	ErrExerciseRPEInvalid       = errors.New("exercise rpe must be between 0 and 10")
	ErrCannotFinishEmptyWorkout = errors.New("workout cannot be finished without exercises")
)

type Clock func() time.Time

type Store interface {
	CreateWorkout(ctx context.Context, userID uuid.UUID, notes *string) (*store.ApolloWorkout, error)
	GetWorkoutByIDForUser(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID) (*store.ApolloWorkout, error)
	GetInProgressWorkoutByUserID(ctx context.Context, userID uuid.UUID) (*store.ApolloWorkout, error)
	ListWorkoutsByUserID(ctx context.Context, userID uuid.UUID) ([]store.ApolloWorkout, error)
	ListExercisesByWorkoutID(ctx context.Context, workoutID uuid.UUID) ([]store.ApolloExercise, error)
	ReplaceWorkoutDraft(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID, notes *string, exercises []ExerciseDraft) (*store.ApolloWorkout, []store.ApolloExercise, error)
	CountExercisesByWorkoutID(ctx context.Context, workoutID uuid.UUID) (int64, error)
	FinishWorkout(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID, finishedAt time.Time) (*store.ApolloWorkout, error)
}

type Service struct {
	repository Store
	now        Clock
}

type Exercise struct {
	Position int      `json:"position"`
	Name     string   `json:"name"`
	Sets     int      `json:"sets"`
	Reps     int      `json:"reps"`
	WeightKg *float64 `json:"weight_kg,omitempty"`
	RPE      *float64 `json:"rpe,omitempty"`
	Notes    *string  `json:"notes,omitempty"`
}

type Workout struct {
	ID         uuid.UUID  `json:"id"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Notes      *string    `json:"notes,omitempty"`
	Exercises  []Exercise `json:"exercises"`
}

type CreateInput struct {
	Notes *string `json:"notes"`
}

type ExerciseInput struct {
	Name     string   `json:"name"`
	Sets     int      `json:"sets"`
	Reps     int      `json:"reps"`
	WeightKg *float64 `json:"weight_kg"`
	RPE      *float64 `json:"rpe"`
	Notes    *string  `json:"notes"`
}

type UpdateInput struct {
	Notes     *string          `json:"notes"`
	Exercises *[]ExerciseInput `json:"exercises"`
}

func NewService(repository Store) *Service {
	return &Service{
		repository: repository,
		now:        time.Now,
	}
}

func (s *Service) CreateWorkout(ctx context.Context, userID uuid.UUID, input CreateInput) (Workout, error) {
	existingWorkout, err := s.repository.GetInProgressWorkoutByUserID(ctx, userID)
	if err != nil {
		return Workout{}, err
	}
	if existingWorkout != nil {
		return Workout{}, ErrWorkoutAlreadyInProgress
	}

	workout, err := s.repository.CreateWorkout(ctx, userID, normalizeOptionalText(input.Notes))
	if err != nil {
		if isUniqueViolation(err) {
			return Workout{}, ErrWorkoutAlreadyInProgress
		}
		return Workout{}, err
	}

	return buildWorkout(*workout, nil)
}

func (s *Service) ListWorkouts(ctx context.Context, userID uuid.UUID) ([]Workout, error) {
	workouts, err := s.repository.ListWorkoutsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]Workout, 0, len(workouts))
	for _, workout := range workouts {
		exercises, err := s.repository.ListExercisesByWorkoutID(ctx, workout.ID)
		if err != nil {
			return nil, err
		}

		builtWorkout, err := buildWorkout(workout, exercises)
		if err != nil {
			return nil, err
		}
		result = append(result, builtWorkout)
	}

	return result, nil
}

func (s *Service) GetWorkout(ctx context.Context, userID uuid.UUID, workoutID uuid.UUID) (Workout, error) {
	workout, err := s.repository.GetWorkoutByIDForUser(ctx, workoutID, userID)
	if err != nil {
		return Workout{}, err
	}
	if workout == nil {
		return Workout{}, ErrNotFound
	}

	exercises, err := s.repository.ListExercisesByWorkoutID(ctx, workout.ID)
	if err != nil {
		return Workout{}, err
	}

	return buildWorkout(*workout, exercises)
}

func (s *Service) UpdateWorkout(ctx context.Context, userID uuid.UUID, workoutID uuid.UUID, input UpdateInput) (Workout, error) {
	if input.Exercises == nil {
		return Workout{}, ErrExercisePayloadRequired
	}

	workout, err := s.repository.GetWorkoutByIDForUser(ctx, workoutID, userID)
	if err != nil {
		return Workout{}, err
	}
	if workout == nil {
		return Workout{}, ErrNotFound
	}
	if workout.Status == StatusFinished {
		return Workout{}, ErrWorkoutFinished
	}

	drafts, err := validateExerciseInputs(*input.Exercises)
	if err != nil {
		return Workout{}, err
	}

	updatedWorkout, exercises, err := s.repository.ReplaceWorkoutDraft(ctx, workoutID, userID, normalizeOptionalText(input.Notes), drafts)
	if err != nil {
		return Workout{}, err
	}
	if updatedWorkout == nil {
		refreshedWorkout, refreshErr := s.repository.GetWorkoutByIDForUser(ctx, workoutID, userID)
		if refreshErr != nil {
			return Workout{}, refreshErr
		}
		if refreshedWorkout == nil {
			return Workout{}, ErrNotFound
		}
		if refreshedWorkout.Status == StatusFinished {
			return Workout{}, ErrWorkoutFinished
		}
		return Workout{}, ErrNotFound
	}

	return buildWorkout(*updatedWorkout, exercises)
}

func (s *Service) FinishWorkout(ctx context.Context, userID uuid.UUID, workoutID uuid.UUID) (Workout, error) {
	workout, err := s.repository.GetWorkoutByIDForUser(ctx, workoutID, userID)
	if err != nil {
		return Workout{}, err
	}
	if workout == nil {
		return Workout{}, ErrNotFound
	}
	if workout.Status == StatusFinished {
		return Workout{}, ErrWorkoutFinished
	}

	exerciseCount, err := s.repository.CountExercisesByWorkoutID(ctx, workoutID)
	if err != nil {
		return Workout{}, err
	}
	if exerciseCount == 0 {
		return Workout{}, ErrCannotFinishEmptyWorkout
	}

	finishedWorkout, err := s.repository.FinishWorkout(ctx, workoutID, userID, s.now().UTC())
	if err != nil {
		return Workout{}, err
	}
	if finishedWorkout == nil {
		refreshedWorkout, refreshErr := s.repository.GetWorkoutByIDForUser(ctx, workoutID, userID)
		if refreshErr != nil {
			return Workout{}, refreshErr
		}
		if refreshedWorkout == nil {
			return Workout{}, ErrNotFound
		}
		if refreshedWorkout.Status == StatusFinished {
			return Workout{}, ErrWorkoutFinished
		}
		return Workout{}, ErrNotFound
	}

	exercises, err := s.repository.ListExercisesByWorkoutID(ctx, workoutID)
	if err != nil {
		return Workout{}, err
	}

	return buildWorkout(*finishedWorkout, exercises)
}

func validateExerciseInputs(inputs []ExerciseInput) ([]ExerciseDraft, error) {
	drafts := make([]ExerciseDraft, 0, len(inputs))
	for _, input := range inputs {
		name := strings.TrimSpace(input.Name)
		if name == "" {
			return nil, ErrExerciseNameRequired
		}
		if input.Sets <= 0 {
			return nil, ErrExerciseSetsInvalid
		}
		if input.Reps <= 0 {
			return nil, ErrExerciseRepsInvalid
		}

		weightKg, err := numericFromOptionalFloat(input.WeightKg, 9999.99, ErrExerciseWeightInvalid)
		if err != nil {
			return nil, err
		}
		rpe, err := numericFromOptionalFloat(input.RPE, 10.0, ErrExerciseRPEInvalid)
		if err != nil {
			return nil, err
		}

		drafts = append(drafts, ExerciseDraft{
			Name:     name,
			Sets:     int32(input.Sets),
			Reps:     int32(input.Reps),
			WeightKg: weightKg,
			RPE:      rpe,
			Notes:    normalizeOptionalText(input.Notes),
		})
	}

	return drafts, nil
}

func buildWorkout(workout store.ApolloWorkout, exercises []store.ApolloExercise) (Workout, error) {
	builtExercises := make([]Exercise, 0, len(exercises))
	for _, exercise := range exercises {
		builtExercise, err := buildExercise(exercise)
		if err != nil {
			return Workout{}, err
		}
		builtExercises = append(builtExercises, builtExercise)
	}

	return Workout{
		ID:         workout.ID,
		Status:     workout.Status,
		StartedAt:  workout.StartedAt.Time.UTC(),
		FinishedAt: optionalTime(workout.FinishedAt),
		Notes:      workout.Notes,
		Exercises:  builtExercises,
	}, nil
}

func buildExercise(exercise store.ApolloExercise) (Exercise, error) {
	weightKg, err := optionalFloat64(exercise.WeightKg)
	if err != nil {
		return Exercise{}, err
	}
	rpe, err := optionalFloat64(exercise.Rpe)
	if err != nil {
		return Exercise{}, err
	}

	return Exercise{
		Position: int(exercise.Position),
		Name:     exercise.Name,
		Sets:     int(exercise.Sets),
		Reps:     int(exercise.Reps),
		WeightKg: weightKg,
		RPE:      rpe,
		Notes:    exercise.Notes,
	}, nil
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}

	timeValue := value.Time.UTC()
	return &timeValue
}

func optionalFloat64(value pgtype.Numeric) (*float64, error) {
	if !value.Valid {
		return nil, nil
	}

	floatValue, err := value.Float64Value()
	if err != nil {
		return nil, err
	}
	if !floatValue.Valid {
		return nil, nil
	}

	return &floatValue.Float64, nil
}

func numericFromOptionalFloat(value *float64, max float64, errValue error) (pgtype.Numeric, error) {
	if value == nil {
		return pgtype.Numeric{}, nil
	}
	if *value < 0 || *value > max {
		return pgtype.Numeric{}, errValue
	}

	var numeric pgtype.Numeric
	if err := numeric.Scan(strconv.FormatFloat(*value, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}, err
	}

	return numeric, nil
}

func normalizeOptionalText(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
