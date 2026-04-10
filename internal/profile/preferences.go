package profile

import (
	"encoding/json"
	"regexp"
	"strings"
)

var goalKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type PreferenceModes struct {
	VisibilityMode          string
	AvailabilityMode        string
	InvalidVisibilityMode   bool
	InvalidAvailabilityMode bool
}

type CoachingProfile struct {
	GoalKey                *string  `json:"goal_key,omitempty"`
	DaysPerWeek            *int     `json:"days_per_week,omitempty"`
	SessionMinutes         *int     `json:"session_minutes,omitempty"`
	ExperienceLevel        *string  `json:"experience_level,omitempty"`
	PreferredEquipmentKeys []string `json:"preferred_equipment_keys,omitempty"`
}

type MealPreference struct {
	CuisinePreferences []string `json:"cuisine_preferences,omitempty"`
}

type MealPreferenceInput struct {
	CuisinePreferences *[]string `json:"cuisine_preferences"`
}

type NutritionProfile struct {
	DietaryRestrictions []string       `json:"dietary_restrictions,omitempty"`
	MealPreference      MealPreference `json:"meal_preference,omitempty"`
	BudgetPreference    *string        `json:"budget_preference,omitempty"`
	CookingCapability   *string        `json:"cooking_capability,omitempty"`
}

type NutritionProfileInput struct {
	DietaryRestrictions *[]string            `json:"dietary_restrictions"`
	MealPreference      *MealPreferenceInput `json:"meal_preference"`
	BudgetPreference    *string              `json:"budget_preference"`
	CookingCapability   *string              `json:"cooking_capability"`
}

type CoachingProfileInput struct {
	GoalKey                *string   `json:"goal_key"`
	DaysPerWeek            *int      `json:"days_per_week"`
	SessionMinutes         *int      `json:"session_minutes"`
	ExperienceLevel        *string   `json:"experience_level"`
	PreferredEquipmentKeys *[]string `json:"preferred_equipment_keys"`
}

func ReadPreferenceModes(raw []byte) PreferenceModes {
	preferences := decodePreferences(raw)
	modes := PreferenceModes{
		VisibilityMode:   VisibilityModeGhost,
		AvailabilityMode: AvailabilityModeUnavailable,
	}

	if value, ok := preferences["visibility_mode"]; ok {
		asString, stringOK := value.(string)
		if !stringOK || !isValidVisibilityMode(asString) {
			modes.InvalidVisibilityMode = true
		} else {
			modes.VisibilityMode = asString
		}
	}

	if value, ok := preferences["availability_mode"]; ok {
		asString, stringOK := value.(string)
		if !stringOK || !isValidAvailabilityMode(asString) {
			modes.InvalidAvailabilityMode = true
		} else {
			modes.AvailabilityMode = asString
		}
	}

	return modes
}

func ReadCoachingProfile(raw []byte) CoachingProfile {
	preferences := decodePreferences(raw)
	rawProfile, ok := preferences["coaching_profile"]
	if !ok {
		return CoachingProfile{}
	}

	asMap, ok := rawProfile.(map[string]any)
	if !ok {
		return CoachingProfile{}
	}

	profile := CoachingProfile{}
	if value, ok := asMap["goal_key"]; ok {
		asString, stringOK := value.(string)
		asString = strings.TrimSpace(asString)
		if stringOK && goalKeyPattern.MatchString(asString) {
			profile.GoalKey = &asString
		}
	}
	if value, ok := asMap["days_per_week"]; ok {
		if parsed, ok := parsePositiveWholeNumber(value); ok && parsed >= 1 && parsed <= 7 {
			profile.DaysPerWeek = &parsed
		}
	}
	if value, ok := asMap["session_minutes"]; ok {
		if parsed, ok := parsePositiveWholeNumber(value); ok && parsed > 0 {
			profile.SessionMinutes = &parsed
		}
	}
	if value, ok := asMap["experience_level"]; ok {
		asString, stringOK := value.(string)
		asString = strings.TrimSpace(asString)
		if stringOK && isValidExperienceLevel(asString) {
			profile.ExperienceLevel = &asString
		}
	}
	if value, ok := asMap["preferred_equipment_keys"]; ok {
		asSlice, ok := value.([]any)
		if ok {
			keys := make([]string, 0, len(asSlice))
			seen := make(map[string]struct{}, len(asSlice))
			for _, entry := range asSlice {
				asString, stringOK := entry.(string)
				asString = strings.TrimSpace(asString)
				if !stringOK || asString == "" {
					continue
				}
				if _, exists := seen[asString]; exists {
					continue
				}
				seen[asString] = struct{}{}
				keys = append(keys, asString)
			}
			if len(keys) > 0 {
				profile.PreferredEquipmentKeys = keys
			}
		}
	}

	return profile
}

