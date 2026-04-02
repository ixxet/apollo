package workouts

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/store"
	"github.com/ixxet/apollo/internal/testutil"
)

func TestRepositoryCreateWorkoutIsOwnerScopedAndTracksInProgressState(t *testing.T) {
	ctx := context.Background()
	env := newWorkoutPostgresEnv(t, ctx)
	defer closeWorkoutPostgresEnv(t, env)

	repository := NewRepository(env.DB)
	queries := store.New(env.DB)
	firstUser := createWorkoutUser(t, ctx, queries, "student-workout-001", "workout-001@example.com")
	secondUser := createWorkoutUser(t, ctx, queries, "student-workout-002", "workout-002@example.com")
	notes := "push day"

	workout, err := repository.CreateWorkout(ctx, firstUser.ID, &notes)
	if err != nil {
		t.Fatalf("CreateWorkout() error = %v", err)
	}
	if workout.Status != "in_progress" {
		t.Fatalf("workout.Status = %q, want in_progress", workout.Status)
	}
	if workout.FinishedAt.Valid {
		t.Fatalf("workout.FinishedAt.Valid = %v, want false", workout.FinishedAt.Valid)
	}
	if workout.Notes == nil || *workout.Notes != notes {
		t.Fatalf("workout.Notes = %#v, want %q", workout.Notes, notes)
	}

	inProgress, err := repository.GetInProgressWorkoutByUserID(ctx, firstUser.ID)
	if err != nil {
		t.Fatalf("GetInProgressWorkoutByUserID(firstUser) error = %v", err)
	}
	if inProgress == nil || inProgress.ID != workout.ID {
		t.Fatalf("GetInProgressWorkoutByUserID(firstUser) = %#v, want workout %s", inProgress, workout.ID)
	}

	otherUserView, err := repository.GetWorkoutByIDForUser(ctx, workout.ID, secondUser.ID)
	if err != nil {
		t.Fatalf("GetWorkoutByIDForUser(secondUser) error = %v", err)
	}
	if otherUserView != nil {
		t.Fatalf("GetWorkoutByIDForUser(secondUser) = %#v, want nil", otherUserView)
	}

	firstUserWorkouts, err := repository.ListWorkoutsByUserID(ctx, firstUser.ID)
	if err != nil {
		t.Fatalf("ListWorkoutsByUserID(firstUser) error = %v", err)
	}
	if len(firstUserWorkouts) != 1 {
		t.Fatalf("len(firstUserWorkouts) = %d, want 1", len(firstUserWorkouts))
	}

	secondUserWorkouts, err := repository.ListWorkoutsByUserID(ctx, secondUser.ID)
	if err != nil {
		t.Fatalf("ListWorkoutsByUserID(secondUser) error = %v", err)
	}
	if len(secondUserWorkouts) != 0 {
		t.Fatalf("len(secondUserWorkouts) = %d, want 0", len(secondUserWorkouts))
	}
}

func TestRepositoryCreateWorkoutRejectsSecondInProgressWorkoutForSameUser(t *testing.T) {
	ctx := context.Background()
	env := newWorkoutPostgresEnv(t, ctx)
	defer closeWorkoutPostgresEnv(t, env)

	repository := NewRepository(env.DB)
	queries := store.New(env.DB)
	firstUser := createWorkoutUser(t, ctx, queries, "student-workout-003", "workout-003@example.com")
	secondUser := createWorkoutUser(t, ctx, queries, "student-workout-004", "workout-004@example.com")

	if _, err := repository.CreateWorkout(ctx, firstUser.ID, nil); err != nil {
		t.Fatalf("CreateWorkout(firstUser first) error = %v", err)
	}
	if _, err := repository.CreateWorkout(ctx, firstUser.ID, nil); !isUniqueViolation(err) {
		t.Fatalf("CreateWorkout(firstUser second) error = %v, want unique violation", err)
	}

	if _, err := repository.CreateWorkout(ctx, secondUser.ID, nil); err != nil {
		t.Fatalf("CreateWorkout(secondUser) error = %v", err)
	}
}

