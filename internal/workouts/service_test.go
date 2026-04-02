package workouts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/store"
)

type stubStore struct {
	createWorkoutFunc              func(context.Context, uuid.UUID, *string) (*store.ApolloWorkout, error)
	getWorkoutByIDForUserFunc      func(context.Context, uuid.UUID, uuid.UUID) (*store.ApolloWorkout, error)
	getInProgressWorkoutByUserFunc func(context.Context, uuid.UUID) (*store.ApolloWorkout, error)
	listWorkoutsByUserIDFunc       func(context.Context, uuid.UUID) ([]store.ApolloWorkout, error)
	listExercisesByWorkoutIDFunc   func(context.Context, uuid.UUID) ([]store.ApolloExercise, error)
	replaceWorkoutDraftFunc        func(context.Context, uuid.UUID, uuid.UUID, *string, []ExerciseDraft) (*store.ApolloWorkout, []store.ApolloExercise, error)
	countExercisesByWorkoutIDFunc  func(context.Context, uuid.UUID) (int64, error)
	finishWorkoutFunc              func(context.Context, uuid.UUID, uuid.UUID, time.Time) (*store.ApolloWorkout, error)
}

func (s stubStore) CreateWorkout(ctx context.Context, userID uuid.UUID, notes *string) (*store.ApolloWorkout, error) {
	return s.createWorkoutFunc(ctx, userID, notes)
}

func (s stubStore) GetWorkoutByIDForUser(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID) (*store.ApolloWorkout, error) {
	return s.getWorkoutByIDForUserFunc(ctx, workoutID, userID)
}

func (s stubStore) GetInProgressWorkoutByUserID(ctx context.Context, userID uuid.UUID) (*store.ApolloWorkout, error) {
	return s.getInProgressWorkoutByUserFunc(ctx, userID)
}

func (s stubStore) ListWorkoutsByUserID(ctx context.Context, userID uuid.UUID) ([]store.ApolloWorkout, error) {
	return s.listWorkoutsByUserIDFunc(ctx, userID)
}

func (s stubStore) ListExercisesByWorkoutID(ctx context.Context, workoutID uuid.UUID) ([]store.ApolloExercise, error) {
	return s.listExercisesByWorkoutIDFunc(ctx, workoutID)
}

func (s stubStore) ReplaceWorkoutDraft(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID, notes *string, exercises []ExerciseDraft) (*store.ApolloWorkout, []store.ApolloExercise, error) {
	return s.replaceWorkoutDraftFunc(ctx, workoutID, userID, notes, exercises)
}

func (s stubStore) CountExercisesByWorkoutID(ctx context.Context, workoutID uuid.UUID) (int64, error) {
	return s.countExercisesByWorkoutIDFunc(ctx, workoutID)
}

func (s stubStore) FinishWorkout(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID, finishedAt time.Time) (*store.ApolloWorkout, error) {
	return s.finishWorkoutFunc(ctx, workoutID, userID, finishedAt)
}

func TestCreateWorkoutRejectsSecondInProgressWorkout(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	service := NewService(stubStore{
		getInProgressWorkoutByUserFunc: func(context.Context, uuid.UUID) (*store.ApolloWorkout, error) {
			return &store.ApolloWorkout{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Status: StatusInProgress}, nil
		},
		createWorkoutFunc: func(context.Context, uuid.UUID, *string) (*store.ApolloWorkout, error) {
			t.Fatal("CreateWorkout repository call should not happen when an in-progress workout already exists")
			return nil, nil
		},
	})

	_, err := service.CreateWorkout(context.Background(), userID, CreateInput{})
	if !errors.Is(err, ErrWorkoutAlreadyInProgress) {
		t.Fatalf("CreateWorkout() error = %v, want %v", err, ErrWorkoutAlreadyInProgress)
	}
}

