package profile

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/exercises"
	"github.com/ixxet/apollo/internal/store"
)

type stubRepository struct {
	userToReturn       *store.ApolloUser
	updatedUser        *store.ApolloUser
	updatedPreferences []byte
}

func (s *stubRepository) GetUserByID(context.Context, uuid.UUID) (*store.ApolloUser, error) {
	return s.userToReturn, nil
}

func (s *stubRepository) UpdatePreferences(_ context.Context, _ uuid.UUID, preferences []byte) (*store.ApolloUser, error) {
	s.updatedPreferences = preferences
	return s.updatedUser, nil
}

type stubEquipmentResolver struct {
	items map[string]exercises.EquipmentRef
}

func (s stubEquipmentResolver) ResolveEquipment(context.Context, []string) (map[string]exercises.EquipmentRef, error) {
	return s.items, nil
}

func TestGetProfileDefaultsPredictablyWhenPreferencesAreSparse(t *testing.T) {
	service := NewService(&stubRepository{
		userToReturn: &store.ApolloUser{
			ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			StudentID:   "student-001",
			DisplayName: "student-001",
			Email:       "student@example.com",
			Preferences: []byte(`{}`),
		},
	}, stubEquipmentResolver{})

	memberProfile, err := service.GetProfile(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if memberProfile.VisibilityMode != VisibilityModeGhost {
		t.Fatalf("VisibilityMode = %q, want %q", memberProfile.VisibilityMode, VisibilityModeGhost)
	}
	if memberProfile.AvailabilityMode != AvailabilityModeUnavailable {
		t.Fatalf("AvailabilityMode = %q, want %q", memberProfile.AvailabilityMode, AvailabilityModeUnavailable)
	}
}

func TestUpdateProfileValidatesModesWithTableDrivenCoverage(t *testing.T) {
	testCases := []struct {
		name        string
		input       UpdateInput
		expectedErr error
	}{
		{
			name: "invalid visibility mode",
			input: UpdateInput{
				VisibilityMode: stringPtr("hidden"),
			},
			expectedErr: ErrInvalidVisibilityMode,
		},
		{
			name: "invalid availability mode",
			input: UpdateInput{
				AvailabilityMode: stringPtr("queued"),
			},
			expectedErr: ErrInvalidAvailabilityMode,
		},
		{
			name: "invalid coaching experience level",
			input: UpdateInput{
				CoachingProfile: &CoachingProfileInput{
					ExperienceLevel: stringPtr("elite"),
				},
			},
			expectedErr: ErrInvalidExperienceLevel,
		},
		{
			name: "conflicting dietary restrictions",
			input: UpdateInput{
				NutritionProfile: &NutritionProfileInput{
					DietaryRestrictions: &[]string{"vegetarian", "vegan"},
				},
			},
			expectedErr: ErrInvalidDietaryRestrictions,
		},
		{
			name: "invalid cuisine preferences",
			input: UpdateInput{
				NutritionProfile: &NutritionProfileInput{
					MealPreference: &MealPreferenceInput{
						CuisinePreferences: &[]string{"not valid"},
					},
				},
			},
			expectedErr: ErrInvalidCuisinePreferences,
		},
		{
			name: "invalid budget preference",
			input: UpdateInput{
				NutritionProfile: &NutritionProfileInput{
					BudgetPreference: stringPtr("luxury"),
				},
			},
			expectedErr: ErrInvalidBudgetPreference,
		},
		{
			name: "invalid cooking capability",
			input: UpdateInput{
				NutritionProfile: &NutritionProfileInput{
					CookingCapability: stringPtr("chef_kitchen"),
				},
			},
			expectedErr: ErrInvalidCookingCapability,
		},
		{
			name:        "empty patch",
			input:       UpdateInput{},
			expectedErr: ErrEmptyPatch,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := NewService(&stubRepository{
				userToReturn: &store.ApolloUser{
					ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
					StudentID:   "student-001",
					DisplayName: "student-001",
					Email:       "student@example.com",
					Preferences: []byte(`{"visibility_mode":"ghost","availability_mode":"unavailable"}`),
				},
			}, stubEquipmentResolver{})

			_, err := service.UpdateProfile(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111"), testCase.input)
			if err != testCase.expectedErr {
				t.Fatalf("UpdateProfile() error = %v, want %v", err, testCase.expectedErr)
			}
		})
	}
}

func TestUpdateProfilePreservesUntouchedSettingsAndUnknownPreferences(t *testing.T) {
	repository := &stubRepository{
		userToReturn: &store.ApolloUser{
			ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			StudentID:   "student-001",
			DisplayName: "student-001",
			Email:       "student@example.com",
			Preferences: []byte(`{"visibility_mode":"ghost","availability_mode":"with_team","coaching_profile":{"goal":"endurance"}}`),
		},
		updatedUser: &store.ApolloUser{
			ID:              uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			StudentID:       "student-001",
			DisplayName:     "student-001",
			Email:           "student@example.com",
			Preferences:     []byte(`{"visibility_mode":"discoverable","availability_mode":"with_team","coaching_profile":{"goal":"endurance"}}`),
			EmailVerifiedAt: pgtype.Timestamptz{Valid: true},
		},
	}
	service := NewService(repository, stubEquipmentResolver{})

	memberProfile, err := service.UpdateProfile(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111"), UpdateInput{
		VisibilityMode: stringPtr(VisibilityModeDiscoverable),
	})
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if memberProfile.VisibilityMode != VisibilityModeDiscoverable {
		t.Fatalf("VisibilityMode = %q, want %q", memberProfile.VisibilityMode, VisibilityModeDiscoverable)
	}
	if memberProfile.AvailabilityMode != AvailabilityModeWithTeam {
		t.Fatalf("AvailabilityMode = %q, want %q", memberProfile.AvailabilityMode, AvailabilityModeWithTeam)
	}
	if !memberProfile.EmailVerified {
		t.Fatal("EmailVerified = false, want true")
	}

	var savedPreferences map[string]any
	if err := json.Unmarshal(repository.updatedPreferences, &savedPreferences); err != nil {
		t.Fatalf("json.Unmarshal(updatedPreferences) error = %v", err)
	}
	if savedPreferences["availability_mode"] != AvailabilityModeWithTeam {
		t.Fatalf("availability_mode = %#v, want %q", savedPreferences["availability_mode"], AvailabilityModeWithTeam)
	}
	if _, ok := savedPreferences["coaching_profile"]; !ok {
		t.Fatal("coaching_profile missing after partial update")
	}
}

