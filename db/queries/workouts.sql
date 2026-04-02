-- name: CreateWorkout :one
INSERT INTO apollo.workouts (
  user_id,
  notes,
  metadata
)
VALUES ($1, $2, $3)
RETURNING id, user_id, started_at, status, finished_at, notes, metadata;

-- name: GetWorkoutByIDForUser :one
SELECT w.*
FROM apollo.workouts AS w
WHERE w.id = $1
  AND w.user_id = $2
LIMIT 1;

-- name: GetInProgressWorkoutByUserID :one
SELECT w.*
FROM apollo.workouts AS w
WHERE w.user_id = $1
  AND w.status = 'in_progress'
LIMIT 1;

-- name: ListWorkoutsByUserID :many
SELECT w.*
FROM apollo.workouts AS w
WHERE w.user_id = $1
ORDER BY COALESCE(w.finished_at, w.started_at) DESC,
         w.started_at DESC,
         w.id DESC;

-- name: UpdateWorkoutDraft :one
UPDATE apollo.workouts
SET notes = $3,
    metadata = $4
WHERE id = $1
  AND user_id = $2
  AND status = 'in_progress'
RETURNING id, user_id, started_at, status, finished_at, notes, metadata;

-- name: FinishWorkout :one
UPDATE apollo.workouts
SET status = 'finished',
    finished_at = $3
WHERE id = $1
  AND user_id = $2
  AND status = 'in_progress'
RETURNING id, user_id, started_at, status, finished_at, notes, metadata;

-- name: CountExercisesByWorkoutID :one
SELECT count(*)
FROM apollo.exercises
WHERE workout_id = $1;

-- name: DeleteExercisesByWorkoutID :exec
DELETE FROM apollo.exercises
WHERE workout_id = $1;

-- name: CreateExercise :one
INSERT INTO apollo.exercises (
  workout_id,
  position,
  name,
  sets,
  reps,
  weight_kg,
  rpe,
  notes
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, workout_id, position, name, sets, reps, weight_kg, rpe, notes;

-- name: ListExercisesByWorkoutID :many
SELECT e.*
FROM apollo.exercises AS e
WHERE e.workout_id = $1
ORDER BY e.position ASC, e.id ASC;
