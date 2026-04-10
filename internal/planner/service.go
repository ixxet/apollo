package planner

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/exercises"
)

const weekStartLayout = "2006-01-02"

var (
	ErrTemplateNotFound       = errors.New("template not found")
	ErrTemplateNameRequired   = errors.New("template name is required")
	ErrTemplateItemsRequired  = errors.New("template items are required")
	ErrDuplicateTemplateName  = errors.New("template name already exists")
	ErrExerciseKeyRequired    = errors.New("exercise_key is required")
	ErrExerciseNotFound       = errors.New("exercise_key is invalid")
	ErrEquipmentNotFound      = errors.New("equipment_key is invalid")
	ErrEquipmentNotAllowed    = errors.New("equipment_key is not allowed for exercise")
	ErrSetsInvalid            = errors.New("sets must be positive")
	ErrRepsInvalid            = errors.New("reps must be positive")
	ErrWeightInvalid          = errors.New("weight_kg must be between 0 and 9999.99")
	ErrRPEInvalid             = errors.New("rpe must be between 0 and 10")
	ErrWeekStartInvalid       = errors.New("week_start must be an ISO Monday date")
	ErrSessionDayIndexInvalid = errors.New("day_index must be between 0 and 6")
	ErrSessionItemsRequired   = errors.New("session items are required")
	ErrSessionShapeInvalid    = errors.New("session must specify either template_id or items")
)

var keyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type Template struct {
	ID    uuid.UUID      `json:"id"`
	Name  string         `json:"name"`
	Items []TemplateItem `json:"items"`
}

type TemplateItem struct {
	Position     int      `json:"position"`
	ExerciseKey  string   `json:"exercise_key"`
	EquipmentKey *string  `json:"equipment_key,omitempty"`
	Sets         int      `json:"sets"`
	Reps         int      `json:"reps"`
	WeightKg     *float64 `json:"weight_kg,omitempty"`
	RPE          *float64 `json:"rpe,omitempty"`
	Notes        *string  `json:"notes,omitempty"`
}

type ItemInput struct {
	ExerciseKey  string   `json:"exercise_key"`
	EquipmentKey *string  `json:"equipment_key"`
	Sets         int      `json:"sets"`
	Reps         int      `json:"reps"`
	WeightKg     *float64 `json:"weight_kg"`
	RPE          *float64 `json:"rpe"`
	Notes        *string  `json:"notes"`
}

type TemplateInput struct {
	Name  string      `json:"name"`
	Items []ItemInput `json:"items"`
}

type Week struct {
	WeekStart string        `json:"week_start"`
	Sessions  []WeekSession `json:"sessions"`
}

type WeekSession struct {
	DayIndex   int               `json:"day_index"`
	Position   int               `json:"position"`
	TemplateID *uuid.UUID        `json:"template_id,omitempty"`
	Items      []WeekSessionItem `json:"items"`
}

type WeekSessionItem struct {
	Position     int      `json:"position"`
	ExerciseKey  string   `json:"exercise_key"`
	EquipmentKey *string  `json:"equipment_key,omitempty"`
	Sets         int      `json:"sets"`
	Reps         int      `json:"reps"`
	WeightKg     *float64 `json:"weight_kg,omitempty"`
	RPE          *float64 `json:"rpe,omitempty"`
	Notes        *string  `json:"notes,omitempty"`
}

type WeekInput struct {
	Sessions []WeekSessionInput `json:"sessions"`
}

type WeekSessionInput struct {
	DayIndex   int          `json:"day_index"`
	TemplateID *uuid.UUID   `json:"template_id"`
	Items      *[]ItemInput `json:"items"`
}

type RepositoryStore interface {
	ListTemplates(ctx context.Context, userID uuid.UUID) ([]Template, error)
	GetTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID) (*Template, error)
	CreateTemplate(ctx context.Context, userID uuid.UUID, name string, items []persistedItemInput) (Template, error)
	UpdateTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID, name string, items []persistedItemInput) (*Template, error)
	GetWeek(ctx context.Context, userID uuid.UUID, weekStart string) (*Week, error)
	UpsertWeek(ctx context.Context, userID uuid.UUID, weekStart string, sessions []persistedSessionInput) (Week, error)
}

