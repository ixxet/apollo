package planner

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListTemplates(ctx context.Context, userID uuid.UUID) ([]Template, error) {
	return r.listTemplates(ctx, r.db, userID, nil)
}

func (r *Repository) GetTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID) (*Template, error) {
	templates, err := r.listTemplates(ctx, r.db, userID, &templateID)
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return nil, nil
	}
	return &templates[0], nil
}

func (r *Repository) CreateTemplate(ctx context.Context, userID uuid.UUID, name string, items []persistedItemInput) (Template, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Template{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var templateID uuid.UUID
	if err := tx.QueryRow(ctx, `
INSERT INTO apollo.workout_templates (user_id, name)
VALUES ($1, $2)
RETURNING id
`, userID, name).Scan(&templateID); err != nil {
		if isUniqueViolation(err) {
			return Template{}, ErrDuplicateTemplateName
		}
		return Template{}, err
	}

	if err := insertTemplateItems(ctx, tx, templateID, items); err != nil {
		return Template{}, err
	}

	template, err := r.getTemplateWithQuerier(ctx, tx, userID, templateID)
	if err != nil {
		return Template{}, err
	}
	if template == nil {
		return Template{}, ErrTemplateNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return Template{}, err
	}

	return *template, nil
}

func (r *Repository) UpdateTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID, name string, items []persistedItemInput) (*Template, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var updatedTemplateID uuid.UUID
	if err := tx.QueryRow(ctx, `
UPDATE apollo.workout_templates
SET name = $3,
    updated_at = NOW()
WHERE id = $1
  AND user_id = $2
RETURNING id
`, templateID, userID, name).Scan(&updatedTemplateID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if isUniqueViolation(err) {
			return nil, ErrDuplicateTemplateName
		}
		return nil, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM apollo.workout_template_items WHERE workout_template_id = $1`, updatedTemplateID); err != nil {
		return nil, err
	}
	if err := insertTemplateItems(ctx, tx, updatedTemplateID, items); err != nil {
		return nil, err
	}

	template, err := r.getTemplateWithQuerier(ctx, tx, userID, updatedTemplateID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, nil
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return template, nil
}

func (r *Repository) GetWeek(ctx context.Context, userID uuid.UUID, weekStart string) (*Week, error) {
	return r.getWeekWithQuerier(ctx, r.db, userID, weekStart)
}

func (r *Repository) UpsertWeek(ctx context.Context, userID uuid.UUID, weekStart string, sessions []persistedSessionInput) (Week, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Week{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var weekID uuid.UUID
	if err := tx.QueryRow(ctx, `
INSERT INTO apollo.planner_weeks (user_id, week_start, updated_at)
VALUES ($1, $2::date, NOW())
ON CONFLICT (user_id, week_start)
DO UPDATE SET updated_at = NOW()
RETURNING id
`, userID, weekStart).Scan(&weekID); err != nil {
		return Week{}, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM apollo.planner_sessions WHERE planner_week_id = $1`, weekID); err != nil {
		return Week{}, err
	}

	for _, session := range sessions {
		var sessionID uuid.UUID
		if err := tx.QueryRow(ctx, `
INSERT INTO apollo.planner_sessions (planner_week_id, day_index, position, workout_template_id)
VALUES ($1, $2, $3, $4)
RETURNING id
`, weekID, session.DayIndex, session.Position, session.TemplateID).Scan(&sessionID); err != nil {
			return Week{}, err
		}

		for _, item := range session.Items {
			if _, err := tx.Exec(ctx, `
INSERT INTO apollo.planner_session_items (
    planner_session_id,
    position,
    exercise_definition_id,
    equipment_definition_id,
    sets,
    reps,
    weight_kg,
    rpe,
    notes
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
`, sessionID, item.Position, item.ExerciseDefinitionID, item.EquipmentDefinitionID, item.Sets, item.Reps, item.WeightKg, item.RPE, item.Notes); err != nil {
				return Week{}, err
			}
		}
	}

	week, err := r.getWeekWithQuerier(ctx, tx, userID, weekStart)
	if err != nil {
		return Week{}, err
	}
	if week == nil {
		return Week{}, errors.New("planner week missing after upsert")
	}

	if err := tx.Commit(ctx); err != nil {
		return Week{}, err
	}

	return *week, nil
}

