package eligibility

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/profile"
	"github.com/ixxet/apollo/internal/store"
)

type stubRepository struct {
	userToReturn *store.ApolloUser
	errToReturn  error
}

func (s *stubRepository) GetUserByID(context.Context, uuid.UUID) (*store.ApolloUser, error) {
	if s.errToReturn != nil {
		return nil, s.errToReturn
	}

	return s.userToReturn, nil
}

func TestGetLobbyEligibilityEvaluatesTheMemberStateMatrix(t *testing.T) {
	testCases := []struct {
		name                 string
		preferences          string
		wantEligible         bool
		wantReason           string
		wantVisibilityMode   string
		wantAvailabilityMode string
	}{
		{
			name:                 "discoverable plus available now is eligible",
			preferences:          `{"visibility_mode":"discoverable","availability_mode":"available_now"}`,
			wantEligible:         true,
			wantReason:           ReasonEligible,
			wantVisibilityMode:   profile.VisibilityModeDiscoverable,
			wantAvailabilityMode: profile.AvailabilityModeAvailableNow,
		},
		{
			name:                 "discoverable plus unavailable is ineligible",
			preferences:          `{"visibility_mode":"discoverable","availability_mode":"unavailable"}`,
			wantEligible:         false,
			wantReason:           ReasonAvailabilityUnavailable,
			wantVisibilityMode:   profile.VisibilityModeDiscoverable,
			wantAvailabilityMode: profile.AvailabilityModeUnavailable,
		},
		{
			name:                 "discoverable plus with team is ineligible for open lobby",
			preferences:          `{"visibility_mode":"discoverable","availability_mode":"with_team"}`,
			wantEligible:         false,
			wantReason:           ReasonAvailabilityWithTeam,
			wantVisibilityMode:   profile.VisibilityModeDiscoverable,
			wantAvailabilityMode: profile.AvailabilityModeWithTeam,
		},
		{
			name:                 "ghost plus available now stays ineligible",
			preferences:          `{"visibility_mode":"ghost","availability_mode":"available_now"}`,
			wantEligible:         false,
			wantReason:           ReasonVisibilityGhost,
			wantVisibilityMode:   profile.VisibilityModeGhost,
			wantAvailabilityMode: profile.AvailabilityModeAvailableNow,
		},
		{
			name:                 "invalid visibility value is deterministic",
			preferences:          `{"visibility_mode":"hidden","availability_mode":"available_now"}`,
			wantEligible:         false,
			wantReason:           ReasonInvalidVisibilityMode,
			wantVisibilityMode:   profile.VisibilityModeGhost,
			wantAvailabilityMode: profile.AvailabilityModeAvailableNow,
		},
		{
			name:                 "invalid availability value is deterministic",
			preferences:          `{"visibility_mode":"discoverable","availability_mode":"queued"}`,
			wantEligible:         false,
			wantReason:           ReasonInvalidAvailabilityMode,
			wantVisibilityMode:   profile.VisibilityModeDiscoverable,
			wantAvailabilityMode: profile.AvailabilityModeUnavailable,
		},
		{
			name:                 "missing preferences default predictably",
			preferences:          `{}`,
			wantEligible:         false,
			wantReason:           ReasonAvailabilityUnavailable,
			wantVisibilityMode:   profile.VisibilityModeGhost,
			wantAvailabilityMode: profile.AvailabilityModeUnavailable,
		},
		{
			name:                 "partial preferences still serialize predictably",
			preferences:          `{"availability_mode":"available_now"}`,
			wantEligible:         false,
			wantReason:           ReasonVisibilityGhost,
			wantVisibilityMode:   profile.VisibilityModeGhost,
			wantAvailabilityMode: profile.AvailabilityModeAvailableNow,
		},
		{
			name:                 "non string persisted values are deterministic",
			preferences:          `{"visibility_mode":"discoverable","availability_mode":7}`,
			wantEligible:         false,
			wantReason:           ReasonInvalidAvailabilityMode,
			wantVisibilityMode:   profile.VisibilityModeDiscoverable,
			wantAvailabilityMode: profile.AvailabilityModeUnavailable,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := NewService(&stubRepository{
				userToReturn: &store.ApolloUser{
					ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
					Preferences: []byte(testCase.preferences),
				},
			})

			got, err := service.GetLobbyEligibility(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111"))
			if err != nil {
				t.Fatalf("GetLobbyEligibility() error = %v", err)
			}
			if got.Eligible != testCase.wantEligible {
				t.Fatalf("Eligible = %v, want %v", got.Eligible, testCase.wantEligible)
			}
			if got.Reason != testCase.wantReason {
				t.Fatalf("Reason = %q, want %q", got.Reason, testCase.wantReason)
			}
			if got.VisibilityMode != testCase.wantVisibilityMode {
				t.Fatalf("VisibilityMode = %q, want %q", got.VisibilityMode, testCase.wantVisibilityMode)
			}
			if got.AvailabilityMode != testCase.wantAvailabilityMode {
				t.Fatalf("AvailabilityMode = %q, want %q", got.AvailabilityMode, testCase.wantAvailabilityMode)
			}
		})
	}
}

func TestGetLobbyEligibilityPropagatesLookupFailuresAndMissingMembers(t *testing.T) {
	lookupErr := errors.New("boom")
	serviceWithError := NewService(&stubRepository{errToReturn: lookupErr})

	if _, err := serviceWithError.GetLobbyEligibility(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111")); !errors.Is(err, lookupErr) {
		t.Fatalf("GetLobbyEligibility() error = %v, want %v", err, lookupErr)
	}

	serviceWithoutUser := NewService(&stubRepository{})
	if _, err := serviceWithoutUser.GetLobbyEligibility(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetLobbyEligibility() error = %v, want %v", err, ErrNotFound)
	}
}