type Service struct {
	repository RepositoryStore
	catalog    *exercises.Service
}

func NewService(repository RepositoryStore, catalog *exercises.Service) *Service {
	return &Service{
		repository: repository,
		catalog:    catalog,
	}
}

func (s *Service) ListTemplates(ctx context.Context, userID uuid.UUID) ([]Template, error) {
	return s.repository.ListTemplates(ctx, userID)
}

func (s *Service) GetTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID) (Template, error) {
	template, err := s.repository.GetTemplate(ctx, userID, templateID)
	if err != nil {
		return Template{}, err
	}
	if template == nil {
		return Template{}, ErrTemplateNotFound
	}

	return *template, nil
}

func (s *Service) CreateTemplate(ctx context.Context, userID uuid.UUID, input TemplateInput) (Template, error) {
	name, items, err := s.validateTemplateInput(ctx, input)
	if err != nil {
		return Template{}, err
	}

	return s.repository.CreateTemplate(ctx, userID, name, items)
}

func (s *Service) UpdateTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID, input TemplateInput) (Template, error) {
	name, items, err := s.validateTemplateInput(ctx, input)
	if err != nil {
		return Template{}, err
	}

	template, err := s.repository.UpdateTemplate(ctx, userID, templateID, name, items)
	if err != nil {
		return Template{}, err
	}
	if template == nil {
		return Template{}, ErrTemplateNotFound
	}

	return *template, nil
}

func (s *Service) GetWeek(ctx context.Context, userID uuid.UUID, weekStart string) (Week, error) {
	normalizedWeekStart, err := normalizeWeekStart(weekStart)
	if err != nil {
		return Week{}, err
	}

	week, err := s.repository.GetWeek(ctx, userID, normalizedWeekStart)
	if err != nil {
		return Week{}, err
	}
	if week == nil {
		return Week{WeekStart: normalizedWeekStart, Sessions: []WeekSession{}}, nil
	}

	return *week, nil
}

func (s *Service) PutWeek(ctx context.Context, userID uuid.UUID, weekStart string, input WeekInput) (Week, error) {
	normalizedWeekStart, err := normalizeWeekStart(weekStart)
	if err != nil {
		return Week{}, err
	}

	sessions, err := s.validateWeekInput(ctx, userID, input)
	if err != nil {
		return Week{}, err
	}

	return s.repository.UpsertWeek(ctx, userID, normalizedWeekStart, sessions)
}

func (s *Service) validateTemplateInput(ctx context.Context, input TemplateInput) (string, []persistedItemInput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return "", nil, ErrTemplateNameRequired
	}
	if len(input.Items) == 0 {
		return "", nil, ErrTemplateItemsRequired
	}

	items, err := s.resolveItems(ctx, input.Items)
	if err != nil {
		return "", nil, err
	}

	return name, items, nil
}

func (s *Service) validateWeekInput(ctx context.Context, userID uuid.UUID, input WeekInput) ([]persistedSessionInput, error) {
	if len(input.Sessions) == 0 {
		return []persistedSessionInput{}, nil
	}

	positionByDay := make(map[int]int)
	sessions := make([]persistedSessionInput, 0, len(input.Sessions))
	for _, session := range input.Sessions {
		if session.DayIndex < 0 || session.DayIndex > 6 {
			return nil, ErrSessionDayIndexInvalid
		}
		if session.TemplateID != nil && session.Items != nil {
			return nil, ErrSessionShapeInvalid
		}
		if session.TemplateID == nil && session.Items == nil {
			return nil, ErrSessionItemsRequired
		}

		positionByDay[session.DayIndex]++
		resolved := persistedSessionInput{
			DayIndex: int32(session.DayIndex),
			Position: int32(positionByDay[session.DayIndex]),
		}

		switch {
		case session.TemplateID != nil:
			template, err := s.repository.GetTemplate(ctx, userID, *session.TemplateID)
			if err != nil {
				return nil, err
			}
			if template == nil {
				return nil, ErrTemplateNotFound
			}
			items, err := s.resolveItems(ctx, templateItemsToInputs(template.Items))
			if err != nil {
				return nil, err
			}
			resolved.TemplateID = session.TemplateID
			resolved.Items = items
		case session.Items != nil:
			if len(*session.Items) == 0 {
				return nil, ErrSessionItemsRequired
			}
			items, err := s.resolveItems(ctx, *session.Items)
			if err != nil {
				return nil, err
			}
			resolved.Items = items
		}

		sessions = append(sessions, resolved)
	}

	return sessions, nil
}