func (r *Repository) listTemplates(ctx context.Context, db queryer, userID uuid.UUID, templateID *uuid.UUID) ([]Template, error) {
	query := `
SELECT template.id,
       template.name,
       item.position,
       exercise.exercise_key,
       equipment.equipment_key,
       item.sets,
       item.reps,
       item.weight_kg,
       item.rpe,
       item.notes
FROM apollo.workout_templates AS template
LEFT JOIN apollo.workout_template_items AS item
  ON item.workout_template_id = template.id
LEFT JOIN apollo.exercise_definitions AS exercise
  ON exercise.id = item.exercise_definition_id
LEFT JOIN apollo.equipment_definitions AS equipment
  ON equipment.id = item.equipment_definition_id
WHERE template.user_id = $1
`
	args := []any{userID}
	if templateID != nil {
		query += "  AND template.id = $2\n"
		args = append(args, *templateID)
	}
	query += "ORDER BY lower(template.name), template.id, item.position"

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make([]Template, 0)
	indexByID := make(map[uuid.UUID]int)
	for rows.Next() {
		var (
			rowTemplateID uuid.UUID
			name          string
			itemPosition  *int32
			exerciseKey   *string
			equipmentKey  *string
			sets          *int32
			reps          *int32
			weightKg      pgtype.Numeric
			rpe           pgtype.Numeric
			notes         *string
		)

		if err := rows.Scan(&rowTemplateID, &name, &itemPosition, &exerciseKey, &equipmentKey, &sets, &reps, &weightKg, &rpe, &notes); err != nil {
			return nil, err
		}

		templateIndex, ok := indexByID[rowTemplateID]
		if !ok {
			indexByID[rowTemplateID] = len(templates)
			templates = append(templates, Template{
				ID:    rowTemplateID,
				Name:  name,
				Items: []TemplateItem{},
			})
			templateIndex = len(templates) - 1
		}

		if itemPosition != nil && exerciseKey != nil && sets != nil && reps != nil {
			weight, err := optionalFloat64(weightKg)
			if err != nil {
				return nil, err
			}
			rpeValue, err := optionalFloat64(rpe)
			if err != nil {
				return nil, err
			}
			templates[templateIndex].Items = append(templates[templateIndex].Items, TemplateItem{
				Position:     int(*itemPosition),
				ExerciseKey:  *exerciseKey,
				EquipmentKey: equipmentKey,
				Sets:         int(*sets),
				Reps:         int(*reps),
				WeightKg:     weight,
				RPE:          rpeValue,
				Notes:        notes,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	sortTemplates(templates)
	return templates, nil
}

func (r *Repository) getTemplateWithQuerier(ctx context.Context, db queryer, userID uuid.UUID, templateID uuid.UUID) (*Template, error) {
	templates, err := r.listTemplates(ctx, db, userID, &templateID)
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return nil, nil
	}

	template := cloneTemplate(templates[0])
	return &template, nil
}

func insertTemplateItems(ctx context.Context, db queryer, templateID uuid.UUID, items []persistedItemInput) error {
	for _, item := range items {
		if _, err := db.Exec(ctx, `
INSERT INTO apollo.workout_template_items (
    workout_template_id,
    position,
    exercise_definition_id,
    equipment_definition_id,
    sets,
    reps,
    weight_kg,
    rpe,
    notes
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
`, templateID, item.Position, item.ExerciseDefinitionID, item.EquipmentDefinitionID, item.Sets, item.Reps, item.WeightKg, item.RPE, item.Notes); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) getWeekWithQuerier(ctx context.Context, db queryer, userID uuid.UUID, weekStart string) (*Week, error) {
	rows, err := db.Query(ctx, `
SELECT week.week_start::text,
       session.day_index,
       session.position,
       session.workout_template_id,
       item.position,
       exercise.exercise_key,
       equipment.equipment_key,
       item.sets,
       item.reps,
       item.weight_kg,
       item.rpe,
       item.notes
FROM apollo.planner_weeks AS week
LEFT JOIN apollo.planner_sessions AS session
  ON session.planner_week_id = week.id
LEFT JOIN apollo.planner_session_items AS item
  ON item.planner_session_id = session.id
LEFT JOIN apollo.exercise_definitions AS exercise
  ON exercise.id = item.exercise_definition_id
LEFT JOIN apollo.equipment_definitions AS equipment
  ON equipment.id = item.equipment_definition_id
WHERE week.user_id = $1
  AND week.week_start = $2::date
ORDER BY session.day_index, session.position, item.position
`, userID, weekStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		week        *Week
		lastSession *WeekSession
	)
	for rows.Next() {
		var (
			rowWeekStart string
			dayIndex     *int32
			sessionPos   *int32
			templateID   *uuid.UUID
			itemPosition *int32
			exerciseKey  *string
			equipmentKey *string
			sets         *int32
			reps         *int32
			weightKg     pgtype.Numeric
			rpe          pgtype.Numeric
			notes        *string
		)

		if err := rows.Scan(&rowWeekStart, &dayIndex, &sessionPos, &templateID, &itemPosition, &exerciseKey, &equipmentKey, &sets, &reps, &weightKg, &rpe, &notes); err != nil {
			return nil, err
		}
		if week == nil {
			week = &Week{
				WeekStart: rowWeekStart,
				Sessions:  []WeekSession{},
			}
		}

		if dayIndex == nil || sessionPos == nil {
			continue
		}
		if lastSession == nil || lastSession.DayIndex != int(*dayIndex) || lastSession.Position != int(*sessionPos) {
			week.Sessions = append(week.Sessions, WeekSession{
				DayIndex:   int(*dayIndex),
				Position:   int(*sessionPos),
				TemplateID: templateID,
				Items:      []WeekSessionItem{},
			})
			lastSession = &week.Sessions[len(week.Sessions)-1]
		}
		if itemPosition == nil || exerciseKey == nil || sets == nil || reps == nil {
			continue
		}

		weight, err := optionalFloat64(weightKg)
		if err != nil {
			return nil, err
		}
		rpeValue, err := optionalFloat64(rpe)
		if err != nil {
			return nil, err
		}
		lastSession.Items = append(lastSession.Items, WeekSessionItem{
			Position:     int(*itemPosition),
			ExerciseKey:  *exerciseKey,
			EquipmentKey: equipmentKey,
			Sets:         int(*sets),
			Reps:         int(*reps),
			WeightKg:     weight,
			RPE:          rpeValue,
			Notes:        notes,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	if week == nil {
		return nil, nil
	}

	cloned := cloneWeek(*week)
	return &cloned, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}
