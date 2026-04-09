package sports

import (
	"context"
	"errors"
	"testing"
)

type stubCatalogStore struct {
	sports       []Sport
	facilityKeys []string
	capabilities []FacilityCapability
	err          error
}

func (s stubCatalogStore) ListSports(context.Context) ([]Sport, error) {
	if s.err != nil {
		return nil, s.err
	}

	return slicesCloneSports(s.sports), nil
}

func (s stubCatalogStore) GetSportByKey(_ context.Context, sportKey string) (*Sport, error) {
	if s.err != nil {
		return nil, s.err
	}

	for _, sport := range s.sports {
		if sport.SportKey == sportKey {
			copy := sport
			return &copy, nil
		}
	}

	return nil, nil
}

func (s stubCatalogStore) FacilityExists(_ context.Context, facilityKey string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}

	for _, key := range s.facilityKeys {
		if key == facilityKey {
			return true, nil
		}
	}

	return false, nil
}

func (s stubCatalogStore) ListFacilityCapabilities(context.Context) ([]FacilityCapability, error) {
	if s.err != nil {
		return nil, s.err
	}

	return slicesCloneCapabilities(s.capabilities), nil
}

func TestGetSportReturnsCapabilitiesForRequestedSportOnly(t *testing.T) {
	svc := NewService(stubCatalogStore{
		sports: []Sport{
			{SportKey: "badminton", DisplayName: "Badminton"},
			{SportKey: "basketball", DisplayName: "Basketball"},
		},
		capabilities: []FacilityCapability{
			{SportKey: "basketball", FacilityKey: "ashtonbee", ZoneKeys: []string{"gym-floor"}},
			{SportKey: "badminton", FacilityKey: "ashtonbee", ZoneKeys: []string{"gym-floor"}},
		},
	})

	detail, err := svc.GetSport(context.Background(), "badminton")
	if err != nil {
		t.Fatalf("GetSport() error = %v", err)
	}

	if detail.SportKey != "badminton" {
		t.Fatalf("detail.SportKey = %q, want badminton", detail.SportKey)
	}
	if len(detail.FacilityCapabilities) != 1 {
		t.Fatalf("len(detail.FacilityCapabilities) = %d, want 1", len(detail.FacilityCapabilities))
	}
	if detail.FacilityCapabilities[0].SportKey != "badminton" {
		t.Fatalf("detail.FacilityCapabilities[0].SportKey = %q, want badminton", detail.FacilityCapabilities[0].SportKey)
	}
}

func TestGetSportReturnsNotFoundForUnknownSport(t *testing.T) {
	svc := NewService(stubCatalogStore{})

	_, err := svc.GetSport(context.Background(), "pickleball")
	if !errors.Is(err, ErrSportNotFound) {
		t.Fatalf("GetSport() error = %v, want ErrSportNotFound", err)
	}
}

func TestListFacilityCapabilitiesRejectsUnknownSportFilter(t *testing.T) {
	svc := NewService(stubCatalogStore{
		sports: []Sport{
			{SportKey: "badminton", DisplayName: "Badminton"},
		},
		facilityKeys: []string{"ashtonbee"},
	})

	_, err := svc.ListFacilityCapabilities(context.Background(), CapabilityFilter{SportKey: "basketball"})
	if !errors.Is(err, ErrSportNotFound) {
		t.Fatalf("ListFacilityCapabilities() error = %v, want ErrSportNotFound", err)
	}
}

func TestListFacilityCapabilitiesRejectsUnknownFacilityFilter(t *testing.T) {
	svc := NewService(stubCatalogStore{
		sports: []Sport{
			{SportKey: "badminton", DisplayName: "Badminton"},
		},
		facilityKeys: []string{"ashtonbee", "morningside"},
	})

	_, err := svc.ListFacilityCapabilities(context.Background(), CapabilityFilter{FacilityKey: "unknown-facility"})
	if !errors.Is(err, ErrFacilityNotFound) {
		t.Fatalf("ListFacilityCapabilities() error = %v, want ErrFacilityNotFound", err)
	}
}

func TestListFacilityCapabilitiesAllowsKnownFacilityFilterWithNoCapabilities(t *testing.T) {
	svc := NewService(stubCatalogStore{
		sports: []Sport{
			{SportKey: "badminton", DisplayName: "Badminton"},
		},
		facilityKeys: []string{"ashtonbee", "morningside"},
		capabilities: []FacilityCapability{
			{SportKey: "badminton", FacilityKey: "ashtonbee", ZoneKeys: []string{"gym-floor"}},
		},
	})

	capabilities, err := svc.ListFacilityCapabilities(context.Background(), CapabilityFilter{FacilityKey: "morningside"})
	if err != nil {
		t.Fatalf("ListFacilityCapabilities() error = %v", err)
	}
	if len(capabilities) != 0 {
		t.Fatalf("len(capabilities) = %d, want 0", len(capabilities))
	}
}

func TestListFacilityCapabilitiesStaysDeterministicAcrossRepeatedCalls(t *testing.T) {
	store := stubCatalogStore{
		sports: []Sport{
			{SportKey: "badminton", DisplayName: "Badminton"},
			{SportKey: "basketball", DisplayName: "Basketball"},
		},
		facilityKeys: []string{"ashtonbee", "morningside"},
		capabilities: []FacilityCapability{
			{SportKey: "basketball", FacilityKey: "ashtonbee", ZoneKeys: []string{"gym-floor"}},
			{SportKey: "badminton", FacilityKey: "ashtonbee", ZoneKeys: []string{"gym-floor"}},
		},
	}
	svc := NewService(store)

	first, err := svc.ListFacilityCapabilities(context.Background(), CapabilityFilter{})
	if err != nil {
		t.Fatalf("ListFacilityCapabilities() first error = %v", err)
	}
	second, err := svc.ListFacilityCapabilities(context.Background(), CapabilityFilter{})
	if err != nil {
		t.Fatalf("ListFacilityCapabilities() second error = %v", err)
	}

	if !equalCapabilities(first, second) {
		t.Fatalf("capability output changed between runs: first=%#v second=%#v", first, second)
	}
	if len(first) != 2 {
		t.Fatalf("len(first) = %d, want 2", len(first))
	}
	if first[0].SportKey != "badminton" || first[1].SportKey != "basketball" {
		t.Fatalf("first sport order = %#v, want badminton then basketball", first)
	}
}

func slicesCloneSports(values []Sport) []Sport {
	cloned := make([]Sport, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, cloneSport(value))
	}
	return cloned
}

func slicesCloneCapabilities(values []FacilityCapability) []FacilityCapability {
	cloned := make([]FacilityCapability, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, cloneCapability(value))
	}
	return cloned
}

func equalCapabilities(left, right []FacilityCapability) bool {
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if left[index].SportKey != right[index].SportKey {
			return false
		}
		if left[index].FacilityKey != right[index].FacilityKey {
			return false
		}
		if len(left[index].ZoneKeys) != len(right[index].ZoneKeys) {
			return false
		}
		for zoneIndex := range left[index].ZoneKeys {
			if left[index].ZoneKeys[zoneIndex] != right[index].ZoneKeys[zoneIndex] {
				return false
			}
		}
	}

	return true
}
