CREATE TABLE apollo.equipment_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    equipment_key TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    is_machine BOOLEAN NOT NULL DEFAULT FALSE,
    CONSTRAINT equipment_definitions_key_format
        CHECK (equipment_key ~ '^[a-z0-9][a-z0-9-]*$'),
    CONSTRAINT equipment_definitions_display_name_required
        CHECK (btrim(display_name) <> '')
);

CREATE TABLE apollo.exercise_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exercise_key TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    CONSTRAINT exercise_definitions_key_format
        CHECK (exercise_key ~ '^[a-z0-9][a-z0-9-]*$'),
    CONSTRAINT exercise_definitions_display_name_required
        CHECK (btrim(display_name) <> '')
);

CREATE TABLE apollo.exercise_definition_equipment (
    exercise_definition_id UUID NOT NULL REFERENCES apollo.exercise_definitions(id) ON DELETE CASCADE,
    equipment_definition_id UUID NOT NULL REFERENCES apollo.equipment_definitions(id) ON DELETE CASCADE,
    PRIMARY KEY (exercise_definition_id, equipment_definition_id)
);

CREATE TABLE apollo.workout_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workout_templates_name_required
        CHECK (btrim(name) <> '')
);

CREATE UNIQUE INDEX workout_templates_user_name_lower_idx
    ON apollo.workout_templates (user_id, lower(name));

CREATE TABLE apollo.workout_template_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workout_template_id UUID NOT NULL REFERENCES apollo.workout_templates(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    exercise_definition_id UUID NOT NULL REFERENCES apollo.exercise_definitions(id) ON DELETE RESTRICT,
    equipment_definition_id UUID REFERENCES apollo.equipment_definitions(id) ON DELETE RESTRICT,
    sets INTEGER NOT NULL,
    reps INTEGER NOT NULL,
    weight_kg NUMERIC(6,2),
    rpe NUMERIC(3,1),
    notes TEXT,
    CONSTRAINT workout_template_items_position_positive
        CHECK (position > 0),
    CONSTRAINT workout_template_items_sets_positive
        CHECK (sets > 0),
    CONSTRAINT workout_template_items_reps_positive
        CHECK (reps > 0),
    CONSTRAINT workout_template_items_weight_range
        CHECK (weight_kg IS NULL OR (weight_kg >= 0 AND weight_kg <= 9999.99)),
    CONSTRAINT workout_template_items_rpe_range
        CHECK (rpe IS NULL OR (rpe >= 0 AND rpe <= 10)),
    UNIQUE (workout_template_id, position)
);

CREATE TABLE apollo.planner_weeks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    week_start DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT planner_weeks_is_monday
        CHECK (EXTRACT(ISODOW FROM week_start) = 1),
    UNIQUE (user_id, week_start)
);

CREATE TABLE apollo.planner_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    planner_week_id UUID NOT NULL REFERENCES apollo.planner_weeks(id) ON DELETE CASCADE,
    day_index INTEGER NOT NULL,
    position INTEGER NOT NULL,
    workout_template_id UUID REFERENCES apollo.workout_templates(id) ON DELETE SET NULL,
    CONSTRAINT planner_sessions_day_index_range
        CHECK (day_index BETWEEN 0 AND 6),
    CONSTRAINT planner_sessions_position_positive
        CHECK (position > 0),
    UNIQUE (planner_week_id, day_index, position)
);

CREATE TABLE apollo.planner_session_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    planner_session_id UUID NOT NULL REFERENCES apollo.planner_sessions(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    exercise_definition_id UUID NOT NULL REFERENCES apollo.exercise_definitions(id) ON DELETE RESTRICT,
    equipment_definition_id UUID REFERENCES apollo.equipment_definitions(id) ON DELETE RESTRICT,
    sets INTEGER NOT NULL,
    reps INTEGER NOT NULL,
    weight_kg NUMERIC(6,2),
    rpe NUMERIC(3,1),
    notes TEXT,
    CONSTRAINT planner_session_items_position_positive
        CHECK (position > 0),
    CONSTRAINT planner_session_items_sets_positive
        CHECK (sets > 0),
    CONSTRAINT planner_session_items_reps_positive
        CHECK (reps > 0),
    CONSTRAINT planner_session_items_weight_range
        CHECK (weight_kg IS NULL OR (weight_kg >= 0 AND weight_kg <= 9999.99)),
    CONSTRAINT planner_session_items_rpe_range
        CHECK (rpe IS NULL OR (rpe >= 0 AND rpe <= 10)),
    UNIQUE (planner_session_id, position)
);

INSERT INTO apollo.equipment_definitions (equipment_key, display_name, is_machine)
VALUES
    ('barbell', 'Barbell', FALSE),
    ('dumbbell', 'Dumbbell', FALSE),
    ('cable-stack', 'Cable Stack', TRUE),
    ('rowing-machine', 'Rowing Machine', TRUE),
    ('leg-press', 'Leg Press', TRUE);

INSERT INTO apollo.exercise_definitions (exercise_key, display_name)
VALUES
    ('barbell-back-squat', 'Barbell Back Squat'),
    ('dumbbell-bench-press', 'Dumbbell Bench Press'),
    ('cable-seated-row', 'Cable Seated Row'),
    ('rowing-interval', 'Rowing Interval'),
    ('leg-press', 'Leg Press'),
    ('push-up', 'Push-Up');

INSERT INTO apollo.exercise_definition_equipment (exercise_definition_id, equipment_definition_id)
SELECT exercise.id, equipment.id
FROM apollo.exercise_definitions AS exercise
JOIN apollo.equipment_definitions AS equipment
  ON (
        (exercise.exercise_key = 'barbell-back-squat' AND equipment.equipment_key = 'barbell')
     OR (exercise.exercise_key = 'dumbbell-bench-press' AND equipment.equipment_key = 'dumbbell')
     OR (exercise.exercise_key = 'cable-seated-row' AND equipment.equipment_key = 'cable-stack')
     OR (exercise.exercise_key = 'rowing-interval' AND equipment.equipment_key = 'rowing-machine')
     OR (exercise.exercise_key = 'leg-press' AND equipment.equipment_key = 'leg-press')
     );
