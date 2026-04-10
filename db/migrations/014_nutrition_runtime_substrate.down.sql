DROP INDEX IF EXISTS apollo.nutrition_meal_logs_user_logged_at_idx;
DROP TABLE IF EXISTS apollo.nutrition_meal_logs;

DROP INDEX IF EXISTS apollo.nutrition_meal_templates_user_created_idx;
DROP INDEX IF EXISTS apollo.nutrition_meal_templates_user_name_lower_idx;
DROP TABLE IF EXISTS apollo.nutrition_meal_templates;

UPDATE apollo.users
SET preferences = preferences - 'nutrition_profile'
WHERE preferences ? 'nutrition_profile';

ALTER TABLE apollo.users
    ALTER COLUMN preferences SET DEFAULT '{
        "visibility_mode": "ghost",
        "availability_mode": "unavailable",
        "coaching_profile": {}
    }'::jsonb;
