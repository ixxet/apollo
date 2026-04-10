package nutrition

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/profile"
)

type stubStore struct {
	mealLogs                 []MealLog
	mealLogsWithinRange      []MealLog
	mealTemplates            []MealTemplate
	templateByID             map[uuid.UUID]MealTemplate
	createdMealLogInput      *PersistedMealLogInput
	updatedMealLogInput      *PersistedMealLogInput
	updatedMealLogID         uuid.UUID
	createdMealTemplateInput *MealTemplateInput
	updatedMealTemplateInput *MealTemplateInput
	updatedTemplateID        uuid.UUID
	createMealLogErr         error
	updateMealLogErr         error
	createMealTemplateErr    error
	updateMealTemplateErr    error
	listMealLogsErr          error
	listMealTemplatesErr     error
	listMealLogsRangeErr     error
	getMealTemplateErr       error
}

func (s *stubStore) ListMealLogs(context.Context, uuid.UUID) ([]MealLog, error) {
	if s.listMealLogsErr != nil {
		return nil, s.listMealLogsErr
	}
	return append([]MealLog(nil), s.mealLogs...), nil
}

func (s *stubStore) GetMealTemplate(_ context.Context, _ uuid.UUID, templateID uuid.UUID) (*MealTemplate, error) {
	if s.getMealTemplateErr != nil {
		return nil, s.getMealTemplateErr
	}
	if template, ok := s.templateByID[templateID]; ok {
		templateCopy := template
		return &templateCopy, nil
	}
	return nil, nil
}

func (s *stubStore) ListMealTemplates(context.Context, uuid.UUID) ([]MealTemplate, error) {
	if s.listMealTemplatesErr != nil {
		return nil, s.listMealTemplatesErr
	}
	return append([]MealTemplate(nil), s.mealTemplates...), nil
}

func (s *stubStore) CreateMealLog(_ context.Context, _ uuid.UUID, input PersistedMealLogInput) (MealLog, error) {
	if s.createMealLogErr != nil {
		return MealLog{}, s.createMealLogErr
	}
	s.createdMealLogInput = &input
	return MealLog{
		ID:               uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		MealType:         input.MealType,
		LoggedAt:         input.LoggedAt,
		Name:             input.Name,
		SourceTemplateID: input.SourceTemplateID,
		Calories:         input.Calories,
		ProteinGrams:     input.ProteinGrams,
		CarbsGrams:       input.CarbsGrams,
		FatGrams:         input.FatGrams,
		Notes:            input.Notes,
	}, nil
}

func (s *stubStore) UpdateMealLog(_ context.Context, _ uuid.UUID, mealLogID uuid.UUID, input PersistedMealLogInput) (*MealLog, error) {
	if s.updateMealLogErr != nil {
		return nil, s.updateMealLogErr
	}
	s.updatedMealLogID = mealLogID
	s.updatedMealLogInput = &input
	mealLog := MealLog{
		ID:               mealLogID,
		MealType:         input.MealType,
		LoggedAt:         input.LoggedAt,
		Name:             input.Name,
		SourceTemplateID: input.SourceTemplateID,
		Calories:         input.Calories,
		ProteinGrams:     input.ProteinGrams,
		CarbsGrams:       input.CarbsGrams,
		FatGrams:         input.FatGrams,
		Notes:            input.Notes,
	}
	return &mealLog, nil
}

func (s *stubStore) CreateMealTemplate(_ context.Context, _ uuid.UUID, input MealTemplateInput) (MealTemplate, error) {
	if s.createMealTemplateErr != nil {
		return MealTemplate{}, s.createMealTemplateErr
	}
	s.createdMealTemplateInput = &input
	return MealTemplate{}, nil
}

func (s *stubStore) UpdateMealTemplate(_ context.Context, _ uuid.UUID, templateID uuid.UUID, input MealTemplateInput) (*MealTemplate, error) {
	if s.updateMealTemplateErr != nil {
		return nil, s.updateMealTemplateErr
	}
	s.updatedTemplateID = templateID
	s.updatedMealTemplateInput = &input
	template := MealTemplate{
		ID:           templateID,
		Name:         input.Name,
		MealType:     input.MealType,
		Calories:     input.Calories,
		ProteinGrams: input.ProteinGrams,
		CarbsGrams:   input.CarbsGrams,
		FatGrams:     input.FatGrams,
		Notes:        input.Notes,
	}
	return &template, nil
}

func (s *stubStore) ListMealLogsWithinRange(context.Context, uuid.UUID, time.Time, time.Time) ([]MealLog, error) {
	if s.listMealLogsRangeErr != nil {
		return nil, s.listMealLogsRangeErr
	}
	return append([]MealLog(nil), s.mealLogsWithinRange...), nil
}

type stubProfileReader struct {
	memberProfile profile.MemberProfile
	err           error
}

