package sports

import (
	"context"
	"strings"
	"testing"

	"github.com/ixxet/apollo/internal/testutil"
)

func TestRepositoryReturnsSeededSportSubstrateDeterministically(t *testing.T) {
	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if closeErr := postgresEnv.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	repository := NewRepository(postgresEnv.DB)

	firstSports, err := repository.ListSports(ctx)
	if err != nil {
		t.Fatalf("ListSports() first error = %v", err)
	}
	secondSports, err := repository.ListSports(ctx)
	if err != nil {
		t.Fatalf("ListSports() second error = %v", err)
	}

	if len(firstSports) != 2 {
		t.Fatalf("len(firstSports) = %d, want 2", len(firstSports))
	}
	if !equalSports(firstSports, secondSports) {
		t.Fatalf("sports output changed between runs: first=%#v second=%#v", firstSports, secondSports)
	}
	if firstSports[0].SportKey != "badminton" || firstSports[1].SportKey != "basketball" {
		t.Fatalf("sport order = %#v, want badminton then basketball", firstSports)
	}

	capabilities, err := repository.ListFacilityCapabilities(ctx)
	if err != nil {
		t.Fatalf("ListFacilityCapabilities() error = %v", err)
	}
	if len(capabilities) != 2 {
		t.Fatalf("len(capabilities) = %d, want 2", len(capabilities))
	}
	for _, capability := range capabilities {
		if capability.FacilityKey != "ashtonbee" {
			t.Fatalf("capability.FacilityKey = %q, want ashtonbee", capability.FacilityKey)
		}
		if len(capability.ZoneKeys) != 1 || capability.ZoneKeys[0] != "gym-floor" {
			t.Fatalf("capability.ZoneKeys = %#v, want [gym-floor]", capability.ZoneKeys)
		}
	}
}

func TestRepositoryFacilityExistsDistinguishesKnownAndUnknownFacilities(t *testing.T) {
	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if closeErr := postgresEnv.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	repository := NewRepository(postgresEnv.DB)

	ashtonbeeExists, err := repository.FacilityExists(ctx, "ashtonbee")
	if err != nil {
		t.Fatalf("FacilityExists(ashtonbee) error = %v", err)
	}
	if !ashtonbeeExists {
		t.Fatal("FacilityExists(ashtonbee) = false, want true")
	}

	morningsideExists, err := repository.FacilityExists(ctx, "morningside")
	if err != nil {
		t.Fatalf("FacilityExists(morningside) error = %v", err)
	}
	if !morningsideExists {
		t.Fatal("FacilityExists(morningside) = false, want true even without capability rows")
	}

	unknownExists, err := repository.FacilityExists(ctx, "unknown-facility")
	if err != nil {
		t.Fatalf("FacilityExists(unknown-facility) error = %v", err)
	}
	if unknownExists {
		t.Fatal("FacilityExists(unknown-facility) = true, want false")
	}
}

func TestSportSubstrateConstraintsRejectInvalidWritesClearly(t *testing.T) {
	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if closeErr := postgresEnv.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	tests := []struct {
		name    string
		sql     string
		wantErr string
	}{
		{
			name: "duplicate sport key",
			sql: `
INSERT INTO apollo.sports (
    sport_key,
    display_name,
    description,
    competition_mode,
    sides_per_match,
    participants_per_side_min,
    participants_per_side_max,
    scoring_model,
    default_match_duration_minutes,
    rules_summary
) VALUES (
    'badminton',
    'Badminton Duplicate',
    'Duplicate sport.',
    'head_to_head',
    2,
    1,
    2,
    'best_of_games',
    30,
    'Duplicate.'
)`,
			wantErr: "duplicate key value violates unique constraint",
		},
		{
			name: "invalid sport definition",
			sql: `
INSERT INTO apollo.sports (
    sport_key,
    display_name,
    description,
    competition_mode,
    sides_per_match,
    participants_per_side_min,
    participants_per_side_max,
    scoring_model,
    default_match_duration_minutes,
    rules_summary
) VALUES (
    'invalid-sport',
    'Invalid Sport',
    'Broken participant range.',
    'head_to_head',
    2,
    3,
    2,
    'running_score',
    30,
    'Broken.'
)`,
			wantErr: "participants_per_side_max",
		},
		{
			name:    "unknown facility mapping",
			sql:     `INSERT INTO apollo.sport_facility_capabilities (sport_key, facility_key) VALUES ('badminton', 'unknown-facility')`,
			wantErr: "sport_facility_capabilities_facility_key_fkey",
		},
		{
			name:    "unknown sport ownership",
			sql:     `INSERT INTO apollo.sport_facility_capabilities (sport_key, facility_key) VALUES ('unknown-sport', 'ashtonbee')`,
			wantErr: "sport_facility_capabilities_sport_key_fkey",
		},
		{
			name:    "unknown facility zone mapping",
			sql:     `INSERT INTO apollo.sport_facility_capability_zones (sport_key, facility_key, zone_key) VALUES ('badminton', 'ashtonbee', 'weight-room')`,
			wantErr: "sport_facility_capability_zones_facility_key_zone_key_fkey",
		},
		{
			name:    "zone mapping without owning capability",
			sql:     `INSERT INTO apollo.sport_facility_capability_zones (sport_key, facility_key, zone_key) VALUES ('badminton', 'morningside', 'weight-room')`,
			wantErr: "sport_facility_capability_zones_sport_key_facility_key_fkey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := postgresEnv.DB.Exec(ctx, tt.sql)
			if err == nil {
				t.Fatal("Exec() error = nil, want constraint failure")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Exec() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func equalSports(left, right []Sport) bool {
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}
