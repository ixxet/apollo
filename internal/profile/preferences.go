package profile

import "encoding/json"

type PreferenceModes struct {
	VisibilityMode          string
	AvailabilityMode        string
	InvalidVisibilityMode   bool
	InvalidAvailabilityMode bool
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
