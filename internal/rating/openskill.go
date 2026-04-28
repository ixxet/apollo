package rating

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	EngineOpenSkill        = "openskill"
	EngineVersionOpenSkill = "openskill_weng_lin_bradley_terry_full.v1"
	PolicyVersionOpenSkill = "apollo_openskill_dual_run_v1"

	AcceptedOpenSkillDeltaBudget = 0.75

	openskillBeta  = InitialMu / 6.0
	openskillKappa = 0.0001
	openskillTau   = InitialMu / 300.0
)

type ComparisonFact struct {
	UserID              uuid.UUID
	ModeKey             string
	SourceResultID      uuid.UUID
	LegacyMu            float64
	LegacySigma         float64
	OpenSkillMu         float64
	OpenSkillSigma      float64
	DeltaFromLegacy     float64
	AcceptedDeltaBudget float64
	ComparisonScenario  string
	DeltaFlagged        bool
	Watermark           string
	OccurredAt          time.Time
}

type ComparisonProjection struct {
	OpenSkillStates []State
	Facts           []ComparisonFact
	Watermark       string
}

func RebuildOpenSkillComparison(matches []Match, legacy Projection) ComparisonProjection {
	return RebuildOpenSkillComparisonWithBudget(matches, legacy, AcceptedOpenSkillDeltaBudget)
}

func RebuildOpenSkillComparisonWithBudget(matches []Match, legacy Projection, acceptedDeltaBudget float64) ComparisonProjection {
	openSkillProjection := rebuildOpenSkill(matches)
	legacyEvents := make(map[ratingEventKey]ComputedEvent, len(legacy.Events))
	for _, event := range legacy.Events {
		legacyEvents[ratingEventKey{
			modeKey:        event.ModeKey,
			userID:         event.UserID,
			sourceResultID: event.SourceResultID,
		}] = event
	}

	facts := make([]ComparisonFact, 0, len(openSkillProjection.Events))
	for _, event := range openSkillProjection.Events {
		legacyEvent, exists := legacyEvents[ratingEventKey{
			modeKey:        event.ModeKey,
			userID:         event.UserID,
			sourceResultID: event.SourceResultID,
		}]
		if !exists {
			continue
		}

		deltaFromLegacy := event.Mu - legacyEvent.Mu
		facts = append(facts, ComparisonFact{
			UserID:              event.UserID,
			ModeKey:             event.ModeKey,
			SourceResultID:      event.SourceResultID,
			LegacyMu:            legacyEvent.Mu,
			LegacySigma:         legacyEvent.Sigma,
			OpenSkillMu:         event.Mu,
			OpenSkillSigma:      event.Sigma,
			DeltaFromLegacy:     deltaFromLegacy,
			AcceptedDeltaBudget: acceptedDeltaBudget,
			ComparisonScenario:  event.ComparisonScenario,
			DeltaFlagged:        math.Abs(deltaFromLegacy) > acceptedDeltaBudget,
			Watermark:           event.Watermark,
			OccurredAt:          event.OccurredAt,
		})
	}

	return ComparisonProjection{
		OpenSkillStates: openSkillProjection.States,
		Facts:           facts,
		Watermark:       openSkillProjection.Watermark,
	}
}

type ratingEventKey struct {
	modeKey        string
	userID         uuid.UUID
	sourceResultID uuid.UUID
}

type openSkillProjection struct {
	States    []State
	Events    []openSkillComputedEvent
	Watermark string
}

type openSkillComputedEvent struct {
	UserID             uuid.UUID
	ModeKey            string
	SourceResultID     uuid.UUID
	Mu                 float64
	Sigma              float64
	Watermark          string
	OccurredAt         time.Time
	ComparisonScenario string
}

type openSkillTeamRating struct {
	mu           float64
	sigmaSquared float64
	rank         int
	side         Side
}

