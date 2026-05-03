package competition

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/store"
)

const (
	maxRatingCanonicalResultsPerMode  = 10000
	maxRatingResultSideRows           = 20000
	maxRatingRatedParticipantsPerMode = 2500

	ratingRecomputeP95Ceiling  = 3 * time.Second
	ratingRecomputeHardCeiling = 5 * time.Second

	publicReadinessP95Ceiling  = 150 * time.Millisecond
	publicReadinessHardCeiling = 300 * time.Millisecond

	maxPublicLeaderboardRowsScanned = 5000
	publicLeaderboardP95Ceiling     = 300 * time.Millisecond
	publicLeaderboardHardCeiling    = 750 * time.Millisecond

	maxGameIdentityPublicProjectionRows   = 10000
	maxGameIdentityParticipantContextRows = 2500
	maxGameIdentityCandidateComparisons   = 25000
	gameIdentityProjectionP95Ceiling      = 500 * time.Millisecond
	gameIdentityProjectionHardCeiling     = time.Second
	competitionSmokeSequenceHardCeiling   = 30 * time.Second
)

var ErrRatingScaleCeilingExceeded = errors.New("rating scale ceiling exceeded")

type ratingRebuildScaleMeasurement struct {
	ResultSideRows              int
	MaxCanonicalResultsPerMode  int
	MaxRatedParticipantsPerMode int
}

func measureRatingRebuildScale(rows []store.ListCompetitionRatingParticipantsBySportRow) (ratingRebuildScaleMeasurement, error) {
	measurement := ratingRebuildScaleMeasurement{
		ResultSideRows: len(rows),
	}
	if len(rows) > maxRatingResultSideRows {
		return measurement, fmt.Errorf("%w: result-side rows %d exceed ceiling %d", ErrRatingScaleCeilingExceeded, len(rows), maxRatingResultSideRows)
	}

	resultsByMode := make(map[string]map[uuid.UUID]struct{})
	participantsByMode := make(map[string]map[uuid.UUID]struct{})
	for _, row := range rows {
		modeKey := buildModeKey(row.CompetitionMode, int(row.SidesPerMatch), int(row.ParticipantsPerSide))
		if _, ok := resultsByMode[modeKey]; !ok {
			resultsByMode[modeKey] = make(map[uuid.UUID]struct{})
		}
		resultsByMode[modeKey][row.CompetitionMatchResultID] = struct{}{}

		if _, ok := participantsByMode[modeKey]; !ok {
			participantsByMode[modeKey] = make(map[uuid.UUID]struct{})
		}
		participantsByMode[modeKey][row.UserID] = struct{}{}
	}

	for modeKey, resultIDs := range resultsByMode {
		count := len(resultIDs)
		if count > measurement.MaxCanonicalResultsPerMode {
			measurement.MaxCanonicalResultsPerMode = count
		}
		if count > maxRatingCanonicalResultsPerMode {
			return measurement, fmt.Errorf("%w: canonical results for mode %q %d exceed ceiling %d", ErrRatingScaleCeilingExceeded, modeKey, count, maxRatingCanonicalResultsPerMode)
		}
	}

	for modeKey, userIDs := range participantsByMode {
		count := len(userIDs)
		if count > measurement.MaxRatedParticipantsPerMode {
			measurement.MaxRatedParticipantsPerMode = count
		}
		if count > maxRatingRatedParticipantsPerMode {
			return measurement, fmt.Errorf("%w: rated participants for mode %q %d exceed ceiling %d", ErrRatingScaleCeilingExceeded, modeKey, count, maxRatingRatedParticipantsPerMode)
		}
	}

	return measurement, nil
}
