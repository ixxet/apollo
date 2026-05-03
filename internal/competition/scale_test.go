package competition

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/rating"
	"github.com/ixxet/apollo/internal/store"
)

func TestCompetitionScaleCeilingRatingRecomputeProof(t *testing.T) {
	rows := buildScaleRatingRows(maxRatingCanonicalResultsPerMode, maxRatingRatedParticipantsPerMode)
	measurement, err := measureRatingRebuildScale(rows)
	if err != nil {
		t.Fatalf("measureRatingRebuildScale() error = %v", err)
	}
	if measurement.ResultSideRows != maxRatingResultSideRows {
		t.Fatalf("ResultSideRows = %d, want %d", measurement.ResultSideRows, maxRatingResultSideRows)
	}
	if measurement.MaxCanonicalResultsPerMode != maxRatingCanonicalResultsPerMode {
		t.Fatalf("MaxCanonicalResultsPerMode = %d, want %d", measurement.MaxCanonicalResultsPerMode, maxRatingCanonicalResultsPerMode)
	}
	if measurement.MaxRatedParticipantsPerMode != maxRatingRatedParticipantsPerMode {
		t.Fatalf("MaxRatedParticipantsPerMode = %d, want %d", measurement.MaxRatedParticipantsPerMode, maxRatingRatedParticipantsPerMode)
	}

	matches := buildRatingMatches(rows)
	durations := measureScaleDurations(t, 5, func(t *testing.T) {
		projection := rating.RebuildLegacy(matches)
		comparison := rating.RebuildOpenSkillComparison(matches, projection)
		if got, want := len(projection.States), maxRatingRatedParticipantsPerMode; got != want {
			t.Fatalf("len(projection.States) = %d, want %d", got, want)
		}
		if got, want := len(comparison.Facts), maxRatingResultSideRows; got != want {
			t.Fatalf("len(comparison.Facts) = %d, want %d", got, want)
		}
	})
	p95 := percentileDuration(durations, 0.95)
	hardMax := maxDuration(durations)
	if p95 > ratingRecomputeP95Ceiling {
		t.Fatalf("rating recompute p95 = %s, ceiling %s", p95, ratingRecomputeP95Ceiling)
	}
	if hardMax > ratingRecomputeHardCeiling {
		t.Fatalf("rating recompute hard max = %s, ceiling %s", hardMax, ratingRecomputeHardCeiling)
	}

	t.Logf("scale_ceiling path=rating_recompute canonical_results_per_mode=%d result_side_rows=%d rated_participants_per_mode=%d samples=%d p95=%s hard_max=%s p95_ceiling=%s hard_ceiling=%s",
		measurement.MaxCanonicalResultsPerMode,
		measurement.ResultSideRows,
		measurement.MaxRatedParticipantsPerMode,
		len(durations),
		p95,
		hardMax,
		ratingRecomputeP95Ceiling,
		ratingRecomputeHardCeiling,
	)
}

func TestCompetitionScaleCeilingRatingRecomputeRejectsOversizeRows(t *testing.T) {
	rows := buildScaleRatingRows(maxRatingCanonicalResultsPerMode+1, maxRatingRatedParticipantsPerMode)
	measurement, err := measureRatingRebuildScale(rows)
	if !errors.Is(err, ErrRatingScaleCeilingExceeded) {
		t.Fatalf("measureRatingRebuildScale() error = %v, want ErrRatingScaleCeilingExceeded", err)
	}
	if measurement.ResultSideRows <= maxRatingResultSideRows {
		t.Fatalf("ResultSideRows = %d, want over ceiling %d", measurement.ResultSideRows, maxRatingResultSideRows)
	}
}

func TestCompetitionScaleCeilingRatingRecomputeRejectsOversizeParticipants(t *testing.T) {
	rows := buildScaleRatingRows(maxRatingCanonicalResultsPerMode, maxRatingRatedParticipantsPerMode+1)
	measurement, err := measureRatingRebuildScale(rows)
	if !errors.Is(err, ErrRatingScaleCeilingExceeded) {
		t.Fatalf("measureRatingRebuildScale() error = %v, want ErrRatingScaleCeilingExceeded", err)
	}
	if measurement.ResultSideRows != maxRatingResultSideRows {
		t.Fatalf("ResultSideRows = %d, want at ceiling %d", measurement.ResultSideRows, maxRatingResultSideRows)
	}
	if measurement.MaxRatedParticipantsPerMode <= maxRatingRatedParticipantsPerMode {
		t.Fatalf("MaxRatedParticipantsPerMode = %d, want over ceiling %d", measurement.MaxRatedParticipantsPerMode, maxRatingRatedParticipantsPerMode)
	}
}

