-- name: UpsertEffortFeedbackForFinishedWorkout :one
WITH owned_finished_workout AS (
  SELECT w.id, w.user_id
  FROM apollo.workouts AS w
  WHERE w.id = $1
    AND w.user_id = $2
    AND w.status = 'finished'
)
INSERT INTO apollo.workout_effort_feedback (
  workout_id,
  user_id,
  effort_level
)
SELECT id, user_id, $3
FROM owned_finished_workout
ON CONFLICT (workout_id)
DO UPDATE
SET effort_level = EXCLUDED.effort_level,
    user_id = EXCLUDED.user_id,
    updated_at = NOW()
RETURNING workout_id, user_id, effort_level, created_at, updated_at;

-- name: UpsertRecoveryFeedbackForFinishedWorkout :one
WITH owned_finished_workout AS (
  SELECT w.id, w.user_id
  FROM apollo.workouts AS w
  WHERE w.id = $1
    AND w.user_id = $2
    AND w.status = 'finished'
)
INSERT INTO apollo.workout_recovery_feedback (
  workout_id,
  user_id,
  recovery_level
)
SELECT id, user_id, $3
FROM owned_finished_workout
ON CONFLICT (workout_id)
DO UPDATE
SET recovery_level = EXCLUDED.recovery_level,
    user_id = EXCLUDED.user_id,
    updated_at = NOW()
RETURNING workout_id, user_id, recovery_level, created_at, updated_at;

-- name: GetEffortFeedbackByWorkoutID :one
SELECT workout_id, user_id, effort_level, created_at, updated_at
FROM apollo.workout_effort_feedback
WHERE workout_id = $1
LIMIT 1;

-- name: GetRecoveryFeedbackByWorkoutID :one
SELECT workout_id, user_id, recovery_level, created_at, updated_at
FROM apollo.workout_recovery_feedback
WHERE workout_id = $1
LIMIT 1;