func TestRepositoryReplaceWorkoutDraftReplacesExercisesInStableOrder(t *testing.T) {
	ctx := context.Background()
	env := newWorkoutPostgresEnv(t, ctx)
	defer closeWorkoutPostgresEnv(t, env)

	repository := NewRepository(env.DB)
	queries := store.New(env.DB)
	user := createWorkoutUser(t, ctx, queries, "student-workout-005", "workout-005@example.com")

	workout, err := repository.CreateWorkout(ctx, user.ID, nil)
	if err != nil {
		t.Fatalf("CreateWorkout() error = %v", err)
	}

	firstNotes := "legs and shoulders"
	firstExercises := []ExerciseDraft{
		{
			Name:     "back squat",
			Sets:     3,
			Reps:     5,
			WeightKg: mustNumeric(t, 140.0),
			RPE:      mustNumeric(t, 8.0),
		},
		{
			Name: "dumbbell press",
			Sets: 4,
			Reps: 10,
		},
	}

	updatedWorkout, storedExercises, err := repository.ReplaceWorkoutDraft(ctx, workout.ID, user.ID, &firstNotes, firstExercises)
	if err != nil {
		t.Fatalf("ReplaceWorkoutDraft(first) error = %v", err)
	}
	if updatedWorkout == nil {
		t.Fatal("ReplaceWorkoutDraft(first) returned nil workout")
	}
	if len(storedExercises) != 2 {
		t.Fatalf("len(storedExercises) = %d, want 2", len(storedExercises))
	}
	if storedExercises[0].Position != 1 || storedExercises[1].Position != 2 {
		t.Fatalf("storedExercises positions = [%d %d], want [1 2]", storedExercises[0].Position, storedExercises[1].Position)
	}

	listedFirstPass, err := repository.ListExercisesByWorkoutID(ctx, workout.ID)
	if err != nil {
		t.Fatalf("ListExercisesByWorkoutID(first) error = %v", err)
	}
	if len(listedFirstPass) != 2 {
		t.Fatalf("len(listedFirstPass) = %d, want 2", len(listedFirstPass))
	}
	if listedFirstPass[0].Name != "back squat" || listedFirstPass[1].Name != "dumbbell press" {
		t.Fatalf("listedFirstPass names = [%q %q], want [back squat dumbbell press]", listedFirstPass[0].Name, listedFirstPass[1].Name)
	}

	secondNotes := "legs only"
	secondExercises := []ExerciseDraft{
		{
			Name:     "romanian deadlift",
			Sets:     3,
			Reps:     8,
			WeightKg: mustNumeric(t, 95.0),
		},
	}

	updatedAgain, replacedExercises, err := repository.ReplaceWorkoutDraft(ctx, workout.ID, user.ID, &secondNotes, secondExercises)
	if err != nil {
		t.Fatalf("ReplaceWorkoutDraft(second) error = %v", err)
	}
	if updatedAgain == nil || updatedAgain.Notes == nil || *updatedAgain.Notes != secondNotes {
		t.Fatalf("updatedAgain.Notes = %#v, want %q", updatedAgain.Notes, secondNotes)
	}
	if len(replacedExercises) != 1 {
		t.Fatalf("len(replacedExercises) = %d, want 1", len(replacedExercises))
	}
	if replacedExercises[0].Name != "romanian deadlift" || replacedExercises[0].Position != 1 {
		t.Fatalf("replacedExercises[0] = %#v, want romanian deadlift at position 1", replacedExercises[0])
	}

	listedSecondPass, err := repository.ListExercisesByWorkoutID(ctx, workout.ID)
	if err != nil {
		t.Fatalf("ListExercisesByWorkoutID(second) error = %v", err)
	}
	if len(listedSecondPass) != 1 {
		t.Fatalf("len(listedSecondPass) = %d, want 1", len(listedSecondPass))
	}
	if listedSecondPass[0].Name != "romanian deadlift" {
		t.Fatalf("listedSecondPass[0].Name = %q, want romanian deadlift", listedSecondPass[0].Name)
	}
}