func TestCompetitionScaleCeilingPublicProjectionProof(t *testing.T) {
	leaderboardRows := buildScaleLeaderboardRows(maxPublicLeaderboardRowsScanned)
	gameIdentityRows := buildScaleGameIdentityRows(maxGameIdentityParticipantContextRows)
	svc := NewService(stubStore{
		publicReadiness: func(context.Context) (publicCompetitionReadinessRecord, error) {
			return publicCompetitionReadinessRecord{
				AvailableLeaderboards:     len(leaderboardRows),
				AvailableCanonicalResults: maxRatingCanonicalResultsPerMode,
			}, nil
		},
		publicLeaderboard: func(context.Context, PublicCompetitionLeaderboardInput) ([]publicCompetitionLeaderboardRowRecord, error) {
			return leaderboardRows, nil
		},
		gameIdentityRows: func(context.Context, GameIdentityProjectionInput) ([]gameIdentityProjectionRowRecord, error) {
			return gameIdentityRows, nil
		},
	})

	readinessDurations := measureScaleDurations(t, 7, func(t *testing.T) {
		readiness, err := svc.PublicCompetitionReadiness(context.Background())
		if err != nil {
			t.Fatalf("PublicCompetitionReadiness() error = %v", err)
		}
		if readiness.AvailableLeaderboards != maxPublicLeaderboardRowsScanned {
			t.Fatalf("AvailableLeaderboards = %d, want %d", readiness.AvailableLeaderboards, maxPublicLeaderboardRowsScanned)
		}
	})
	readinessP95 := percentileDuration(readinessDurations, 0.95)
	readinessHardMax := maxDuration(readinessDurations)
	if readinessP95 > publicReadinessP95Ceiling {
		t.Fatalf("public readiness p95 = %s, ceiling %s", readinessP95, publicReadinessP95Ceiling)
	}
	if readinessHardMax > publicReadinessHardCeiling {
		t.Fatalf("public readiness hard max = %s, ceiling %s", readinessHardMax, publicReadinessHardCeiling)
	}

	leaderboardDurations := measureScaleDurations(t, 7, func(t *testing.T) {
		leaderboard, err := svc.ListPublicCompetitionLeaderboard(context.Background(), PublicCompetitionLeaderboardInput{Limit: maxPublicLeaderboardLimit + 1})
		if err != nil {
			t.Fatalf("ListPublicCompetitionLeaderboard() error = %v", err)
		}
		if got, want := len(leaderboard.Leaderboard), maxPublicLeaderboardLimit; got != want {
			t.Fatalf("len(leaderboard.Leaderboard) = %d, want response cap %d", got, want)
		}
	})
	leaderboardP95 := percentileDuration(leaderboardDurations, 0.95)
	leaderboardHardMax := maxDuration(leaderboardDurations)
	if leaderboardP95 > publicLeaderboardP95Ceiling {
		t.Fatalf("public leaderboard p95 = %s, ceiling %s", leaderboardP95, publicLeaderboardP95Ceiling)
	}
	if leaderboardHardMax > publicLeaderboardHardCeiling {
		t.Fatalf("public leaderboard hard max = %s, ceiling %s", leaderboardHardMax, publicLeaderboardHardCeiling)
	}

	gameIdentityDurations := measureScaleDurations(t, 7, func(t *testing.T) {
		identity, err := svc.PublicGameIdentity(context.Background(), PublicGameIdentityInput{Limit: maxGameIdentityLimit + 1})
		if err != nil {
			t.Fatalf("PublicGameIdentity() error = %v", err)
		}
		if got, want := len(identity.CP), maxGameIdentityLimit; got != want {
			t.Fatalf("len(identity.CP) = %d, want response cap %d", got, want)
		}
		candidateComparisons := candidatePairComparisons(len(identity.CP))
		if candidateComparisons > maxGameIdentityCandidateComparisons {
			t.Fatalf("candidate comparisons = %d, ceiling %d", candidateComparisons, maxGameIdentityCandidateComparisons)
		}
	})
	gameIdentityP95 := percentileDuration(gameIdentityDurations, 0.95)
	gameIdentityHardMax := maxDuration(gameIdentityDurations)
	if gameIdentityP95 > gameIdentityProjectionP95Ceiling {
		t.Fatalf("game identity p95 = %s, ceiling %s", gameIdentityP95, gameIdentityProjectionP95Ceiling)
	}
	if gameIdentityHardMax > gameIdentityProjectionHardCeiling {
		t.Fatalf("game identity hard max = %s, ceiling %s", gameIdentityHardMax, gameIdentityProjectionHardCeiling)
	}

	t.Logf("scale_ceiling path=public_readiness samples=%d p95=%s hard_max=%s p95_ceiling=%s hard_ceiling=%s",
		len(readinessDurations), readinessP95, readinessHardMax, publicReadinessP95Ceiling, publicReadinessHardCeiling)
	t.Logf("scale_ceiling path=public_leaderboard candidate_rows=%d response_rows=%d samples=%d p95=%s hard_max=%s p95_ceiling=%s hard_ceiling=%s",
		maxPublicLeaderboardRowsScanned, maxPublicLeaderboardLimit, len(leaderboardDurations), leaderboardP95, leaderboardHardMax, publicLeaderboardP95Ceiling, publicLeaderboardHardCeiling)
	t.Logf("scale_ceiling path=game_identity public_projection_rows_ceiling=%d participant_context_rows=%d response_rows=%d candidate_comparisons=%d samples=%d p95=%s hard_max=%s p95_ceiling=%s hard_ceiling=%s",
		maxGameIdentityPublicProjectionRows,
		maxGameIdentityParticipantContextRows,
		maxGameIdentityLimit,
		candidatePairComparisons(maxGameIdentityLimit),
		len(gameIdentityDurations),
		gameIdentityP95,
		gameIdentityHardMax,
		gameIdentityProjectionP95Ceiling,
		gameIdentityProjectionHardCeiling,
	)
}

