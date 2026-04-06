package eligibility

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/profile"
	"github.com/ixxet/apollo/internal/store"
)

const (
	ReasonEligible                = "eligible"
	ReasonAvailabilityUnavailable = "availability_unavailable"
	ReasonAvailabilityWithTeam    = "availability_with_team"
	ReasonVisibilityGhost         = "visibility_ghost"
	ReasonInvalidVisibilityMode   = "invalid_visibility_mode"
	ReasonInvalidAvailabilityMode = "invalid_availability_mode"
)

var ErrNotFound = errors.New("member not found")

type Finder interface {
	GetUserByID(ctx context.Context, userID uuid.UUID) (*store.ApolloUser, error)
}

type Service struct {
	repository Finder
}

type LobbyEligibility struct {
	Eligible         bool   `json:"eligible"`
	Reason           string `json:"reason"`
	VisibilityMode   string `json:"visibility_mode"`
	AvailabilityMode string `json:"availability_mode"`
}

func NewService(repository Finder) *Service {
	return &Service{repository: repository}
}

func (s *Service) GetLobbyEligibility(ctx context.Context, userID uuid.UUID) (LobbyEligibility, error) {
	user, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return LobbyEligibility{}, err
	}
	if user == nil {
		return LobbyEligibility{}, ErrNotFound
	}

	return FromPreferenceModes(profile.ReadPreferenceModes(user.Preferences)), nil
}

func FromPreferenceModes(modes profile.PreferenceModes) LobbyEligibility {
	eligibility := LobbyEligibility{
		Eligible:         false,
		VisibilityMode:   modes.VisibilityMode,
		AvailabilityMode: modes.AvailabilityMode,
	}

	switch {
	case modes.InvalidAvailabilityMode:
		eligibility.Reason = ReasonInvalidAvailabilityMode
	case modes.InvalidVisibilityMode:
		eligibility.Reason = ReasonInvalidVisibilityMode
	case modes.AvailabilityMode == profile.AvailabilityModeUnavailable:
		eligibility.Reason = ReasonAvailabilityUnavailable
	case modes.AvailabilityMode == profile.AvailabilityModeWithTeam:
		eligibility.Reason = ReasonAvailabilityWithTeam
	case modes.VisibilityMode == profile.VisibilityModeGhost:
		eligibility.Reason = ReasonVisibilityGhost
	default:
		eligibility.Eligible = true
		eligibility.Reason = ReasonEligible
	}

	return eligibility
}