func rebuildOpenSkill(matches []Match) openSkillProjection {
	statesByMode := make(map[string]map[uuid.UUID]*State, len(matches))
	events := make([]openSkillComputedEvent, 0)

	for _, match := range matches {
		states, exists := statesByMode[match.ModeKey]
		if !exists {
			states = make(map[uuid.UUID]*State)
			statesByMode[match.ModeKey] = states
		}
		events = append(events, applyOpenSkillMatch(states, match)...)
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

	return openSkillProjection{
		States:    states,
		Events:    events,
		Watermark: ProjectionWatermark(matches),
	}
}

func applyOpenSkillMatch(states map[uuid.UUID]*State, match Match) []openSkillComputedEvent {
	for _, side := range match.Sides {
		for _, userID := range side.UserIDs {
			state := ensureState(states, userID, match.ModeKey)
			state.Sigma = math.Sqrt(state.Sigma*state.Sigma + openskillTau*openskillTau)
		}
	}

	teamRatings := make([]openSkillTeamRating, 0, len(match.Sides))
	for _, side := range match.Sides {
		teamRating := openSkillTeamRating{
			rank: rankForOutcome(side.Outcome, match.Sides),
			side: side,
		}
		for _, userID := range side.UserIDs {
			state := ensureState(states, userID, match.ModeKey)
			teamRating.mu += state.Mu
			teamRating.sigmaSquared += state.Sigma * state.Sigma
		}
		teamRatings = append(teamRatings, teamRating)
	}

	watermark := ProjectionWatermark([]Match{match})
	recordedAt := match.RecordedAt.UTC()
	scenario := ComparisonScenario(match)
	events := make([]openSkillComputedEvent, 0)

	for teamIndex, teamRating := range teamRatings {
		if teamRating.sigmaSquared == 0 {
			continue
		}

		omega := 0.0
		delta := 0.0
		for opponentIndex, opponentRating := range teamRatings {
			if opponentIndex == teamIndex {
				continue
			}

			c := math.Sqrt(teamRating.sigmaSquared + opponentRating.sigmaSquared + (2 * openskillBeta * openskillBeta))
			if c == 0 {
				continue
			}

			probability := 1.0 / (1.0 + math.Exp((opponentRating.mu-teamRating.mu)/c))
			score := 0.0
			switch {
			case opponentRating.rank > teamRating.rank:
				score = 1.0
			case opponentRating.rank == teamRating.rank:
				score = 0.5
			}

			sigmaSquaredToC := teamRating.sigmaSquared / c
			omega += sigmaSquaredToC * (score - probability)
			gamma := math.Sqrt(teamRating.sigmaSquared) / c
			delta += ((gamma * sigmaSquaredToC) / c) * probability * (1 - probability)
		}

		for _, userID := range teamRating.side.UserIDs {
			state := ensureState(states, userID, match.ModeKey)
			share := (state.Sigma * state.Sigma) / teamRating.sigmaSquared

			state.Mu += share * omega
			state.Sigma *= math.Sqrt(math.Max(1-(share*delta), openskillKappa))
			state.MatchesPlayed++
			state.LastPlayedAt = &recordedAt
			state.SourceResultID = match.SourceResultID

			events = append(events, openSkillComputedEvent{
				UserID:             userID,
				ModeKey:            match.ModeKey,
				SourceResultID:     match.SourceResultID,
				Mu:                 state.Mu,
				Sigma:              state.Sigma,
				Watermark:          watermark,
				OccurredAt:         recordedAt,
				ComparisonScenario: scenario,
			})
		}
	}

	return events
}

func ComparisonScenario(match Match) string {
	outcomes := make([]string, 0, len(match.Sides))
	for _, side := range match.Sides {
		outcomes = append(outcomes, side.Outcome)
	}
	return match.ModeKey + ":" + strings.Join(outcomes, "_")
}

func rankForOutcome(outcome string, sides []Side) int {
	if outcome == "draw" && allSidesDraw(sides) {
		return 0
	}

	switch outcome {
	case "win":
		return 0
	case "draw":
		return 1
	default:
		return 2
	}
}

func allSidesDraw(sides []Side) bool {
	for _, side := range sides {
		if side.Outcome != "draw" {
			return false
		}
	}
	return true
}