func (s stubProfileReader) GetProfile(context.Context, uuid.UUID) (profile.MemberProfile, error) {
	if s.err != nil {
		return profile.MemberProfile{}, s.err
	}
	return s.memberProfile, nil
}

func TestCreateMealLogResolvesTemplateDefaultsAndDefaultsLoggedAt(t *testing.T) {
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	templateID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	fixedNow := time.Date(2026, 4, 10, 15, 0, 0, 0, time.UTC)
	templateNotes := "template default"
	store := &stubStore{
		templateByID: map[uuid.UUID]MealTemplate{
			templateID: {
				ID:           templateID,
				Name:         "Microwave oats",
				MealType:     MealTypeBreakfast,
				Calories:     intPtr(520),
				ProteinGrams: intPtr(28),
				CarbsGrams:   intPtr(68),
				FatGrams:     intPtr(14),
				Notes:        &templateNotes,
			},
		},
	}
	service := NewService(store, stubProfileReader{})
	service.now = func() time.Time { return fixedNow }

	overrideNotes := "after training"
	mealLog, err := service.CreateMealLog(context.Background(), userID, MealLogInput{
		SourceTemplateID: &templateID,
		Notes:            &overrideNotes,
	})
	if err != nil {
		t.Fatalf("CreateMealLog() error = %v", err)
	}
	if store.createdMealLogInput == nil {
		t.Fatal("CreateMealLog() did not persist any input")
	}
	if store.createdMealLogInput.Name != "Microwave oats" {
		t.Fatalf("Name = %q, want Microwave oats", store.createdMealLogInput.Name)
	}
	if store.createdMealLogInput.MealType != MealTypeBreakfast {
		t.Fatalf("MealType = %q, want %q", store.createdMealLogInput.MealType, MealTypeBreakfast)
	}
	if !store.createdMealLogInput.LoggedAt.Equal(fixedNow) {
		t.Fatalf("LoggedAt = %s, want %s", store.createdMealLogInput.LoggedAt, fixedNow)
	}
	if store.createdMealLogInput.Notes == nil || *store.createdMealLogInput.Notes != overrideNotes {
		t.Fatalf("Notes = %#v, want %q", store.createdMealLogInput.Notes, overrideNotes)
	}
	if mealLog.SourceTemplateID == nil || *mealLog.SourceTemplateID != templateID {
		t.Fatalf("SourceTemplateID = %#v, want %s", mealLog.SourceTemplateID, templateID)
	}
	if mealLog.Calories == nil || *mealLog.Calories != 520 {
		t.Fatalf("Calories = %#v, want 520", mealLog.Calories)
	}
}

func TestMealLogValidationRejectsMissingNutritionOrRequiredLoggedAt(t *testing.T) {
	service := NewService(&stubStore{}, stubProfileReader{})
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	mealLogID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	name := "Lunch"
	mealType := MealTypeLunch
	calories := 640

	if _, err := service.CreateMealLog(context.Background(), userID, MealLogInput{
		Name:     &name,
		MealType: &mealType,
	}); err != ErrMealLogNutritionRequired {
		t.Fatalf("CreateMealLog() error = %v, want %v", err, ErrMealLogNutritionRequired)
	}

	if _, err := service.UpdateMealLog(context.Background(), userID, mealLogID, MealLogInput{
		Name:     &name,
		MealType: &mealType,
		Calories: &calories,
	}); err != ErrLoggedAtRequired {
		t.Fatalf("UpdateMealLog() error = %v, want %v", err, ErrLoggedAtRequired)
	}
}

