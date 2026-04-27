package rating

import (
	"math"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	EngineLegacyEloLike = "legacy_elo_like"
	EngineVersionLegacy = "legacy_elo_like.v1"
	PolicyVersionLegacy = "apollo_legacy_rating_v1"

	EventLegacyComputed    = "competition.rating.legacy_computed"
	EventPolicySelected    = "competition.rating.policy_selected"
	EventProjectionRebuilt = "competition.rating.projection_rebuilt"

	InitialMu    = 25.0
	InitialSigma = 8.3333

	minSigma   = 2.5
	sigmaDecay = 0.96
	kFactor    = 4.0
	scale      = 8.0
)

type Match struct {
	CompetitionMatchID uuid.UUID
	SourceResultID     uuid.UUID
	ModeKey            string
	RecordedAt         time.Time
	Sides              []Side
}

type Side struct {
	CompetitionSessionTeamID uuid.UUID
	SideIndex                int
	Outcome                  string
	UserIDs                  []uuid.UUID
}

type State struct {
	UserID         uuid.UUID
	ModeKey        string
	Mu             float64
	Sigma          float64
	MatchesPlayed  int
	LastPlayedAt   *time.Time
	SourceResultID uuid.UUID
}

type ComputedEvent struct {
	UserID         uuid.UUID
	ModeKey        string
	SourceResultID uuid.UUID
	Mu             float64
	Sigma          float64
	DeltaMu        float64
	DeltaSigma     float64
	Watermark      string
	OccurredAt     time.Time
}

type Projection struct {
	States    []State
	Events    []ComputedEvent
	Watermark string
}

// RebuildLegacy projects the current APOLLO legacy rating behavior without
// changing the math. Input ordering is part of the policy contract.
func RebuildLegacy(matches []Match) Projection {
	statesByMode := make(map[string]map[uuid.UUID]*State, len(matches))
	events := make([]ComputedEvent, 0)

	for _, match := range matches {
		states, exists := statesByMode[match.ModeKey]
		if !exists {
			states = make(map[uuid.UUID]*State)
			statesByMode[match.ModeKey] = states
		}
		events = append(events, applyLegacyMatch(states, match)...)
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
		States:    states,
		Events:    events,
		Watermark: ProjectionWatermark(matches),
	}
}

func ProjectionWatermark(matches []Match) string {
	if len(matches) == 0 {
		return "no_results"
	}

	last := matches[len(matches)-1]
	return last.RecordedAt.UTC().Format(time.RFC3339Nano) + "#" + last.SourceResultID.String()
}

func applyLegacyMatch(states map[uuid.UUID]*State, match Match) []ComputedEvent {
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
		deltaMu := deltasByTeam[side.CompetitionSessionTeamID]
		for _, userID := range side.UserIDs {
			state := ensureState(states, userID, match.ModeKey)
			previousMu := state.Mu
			previousSigma := state.Sigma

			state.Mu += deltaMu
			state.Sigma = math.Max(minSigma, state.Sigma*sigmaDecay)
			state.MatchesPlayed++
			recordedAt := match.RecordedAt.UTC()
			state.LastPlayedAt = &recordedAt
			state.SourceResultID = match.SourceResultID

			events = append(events, ComputedEvent{
				UserID:         userID,
				ModeKey:        match.ModeKey,
				SourceResultID: match.SourceResultID,
				Mu:             state.Mu,
				Sigma:          state.Sigma,
				DeltaMu:        state.Mu - previousMu,
				DeltaSigma:     state.Sigma - previousSigma,
				Watermark:      watermark,
				OccurredAt:     recordedAt,
			})
		}
	}

	return events
}

func ensureState(states map[uuid.UUID]*State, userID uuid.UUID, modeKey string) *State {
	state, exists := states[userID]
	if !exists {
		state = &State{
			UserID:  userID,
			ModeKey: modeKey,
			Mu:      InitialMu,
			Sigma:   InitialSigma,
		}
		states[userID] = state
	}
	return state
}

func logisticExpectation(leftMu float64, rightMu float64) float64 {
	return 1.0 / (1.0 + math.Exp((rightMu-leftMu)/scale))
}

func compareUUID(left uuid.UUID, right uuid.UUID) int {
	return slices.Compare(left[:], right[:])
}
