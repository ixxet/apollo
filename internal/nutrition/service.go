package nutrition

import (
	"context"
	"errors"
	"sort"
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

	KindStartConservative RecommendationKind = "start_conservative"
	KindHold              RecommendationKind = "hold"
	KindTighten           RecommendationKind = "tighten"
	KindRelax             RecommendationKind = "relax"

	PolicyVersion = "tracer25/v1"

	defaultGoalKey = "general-fitness"
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

type RecommendationKind string

type Clock func() time.Time

type Store interface {
	ListMealLogs(ctx context.Context, userID uuid.UUID) ([]MealLog, error)
	GetMealTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID) (*MealTemplate, error)
	ListMealTemplates(ctx context.Context, userID uuid.UUID) ([]MealTemplate, error)
	CreateMealLog(ctx context.Context, userID uuid.UUID, input PersistedMealLogInput) (MealLog, error)
	UpdateMealLog(ctx context.Context, userID uuid.UUID, mealLogID uuid.UUID, input PersistedMealLogInput) (*MealLog, error)
	CreateMealTemplate(ctx context.Context, userID uuid.UUID, input MealTemplateInput) (MealTemplate, error)
	UpdateMealTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID, input MealTemplateInput) (*MealTemplate, error)
	ListMealLogsWithinRange(ctx context.Context, userID uuid.UUID, start time.Time, end time.Time) ([]MealLog, error)
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

type Range struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type RecommendationEvidence struct {
	DietaryRestrictions       []string `json:"dietary_restrictions,omitempty"`
	CuisinePreferences        []string `json:"cuisine_preferences,omitempty"`
	BudgetPreference          *string  `json:"budget_preference,omitempty"`
	CookingCapability         *string  `json:"cooking_capability,omitempty"`
	CoachingGoalKey           *string  `json:"coaching_goal_key,omitempty"`
	TrainingDaysPerWeek       *int     `json:"training_days_per_week,omitempty"`
	TrainingSessionMinutes    *int     `json:"training_session_minutes,omitempty"`
	RecentMealLogCount        int      `json:"recent_meal_log_count"`
	RecentLoggedDayCount      int      `json:"recent_logged_day_count"`
	RecentAverageCalories     *int     `json:"recent_average_calories,omitempty"`
	RecentAverageProteinGrams *int     `json:"recent_average_protein_grams,omitempty"`
	RecentAverageCarbsGrams   *int     `json:"recent_average_carbs_grams,omitempty"`
	RecentAverageFatGrams     *int     `json:"recent_average_fat_grams,omitempty"`
}

type RecommendationExplanation struct {
	ReasonCodes []string               `json:"reason_codes"`
	Evidence    RecommendationEvidence `json:"evidence"`
	Limitations []string               `json:"limitations"`
}