func TestGetRecommendationStaysConservativeAndStableForSparseHistory(t *testing.T) {
	fixedNow := time.Date(2026, 4, 10, 16, 0, 0, 0, time.UTC)
	store := &stubStore{
		mealLogsWithinRange: []MealLog{
			{
				ID:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				MealType: MealTypeBreakfast,
				LoggedAt: fixedNow.Add(-24 * time.Hour),
				Name:     "Breakfast",
				Calories: intPtr(520),
			},
		},
		mealTemplates: []MealTemplate{
			{
				ID:           uuid.MustParse("22222222-2222-2222-2222-222222222222"),
				Name:         "Overnight oats",
				MealType:     MealTypeBreakfast,
				Calories:     intPtr(520),
				ProteinGrams: intPtr(28),
				CarbsGrams:   intPtr(68),
				FatGrams:     intPtr(14),
			},
		},
	}
	service := NewService(store, stubProfileReader{
		memberProfile: profile.MemberProfile{
			NutritionProfile: profile.NutritionProfile{
				DietaryRestrictions: []string{"vegetarian", "dairy_free"},
				MealPreference: profile.MealPreference{
					CuisinePreferences: []string{"mediterranean"},
				},
				BudgetPreference:  stringPtr("budget_constrained"),
				CookingCapability: stringPtr("microwave_only"),
			},
		},
	})
	service.now = func() time.Time { return fixedNow }

	first, err := service.GetRecommendation(context.Background(), uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
	if err != nil {
		t.Fatalf("GetRecommendation() error = %v", err)
	}
	second, err := service.GetRecommendation(context.Background(), uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
	if err != nil {
		t.Fatalf("GetRecommendation() second error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("recommendations changed across rerun:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if first.Kind != KindStartConservative {
		t.Fatalf("Kind = %q, want %q", first.Kind, KindStartConservative)
	}
	if first.GoalKey != defaultGoalKey {
		t.Fatalf("GoalKey = %q, want %q", first.GoalKey, defaultGoalKey)
	}
	if !reflect.DeepEqual(first.StrategyFlags, []string{"assembly_first", "budget_first", "template_reuse_first"}) {
		t.Fatalf("StrategyFlags = %#v, want conservative strategy flags", first.StrategyFlags)
	}
	if !containsString(first.Explanation.ReasonCodes, "default_goal_fallback") {
		t.Fatalf("ReasonCodes = %#v, want default_goal_fallback", first.Explanation.ReasonCodes)
	}
	if !containsString(first.Explanation.ReasonCodes, "sparse_recent_history") {
		t.Fatalf("ReasonCodes = %#v, want sparse_recent_history", first.Explanation.ReasonCodes)
	}
	if !containsString(first.Explanation.Limitations, "non-clinical guidance only; not medical advice or diagnosis") {
		t.Fatalf("Limitations = %#v, want non-clinical limitation", first.Explanation.Limitations)
	}
	if first.Explanation.Evidence.RecentMealLogCount != 1 {
		t.Fatalf("RecentMealLogCount = %d, want 1", first.Explanation.Evidence.RecentMealLogCount)
	}
}

func TestGetRecommendationTightensWhenRecentCaloriesSitAboveConservativeRange(t *testing.T) {
	fixedNow := time.Date(2026, 4, 10, 18, 0, 0, 0, time.UTC)
	store := &stubStore{
		mealLogsWithinRange: []MealLog{
			{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), MealType: MealTypeBreakfast, LoggedAt: fixedNow.AddDate(0, 0, -1), Name: "Day 1", Calories: intPtr(3600), ProteinGrams: intPtr(210), CarbsGrams: intPtr(380), FatGrams: intPtr(120)},
			{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), MealType: MealTypeLunch, LoggedAt: fixedNow.AddDate(0, 0, -2), Name: "Day 2", Calories: intPtr(3600), ProteinGrams: intPtr(210), CarbsGrams: intPtr(380), FatGrams: intPtr(120)},
			{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), MealType: MealTypeDinner, LoggedAt: fixedNow.AddDate(0, 0, -3), Name: "Day 3", Calories: intPtr(3600), ProteinGrams: intPtr(210), CarbsGrams: intPtr(380), FatGrams: intPtr(120)},
			{ID: uuid.MustParse("44444444-4444-4444-4444-444444444444"), MealType: MealTypeSnack, LoggedAt: fixedNow.AddDate(0, 0, -4), Name: "Day 4", Calories: intPtr(3600), ProteinGrams: intPtr(210), CarbsGrams: intPtr(380), FatGrams: intPtr(120)},
			{ID: uuid.MustParse("55555555-5555-5555-5555-555555555555"), MealType: MealTypeOther, LoggedAt: fixedNow.AddDate(0, 0, -5), Name: "Day 5", Calories: intPtr(3600), ProteinGrams: intPtr(210), CarbsGrams: intPtr(380), FatGrams: intPtr(120)},
		},
	}
	service := NewService(store, stubProfileReader{
		memberProfile: profile.MemberProfile{
			CoachingProfile: profile.CoachingProfile{
				GoalKey:        stringPtr("build-strength"),
				DaysPerWeek:    intPtr(4),
				SessionMinutes: intPtr(60),
			},
		},
	})
	service.now = func() time.Time { return fixedNow }

	recommendation, err := service.GetRecommendation(context.Background(), uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
	if err != nil {
		t.Fatalf("GetRecommendation() error = %v", err)
	}
	if recommendation.Kind != KindTighten {
		t.Fatalf("Kind = %q, want %q", recommendation.Kind, KindTighten)
	}
	if recommendation.DailyCalories != (Range{Min: 2725, Max: 3200}) {
		t.Fatalf("DailyCalories = %#v, want %#v", recommendation.DailyCalories, Range{Min: 2725, Max: 3200})
	}
	if !containsString(recommendation.Explanation.ReasonCodes, "recent_history_stabilized") {
		t.Fatalf("ReasonCodes = %#v, want recent_history_stabilized", recommendation.Explanation.ReasonCodes)
	}
	if !containsString(recommendation.Explanation.ReasonCodes, "recent_intake_above_range") {
		t.Fatalf("ReasonCodes = %#v, want recent_intake_above_range", recommendation.Explanation.ReasonCodes)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func intPtr(value int) *int {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