func TestUpdateProfileWritesTypedCoachingProfileFields(t *testing.T) {
	repository := &stubRepository{
		userToReturn: &store.ApolloUser{
			ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			StudentID:   "student-001",
			DisplayName: "student-001",
			Email:       "student@example.com",
			Preferences: []byte(`{"visibility_mode":"ghost","availability_mode":"with_team","coaching_profile":{"legacy":"preserved"}}`),
		},
		updatedUser: &store.ApolloUser{
			ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			StudentID:   "student-001",
			DisplayName: "student-001",
			Email:       "student@example.com",
			Preferences: []byte(`{"visibility_mode":"ghost","availability_mode":"with_team","coaching_profile":{"legacy":"preserved","goal_key":"build-strength","days_per_week":4,"session_minutes":60,"experience_level":"intermediate","preferred_equipment_keys":["barbell","dumbbell"]}}`),
		},
	}
	service := NewService(repository, stubEquipmentResolver{
		items: map[string]exercises.EquipmentRef{
			"barbell":  {EquipmentKey: "barbell"},
			"dumbbell": {EquipmentKey: "dumbbell"},
		},
	})

	daysPerWeek := 4
	sessionMinutes := 60
	profile, err := service.UpdateProfile(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111"), UpdateInput{
		CoachingProfile: &CoachingProfileInput{
			GoalKey:                stringPtr("build-strength"),
			DaysPerWeek:            &daysPerWeek,
			SessionMinutes:         &sessionMinutes,
			ExperienceLevel:        stringPtr("intermediate"),
			PreferredEquipmentKeys: &[]string{"barbell", "dumbbell"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if profile.CoachingProfile.GoalKey == nil || *profile.CoachingProfile.GoalKey != "build-strength" {
		t.Fatalf("GoalKey = %#v, want build-strength", profile.CoachingProfile.GoalKey)
	}
	if len(profile.CoachingProfile.PreferredEquipmentKeys) != 2 {
		t.Fatalf("PreferredEquipmentKeys = %#v, want 2 keys", profile.CoachingProfile.PreferredEquipmentKeys)
	}
	if profile.CoachingProfile.ExperienceLevel == nil || *profile.CoachingProfile.ExperienceLevel != "intermediate" {
		t.Fatalf("ExperienceLevel = %#v, want intermediate", profile.CoachingProfile.ExperienceLevel)
	}

	var savedPreferences map[string]any
	if err := json.Unmarshal(repository.updatedPreferences, &savedPreferences); err != nil {
		t.Fatalf("json.Unmarshal(updatedPreferences) error = %v", err)
	}
	coachingProfile, ok := savedPreferences["coaching_profile"].(map[string]any)
	if !ok {
		t.Fatalf("coaching_profile = %#v, want object", savedPreferences["coaching_profile"])
	}
	if coachingProfile["legacy"] != "preserved" {
		t.Fatalf("legacy = %#v, want preserved", coachingProfile["legacy"])
	}
}

func TestUpdateProfileWritesTypedNutritionProfileFields(t *testing.T) {
	repository := &stubRepository{
		userToReturn: &store.ApolloUser{
			ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			StudentID:   "student-001",
			DisplayName: "student-001",
			Email:       "student@example.com",
			Preferences: []byte(`{"visibility_mode":"ghost","availability_mode":"with_team","nutrition_profile":{"legacy":"preserved"}}`),
		},
		updatedUser: &store.ApolloUser{
			ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			StudentID:   "student-001",
			DisplayName: "student-001",
			Email:       "student@example.com",
			Preferences: []byte(`{"visibility_mode":"ghost","availability_mode":"with_team","nutrition_profile":{"legacy":"preserved","dietary_restrictions":["vegetarian","dairy_free"],"meal_preference":{"cuisine_preferences":["mediterranean","latin-american"]},"budget_preference":"budget_constrained","cooking_capability":"microwave_only"}}`),
		},
	}
	service := NewService(repository, stubEquipmentResolver{})

	profile, err := service.UpdateProfile(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111"), UpdateInput{
		NutritionProfile: &NutritionProfileInput{
			DietaryRestrictions: &[]string{" vegetarian ", "dairy_free", "vegetarian"},
			MealPreference: &MealPreferenceInput{
				CuisinePreferences: &[]string{" mediterranean ", "latin-american", "mediterranean"},
			},
			BudgetPreference:  stringPtr("budget_constrained"),
			CookingCapability: stringPtr("microwave_only"),
		},
	})
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if len(profile.NutritionProfile.DietaryRestrictions) != 2 {
		t.Fatalf("DietaryRestrictions = %#v, want 2 entries", profile.NutritionProfile.DietaryRestrictions)
	}
	if profile.NutritionProfile.BudgetPreference == nil || *profile.NutritionProfile.BudgetPreference != "budget_constrained" {
		t.Fatalf("BudgetPreference = %#v, want budget_constrained", profile.NutritionProfile.BudgetPreference)
	}
	if profile.NutritionProfile.CookingCapability == nil || *profile.NutritionProfile.CookingCapability != "microwave_only" {
		t.Fatalf("CookingCapability = %#v, want microwave_only", profile.NutritionProfile.CookingCapability)
	}
	if len(profile.NutritionProfile.MealPreference.CuisinePreferences) != 2 {
		t.Fatalf("CuisinePreferences = %#v, want 2 entries", profile.NutritionProfile.MealPreference.CuisinePreferences)
	}

	var savedPreferences map[string]any
	if err := json.Unmarshal(repository.updatedPreferences, &savedPreferences); err != nil {
		t.Fatalf("json.Unmarshal(updatedPreferences) error = %v", err)
	}
	nutritionProfile, ok := savedPreferences["nutrition_profile"].(map[string]any)
	if !ok {
		t.Fatalf("nutrition_profile = %#v, want object", savedPreferences["nutrition_profile"])
	}
	if nutritionProfile["legacy"] != "preserved" {
		t.Fatalf("legacy = %#v, want preserved", nutritionProfile["legacy"])
	}
	mealPreference, ok := nutritionProfile["meal_preference"].(map[string]any)
	if !ok {
		t.Fatalf("meal_preference = %#v, want object", nutritionProfile["meal_preference"])
	}
	if cuisines, ok := mealPreference["cuisine_preferences"].([]any); !ok || len(cuisines) != 2 {
		t.Fatalf("cuisine_preferences = %#v, want 2 entries", mealPreference["cuisine_preferences"])
	}
}

func stringPtr(value string) *string {
	return &value
}