func buildScaleRatingRows(resultCount int, participantCount int) []store.ListCompetitionRatingParticipantsBySportRow {
	base := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	rows := make([]store.ListCompetitionRatingParticipantsBySportRow, 0, resultCount*2)
	for index := range resultCount {
		matchID := scaleUUID(1, index)
		resultID := scaleUUID(2, index)
		leftTeamID := scaleUUID(3, index*2)
		rightTeamID := scaleUUID(3, index*2+1)
		leftUserID := scaleUUID(4, index%participantCount)
		rightUserID := scaleUUID(4, (index+1)%participantCount)
		recordedAt := pgtype.Timestamptz{Time: base.Add(time.Duration(index) * time.Second), Valid: true}
		rows = append(rows,
			store.ListCompetitionRatingParticipantsBySportRow{
				CompetitionMatchID:       matchID,
				CompetitionMatchResultID: resultID,
				SportKey:                 "badminton",
				CompetitionMode:          "head_to_head",
				SidesPerMatch:            2,
				ParticipantsPerSide:      1,
				RecordedAt:               recordedAt,
				CompetitionSessionTeamID: leftTeamID,
				SideIndex:                1,
				Outcome:                  matchOutcomeWin,
				UserID:                   leftUserID,
			},
			store.ListCompetitionRatingParticipantsBySportRow{
				CompetitionMatchID:       matchID,
				CompetitionMatchResultID: resultID,
				SportKey:                 "badminton",
				CompetitionMode:          "head_to_head",
				SidesPerMatch:            2,
				ParticipantsPerSide:      1,
				RecordedAt:               recordedAt,
				CompetitionSessionTeamID: rightTeamID,
				SideIndex:                2,
				Outcome:                  matchOutcomeLoss,
				UserID:                   rightUserID,
			},
		)
	}
	return rows
}

func buildScaleLeaderboardRows(count int) []publicCompetitionLeaderboardRowRecord {
	base := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	rows := make([]publicCompetitionLeaderboardRowRecord, 0, count)
	for index := range count {
		rows = append(rows, publicCompetitionLeaderboardRowRecord{
			SportKey:    "badminton",
			ModeKey:     "head_to_head:s2-p1",
			FacilityKey: "ashtonbee",
			TeamScope:   analyticsDimensionAll,
			StatType:    analyticsStatWins,
			StatValue:   float64(count - index),
			SampleSize:  count - index,
			Confidence:  1,
			ComputedAt:  base.Add(time.Duration(index) * time.Millisecond),
		})
	}
	return rows
}

func buildScaleGameIdentityRows(count int) []gameIdentityProjectionRowRecord {
	base := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	rows := make([]gameIdentityProjectionRowRecord, 0, count)
	for index := range count {
		rows = append(rows, gameIdentityProjectionRowRecord{
			UserID:        scaleUUID(5, index),
			SportKey:      "badminton",
			ModeKey:       "head_to_head:s2-p1",
			FacilityKey:   "ashtonbee",
			TeamScope:     analyticsDimensionAll,
			MatchesPlayed: float64(1 + (index % 11)),
			Wins:          float64(index % 7),
			Losses:        float64(index % 5),
			Draws:         float64(index % 3),
			ComputedAt:    base.Add(time.Duration(index) * time.Millisecond),
		})
	}
	return rows
}

func measureScaleDurations(t *testing.T, samples int, run func(*testing.T)) []time.Duration {
	t.Helper()
	durations := make([]time.Duration, 0, samples)
	for range samples {
		startedAt := time.Now()
		run(t)
		durations = append(durations, time.Since(startedAt))
	}
	return durations
}

func percentileDuration(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	index := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func maxDuration(values []time.Duration) time.Duration {
	var maxValue time.Duration
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func candidatePairComparisons(candidateCount int) int {
	if candidateCount < 2 {
		return 0
	}
	return (candidateCount * (candidateCount - 1)) / 2
}

func scaleUUID(namespace byte, value int) uuid.UUID {
	var id uuid.UUID
	id[0] = namespace
	binary.BigEndian.PutUint64(id[8:], uint64(value+1))
	return id
}