func TestRepositoryFinishWorkoutTransitionsOnceAndLeavesFinishedRowsReadOnly(t *testing.T) {
	ctx := context.Background()
	env := newWorkoutPostgresEnv(t, ctx)
	defer closeWorkoutPostgresEnv(t, env)

	repository := NewRepository(env.DB)
	queries := store.New(env.DB)
	user := createWorkoutUser(t, ctx, queries, "student-workout-006", "workout-006@example.com")

	workout, err := repository.CreateWorkout(ctx, user.ID, nil)
	if err != nil {
		t.Fatalf("CreateWorkout() error = %v", err)
	}
	if _, _, err := repository.ReplaceWorkoutDraft(ctx, workout.ID, user.ID, nil, []ExerciseDraft{
		{Name: "rowing machine", Sets: 5, Reps: 500},
	}); err != nil {
		t.Fatalf("ReplaceWorkoutDraft() error = %v", err)
	}

	exerciseCount, err := repository.CountExercisesByWorkoutID(ctx, workout.ID)
	if err != nil {
		t.Fatalf("CountExercisesByWorkoutID() error = %v", err)
	}
	if exerciseCount != 1 {
		t.Fatalf("exerciseCount = %d, want 1", exerciseCount)
	}

	finishedAt := time.Date(2026, 4, 2, 19, 45, 0, 0, time.UTC)
	finishedWorkout, err := repository.FinishWorkout(ctx, workout.ID, user.ID, finishedAt)
	if err != nil {
		t.Fatalf("FinishWorkout(first) error = %v", err)
	}
	if finishedWorkout == nil {
		t.Fatal("FinishWorkout(first) returned nil workout")
	}
	if finishedWorkout.Status != "finished" {
		t.Fatalf("finishedWorkout.Status = %q, want finished", finishedWorkout.Status)
	}
	if !finishedWorkout.FinishedAt.Valid || !finishedWorkout.FinishedAt.Time.Equal(finishedAt) {
		t.Fatalf("finishedWorkout.FinishedAt = %#v, want %s", finishedWorkout.FinishedAt, finishedAt)
	}

	secondFinish, err := repository.FinishWorkout(ctx, workout.ID, user.ID, finishedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("FinishWorkout(second) error = %v", err)
	}
	if secondFinish != nil {
		t.Fatalf("FinishWorkout(second) = %#v, want nil", secondFinish)
	}

	updatedAfterFinish, exercisesAfterFinish, err := repository.ReplaceWorkoutDraft(ctx, workout.ID, user.ID, nil, []ExerciseDraft{
		{Name: "should not write", Sets: 1, Reps: 1},
	})
	if err != nil {
		t.Fatalf("ReplaceWorkoutDraft(after finish) error = %v", err)
	}
	if updatedAfterFinish != nil || exercisesAfterFinish != nil {
		t.Fatalf("ReplaceWorkoutDraft(after finish) = (%#v, %#v), want nils", updatedAfterFinish, exercisesAfterFinish)
	}
}

func newWorkoutPostgresEnv(t *testing.T, ctx context.Context) *testutil.PostgresEnv {
	t.Helper()

	env, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	if err := testutil.ApplyApolloSchema(ctx, env.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	return env
}

func closeWorkoutPostgresEnv(t *testing.T, env *testutil.PostgresEnv) {
	t.Helper()
	if err := env.Close(); err != nil {
		t.Fatalf("env.Close() error = %v", err)
	}
}

func createWorkoutUser(t *testing.T, ctx context.Context, queries *store.Queries, studentID string, email string) store.ApolloUser {
	t.Helper()

	user, err := queries.CreateUser(ctx, store.CreateUserParams{
		StudentID:   studentID,
		DisplayName: studentID,
		Email:       email,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	return user
}

func mustNumeric(t *testing.T, value float64) pgtype.Numeric {
	t.Helper()

	var numeric pgtype.Numeric
	if err := numeric.Scan(strconv.FormatFloat(value, 'f', -1, 64)); err != nil {
		t.Fatalf("numeric.Scan(%v) error = %v", value, err)
	}

	return numeric
}
