package recommendations

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/store"
	"github.com/ixxet/apollo/internal/testutil"
	"github.com/ixxet/apollo/internal/workouts"
)

func TestRepositoryGetLatestFinishedWorkoutByUserIDReturnsMostRecentlyFinishedWorkout(t *testing.T) {
	ctx := context.Background()
	db, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("db.Close() error = %v", err)
		}
	}()
	if err := testutil.ApplyApolloSchema(ctx, db.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	userID := createRecommendationUser(t, ctx, db.DB, "student-recommendation-001", "recommendation-001@example.com")
	workoutRepository := workouts.NewRepository(db.DB)
	repository := NewRepository(db.DB)

	firstWorkout, err := workoutRepository.CreateWorkout(ctx, userID, stringPtr("first"))
	if err != nil {
		t.Fatalf("CreateWorkout(first) error = %v", err)
	}
	if _, _, err := workoutRepository.ReplaceWorkoutDraft(ctx, firstWorkout.ID, userID, stringPtr("first"), []workouts.ExerciseDraft{{Name: "squat", Sets: 3, Reps: 5}}); err != nil {
		t.Fatalf("ReplaceWorkoutDraft(first) error = %v", err)
	}
	firstFinishedAt := time.Date(2026, 4, 2, 15, 0, 0, 0, time.UTC)
	if _, err := workoutRepository.FinishWorkout(ctx, firstWorkout.ID, userID, firstFinishedAt); err != nil {
		t.Fatalf("FinishWorkout(first) error = %v", err)
	}

	secondWorkout, err := workoutRepository.CreateWorkout(ctx, userID, stringPtr("second"))
	if err != nil {
		t.Fatalf("CreateWorkout(second) error = %v", err)
	}
	if _, _, err := workoutRepository.ReplaceWorkoutDraft(ctx, secondWorkout.ID, userID, stringPtr("second"), []workouts.ExerciseDraft{{Name: "row", Sets: 4, Reps: 8}}); err != nil {
		t.Fatalf("ReplaceWorkoutDraft(second) error = %v", err)
	}
	secondFinishedAt := time.Date(2026, 4, 2, 18, 30, 0, 0, time.UTC)
	if _, err := workoutRepository.FinishWorkout(ctx, secondWorkout.ID, userID, secondFinishedAt); err != nil {
		t.Fatalf("FinishWorkout(second) error = %v", err)
	}

	workout, err := repository.GetLatestFinishedWorkoutByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetLatestFinishedWorkoutByUserID() error = %v", err)
	}
	if workout == nil {
		t.Fatal("GetLatestFinishedWorkoutByUserID() = nil, want workout")
	}
	if workout.ID != secondWorkout.ID {
		t.Fatalf("workout.ID = %s, want %s", workout.ID, secondWorkout.ID)
	}
	if workout.FinishedAt == nil || !workout.FinishedAt.Equal(secondFinishedAt) {
		t.Fatalf("workout.FinishedAt = %#v, want %s", workout.FinishedAt, secondFinishedAt)
	}
}

func createRecommendationUser(t *testing.T, ctx context.Context, db store.DBTX, studentID string, email string) uuid.UUID {
	t.Helper()

	queries := store.New(db)
	user, err := queries.CreateUser(ctx, store.CreateUserParams{
		StudentID:   studentID,
		DisplayName: studentID,
		Email:       email,
	})
	if err != nil {
		t.Fatalf("CreateUser(%q) error = %v", studentID, err)
	}

	return user.ID
}

func stringPtr(value string) *string {
	result := value
	return &result
}
