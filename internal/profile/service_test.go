package profile

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

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

func TestGetProfileDefaultsPredictablyWhenPreferencesAreSparse(t *testing.T) {
	service := NewService(&stubRepository{
		userToReturn: &store.ApolloUser{
			ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			StudentID:   "student-001",
			DisplayName: "student-001",
			Email:       "student@example.com",
			Preferences: []byte(`{}`),
		},
	})

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
			})

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
	service := NewService(repository)

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

func stringPtr(value string) *string {
	return &value
}
