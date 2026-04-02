package profile

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"

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
	ErrEmptyPatch              = errors.New("profile patch is empty")
)

type Finder interface {
	GetUserByID(ctx context.Context, userID uuid.UUID) (*store.ApolloUser, error)
	UpdatePreferences(ctx context.Context, userID uuid.UUID, preferences []byte) (*store.ApolloUser, error)
}

type Service struct {
	repository Finder
}

type MemberProfile struct {
	UserID           uuid.UUID `json:"user_id"`
	StudentID        string    `json:"student_id"`
	DisplayName      string    `json:"display_name"`
	Email            string    `json:"email"`
	EmailVerified    bool      `json:"email_verified"`
	VisibilityMode   string    `json:"visibility_mode"`
	AvailabilityMode string    `json:"availability_mode"`
}

type UpdateInput struct {
	VisibilityMode   *string `json:"visibility_mode"`
	AvailabilityMode *string `json:"availability_mode"`
}

func NewService(repository Finder) *Service {
	return &Service{repository: repository}
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
	if input.VisibilityMode == nil && input.AvailabilityMode == nil {
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
