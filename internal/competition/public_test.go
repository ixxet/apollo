package competition

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestPublicCompetitionReadinessUsesExplicitPublicContract(t *testing.T) {
	svc := NewService(stubStore{
		publicReadiness: func(context.Context) (publicCompetitionReadinessRecord, error) {
			return publicCompetitionReadinessRecord{
				AvailableLeaderboards:     3,
				AvailableCanonicalResults: 2,
			}, nil
		},
	})

	readiness, err := svc.PublicCompetitionReadiness(context.Background())
	if err != nil {
		t.Fatalf("PublicCompetitionReadiness() error = %v", err)
	}
	if readiness.ContractVersion != publicCompetitionContractVersion {
		t.Fatalf("ContractVersion = %q, want %q", readiness.ContractVersion, publicCompetitionContractVersion)
	}
	if readiness.ProjectionVersion != competitionAnalyticsProjectionVersion {
		t.Fatalf("ProjectionVersion = %q, want %q", readiness.ProjectionVersion, competitionAnalyticsProjectionVersion)
	}
	if readiness.ResultSource != publicCompetitionResultSource {
		t.Fatalf("ResultSource = %q, want %q", readiness.ResultSource, publicCompetitionResultSource)
	}
	if readiness.RatingSource != publicCompetitionRatingSource {
		t.Fatalf("RatingSource = %q, want %q", readiness.RatingSource, publicCompetitionRatingSource)
	}
	if readiness.Status != publicCompetitionStatusAvailable {
		t.Fatalf("Status = %q, want %q", readiness.Status, publicCompetitionStatusAvailable)
	}
	if !slices.Contains(readiness.Deferred, "cp") || !slices.Contains(readiness.Deferred, "rating_read_path_switch") {
		t.Fatalf("Deferred = %#v, want public competition deferments", readiness.Deferred)
	}
}

func TestListPublicCompetitionLeaderboardNormalizesAndRedactsParticipants(t *testing.T) {
	computedAt := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	var captured PublicCompetitionLeaderboardInput
	svc := NewService(stubStore{
		publicLeaderboard: func(_ context.Context, input PublicCompetitionLeaderboardInput) ([]publicCompetitionLeaderboardRowRecord, error) {
			captured = input
			return []publicCompetitionLeaderboardRowRecord{{
				SportKey:    "badminton",
				ModeKey:     "head_to_head:s2-p1",
				FacilityKey: "gym-floor",
				TeamScope:   analyticsDimensionAll,
				StatType:    analyticsStatWins,
				StatValue:   4,
				SampleSize:  4,
				Confidence:  1,
				ComputedAt:  computedAt,
			}}, nil
		},
	})

	leaderboard, err := svc.ListPublicCompetitionLeaderboard(context.Background(), PublicCompetitionLeaderboardInput{
		StatType:  "openskill_delta",
		TeamScope: "private_scope",
		Limit:     1000,
	})
	if err != nil {
		t.Fatalf("ListPublicCompetitionLeaderboard() error = %v", err)
	}
	if captured.StatType != analyticsStatWins {
		t.Fatalf("captured.StatType = %q, want %q", captured.StatType, analyticsStatWins)
	}
	if captured.TeamScope != analyticsDimensionAll {
		t.Fatalf("captured.TeamScope = %q, want %q", captured.TeamScope, analyticsDimensionAll)
	}
	if captured.Limit != maxPublicLeaderboardLimit {
		t.Fatalf("captured.Limit = %d, want %d", captured.Limit, maxPublicLeaderboardLimit)
	}
	if got, want := len(leaderboard.Leaderboard), 1; got != want {
		t.Fatalf("len(leaderboard.Leaderboard) = %d, want %d", got, want)
	}
	row := leaderboard.Leaderboard[0]
	if row.Participant != "participant_1" {
		t.Fatalf("Participant = %q, want redacted participant label", row.Participant)
	}
	if row.StatType != analyticsStatWins || row.StatValue != 4 {
		t.Fatalf("row = %+v, want wins projection", row)
	}
}
