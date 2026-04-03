package recommendations

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

const defaultRecoveryWindow = 24 * time.Hour

type RecommendationType string
type RecommendationReason string

const (
	TypeResumeInProgressWorkout  RecommendationType = "resume_in_progress_workout"
	TypeStartFirstWorkout        RecommendationType = "start_first_workout"
	TypeRepeatLastFinishedWorkout RecommendationType = "repeat_last_finished_workout"
	TypeRecoveryDay              RecommendationType = "recovery_day"
)

const (
	ReasonInProgressWorkoutExists           RecommendationReason = "in_progress_workout_exists"
	ReasonNoFinishedWorkouts                RecommendationReason = "no_finished_workouts"
	ReasonLastFinishedWithinRecoveryWindow  RecommendationReason = "last_finished_within_recovery_window"
	ReasonLastFinishedOutsideRecoveryWindow RecommendationReason = "last_finished_outside_recovery_window"
)

var ErrInvalidFinishedWorkoutState = errors.New("latest finished workout is missing finished_at")

type Clock func() time.Time

type Store interface {
	GetInProgressWorkoutByUserID(ctx context.Context, userID uuid.UUID) (*WorkoutSnapshot, error)
	GetLatestFinishedWorkoutByUserID(ctx context.Context, userID uuid.UUID) (*WorkoutSnapshot, error)
}

type Service struct {
	store          Store
	now            Clock
	recoveryWindow time.Duration
}

type WorkoutSnapshot struct {
	ID         uuid.UUID
	StartedAt  time.Time
	FinishedAt *time.Time
}

type Evidence struct {
	InProgressWorkoutID  *uuid.UUID `json:"in_progress_workout_id,omitempty"`
	InProgressStartedAt  *time.Time `json:"in_progress_started_at,omitempty"`
	LastFinishedWorkoutID *uuid.UUID `json:"last_finished_workout_id,omitempty"`
	LastFinishedAt       *time.Time `json:"last_finished_at,omitempty"`
	RecoveryWindowHours  int        `json:"recovery_window_hours,omitempty"`
}

type WorkoutRecommendation struct {
	Type        RecommendationType   `json:"type"`
	Reason      RecommendationReason `json:"reason"`
	Evidence    Evidence             `json:"evidence"`
	WorkoutID   *uuid.UUID           `json:"workout_id,omitempty"`
	GeneratedAt time.Time            `json:"generated_at"`
}

func NewService(store Store) *Service {
	return &Service{
		store:          store,
		now:            time.Now,
		recoveryWindow: defaultRecoveryWindow,
	}
}

func (s *Service) GetWorkoutRecommendation(ctx context.Context, userID uuid.UUID) (WorkoutRecommendation, error) {
	generatedAt := s.now().UTC()

	inProgress, err := s.store.GetInProgressWorkoutByUserID(ctx, userID)
	if err != nil {
		return WorkoutRecommendation{}, err
	}
	if inProgress != nil {
		return WorkoutRecommendation{
			Type:      TypeResumeInProgressWorkout,
			Reason:    ReasonInProgressWorkoutExists,
			WorkoutID: uuidPtr(inProgress.ID),
			Evidence: Evidence{
				InProgressWorkoutID: uuidPtr(inProgress.ID),
				InProgressStartedAt: timePtr(inProgress.StartedAt.UTC()),
			},
			GeneratedAt: generatedAt,
		}, nil
	}

	lastFinished, err := s.store.GetLatestFinishedWorkoutByUserID(ctx, userID)
	if err != nil {
		return WorkoutRecommendation{}, err
	}
	if lastFinished == nil {
		return WorkoutRecommendation{
			Type:        TypeStartFirstWorkout,
			Reason:      ReasonNoFinishedWorkouts,
			Evidence:    Evidence{},
			GeneratedAt: generatedAt,
		}, nil
	}
	if lastFinished.FinishedAt == nil {
		return WorkoutRecommendation{}, ErrInvalidFinishedWorkoutState
	}

	evidence := Evidence{
		LastFinishedWorkoutID: uuidPtr(lastFinished.ID),
		LastFinishedAt:        timePtr(lastFinished.FinishedAt.UTC()),
		RecoveryWindowHours:   int(s.recoveryWindow / time.Hour),
	}
	if generatedAt.Sub(lastFinished.FinishedAt.UTC()) < s.recoveryWindow {
		return WorkoutRecommendation{
			Type:        TypeRecoveryDay,
			Reason:      ReasonLastFinishedWithinRecoveryWindow,
			WorkoutID:   uuidPtr(lastFinished.ID),
			Evidence:    evidence,
			GeneratedAt: generatedAt,
		}, nil
	}

	return WorkoutRecommendation{
		Type:        TypeRepeatLastFinishedWorkout,
		Reason:      ReasonLastFinishedOutsideRecoveryWindow,
		WorkoutID:   uuidPtr(lastFinished.ID),
		Evidence:    evidence,
		GeneratedAt: generatedAt,
	}, nil
}

func uuidPtr(value uuid.UUID) *uuid.UUID {
	result := value
	return &result
}

func timePtr(value time.Time) *time.Time {
	result := value
	return &result
}