type Recommendation struct {
	Kind              RecommendationKind        `json:"kind"`
	GoalKey           string                    `json:"goal_key"`
	DailyCalories     Range                     `json:"daily_calories"`
	DailyProteinGrams Range                     `json:"daily_protein_grams"`
	DailyCarbsGrams   Range                     `json:"daily_carbs_grams"`
	DailyFatGrams     Range                     `json:"daily_fat_grams"`
	StrategyFlags     []string                  `json:"strategy_flags"`
	Explanation       RecommendationExplanation `json:"explanation"`
	PolicyVersion     string                    `json:"policy_version"`
	GeneratedAt       time.Time                 `json:"generated_at"`
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

func (s *Service) GetRecommendation(ctx context.Context, userID uuid.UUID) (Recommendation, error) {
	memberProfile, err := s.profileReader.GetProfile(ctx, userID)
	if err != nil {
		return Recommendation{}, err
	}

	end := s.now().UTC()
	start := end.AddDate(0, 0, -7)
	recentLogs, err := s.store.ListMealLogsWithinRange(ctx, userID, start, end)
	if err != nil {
		return Recommendation{}, err
	}
	templates, err := s.store.ListMealTemplates(ctx, userID)
	if err != nil {
		return Recommendation{}, err
	}

	goalKey := defaultGoalKey
	if memberProfile.CoachingProfile.GoalKey != nil {
		goalKey = strings.TrimSpace(*memberProfile.CoachingProfile.GoalKey)
	}

	calories := baseCalorieRange(goalKey, memberProfile.CoachingProfile.DaysPerWeek, memberProfile.CoachingProfile.SessionMinutes)
	protein := baseProteinRange(goalKey)
	carbs := baseCarbRange(goalKey, memberProfile.CoachingProfile.DaysPerWeek, memberProfile.CoachingProfile.SessionMinutes)
	fat := baseFatRange(goalKey)

	reasonCodes := []string{}
	if memberProfile.CoachingProfile.GoalKey != nil {
		reasonCodes = append(reasonCodes, "coaching_goal_fallback")
	} else {
		reasonCodes = append(reasonCodes, "default_goal_fallback")
	}

	strategyFlags := make([]string, 0, 3)
	if memberProfile.NutritionProfile.BudgetPreference != nil && *memberProfile.NutritionProfile.BudgetPreference == "budget_constrained" {
		strategyFlags = append(strategyFlags, "budget_first")
		reasonCodes = append(reasonCodes, "budget_constrained")
	}
	if memberProfile.NutritionProfile.CookingCapability != nil {
		switch *memberProfile.NutritionProfile.CookingCapability {
		case "no_kitchen", "microwave_only":
			strategyFlags = append(strategyFlags, "assembly_first")
			reasonCodes = append(reasonCodes, "limited_cooking_capability")
		}
	}
	if len(templates) > 0 || hasTemplateLinkedHistory(recentLogs) {
		strategyFlags = append(strategyFlags, "template_reuse_first")
	}
	strategyFlags = dedupeAndSort(strategyFlags)

	history := summarizeRecentHistory(recentLogs)
	if history.loggedMealCount < 5 || history.loggedDayCount < 3 {
		reasonCodes = append(reasonCodes, "sparse_recent_history")
	} else {
		reasonCodes = append(reasonCodes, "recent_history_stabilized")
		if history.averageCalories != nil {
			calories = stabilizeRange(calories, *history.averageCalories, 1600, 3200, 250, 350)
		}
		if history.averageProtein != nil {
			protein = stabilizeRange(protein, *history.averageProtein, 80, 220, 20, 40)
		}
		if history.averageCarbs != nil {
			carbs = stabilizeRange(carbs, *history.averageCarbs, 120, 420, 35, 60)
		}
		if history.averageFat != nil {
			fat = stabilizeRange(fat, *history.averageFat, 40, 140, 15, 30)
		}
	}

	kind := KindStartConservative
	if history.loggedMealCount >= 5 && history.loggedDayCount >= 3 {
		kind = KindHold
		if history.averageCalories != nil {
			switch {
			case *history.averageCalories > calories.Max+150:
				kind = KindTighten
				reasonCodes = append(reasonCodes, "recent_intake_above_range")
			case *history.averageCalories < calories.Min-150:
				kind = KindRelax
				reasonCodes = append(reasonCodes, "recent_intake_below_range")
			}
		}
	}

	evidence := RecommendationEvidence{
		DietaryRestrictions:       append([]string(nil), memberProfile.NutritionProfile.DietaryRestrictions...),
		CuisinePreferences:        append([]string(nil), memberProfile.NutritionProfile.MealPreference.CuisinePreferences...),
		BudgetPreference:          memberProfile.NutritionProfile.BudgetPreference,
		CookingCapability:         memberProfile.NutritionProfile.CookingCapability,
		CoachingGoalKey:           memberProfile.CoachingProfile.GoalKey,
		TrainingDaysPerWeek:       memberProfile.CoachingProfile.DaysPerWeek,
		TrainingSessionMinutes:    memberProfile.CoachingProfile.SessionMinutes,
		RecentMealLogCount:        history.loggedMealCount,
		RecentLoggedDayCount:      history.loggedDayCount,
		RecentAverageCalories:     history.averageCalories,
		RecentAverageProteinGrams: history.averageProtein,
		RecentAverageCarbsGrams:   history.averageCarbs,
		RecentAverageFatGrams:     history.averageFat,
	}

	return Recommendation{
		Kind:              kind,
		GoalKey:           goalKey,
		DailyCalories:     calories,
		DailyProteinGrams: protein,
		DailyCarbsGrams:   carbs,
		DailyFatGrams:     fat,
		StrategyFlags:     strategyFlags,
		Explanation: RecommendationExplanation{
			ReasonCodes: dedupeAndSort(reasonCodes),
			Evidence:    evidence,
			Limitations: []string{
				"non-clinical guidance only; not medical advice or diagnosis",
				"ranges use explicit profile inputs and recent meal logs only",
				"body metrics, lab values, allergies outside the allowed restriction set, and detailed meal planning are out of scope",
			},
		},
		PolicyVersion: PolicyVersion,
		GeneratedAt:   end,
	}, nil
}

type recentHistorySummary struct {
	loggedMealCount int
	loggedDayCount  int
	averageCalories *int
	averageProtein  *int
	averageCarbs    *int
	averageFat      *int
}

func summarizeRecentHistory(logs []MealLog) recentHistorySummary {
	if len(logs) == 0 {
		return recentHistorySummary{}
	}

	type dayTotals struct {
		calories int
		protein  int
		carbs    int
		fat      int
		hasCal   bool
		hasPro   bool
		hasCarb  bool
		hasFat   bool
	}

	byDay := make(map[string]dayTotals, len(logs))
	for _, mealLog := range logs {
		dayKey := mealLog.LoggedAt.UTC().Format("2006-01-02")
		totals := byDay[dayKey]
		if mealLog.Calories != nil {
			totals.calories += *mealLog.Calories
			totals.hasCal = true
		}
		if mealLog.ProteinGrams != nil {
			totals.protein += *mealLog.ProteinGrams
			totals.hasPro = true
		}
		if mealLog.CarbsGrams != nil {
			totals.carbs += *mealLog.CarbsGrams
			totals.hasCarb = true
		}
		if mealLog.FatGrams != nil {
			totals.fat += *mealLog.FatGrams
			totals.hasFat = true
		}
		byDay[dayKey] = totals
	}

	summary := recentHistorySummary{
		loggedMealCount: len(logs),
		loggedDayCount:  len(byDay),
	}

	var calorieTotal, calorieDays int
	var proteinTotal, proteinDays int
	var carbsTotal, carbsDays int
	var fatTotal, fatDays int
	for _, totals := range byDay {
		if totals.hasCal {
			calorieTotal += totals.calories
			calorieDays++
		}
		if totals.hasPro {
			proteinTotal += totals.protein
			proteinDays++
		}
		if totals.hasCarb {
			carbsTotal += totals.carbs
			carbsDays++
		}
		if totals.hasFat {
			fatTotal += totals.fat
			fatDays++
		}
	}

	if calorieDays >= 3 {
		average := calorieTotal / calorieDays
		summary.averageCalories = &average
	}
	if proteinDays >= 3 {
		average := proteinTotal / proteinDays
		summary.averageProtein = &average
	}
	if carbsDays >= 3 {
		average := carbsTotal / carbsDays
		summary.averageCarbs = &average
	}
	if fatDays >= 3 {
		average := fatTotal / fatDays
		summary.averageFat = &average
	}

	return summary
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

func hasTemplateLinkedHistory(logs []MealLog) bool {
	for _, mealLog := range logs {
		if mealLog.SourceTemplateID != nil {
			return true
		}
	}
	return false
}

func dedupeAndSort(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func baseCalorieRange(goalKey string, daysPerWeek *int, sessionMinutes *int) Range {
	result := Range{Min: 1900, Max: 2500}
	switch {
	case strings.Contains(goalKey, "loss"), strings.Contains(goalKey, "cut"):
		result = Range{Min: 1700, Max: 2200}
	case strings.Contains(goalKey, "strength"), strings.Contains(goalKey, "build"), strings.Contains(goalKey, "muscle"):
		result = Range{Min: 2100, Max: 2700}
	case strings.Contains(goalKey, "endurance"):
		result = Range{Min: 2100, Max: 2800}
	}

	if trainingVolume(daysPerWeek, sessionMinutes) >= 300 {
		result.Min += 150
		result.Max += 150
	} else if trainingVolume(daysPerWeek, sessionMinutes) > 0 && trainingVolume(daysPerWeek, sessionMinutes) < 120 {
		result.Min -= 100
		result.Max -= 100
	}

	return result
}

func baseProteinRange(goalKey string) Range {
	switch {
	case strings.Contains(goalKey, "loss"), strings.Contains(goalKey, "cut"):
		return Range{Min: 110, Max: 165}
	case strings.Contains(goalKey, "strength"), strings.Contains(goalKey, "build"), strings.Contains(goalKey, "muscle"):
		return Range{Min: 120, Max: 175}
	case strings.Contains(goalKey, "endurance"):
		return Range{Min: 100, Max: 155}
	default:
		return Range{Min: 95, Max: 150}
	}
}

func baseCarbRange(goalKey string, daysPerWeek *int, sessionMinutes *int) Range {
	result := Range{Min: 200, Max: 300}
	switch {
	case strings.Contains(goalKey, "loss"), strings.Contains(goalKey, "cut"):
		result = Range{Min: 160, Max: 240}
	case strings.Contains(goalKey, "strength"), strings.Contains(goalKey, "build"), strings.Contains(goalKey, "muscle"):
		result = Range{Min: 220, Max: 320}
	case strings.Contains(goalKey, "endurance"):
		result = Range{Min: 240, Max: 360}
	}

	if trainingVolume(daysPerWeek, sessionMinutes) >= 300 {
		result.Min += 20
		result.Max += 40
	}

	return result
}

func baseFatRange(goalKey string) Range {
	switch {
	case strings.Contains(goalKey, "loss"), strings.Contains(goalKey, "cut"):
		return Range{Min: 50, Max: 75}
	case strings.Contains(goalKey, "strength"), strings.Contains(goalKey, "build"), strings.Contains(goalKey, "muscle"):
		return Range{Min: 60, Max: 90}
	default:
		return Range{Min: 55, Max: 85}
	}
}

func trainingVolume(daysPerWeek *int, sessionMinutes *int) int {
	if daysPerWeek == nil || sessionMinutes == nil {
		return 0
	}
	return *daysPerWeek * *sessionMinutes
}

func stabilizeRange(base Range, average int, floor int, ceiling int, halfWidth int, minWidth int) Range {
	lower := clampInt((base.Min+average-halfWidth)/2, floor, ceiling)
	upper := clampInt((base.Max+average+halfWidth)/2, floor, ceiling)
	if upper < lower {
		upper = lower
	}
	if upper-lower < minWidth {
		padding := (minWidth - (upper - lower)) / 2
		lower = clampInt(lower-padding, floor, ceiling)
		upper = clampInt(lower+minWidth, floor, ceiling)
		if upper > ceiling {
			upper = ceiling
			lower = clampInt(upper-minWidth, floor, ceiling)
		}
	}
	return Range{Min: lower, Max: upper}
}

func clampInt(value int, minimum int, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