func (s *Service) resolveItems(ctx context.Context, inputs []ItemInput) ([]persistedItemInput, error) {
	exerciseKeys := make([]string, 0, len(inputs))
	equipmentKeys := make([]string, 0, len(inputs))
	normalizedInputs := make([]normalizedItemInput, 0, len(inputs))
	for _, input := range inputs {
		exerciseKey := strings.TrimSpace(input.ExerciseKey)
		if exerciseKey == "" {
			return nil, ErrExerciseKeyRequired
		}

		var equipmentKey *string
		if input.EquipmentKey != nil {
			trimmed := strings.TrimSpace(*input.EquipmentKey)
			if trimmed != "" {
				equipmentKey = &trimmed
				equipmentKeys = append(equipmentKeys, trimmed)
			}
		}

		exerciseKeys = append(exerciseKeys, exerciseKey)
		normalizedInputs = append(normalizedInputs, normalizedItemInput{
			ExerciseKey:  exerciseKey,
			EquipmentKey: equipmentKey,
			Sets:         input.Sets,
			Reps:         input.Reps,
			WeightKg:     input.WeightKg,
			RPE:          input.RPE,
			Notes:        normalizeOptionalText(input.Notes),
		})
	}

	exerciseRefs, err := s.catalog.ResolveExercises(ctx, exerciseKeys)
	if err != nil {
		return nil, err
	}
	equipmentRefs, err := s.catalog.ResolveEquipment(ctx, equipmentKeys)
	if err != nil {
		return nil, err
	}

	items := make([]persistedItemInput, 0, len(normalizedInputs))
	for index, input := range normalizedInputs {
		exerciseRef, ok := exerciseRefs[input.ExerciseKey]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrExerciseNotFound, input.ExerciseKey)
		}
		if input.Sets <= 0 {
			return nil, ErrSetsInvalid
		}
		if input.Reps <= 0 {
			return nil, ErrRepsInvalid
		}

		var equipmentID *uuid.UUID
		if input.EquipmentKey != nil {
			equipmentRef, ok := equipmentRefs[*input.EquipmentKey]
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrEquipmentNotFound, *input.EquipmentKey)
			}
			if !exerciseRef.AllowsEquipmentKey(*input.EquipmentKey) {
				return nil, fmt.Errorf("%w: %s", ErrEquipmentNotAllowed, *input.EquipmentKey)
			}
			equipmentID = &equipmentRef.ID
		}

		weightKg, err := numericFromOptionalFloat(input.WeightKg, 9999.99, ErrWeightInvalid)
		if err != nil {
			return nil, err
		}
		rpe, err := numericFromOptionalFloat(input.RPE, 10.0, ErrRPEInvalid)
		if err != nil {
			return nil, err
		}

		items = append(items, persistedItemInput{
			Position:              int32(index + 1),
			ExerciseDefinitionID:  exerciseRef.ID,
			ExerciseKey:           exerciseRef.ExerciseKey,
			EquipmentDefinitionID: equipmentID,
			EquipmentKey:          input.EquipmentKey,
			Sets:                  int32(input.Sets),
			Reps:                  int32(input.Reps),
			WeightKg:              weightKg,
			RPE:                   rpe,
			Notes:                 input.Notes,
		})
	}

	return items, nil
}

func templateItemsToInputs(items []TemplateItem) []ItemInput {
	result := make([]ItemInput, 0, len(items))
	for _, item := range items {
		result = append(result, ItemInput{
			ExerciseKey:  item.ExerciseKey,
			EquipmentKey: item.EquipmentKey,
			Sets:         item.Sets,
			Reps:         item.Reps,
			WeightKg:     item.WeightKg,
			RPE:          item.RPE,
			Notes:        item.Notes,
		})
	}
	return result
}

