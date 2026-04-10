package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/exercises"
	"github.com/ixxet/apollo/internal/store"
)

const (
	VisibilityModeGhost        = "ghost"
	VisibilityModeDiscoverable = "discoverable"

	AvailabilityModeUnavailable  = "unavailable"
	AvailabilityModeAvailableNow = "available_now"
	AvailabilityModeWithTeam     = "with_team"
)

var (
	ErrNotFound                = errors.New("profile not found")
	ErrInvalidVisibilityMode   = errors.New("invalid visibility_mode")
	ErrInvalidAvailabilityMode = errors.New("invalid availability_mode")
	ErrInvalidGoalKey          = errors.New("invalid coaching_profile.goal_key")
	ErrInvalidDaysPerWeek      = errors.New("invalid coaching_profile.days_per_week")
	ErrInvalidSessionMinutes   = errors.New("invalid coaching_profile.session_minutes")
	ErrInvalidEquipmentKeys    = errors.New("invalid coaching_profile.preferred_equipment_keys")
	ErrEmptyPatch              = errors.New("profile patch is empty")
)

type Finder interface {
	GetUserByID(ctx context.Context, userID uuid.UUID) (*store.ApolloUser, error)
	UpdatePreferences(ctx context.Context, userID uuid.UUID, preferences []byte) (*store.ApolloUser, error)
}

type Service struct {
	repository        Finder
	equipmentResolver EquipmentResolver
}

type MemberProfile struct {
	UserID           uuid.UUID       `json:"user_id"`
	StudentID        string          `json:"student_id"`
	DisplayName      string          `json:"display_name"`
	Email            string          `json:"email"`
	EmailVerified    bool            `json:"email_verified"`
	VisibilityMode   string          `json:"visibility_mode"`
	AvailabilityMode string          `json:"availability_mode"`
	CoachingProfile  CoachingProfile `json:"coaching_profile"`
}

type UpdateInput struct {
	VisibilityMode   *string               `json:"visibility_mode"`
	AvailabilityMode *string               `json:"availability_mode"`
	CoachingProfile  *CoachingProfileInput `json:"coaching_profile"`
}

type EquipmentResolver interface {
	ResolveEquipment(ctx context.Context, keys []string) (map[string]exercises.EquipmentRef, error)
}

func NewService(repository Finder, equipmentResolver EquipmentResolver) *Service {
	return &Service{
		repository:        repository,
		equipmentResolver: equipmentResolver,
	}
}

func (s *Service) GetProfile(ctx context.Context, userID uuid.UUID) (MemberProfile, error) {
	user, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return MemberProfile{}, err
	}
	if user == nil {
		return MemberProfile{}, ErrNotFound
	}

	return buildProfile(*user), nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, input UpdateInput) (MemberProfile, error) {
	if input.VisibilityMode == nil && input.AvailabilityMode == nil && !hasCoachingProfileUpdates(input.CoachingProfile) {
		return MemberProfile{}, ErrEmptyPatch
	}

	user, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return MemberProfile{}, err
	}
	if user == nil {
		return MemberProfile{}, ErrNotFound
	}

	preferences := decodePreferences(user.Preferences)
	if input.VisibilityMode != nil {
		normalizedVisibility := strings.TrimSpace(*input.VisibilityMode)
		if !isValidVisibilityMode(normalizedVisibility) {
			return MemberProfile{}, ErrInvalidVisibilityMode
		}
		preferences["visibility_mode"] = normalizedVisibility
	}
	if input.AvailabilityMode != nil {
		normalizedAvailability := strings.TrimSpace(*input.AvailabilityMode)
		if !isValidAvailabilityMode(normalizedAvailability) {
			return MemberProfile{}, ErrInvalidAvailabilityMode
		}
		preferences["availability_mode"] = normalizedAvailability
	}
	if hasCoachingProfileUpdates(input.CoachingProfile) {
		updatedProfile, err := s.mergeCoachingProfile(ctx, preferences["coaching_profile"], *input.CoachingProfile)
		if err != nil {
			return MemberProfile{}, err
		}
		preferences["coaching_profile"] = updatedProfile
	}

	encodedPreferences, err := json.Marshal(preferences)
	if err != nil {
		return MemberProfile{}, err
	}

	updatedUser, err := s.repository.UpdatePreferences(ctx, userID, encodedPreferences)
	if err != nil {
		return MemberProfile{}, err
	}

	return buildProfile(*updatedUser), nil
}

func buildProfile(user store.ApolloUser) MemberProfile {
	modes := ReadPreferenceModes(user.Preferences)

	return MemberProfile{
		UserID:           user.ID,
		StudentID:        user.StudentID,
		DisplayName:      user.DisplayName,
		Email:            user.Email,
		EmailVerified:    user.EmailVerifiedAt.Valid,
		VisibilityMode:   modes.VisibilityMode,
		AvailabilityMode: modes.AvailabilityMode,
		CoachingProfile:  ReadCoachingProfile(user.Preferences),
	}
}

func isValidVisibilityMode(value string) bool {
	switch value {
	case VisibilityModeGhost, VisibilityModeDiscoverable:
		return true
	default:
		return false
	}
}

func isValidAvailabilityMode(value string) bool {
	switch value {
	case AvailabilityModeUnavailable, AvailabilityModeAvailableNow, AvailabilityModeWithTeam:
		return true
	default:
		return false
	}
}

func hasCoachingProfileUpdates(input *CoachingProfileInput) bool {
	if input == nil {
		return false
	}
	return input.GoalKey != nil || input.DaysPerWeek != nil || input.SessionMinutes != nil || input.PreferredEquipmentKeys != nil
}

func (s *Service) mergeCoachingProfile(ctx context.Context, current any, input CoachingProfileInput) (map[string]any, error) {
	merged := map[string]any{}
	if currentMap, ok := current.(map[string]any); ok {
		for key, value := range currentMap {
			merged[key] = value
		}
	}

	if input.GoalKey != nil {
		goalKey := strings.TrimSpace(*input.GoalKey)
		if !goalKeyPattern.MatchString(goalKey) {
			return nil, ErrInvalidGoalKey
		}
		merged["goal_key"] = goalKey
	}
	if input.DaysPerWeek != nil {
		if *input.DaysPerWeek < 1 || *input.DaysPerWeek > 7 {
			return nil, ErrInvalidDaysPerWeek
		}
		merged["days_per_week"] = *input.DaysPerWeek
	}
	if input.SessionMinutes != nil {
		if *input.SessionMinutes <= 0 {
			return nil, ErrInvalidSessionMinutes
		}
		merged["session_minutes"] = *input.SessionMinutes
	}
	if input.PreferredEquipmentKeys != nil {
		if s.equipmentResolver == nil {
			return nil, fmt.Errorf("equipment resolver is required")
		}
		normalizedKeys := make([]string, 0, len(*input.PreferredEquipmentKeys))
		seen := make(map[string]struct{}, len(*input.PreferredEquipmentKeys))
		for _, key := range *input.PreferredEquipmentKeys {
			trimmed := strings.TrimSpace(key)
			if trimmed == "" {
				return nil, ErrInvalidEquipmentKeys
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			normalizedKeys = append(normalizedKeys, trimmed)
		}

		resolved, err := s.equipmentResolver.ResolveEquipment(ctx, normalizedKeys)
		if err != nil {
			return nil, err
		}
		if len(resolved) != len(normalizedKeys) {
			return nil, ErrInvalidEquipmentKeys
		}
		merged["preferred_equipment_keys"] = normalizedKeys
	}

	return merged, nil
}
