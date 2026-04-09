package sports

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	ErrSportNotFound    = errors.New("sport not found")
	ErrFacilityNotFound = errors.New("facility not found")
)

type Sport struct {
	SportKey                    string `json:"sport_key"`
	DisplayName                 string `json:"display_name"`
	Description                 string `json:"description"`
	CompetitionMode             string `json:"competition_mode"`
	SidesPerMatch               int    `json:"sides_per_match"`
	ParticipantsPerSideMin      int    `json:"participants_per_side_min"`
	ParticipantsPerSideMax      int    `json:"participants_per_side_max"`
	ScoringModel                string `json:"scoring_model"`
	DefaultMatchDurationMinutes int    `json:"default_match_duration_minutes"`
	RulesSummary                string `json:"rules_summary"`
}

type FacilityCapability struct {
	SportKey    string   `json:"sport_key"`
	FacilityKey string   `json:"facility_key"`
	ZoneKeys    []string `json:"zone_keys"`
}

type SportDetail struct {
	Sport
	FacilityCapabilities []FacilityCapability `json:"facility_capabilities"`
}

type CapabilityFilter struct {
	SportKey    string
	FacilityKey string
}

type CatalogStore interface {
	ListSports(ctx context.Context) ([]Sport, error)
	GetSportByKey(ctx context.Context, sportKey string) (*Sport, error)
	FacilityExists(ctx context.Context, facilityKey string) (bool, error)
	ListFacilityCapabilities(ctx context.Context) ([]FacilityCapability, error)
}

type Service struct {
	store CatalogStore
}

func NewService(store CatalogStore) *Service {
	return &Service{store: store}
}

func (s *Service) ListSports(ctx context.Context) ([]Sport, error) {
	return s.store.ListSports(ctx)
}

func (s *Service) GetSport(ctx context.Context, sportKey string) (SportDetail, error) {
	normalizedSportKey := strings.TrimSpace(sportKey)
	if normalizedSportKey == "" {
		return SportDetail{}, fmt.Errorf("sport_key is required")
	}

	sport, err := s.store.GetSportByKey(ctx, normalizedSportKey)
	if err != nil {
		return SportDetail{}, err
	}
	if sport == nil {
		return SportDetail{}, ErrSportNotFound
	}

	capabilities, err := s.store.ListFacilityCapabilities(ctx)
	if err != nil {
		return SportDetail{}, err
	}

	filtered := make([]FacilityCapability, 0, len(capabilities))
	for _, capability := range capabilities {
		if capability.SportKey != normalizedSportKey {
			continue
		}
		filtered = append(filtered, cloneCapability(capability))
	}

	return SportDetail{
		Sport:                cloneSport(*sport),
		FacilityCapabilities: filtered,
	}, nil
}

func (s *Service) ListFacilityCapabilities(ctx context.Context, filter CapabilityFilter) ([]FacilityCapability, error) {
	normalizedSportKey := strings.TrimSpace(filter.SportKey)
	normalizedFacilityKey := strings.TrimSpace(filter.FacilityKey)

	if normalizedSportKey != "" {
		sport, err := s.store.GetSportByKey(ctx, normalizedSportKey)
		if err != nil {
			return nil, err
		}
		if sport == nil {
			return nil, ErrSportNotFound
		}
	}
	if normalizedFacilityKey != "" {
		exists, err := s.store.FacilityExists(ctx, normalizedFacilityKey)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrFacilityNotFound
		}
	}

	capabilities, err := s.store.ListFacilityCapabilities(ctx)
	if err != nil {
		return nil, err
	}

	filtered := make([]FacilityCapability, 0, len(capabilities))
	for _, capability := range capabilities {
		if normalizedSportKey != "" && capability.SportKey != normalizedSportKey {
			continue
		}
		if normalizedFacilityKey != "" && capability.FacilityKey != normalizedFacilityKey {
			continue
		}
		filtered = append(filtered, cloneCapability(capability))
	}

	slices.SortFunc(filtered, func(left, right FacilityCapability) int {
		if left.SportKey != right.SportKey {
			if left.SportKey < right.SportKey {
				return -1
			}
			return 1
		}
		if left.FacilityKey != right.FacilityKey {
			if left.FacilityKey < right.FacilityKey {
				return -1
			}
			return 1
		}
		return 0
	})

	return filtered, nil
}

func cloneSport(value Sport) Sport {
	return value
}

func cloneCapability(value FacilityCapability) FacilityCapability {
	return FacilityCapability{
		SportKey:    value.SportKey,
		FacilityKey: value.FacilityKey,
		ZoneKeys:    slices.Clone(value.ZoneKeys),
	}
}
