ALTER TABLE apollo.workouts
RENAME COLUMN logged_at TO started_at;

ALTER TABLE apollo.workouts
ADD COLUMN status TEXT,
ADD COLUMN finished_at TIMESTAMPTZ;

UPDATE apollo.workouts
SET status = 'finished',
    finished_at = started_at
WHERE status IS NULL;

ALTER TABLE apollo.workouts
ALTER COLUMN status SET DEFAULT 'in_progress',
ALTER COLUMN status SET NOT NULL;

ALTER TABLE apollo.workouts
ADD CONSTRAINT workouts_status_check
CHECK (status IN ('in_progress', 'finished')),
ADD CONSTRAINT workouts_lifecycle_check
CHECK (
    (status = 'in_progress' AND finished_at IS NULL)
    OR (status = 'finished' AND finished_at IS NOT NULL)
);

DROP INDEX IF EXISTS apollo.idx_workouts_user_logged_at;

CREATE INDEX idx_workouts_user_started_at
    ON apollo.workouts (user_id, started_at DESC);

CREATE UNIQUE INDEX idx_workouts_user_in_progress
    ON apollo.workouts (user_id)
    WHERE status = 'in_progress';

ALTER TABLE apollo.exercises
ADD COLUMN position INTEGER;

WITH ranked_exercises AS (
    SELECT id,
           row_number() OVER (PARTITION BY workout_id ORDER BY id) AS position
    FROM apollo.exercises
)
UPDATE apollo.exercises AS exercises
SET position = ranked_exercises.position
FROM ranked_exercises
WHERE exercises.id = ranked_exercises.id;

ALTER TABLE apollo.exercises
ALTER COLUMN position SET NOT NULL;

ALTER TABLE apollo.exercises
ADD CONSTRAINT exercises_position_check
CHECK (position > 0);

CREATE UNIQUE INDEX idx_exercises_workout_position
    ON apollo.exercises (workout_id, position);
