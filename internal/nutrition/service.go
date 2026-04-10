package nutrition

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/profile"
)

const (
	MealTypeBreakfast = "breakfast"
	MealTypeLunch     = "lunch"
	MealTypeDinner    = "dinner"
	MealTypeSnack     = "snack"
	MealTypeOther     = "other"
)

const (
	minCalories = 1
	maxCalories = 6000
	minGrams    = 1
	maxGrams    = 500
)

var (
	ErrMealLogNotFound               = errors.New("meal log not found")
	ErrMealTemplateNotFound          = errors.New("meal template not found")
	ErrDuplicateMealTemplateName     = errors.New("meal template name already exists")
	ErrMealNameRequired              = errors.New("meal name is required")
	ErrMealTemplateNameRequired      = errors.New("meal template name is required")
	ErrMealTypeInvalid               = errors.New("meal_type is invalid")
	ErrCaloriesInvalid               = errors.New("calories must be between 1 and 6000")
	ErrProteinInvalid                = errors.New("protein_grams must be between 1 and 500")
	ErrCarbsInvalid                  = errors.New("carbs_grams must be between 1 and 500")
	ErrFatInvalid                    = errors.New("fat_grams must be between 1 and 500")
	ErrMealTemplateNutritionRequired = errors.New("meal template requires at least one nutrition numeric field")
	ErrMealLogNutritionRequired      = errors.New("meal log requires at least one nutrition numeric field or source_template_id")
	ErrLoggedAtRequired              = errors.New("logged_at is required")
)

type Clock func() time.Time

type Store interface {
	ListMealLogs(ctx context.Context, userID uuid.UUID) ([]MealLog, error)
	GetMealTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID) (*MealTemplate, error)
	ListMealTemplates(ctx context.Context, userID uuid.UUID) ([]MealTemplate, error)
	CreateMealLog(ctx context.Context, userID uuid.UUID, input PersistedMealLogInput) (MealLog, error)
	UpdateMealLog(ctx context.Context, userID uuid.UUID, mealLogID uuid.UUID, input PersistedMealLogInput) (*MealLog, error)
	CreateMealTemplate(ctx context.Context, userID uuid.UUID, input MealTemplateInput) (MealTemplate, error)
	UpdateMealTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID, input MealTemplateInput) (*MealTemplate, error)
}

type ProfileReader interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (profile.MemberProfile, error)
}

type Service struct {
	store         Store
	profileReader ProfileReader
	now           Clock
}

type MealLog struct {
	ID               uuid.UUID  `json:"id"`
	MealType         string     `json:"meal_type"`
	LoggedAt         time.Time  `json:"logged_at"`
	Name             string     `json:"name"`
	SourceTemplateID *uuid.UUID `json:"source_template_id,omitempty"`
	Calories         *int       `json:"calories,omitempty"`
	ProteinGrams     *int       `json:"protein_grams,omitempty"`
	CarbsGrams       *int       `json:"carbs_grams,omitempty"`
	FatGrams         *int       `json:"fat_grams,omitempty"`
	Notes            *string    `json:"notes,omitempty"`
}

type MealLogInput struct {
	MealType         *string    `json:"meal_type"`
	LoggedAt         *time.Time `json:"logged_at"`
	Name             *string    `json:"name"`
	SourceTemplateID *uuid.UUID `json:"source_template_id"`
	Calories         *int       `json:"calories"`
	ProteinGrams     *int       `json:"protein_grams"`
	CarbsGrams       *int       `json:"carbs_grams"`
	FatGrams         *int       `json:"fat_grams"`
	Notes            *string    `json:"notes"`
}

type PersistedMealLogInput struct {
	MealType         string
	LoggedAt         time.Time
	Name             string
	SourceTemplateID *uuid.UUID
	Calories         *int
	ProteinGrams     *int
	CarbsGrams       *int
	FatGrams         *int
	Notes            *string
}