func ReadNutritionProfile(raw []byte) NutritionProfile {
	preferences := decodePreferences(raw)
	rawProfile, ok := preferences["nutrition_profile"]
	if !ok {
		return NutritionProfile{}
	}

	asMap, ok := rawProfile.(map[string]any)
	if !ok {
		return NutritionProfile{}
	}

	profile := NutritionProfile{}
	if value, ok := asMap["dietary_restrictions"]; ok {
		if restrictions := normalizeRestrictionEntries(value); len(restrictions) > 0 {
			profile.DietaryRestrictions = restrictions
		}
	}
	if value, ok := asMap["meal_preference"]; ok {
		if mealPreference, ok := value.(map[string]any); ok {
			if cuisines := normalizeSlugEntries(mealPreference["cuisine_preferences"]); len(cuisines) > 0 {
				profile.MealPreference = MealPreference{CuisinePreferences: cuisines}
			}
		}
	}
	if value, ok := asMap["budget_preference"]; ok {
		asString, stringOK := value.(string)
		asString = strings.TrimSpace(asString)
		if stringOK && isValidBudgetPreference(asString) {
			profile.BudgetPreference = &asString
		}
	}
	if value, ok := asMap["cooking_capability"]; ok {
		asString, stringOK := value.(string)
		asString = strings.TrimSpace(asString)
		if stringOK && isValidCookingCapability(asString) {
			profile.CookingCapability = &asString
		}
	}

	return profile
}

func decodePreferences(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return map[string]any{}
	}

	asMap, ok := decoded.(map[string]any)
	if !ok {
		return map[string]any{}
	}

	return asMap
}

func parsePositiveWholeNumber(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		asInt := int(typed)
		if float64(asInt) != typed {
			return 0, false
		}
		return asInt, true
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func isValidExperienceLevel(value string) bool {
	switch value {
	case "beginner", "intermediate", "advanced":
		return true
	default:
		return false
	}
}

func isValidDietaryRestriction(value string) bool {
	switch value {
	case "vegetarian", "vegan", "pescatarian", "halal", "kosher", "gluten_free", "dairy_free", "nut_free", "shellfish_free", "egg_free", "soy_free":
		return true
	default:
		return false
	}
}

func isValidBudgetPreference(value string) bool {
	switch value {
	case "budget_constrained", "moderate", "flexible":
		return true
	default:
		return false
	}
}

func isValidCookingCapability(value string) bool {
	switch value {
	case "no_kitchen", "microwave_only", "basic_kitchen", "full_kitchen":
		return true
	default:
		return false
	}
}

func normalizeRestrictionEntries(raw any) []string {
	asSlice, ok := raw.([]any)
	if !ok {
		return nil
	}

	seen := make(map[string]struct{}, len(asSlice))
	result := make([]string, 0, len(asSlice))
	for _, entry := range asSlice {
		asString, stringOK := entry.(string)
		asString = strings.TrimSpace(asString)
		if !stringOK || !isValidDietaryRestriction(asString) {
			continue
		}
		if _, exists := seen[asString]; exists {
			continue
		}
		seen[asString] = struct{}{}
		result = append(result, asString)
	}

	if hasConflictingDietPatterns(result) {
		return nil
	}

	return result
}

func normalizeSlugEntries(raw any) []string {
	asSlice, ok := raw.([]any)
	if !ok {
		return nil
	}

	seen := make(map[string]struct{}, len(asSlice))
	result := make([]string, 0, len(asSlice))
	for _, entry := range asSlice {
		asString, stringOK := entry.(string)
		asString = strings.TrimSpace(asString)
		if !stringOK || !goalKeyPattern.MatchString(asString) {
			continue
		}
		if _, exists := seen[asString]; exists {
			continue
		}
		seen[asString] = struct{}{}
		result = append(result, asString)
	}

	return result
}

func hasConflictingDietPatterns(values []string) bool {
	patterns := make(map[string]struct{}, 3)
	for _, value := range values {
		switch value {
		case "vegetarian", "vegan", "pescatarian":
			patterns[value] = struct{}{}
		}
	}

	return len(patterns) > 1
}
