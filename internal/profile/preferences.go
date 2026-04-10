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
	PreferredEquipmentKeys []string `json:"preferred_equipment_keys,omitempty"`
}

type CoachingProfileInput struct {
	GoalKey                *string   `json:"goal_key"`
	DaysPerWeek            *int      `json:"days_per_week"`
	SessionMinutes         *int      `json:"session_minutes"`
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
