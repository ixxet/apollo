ALTER TABLE apollo.users
    ALTER COLUMN preferences SET DEFAULT '{
        "visibility_mode": "ghost",
        "availability_mode": "unavailable",
        "coaching_profile": {},
        "nutrition_profile": {}
    }'::jsonb;

UPDATE apollo.users
SET preferences = jsonb_set(
    preferences,
    '{nutrition_profile}',
    COALESCE(preferences->'nutrition_profile', '{}'::jsonb),
    TRUE
)
WHERE NOT (preferences ? 'nutrition_profile');

CREATE TABLE apollo.nutrition_meal_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    meal_type TEXT NOT NULL,
    calories INTEGER,
    protein_grams INTEGER,
    carbs_grams INTEGER,
    fat_grams INTEGER,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT nutrition_meal_templates_name_required
        CHECK (btrim(name) <> ''),
    CONSTRAINT nutrition_meal_templates_type_check
        CHECK (meal_type IN ('breakfast', 'lunch', 'dinner', 'snack', 'other')),
    CONSTRAINT nutrition_meal_templates_numeric_required
        CHECK (
            calories IS NOT NULL
            OR protein_grams IS NOT NULL
            OR carbs_grams IS NOT NULL
            OR fat_grams IS NOT NULL
        ),
    CONSTRAINT nutrition_meal_templates_calories_range
        CHECK (calories IS NULL OR calories BETWEEN 1 AND 6000),
    CONSTRAINT nutrition_meal_templates_protein_range
        CHECK (protein_grams IS NULL OR protein_grams BETWEEN 1 AND 500),
    CONSTRAINT nutrition_meal_templates_carbs_range
        CHECK (carbs_grams IS NULL OR carbs_grams BETWEEN 1 AND 500),
    CONSTRAINT nutrition_meal_templates_fat_range
        CHECK (fat_grams IS NULL OR fat_grams BETWEEN 1 AND 500)
);

CREATE UNIQUE INDEX nutrition_meal_templates_user_name_lower_idx
    ON apollo.nutrition_meal_templates (user_id, lower(name));

CREATE INDEX nutrition_meal_templates_user_created_idx
    ON apollo.nutrition_meal_templates (user_id, created_at DESC, id DESC);

CREATE TABLE apollo.nutrition_meal_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    source_template_id UUID REFERENCES apollo.nutrition_meal_templates(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    meal_type TEXT NOT NULL,
    logged_at TIMESTAMPTZ NOT NULL,
    calories INTEGER,
    protein_grams INTEGER,
    carbs_grams INTEGER,
    fat_grams INTEGER,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT nutrition_meal_logs_name_required
        CHECK (btrim(name) <> ''),
    CONSTRAINT nutrition_meal_logs_type_check
        CHECK (meal_type IN ('breakfast', 'lunch', 'dinner', 'snack', 'other')),
    CONSTRAINT nutrition_meal_logs_numeric_or_template_required
        CHECK (
            source_template_id IS NOT NULL
            OR calories IS NOT NULL
            OR protein_grams IS NOT NULL
            OR carbs_grams IS NOT NULL
            OR fat_grams IS NOT NULL
        ),
    CONSTRAINT nutrition_meal_logs_calories_range
        CHECK (calories IS NULL OR calories BETWEEN 1 AND 6000),
    CONSTRAINT nutrition_meal_logs_protein_range
        CHECK (protein_grams IS NULL OR protein_grams BETWEEN 1 AND 500),
    CONSTRAINT nutrition_meal_logs_carbs_range
        CHECK (carbs_grams IS NULL OR carbs_grams BETWEEN 1 AND 500),
    CONSTRAINT nutrition_meal_logs_fat_range
        CHECK (fat_grams IS NULL OR fat_grams BETWEEN 1 AND 500)
);

CREATE INDEX nutrition_meal_logs_user_logged_at_idx
    ON apollo.nutrition_meal_logs (user_id, logged_at DESC, id DESC);