func TestUpdateWorkoutValidatesExercisePayloadWithTableDrivenCoverage(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	workoutID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	baseWorkout := &store.ApolloWorkout{
		ID:        workoutID,
		UserID:    userID,
		Status:    StatusInProgress,
		StartedAt: timestamptz(time.Date(2026, 4, 2, 20, 0, 0, 0, time.UTC)),
	}

	testCases := []struct {
		name        string
		input       UpdateInput
		expectedErr error
	}{
		{
			name:        "missing exercise payload",
			input:       UpdateInput{},
			expectedErr: ErrExercisePayloadRequired,
		},
		{
			name: "blank exercise name",
			input: UpdateInput{
				Exercises: &[]ExerciseInput{{Name: "   ", Sets: 3, Reps: 8}},
			},
			expectedErr: ErrExerciseNameRequired,
		},
		{
			name: "sets must be positive",
			input: UpdateInput{
				Exercises: &[]ExerciseInput{{Name: "bench press", Sets: 0, Reps: 8}},
			},
			expectedErr: ErrExerciseSetsInvalid,
		},
		{
			name: "reps must be positive",
			input: UpdateInput{
				Exercises: &[]ExerciseInput{{Name: "bench press", Sets: 3, Reps: 0}},
			},
			expectedErr: ErrExerciseRepsInvalid,
		},
		{
			name: "weight out of range",
			input: UpdateInput{
				Exercises: &[]ExerciseInput{{Name: "bench press", Sets: 3, Reps: 8, WeightKg: float64Ptr(12000)}},
			},
			expectedErr: ErrExerciseWeightInvalid,
		},
		{
			name: "rpe out of range",
			input: UpdateInput{
				Exercises: &[]ExerciseInput{{Name: "bench press", Sets: 3, Reps: 8, RPE: float64Ptr(11)}},
			},
			expectedErr: ErrExerciseRPEInvalid,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := NewService(stubStore{
				getWorkoutByIDForUserFunc: func(context.Context, uuid.UUID, uuid.UUID) (*store.ApolloWorkout, error) {
					return baseWorkout, nil
				},
				replaceWorkoutDraftFunc: func(context.Context, uuid.UUID, uuid.UUID, *string, []ExerciseDraft) (*store.ApolloWorkout, []store.ApolloExercise, error) {
					t.Fatal("ReplaceWorkoutDraft repository call should not happen when validation fails")
					return nil, nil, nil
				},
			})

			_, err := service.UpdateWorkout(context.Background(), userID, workoutID, testCase.input)
			if !errors.Is(err, testCase.expectedErr) {
				t.Fatalf("UpdateWorkout() error = %v, want %v", err, testCase.expectedErr)
			}
		})
	}
}

func TestUpdateWorkoutRejectsFinishedWorkoutAndPreservesExerciseOrder(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	workoutID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	finishedWorkout := &store.ApolloWorkout{
		ID:         workoutID,
		UserID:     userID,
		Status:     StatusFinished,
		StartedAt:  timestamptz(time.Date(2026, 4, 2, 20, 0, 0, 0, time.UTC)),
		FinishedAt: timestamptz(time.Date(2026, 4, 2, 20, 45, 0, 0, time.UTC)),
	}
	service := NewService(stubStore{
		getWorkoutByIDForUserFunc: func(context.Context, uuid.UUID, uuid.UUID) (*store.ApolloWorkout, error) {
			return finishedWorkout, nil
		},
	})

	_, err := service.UpdateWorkout(context.Background(), userID, workoutID, UpdateInput{
		Exercises: &[]ExerciseInput{{Name: "bench press", Sets: 3, Reps: 8}},
	})
	if !errors.Is(err, ErrWorkoutFinished) {
		t.Fatalf("UpdateWorkout() error = %v, want %v", err, ErrWorkoutFinished)
	}

	inProgressWorkout := &store.ApolloWorkout{
		ID:        workoutID,
		UserID:    userID,
		Status:    StatusInProgress,
		StartedAt: timestamptz(time.Date(2026, 4, 2, 20, 0, 0, 0, time.UTC)),
	}
	var capturedExercises []ExerciseDraft
	service = NewService(stubStore{
		getWorkoutByIDForUserFunc: func(context.Context, uuid.UUID, uuid.UUID) (*store.ApolloWorkout, error) {
			return inProgressWorkout, nil
		},
		replaceWorkoutDraftFunc: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, notes *string, exercises []ExerciseDraft) (*store.ApolloWorkout, []store.ApolloExercise, error) {
			capturedExercises = append([]ExerciseDraft(nil), exercises...)
			return &store.ApolloWorkout{
					ID:        inProgressWorkout.ID,
					UserID:    inProgressWorkout.UserID,
					Status:    inProgressWorkout.Status,
					StartedAt: inProgressWorkout.StartedAt,
					Notes:     notes,
				}, []store.ApolloExercise{
					{Name: "front squat", Sets: 4, Reps: 5, Position: 1},
					{Name: "pull-up", Sets: 3, Reps: 8, Position: 2},
				}, nil
		},
	})

	workout, err := service.UpdateWorkout(context.Background(), userID, workoutID, UpdateInput{
		Notes: stringPtr(" lower body "),
		Exercises: &[]ExerciseInput{
			{Name: " front squat ", Sets: 4, Reps: 5},
			{Name: "pull-up", Sets: 3, Reps: 8},
		},
	})
	if err != nil {
		t.Fatalf("UpdateWorkout() error = %v", err)
	}
	if len(capturedExercises) != 2 {
		t.Fatalf("len(capturedExercises) = %d, want 2", len(capturedExercises))
	}
	if capturedExercises[0].Name != "front squat" || capturedExercises[1].Name != "pull-up" {
		t.Fatalf("capturedExercises names = [%q %q], want [front squat pull-up]", capturedExercises[0].Name, capturedExercises[1].Name)
	}
	if workout.Notes == nil || *workout.Notes != "lower body" {
		t.Fatalf("workout.Notes = %#v, want lower body", workout.Notes)
	}
	if len(workout.Exercises) != 2 || workout.Exercises[0].Position != 1 || workout.Exercises[1].Position != 2 {
		t.Fatalf("workout.Exercises = %#v, want stable ordered positions", workout.Exercises)
	}
}

