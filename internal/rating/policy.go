package rating

import (
	"math"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	CalibrationStatusProvisional = "provisional"
	CalibrationStatusRanked      = "ranked"

	CalibrationMatchThreshold      = 5
	InactivityThresholdDays        = 90
	InactivitySigmaInflationFactor = 1.25
	MaxPositiveMuDeltaPerResult    = 1.5
)

var inactivityThreshold = time.Duration(InactivityThresholdDays) * 24 * time.Hour

// RebuildActivePolicy wraps the legacy APOLLO rating kernel with explicit
// product policy. RebuildLegacy remains the unchanged characterization
// baseline; this path is the active APOLLO projection policy.
func RebuildActivePolicy(matches []Match) Projection {
	statesByMode := make(map[string]map[uuid.UUID]*State, len(matches))
	events := make([]ComputedEvent, 0)

	for _, match := range matches {
		states, exists := statesByMode[match.ModeKey]
		if !exists {
			states = make(map[uuid.UUID]*State)
			statesByMode[match.ModeKey] = states
		}
		events = append(events, applyPolicyMatch(states, match)...)
	}

	states := make([]State, 0)
	modeKeys := make([]string, 0, len(statesByMode))
	for modeKey := range statesByMode {
		modeKeys = append(modeKeys, modeKey)
	}
	slices.Sort(modeKeys)

	for _, modeKey := range modeKeys {
		userIDs := make([]uuid.UUID, 0, len(statesByMode[modeKey]))
		for userID := range statesByMode[modeKey] {
			userIDs = append(userIDs, userID)
		}
		slices.SortFunc(userIDs, compareUUID)

		for _, userID := range userIDs {
			states = append(states, *statesByMode[modeKey][userID])
		}
	}

	return Projection{
		RatingEngine:  EngineLegacyEloLike,
		EngineVersion: EngineVersionLegacy,
		PolicyVersion: PolicyVersionActive,
		States:        states,
		Events:        events,
		Watermark:     ProjectionWatermark(matches),
	}
}

func applyPolicyMatch(states map[uuid.UUID]*State, match Match) []ComputedEvent {
	recordedAt := match.RecordedAt.UTC()
	decayAppliedByUser := make(map[uuid.UUID]bool)
	for _, side := range match.Sides {
		for _, userID := range side.UserIDs {
			state := ensureState(states, userID, match.ModeKey)
			if applyInactivityDecay(state, recordedAt) {
				decayAppliedByUser[userID] = true
			}
		}
	}

	teamMus := make(map[uuid.UUID]float64, len(match.Sides))
	for _, side := range match.Sides {
		totalMu := 0.0
		for _, userID := range side.UserIDs {
			state := ensureState(states, userID, match.ModeKey)
			totalMu += state.Mu
		}
		teamMus[side.CompetitionSessionTeamID] = totalMu / float64(len(side.UserIDs))
	}

	deltasByTeam := make(map[uuid.UUID]float64, len(match.Sides))
	for _, side := range match.Sides {
		expectedTotal := 0.0
		opponentCount := 0
		for _, opponent := range match.Sides {
			if opponent.CompetitionSessionTeamID == side.CompetitionSessionTeamID {
				continue
			}
			expectedTotal += logisticExpectation(teamMus[side.CompetitionSessionTeamID], teamMus[opponent.CompetitionSessionTeamID])
			opponentCount++
		}
		if opponentCount == 0 {
			continue
		}

		expected := expectedTotal / float64(opponentCount)
		actual := 0.0
		switch side.Outcome {
		case "win":
			actual = 1.0
		case "draw":
			actual = 0.5
		case "loss":
			actual = 0.0
		}
		deltasByTeam[side.CompetitionSessionTeamID] = kFactor * (actual - expected)
	}

	watermark := ProjectionWatermark([]Match{match})
	events := make([]ComputedEvent, 0)
	for _, side := range match.Sides {
		rawDeltaMu := deltasByTeam[side.CompetitionSessionTeamID]
		deltaMu, capApplied := applyClimbingCap(rawDeltaMu)
		for _, userID := range side.UserIDs {
			state := ensureState(states, userID, match.ModeKey)
			previousMu := state.Mu
			previousSigma := state.Sigma

			state.Mu += deltaMu
			state.Sigma = math.Max(minSigma, state.Sigma*sigmaDecay)
			state.MatchesPlayed++
			state.CalibrationStatus = CalibrationStatusForMatches(state.MatchesPlayed)
			state.LastPlayedAt = &recordedAt
			state.SourceResultID = match.SourceResultID
			state.ClimbingCapApplied = capApplied

			events = append(events, ComputedEvent{
				UserID:                 userID,
				ModeKey:                match.ModeKey,
				SourceResultID:         match.SourceResultID,
				Mu:                     state.Mu,
				Sigma:                  state.Sigma,
				DeltaMu:                state.Mu - previousMu,
				DeltaSigma:             state.Sigma - previousSigma,
				CalibrationStatus:      state.CalibrationStatus,
				InactivityDecayApplied: decayAppliedByUser[userID],
				ClimbingCapApplied:     capApplied,
				Watermark:              watermark,
				OccurredAt:             recordedAt,
			})
		}
	}

	return events
}

func CalibrationStatusForMatches(matchesPlayed int) string {
	if matchesPlayed >= CalibrationMatchThreshold {
		return CalibrationStatusRanked
	}
	return CalibrationStatusProvisional
}

func applyInactivityDecay(state *State, recordedAt time.Time) bool {
	if state.LastPlayedAt == nil {
		return false
	}
	if recordedAt.Sub(state.LastPlayedAt.UTC()) < inactivityThreshold {
		return false
	}

	inflatedSigma := math.Min(InitialSigma, state.Sigma*InactivitySigmaInflationFactor)
	if inflatedSigma <= state.Sigma {
		return false
	}

	decayedAt := recordedAt.UTC()
	state.Sigma = inflatedSigma
	state.LastInactivityDecayAt = &decayedAt
	state.InactivityDecayCount++
	return true
}

func applyClimbingCap(deltaMu float64) (float64, bool) {
	if deltaMu > MaxPositiveMuDeltaPerResult {
		return MaxPositiveMuDeltaPerResult, true
	}
	return deltaMu, false
}
