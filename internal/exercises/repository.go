package exercises

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListEquipment(ctx context.Context) ([]EquipmentRef, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, equipment_key, display_name, is_machine
FROM apollo.equipment_definitions
ORDER BY equipment_key
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectEquipmentRows(rows)
}

func (r *Repository) ResolveEquipment(ctx context.Context, keys []string) (map[string]EquipmentRef, error) {
	if len(keys) == 0 {
		return map[string]EquipmentRef{}, nil
	}

	rows, err := r.db.Query(ctx, `
SELECT id, equipment_key, display_name, is_machine
FROM apollo.equipment_definitions
WHERE equipment_key = ANY($1::text[])
ORDER BY equipment_key
`, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := collectEquipmentRows(rows)
	if err != nil {
		return nil, err
	}

	result := make(map[string]EquipmentRef, len(items))
	for _, item := range items {
		result[item.EquipmentKey] = item
	}

	return result, nil
}

func (r *Repository) ListExercises(ctx context.Context) ([]ExerciseRef, error) {
	rows, err := r.db.Query(ctx, exerciseSelectQuery(""))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectExerciseRows(rows)
}

func (r *Repository) ResolveExercises(ctx context.Context, keys []string) (map[string]ExerciseRef, error) {
	if len(keys) == 0 {
		return map[string]ExerciseRef{}, nil
	}

	rows, err := r.db.Query(ctx, exerciseSelectQuery("WHERE exercise.exercise_key = ANY($1::text[])"), keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := collectExerciseRows(rows)
	if err != nil {
		return nil, err
	}

	result := make(map[string]ExerciseRef, len(items))
	for _, item := range items {
		result[item.ExerciseKey] = item
	}

	return result, nil
}

func collectEquipmentRows(rows pgxRows) ([]EquipmentRef, error) {
	items := make([]EquipmentRef, 0)
	for rows.Next() {
		var item EquipmentRef
		if err := rows.Scan(&item.ID, &item.EquipmentKey, &item.DisplayName, &item.IsMachine); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func collectExerciseRows(rows pgxRows) ([]ExerciseRef, error) {
	type key struct {
		id uuid.UUID
	}

	byID := make(map[uuid.UUID]*ExerciseRef)
	order := make([]uuid.UUID, 0)

	for rows.Next() {
		var (
			exerciseID   uuid.UUID
			exerciseKey  string
			displayName  string
			equipmentKey *string
		)

		if err := rows.Scan(&exerciseID, &exerciseKey, &displayName, &equipmentKey); err != nil {
			return nil, err
		}

		entry, ok := byID[exerciseID]
		if !ok {
			entry = &ExerciseRef{
				ID:                   exerciseID,
				ExerciseKey:          exerciseKey,
				DisplayName:          displayName,
				AllowedEquipmentKeys: []string{},
				allowedEquipmentSet:  make(map[string]struct{}),
			}
			byID[exerciseID] = entry
			order = append(order, exerciseID)
		}
		if equipmentKey != nil {
			if _, seen := entry.allowedEquipmentSet[*equipmentKey]; !seen {
				entry.allowedEquipmentSet[*equipmentKey] = struct{}{}
				entry.AllowedEquipmentKeys = append(entry.AllowedEquipmentKeys, *equipmentKey)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]ExerciseRef, 0, len(order))
	for _, exerciseID := range order {
		result = append(result, *byID[exerciseID])
	}

	return result, nil
}

func exerciseSelectQuery(where string) string {
	return `
SELECT exercise.id,
       exercise.exercise_key,
       exercise.display_name,
       equipment.equipment_key
FROM apollo.exercise_definitions AS exercise
LEFT JOIN apollo.exercise_definition_equipment AS allowed
  ON allowed.exercise_definition_id = exercise.id
LEFT JOIN apollo.equipment_definitions AS equipment
  ON equipment.id = allowed.equipment_definition_id
` + where + `
ORDER BY exercise.exercise_key, equipment.equipment_key
`
}

type pgxRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}
