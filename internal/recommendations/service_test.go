package recommendations

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type stubStore struct {
	inProgress   *WorkoutSnapshot
	lastFinished *WorkoutSnapshot
	inProgressErr error
	lastFinishedErr error
}

func (s stubStore) GetInProgressWorkoutByUserID(context.Context, uuid.UUID) (*WorkoutSnapshot, error) {
	if s.inProgressErr != nil {
		return nil, s.inProgressErr
	}
	return s.inProgress, nil
}

func (s stubStore) GetLatestFinishedWorkoutByUserID(context.Context, uuid.UUID) (*WorkoutSnapshot, error) {
	if s.lastFinishedErr != nil {
		return nil, s.lastFinishedErr
	}
	return s.lastFinished, nil
}

func TestGetWorkoutRecommendationUsesDeterministicPrecedence(t *testing.T) {
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	inProgressID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	finishedID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	now := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	recentFinish := now.Add(-23 * time.Hour)
	oldFinish := now.Add(-25 * time.Hour)
	startedAt := now.Add(-2 * time.Hour)

	testCases := []struct {
		name         string
		store        stubStore
		wantType     RecommendationType
		wantReason   RecommendationReason
		wantWorkoutID *uuid.UUID
		wantRecoveryHours int
	}{
		{
			name: "in progress workout beats older finished history",
			store: stubStore{
				inProgress: &WorkoutSnapshot{ID: inProgressID, StartedAt: startedAt},
				lastFinished: &WorkoutSnapshot{ID: finishedID, StartedAt: startedAt.Add(-48 * time.Hour), FinishedAt: &oldFinish},
			},
			wantType:      TypeResumeInProgressWorkout,
			wantReason:    ReasonInProgressWorkoutExists,
			wantWorkoutID: &inProgressID,
		},
		{
			name:       "no workouts returns cold start recommendation",
			store:      stubStore{},
			wantType:   TypeStartFirstWorkout,
			wantReason: ReasonNoFinishedWorkouts,
		},
		{
			name: "recently finished workout returns recovery day",
			store: stubStore{
				lastFinished: &WorkoutSnapshot{ID: finishedID, StartedAt: startedAt.Add(-24 * time.Hour), FinishedAt: &recentFinish},
			},
			wantType:      TypeRecoveryDay,
			wantReason:    ReasonLastFinishedWithinRecoveryWindow,
			wantWorkoutID: &finishedID,
			wantRecoveryHours: 24,
		},
		{
			name: "exact recovery boundary falls through to repeat",
			store: stubStore{
				lastFinished: &WorkoutSnapshot{ID: finishedID, StartedAt: startedAt.Add(-30 * time.Hour), FinishedAt: func() *time.Time { value := now.Add(-24 * time.Hour); return &value }()},
			},
			wantType:      TypeRepeatLastFinishedWorkout,
			wantReason:    ReasonLastFinishedOutsideRecoveryWindow,
			wantWorkoutID: &finishedID,
			wantRecoveryHours: 24,
		},
		{
			name: "older finished workout repeats last finished workout",
			store: stubStore{
				lastFinished: &WorkoutSnapshot{ID: finishedID, StartedAt: startedAt.Add(-48 * time.Hour), FinishedAt: &oldFinish},
			},
			wantType:      TypeRepeatLastFinishedWorkout,
			wantReason:    ReasonLastFinishedOutsideRecoveryWindow,
			wantWorkoutID: &finishedID,
			wantRecoveryHours: 24,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := NewService(testCase.store)
			service.now = func() time.Time { return now }

			recommendation, err := service.GetWorkoutRecommendation(context.Background(), userID)
			if err != nil {
				t.Fatalf("GetWorkoutRecommendation() error = %v", err)
			}
			if recommendation.Type != testCase.wantType {
				t.Fatalf("recommendation.Type = %q, want %q", recommendation.Type, testCase.wantType)
			}
			if recommendation.Reason != testCase.wantReason {
				t.Fatalf("recommendation.Reason = %q, want %q", recommendation.Reason, testCase.wantReason)
			}
			if !recommendation.GeneratedAt.Equal(now) {
				t.Fatalf("recommendation.GeneratedAt = %s, want %s", recommendation.GeneratedAt, now)
			}
			switch {
			case testCase.wantWorkoutID == nil && recommendation.WorkoutID != nil:
				t.Fatalf("recommendation.WorkoutID = %s, want nil", *recommendation.WorkoutID)
			case testCase.wantWorkoutID != nil && (recommendation.WorkoutID == nil || *recommendation.WorkoutID != *testCase.wantWorkoutID):
				t.Fatalf("recommendation.WorkoutID = %#v, want %s", recommendation.WorkoutID, *testCase.wantWorkoutID)
			}
			if recommendation.Evidence.RecoveryWindowHours != testCase.wantRecoveryHours {
				t.Fatalf("recommendation.Evidence.RecoveryWindowHours = %d, want %d", recommendation.Evidence.RecoveryWindowHours, testCase.wantRecoveryHours)
			}
		})
	}
}

func TestGetWorkoutRecommendationRejectsInvalidFinishedWorkoutState(t *testing.T) {
	service := NewService(stubStore{
		lastFinished: &WorkoutSnapshot{
			ID:        uuid.MustParse("33333333-3333-3333-3333-333333333333"),
			StartedAt: time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),
		},
	})
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	}

	_, err := service.GetWorkoutRecommendation(context.Background(), uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
	if !errors.Is(err, ErrInvalidFinishedWorkoutState) {
		t.Fatalf("GetWorkoutRecommendation() error = %v, want %v", err, ErrInvalidFinishedWorkoutState)
	}
}

func TestGetWorkoutRecommendationPropagatesStoreErrors(t *testing.T) {
	storeErr := errors.New("db unavailable")
	service := NewService(stubStore{inProgressErr: storeErr})

	_, err := service.GetWorkoutRecommendation(context.Background(), uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
	if !errors.Is(err, storeErr) {
		t.Fatalf("GetWorkoutRecommendation() error = %v, want %v", err, storeErr)
	}
}