func normalizeWeekStart(value string) (string, error) {
	parsed, err := time.Parse(weekStartLayout, strings.TrimSpace(value))
	if err != nil {
		return "", ErrWeekStartInvalid
	}
	if parsed.Weekday() != time.Monday {
		return "", ErrWeekStartInvalid
	}
	return parsed.Format(weekStartLayout), nil
}

func normalizeOptionalText(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func optionalFloat64(value pgtype.Numeric) (*float64, error) {
	if !value.Valid {
		return nil, nil
	}

	floatValue, err := value.Float64Value()
	if err != nil {
		return nil, err
	}
	if !floatValue.Valid {
		return nil, nil
	}

	return &floatValue.Float64, nil
}

func numericFromOptionalFloat(value *float64, max float64, errValue error) (pgtype.Numeric, error) {
	if value == nil {
		return pgtype.Numeric{}, nil
	}
	if *value < 0 || *value > max {
		return pgtype.Numeric{}, errValue
	}

	var numeric pgtype.Numeric
	if err := numeric.Scan(strconv.FormatFloat(*value, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}, err
	}

	return numeric, nil
}

type normalizedItemInput struct {
	ExerciseKey  string
	EquipmentKey *string
	Sets         int
	Reps         int
	WeightKg     *float64
	RPE          *float64
	Notes        *string
}

type persistedItemInput struct {
	Position              int32
	ExerciseDefinitionID  uuid.UUID
	ExerciseKey           string
	EquipmentDefinitionID *uuid.UUID
	EquipmentKey          *string
	Sets                  int32
	Reps                  int32
	WeightKg              pgtype.Numeric
	RPE                   pgtype.Numeric
	Notes                 *string
}

type persistedSessionInput struct {
	DayIndex   int32
	Position   int32
	TemplateID *uuid.UUID
	Items      []persistedItemInput
}

func cloneTemplateItem(item TemplateItem) TemplateItem {
	return TemplateItem{
		Position:     item.Position,
		ExerciseKey:  item.ExerciseKey,
		EquipmentKey: item.EquipmentKey,
		Sets:         item.Sets,
		Reps:         item.Reps,
		WeightKg:     item.WeightKg,
		RPE:          item.RPE,
		Notes:        item.Notes,
	}
}

func cloneWeekSessionItem(item WeekSessionItem) WeekSessionItem {
	return WeekSessionItem{
		Position:     item.Position,
		ExerciseKey:  item.ExerciseKey,
		EquipmentKey: item.EquipmentKey,
		Sets:         item.Sets,
		Reps:         item.Reps,
		WeightKg:     item.WeightKg,
		RPE:          item.RPE,
		Notes:        item.Notes,
	}
}

func cloneTemplate(template Template) Template {
	items := make([]TemplateItem, 0, len(template.Items))
	for _, item := range template.Items {
		items = append(items, cloneTemplateItem(item))
	}
	template.Items = items
	return template
}

func cloneWeek(week Week) Week {
	sessions := make([]WeekSession, 0, len(week.Sessions))
	for _, session := range week.Sessions {
		items := make([]WeekSessionItem, 0, len(session.Items))
		for _, item := range session.Items {
			items = append(items, cloneWeekSessionItem(item))
		}
		sessions = append(sessions, WeekSession{
			DayIndex:   session.DayIndex,
			Position:   session.Position,
			TemplateID: session.TemplateID,
			Items:      items,
		})
	}
	week.Sessions = sessions
	return week
}

func sortTemplates(templates []Template) {
	slices.SortFunc(templates, func(left, right Template) int {
		leftName := strings.ToLower(left.Name)
		rightName := strings.ToLower(right.Name)
		if leftName != rightName {
			if leftName < rightName {
				return -1
			}
			return 1
		}
		if left.ID.String() < right.ID.String() {
			return -1
		}
		return 1
	})
}