func TestFinishWorkoutRejectsMissingAndFinishedStatesAndUsesClock(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	workoutID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	now := time.Date(2026, 4, 2, 21, 15, 0, 0, time.UTC)

	service := NewService(stubStore{
		getWorkoutByIDForUserFunc: func(context.Context, uuid.UUID, uuid.UUID) (*store.ApolloWorkout, error) {
			return nil, nil
		},
	})
	if _, err := service.FinishWorkout(context.Background(), userID, workoutID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("FinishWorkout(not found) error = %v, want %v", err, ErrNotFound)
	}

	service = NewService(stubStore{
		getWorkoutByIDForUserFunc: func(context.Context, uuid.UUID, uuid.UUID) (*store.ApolloWorkout, error) {
			return &store.ApolloWorkout{ID: workoutID, UserID: userID, Status: StatusFinished}, nil
		},
	})
	if _, err := service.FinishWorkout(context.Background(), userID, workoutID); !errors.Is(err, ErrWorkoutFinished) {
		t.Fatalf("FinishWorkout(finished) error = %v, want %v", err, ErrWorkoutFinished)
	}

	inProgressWorkout := &store.ApolloWorkout{
		ID:        workoutID,
		UserID:    userID,
		Status:    StatusInProgress,
		StartedAt: timestamptz(time.Date(2026, 4, 2, 20, 0, 0, 0, time.UTC)),
	}
	service = NewService(stubStore{
		getWorkoutByIDForUserFunc: func(context.Context, uuid.UUID, uuid.UUID) (*store.ApolloWorkout, error) {
			return inProgressWorkout, nil
		},
		countExercisesByWorkoutIDFunc: func(context.Context, uuid.UUID) (int64, error) {
			return 0, nil
		},
	})
	if _, err := service.FinishWorkout(context.Background(), userID, workoutID); !errors.Is(err, ErrCannotFinishEmptyWorkout) {
		t.Fatalf("FinishWorkout(empty) error = %v, want %v", err, ErrCannotFinishEmptyWorkout)
	}

	var capturedFinishedAt time.Time
	service = NewService(stubStore{
		getWorkoutByIDForUserFunc: func(context.Context, uuid.UUID, uuid.UUID) (*store.ApolloWorkout, error) {
			return inProgressWorkout, nil
		},
		countExercisesByWorkoutIDFunc: func(context.Context, uuid.UUID) (int64, error) {
			return 2, nil
		},
		finishWorkoutFunc: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, finishedAt time.Time) (*store.ApolloWorkout, error) {
			capturedFinishedAt = finishedAt
			return &store.ApolloWorkout{
				ID:         workoutID,
				UserID:     userID,
				Status:     StatusFinished,
				StartedAt:  inProgressWorkout.StartedAt,
				FinishedAt: timestamptz(finishedAt),
			}, nil
		},
		listExercisesByWorkoutIDFunc: func(context.Context, uuid.UUID) ([]store.ApolloExercise, error) {
			return []store.ApolloExercise{
				{Name: "bench press", Sets: 3, Reps: 8, Position: 1},
				{Name: "row", Sets: 3, Reps: 10, Position: 2},
			}, nil
		},
	})
	service.now = func() time.Time { return now }

	workout, err := service.FinishWorkout(context.Background(), userID, workoutID)
	if err != nil {
		t.Fatalf("FinishWorkout() error = %v", err)
	}
	if !capturedFinishedAt.Equal(now) {
		t.Fatalf("capturedFinishedAt = %s, want %s", capturedFinishedAt, now)
	}
	if workout.FinishedAt == nil || !workout.FinishedAt.Equal(now) {
		t.Fatalf("workout.FinishedAt = %#v, want %s", workout.FinishedAt, now)
	}
	if len(workout.Exercises) != 2 {
		t.Fatalf("len(workout.Exercises) = %d, want 2", len(workout.Exercises))
	}
}

