CREATE TABLE apollo.workout_effort_feedback (
    workout_id UUID PRIMARY KEY REFERENCES apollo.workouts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    effort_level TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workout_effort_feedback_level_check
        CHECK (effort_level IN ('easy', 'manageable', 'hard', 'maxed'))
);

CREATE INDEX workout_effort_feedback_user_created_idx
    ON apollo.workout_effort_feedback (user_id, created_at DESC);

CREATE TABLE apollo.workout_recovery_feedback (
    workout_id UUID PRIMARY KEY REFERENCES apollo.workouts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    recovery_level TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workout_recovery_feedback_level_check
        CHECK (recovery_level IN ('recovered', 'slightly_fatigued', 'fatigued', 'not_recovered'))
);

CREATE INDEX workout_recovery_feedback_user_created_idx
    ON apollo.workout_recovery_feedback (user_id, created_at DESC);
