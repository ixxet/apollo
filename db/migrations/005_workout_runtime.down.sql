DROP INDEX IF EXISTS apollo.idx_exercises_workout_position;

ALTER TABLE apollo.exercises
DROP CONSTRAINT IF EXISTS exercises_position_check;

ALTER TABLE apollo.exercises
DROP COLUMN IF EXISTS position;

DROP INDEX IF EXISTS apollo.idx_workouts_user_in_progress;
DROP INDEX IF EXISTS apollo.idx_workouts_user_started_at;

ALTER TABLE apollo.workouts
DROP CONSTRAINT IF EXISTS workouts_lifecycle_check,
DROP CONSTRAINT IF EXISTS workouts_status_check;

ALTER TABLE apollo.workouts
DROP COLUMN IF EXISTS finished_at,
DROP COLUMN IF EXISTS status;

ALTER TABLE apollo.workouts
RENAME COLUMN started_at TO logged_at;

CREATE INDEX idx_workouts_user_logged_at
    ON apollo.workouts (user_id, logged_at DESC);
