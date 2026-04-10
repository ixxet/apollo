-- name: CreateMealLog :one
INSERT INTO apollo.nutrition_meal_logs (
  user_id,
  source_template_id,
  name,
  meal_type,
  logged_at,
  calories,
  protein_grams,
  carbs_grams,
  fat_grams,
  notes
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, user_id, source_template_id, name, meal_type, logged_at, calories, protein_grams, carbs_grams, fat_grams, notes, created_at, updated_at;

-- name: CreateMealTemplate :one
INSERT INTO apollo.nutrition_meal_templates (
  user_id,
  name,
  meal_type,
  calories,
  protein_grams,
  carbs_grams,
  fat_grams,
  notes
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, user_id, name, meal_type, calories, protein_grams, carbs_grams, fat_grams, notes, created_at, updated_at;

-- name: GetMealLogByIDForUser :one
SELECT id, user_id, source_template_id, name, meal_type, logged_at, calories, protein_grams, carbs_grams, fat_grams, notes, created_at, updated_at
FROM apollo.nutrition_meal_logs
WHERE id = $1
  AND user_id = $2
LIMIT 1;

-- name: GetMealTemplateByIDForUser :one
SELECT id, user_id, name, meal_type, calories, protein_grams, carbs_grams, fat_grams, notes, created_at, updated_at
FROM apollo.nutrition_meal_templates
WHERE id = $1
  AND user_id = $2
LIMIT 1;

-- name: ListMealLogsByUserID :many
SELECT id, user_id, source_template_id, name, meal_type, logged_at, calories, protein_grams, carbs_grams, fat_grams, notes, created_at, updated_at
FROM apollo.nutrition_meal_logs
WHERE user_id = $1
ORDER BY logged_at DESC, id DESC;

-- name: ListMealLogsByUserIDWithinRange :many
SELECT id, user_id, source_template_id, name, meal_type, logged_at, calories, protein_grams, carbs_grams, fat_grams, notes, created_at, updated_at
FROM apollo.nutrition_meal_logs
WHERE user_id = $1
  AND logged_at >= $2
  AND logged_at < $3
ORDER BY logged_at DESC, id DESC;

-- name: ListMealTemplatesByUserID :many
SELECT id, user_id, name, meal_type, calories, protein_grams, carbs_grams, fat_grams, notes, created_at, updated_at
FROM apollo.nutrition_meal_templates
WHERE user_id = $1
ORDER BY lower(name), id;

-- name: UpdateMealLogForUser :one
UPDATE apollo.nutrition_meal_logs
SET source_template_id = $3,
    name = $4,
    meal_type = $5,
    logged_at = $6,
    calories = $7,
    protein_grams = $8,
    carbs_grams = $9,
    fat_grams = $10,
    notes = $11,
    updated_at = NOW()
WHERE id = $1
  AND user_id = $2
RETURNING id, user_id, source_template_id, name, meal_type, logged_at, calories, protein_grams, carbs_grams, fat_grams, notes, created_at, updated_at;

-- name: UpdateMealTemplateForUser :one
UPDATE apollo.nutrition_meal_templates
SET name = $3,
    meal_type = $4,
    calories = $5,
    protein_grams = $6,
    carbs_grams = $7,
    fat_grams = $8,
    notes = $9,
    updated_at = NOW()
WHERE id = $1
  AND user_id = $2
RETURNING id, user_id, name, meal_type, calories, protein_grams, carbs_grams, fat_grams, notes, created_at, updated_at;