func TestListAndGetWorkoutsBuildStableResponses(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	firstWorkoutID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	secondWorkoutID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	notes := "upper"
	service := NewService(stubStore{
		listWorkoutsByUserIDFunc: func(context.Context, uuid.UUID) ([]store.ApolloWorkout, error) {
			return []store.ApolloWorkout{
				{ID: secondWorkoutID, UserID: userID, Status: StatusFinished, StartedAt: timestamptz(time.Date(2026, 4, 2, 18, 0, 0, 0, time.UTC)), FinishedAt: timestamptz(time.Date(2026, 4, 2, 19, 0, 0, 0, time.UTC)), Notes: &notes},
				{ID: firstWorkoutID, UserID: userID, Status: StatusInProgress, StartedAt: timestamptz(time.Date(2026, 4, 2, 20, 0, 0, 0, time.UTC))},
			}, nil
		},
		listExercisesByWorkoutIDFunc: func(_ context.Context, workoutID uuid.UUID) ([]store.ApolloExercise, error) {
			if workoutID == secondWorkoutID {
				return []store.ApolloExercise{{Name: "bench press", Sets: 3, Reps: 8, Position: 1}}, nil
			}
			return nil, nil
		},
		getWorkoutByIDForUserFunc: func(context.Context, uuid.UUID, uuid.UUID) (*store.ApolloWorkout, error) {
			return &store.ApolloWorkout{ID: secondWorkoutID, UserID: userID, Status: StatusFinished, StartedAt: timestamptz(time.Date(2026, 4, 2, 18, 0, 0, 0, time.UTC)), FinishedAt: timestamptz(time.Date(2026, 4, 2, 19, 0, 0, 0, time.UTC)), Notes: &notes}, nil
		},
	})

	workouts, err := service.ListWorkouts(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListWorkouts() error = %v", err)
	}
	if len(workouts) != 2 {
		t.Fatalf("len(workouts) = %d, want 2", len(workouts))
	}
	if workouts[0].ID != secondWorkoutID || workouts[1].ID != firstWorkoutID {
		t.Fatalf("workout order = [%s %s], want [%s %s]", workouts[0].ID, workouts[1].ID, secondWorkoutID, firstWorkoutID)
	}

	workout, err := service.GetWorkout(context.Background(), userID, secondWorkoutID)
	if err != nil {
		t.Fatalf("GetWorkout() error = %v", err)
	}
	if workout.ID != secondWorkoutID {
		t.Fatalf("workout.ID = %s, want %s", workout.ID, secondWorkoutID)
	}
	if workout.Notes == nil || *workout.Notes != notes {
		t.Fatalf("workout.Notes = %#v, want %q", workout.Notes, notes)
	}
}

func float64Ptr(value float64) *float64 {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