type MealTemplate struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	MealType     string    `json:"meal_type"`
	Calories     *int      `json:"calories,omitempty"`
	ProteinGrams *int      `json:"protein_grams,omitempty"`
	CarbsGrams   *int      `json:"carbs_grams,omitempty"`
	FatGrams     *int      `json:"fat_grams,omitempty"`
	Notes        *string   `json:"notes,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type MealTemplateInput struct {
	Name         string  `json:"name"`
	MealType     string  `json:"meal_type"`
	Calories     *int    `json:"calories"`
	ProteinGrams *int    `json:"protein_grams"`
	CarbsGrams   *int    `json:"carbs_grams"`
	FatGrams     *int    `json:"fat_grams"`
	Notes        *string `json:"notes"`
}

func NewService(store Store, profileReader ProfileReader) *Service {
	return &Service{
		store:         store,
		profileReader: profileReader,
		now:           time.Now,
	}
}

func (s *Service) ListMealLogs(ctx context.Context, userID uuid.UUID) ([]MealLog, error) {
	return s.store.ListMealLogs(ctx, userID)
}

func (s *Service) CreateMealLog(ctx context.Context, userID uuid.UUID, input MealLogInput) (MealLog, error) {
	resolved, err := s.resolveMealLogInput(ctx, userID, input, true)
	if err != nil {
		return MealLog{}, err
	}

	return s.store.CreateMealLog(ctx, userID, resolved)
}

func (s *Service) UpdateMealLog(ctx context.Context, userID uuid.UUID, mealLogID uuid.UUID, input MealLogInput) (MealLog, error) {
	resolved, err := s.resolveMealLogInput(ctx, userID, input, false)
	if err != nil {
		return MealLog{}, err
	}

	mealLog, err := s.store.UpdateMealLog(ctx, userID, mealLogID, resolved)
	if err != nil {
		return MealLog{}, err
	}
	if mealLog == nil {
		return MealLog{}, ErrMealLogNotFound
	}

	return *mealLog, nil
}

func (s *Service) ListMealTemplates(ctx context.Context, userID uuid.UUID) ([]MealTemplate, error) {
	return s.store.ListMealTemplates(ctx, userID)
}

func (s *Service) CreateMealTemplate(ctx context.Context, userID uuid.UUID, input MealTemplateInput) (MealTemplate, error) {
	validated, err := validateMealTemplateInput(input)
	if err != nil {
		return MealTemplate{}, err
	}

	return s.store.CreateMealTemplate(ctx, userID, validated)
}

func (s *Service) UpdateMealTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID, input MealTemplateInput) (MealTemplate, error) {
	validated, err := validateMealTemplateInput(input)
	if err != nil {
		return MealTemplate{}, err
	}

	template, err := s.store.UpdateMealTemplate(ctx, userID, templateID, validated)
	if err != nil {
		return MealTemplate{}, err
	}
	if template == nil {
		return MealTemplate{}, ErrMealTemplateNotFound
	}

	return *template, nil
}

func validateMealTemplateInput(input MealTemplateInput) (MealTemplateInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return MealTemplateInput{}, ErrMealTemplateNameRequired
	}
	if !isValidMealType(input.MealType) {
		return MealTemplateInput{}, ErrMealTypeInvalid
	}
	if !hasAnyNutritionValue(input.Calories, input.ProteinGrams, input.CarbsGrams, input.FatGrams) {
		return MealTemplateInput{}, ErrMealTemplateNutritionRequired
	}
	if err := validateNutritionNumbers(input.Calories, input.ProteinGrams, input.CarbsGrams, input.FatGrams); err != nil {
		return MealTemplateInput{}, err
	}
	return input, nil
}

func (s *Service) resolveMealLogInput(ctx context.Context, userID uuid.UUID, input MealLogInput, allowDefaultLoggedAt bool) (PersistedMealLogInput, error) {
	resolved := PersistedMealLogInput{
		SourceTemplateID: input.SourceTemplateID,
		Calories:         input.Calories,
		ProteinGrams:     input.ProteinGrams,
		CarbsGrams:       input.CarbsGrams,
		FatGrams:         input.FatGrams,
		Notes:            input.Notes,
	}

	if input.SourceTemplateID != nil {
		template, err := s.store.GetMealTemplate(ctx, userID, *input.SourceTemplateID)
		if err != nil {
			return PersistedMealLogInput{}, err
		}
		if template == nil {
			return PersistedMealLogInput{}, ErrMealTemplateNotFound
		}
		resolved.Name = template.Name
		resolved.MealType = template.MealType
		resolved.Calories = template.Calories
		resolved.ProteinGrams = template.ProteinGrams
		resolved.CarbsGrams = template.CarbsGrams
		resolved.FatGrams = template.FatGrams
		if resolved.Notes == nil {
			resolved.Notes = template.Notes
		}
	}

	if input.Name != nil {
		resolved.Name = strings.TrimSpace(*input.Name)
	}
	if resolved.Name == "" {
		return PersistedMealLogInput{}, ErrMealNameRequired
	}

	if input.MealType != nil {
		resolved.MealType = strings.TrimSpace(*input.MealType)
	}
	if !isValidMealType(resolved.MealType) {
		return PersistedMealLogInput{}, ErrMealTypeInvalid
	}

	if input.Calories != nil {
		resolved.Calories = input.Calories
	}
	if input.ProteinGrams != nil {
		resolved.ProteinGrams = input.ProteinGrams
	}
	if input.CarbsGrams != nil {
		resolved.CarbsGrams = input.CarbsGrams
	}
	if input.FatGrams != nil {
		resolved.FatGrams = input.FatGrams
	}
	if !hasAnyNutritionValue(resolved.Calories, resolved.ProteinGrams, resolved.CarbsGrams, resolved.FatGrams) && resolved.SourceTemplateID == nil {
		return PersistedMealLogInput{}, ErrMealLogNutritionRequired
	}
	if err := validateNutritionNumbers(resolved.Calories, resolved.ProteinGrams, resolved.CarbsGrams, resolved.FatGrams); err != nil {
		return PersistedMealLogInput{}, err
	}

	switch {
	case input.LoggedAt != nil:
		resolved.LoggedAt = input.LoggedAt.UTC()
	case allowDefaultLoggedAt:
		resolved.LoggedAt = s.now().UTC()
	default:
		return PersistedMealLogInput{}, ErrLoggedAtRequired
	}

	return resolved, nil
}

func validateNutritionNumbers(calories, protein, carbs, fat *int) error {
	if calories != nil && (*calories < minCalories || *calories > maxCalories) {
		return ErrCaloriesInvalid
	}
	if protein != nil && (*protein < minGrams || *protein > maxGrams) {
		return ErrProteinInvalid
	}
	if carbs != nil && (*carbs < minGrams || *carbs > maxGrams) {
		return ErrCarbsInvalid
	}
	if fat != nil && (*fat < minGrams || *fat > maxGrams) {
		return ErrFatInvalid
	}
	return nil
}

func hasAnyNutritionValue(calories, protein, carbs, fat *int) bool {
	return calories != nil || protein != nil || carbs != nil || fat != nil
}

func isValidMealType(value string) bool {
	switch strings.TrimSpace(value) {
	case MealTypeBreakfast, MealTypeLunch, MealTypeDinner, MealTypeSnack, MealTypeOther:
		return true
	default:
		return false
	}
}
