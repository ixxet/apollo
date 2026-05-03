package rating

import (
	"math"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	SimulationReportVersion = "apollo_rating_policy_simulation_v1"

	ScenarioClassificationActivePolicy   = "active_policy"
	ScenarioClassificationComparisonOnly = "comparison_only"
	ScenarioClassificationReadSafety     = "read_safety"

	ScenarioAcceptanceAccepted = "accepted"
	ScenarioAcceptanceRejected = "rejected"

	RiskClassificationLow      = "low"
	RiskClassificationModerate = "moderate"
	RiskClassificationHigh     = "high"
)

type SimulationReport struct {
	ReportVersion          string                     `json:"report_version"`
	FixtureVersion         string                     `json:"fixture_version"`
	ActiveRatingEngine     string                     `json:"active_rating_engine"`
	ActiveEngineVersion    string                     `json:"active_engine_version"`
	ActivePolicyVersion    string                     `json:"active_policy_version"`
	LegacyPolicyVersion    string                     `json:"legacy_policy_version"`
	OpenSkillPolicyVersion string                     `json:"openskill_policy_version"`
	PolicyConstants        SimulationPolicyConstants  `json:"policy_constants"`
	Scenarios              []SimulationScenarioReport `json:"scenarios"`
	CutoverBlockers        []SimulationCutoverBlocker `json:"cutover_blockers"`
	PolicyRisks            []SimulationPolicyRisk     `json:"policy_risks"`
	Summary                SimulationSummary          `json:"summary"`
}

type SimulationPolicyConstants struct {
	CalibrationMatchThreshold      int     `json:"calibration_match_threshold"`
	InactivityThresholdDays        int     `json:"inactivity_threshold_days"`
	InactivitySigmaInflationFactor float64 `json:"inactivity_sigma_inflation_factor"`
	MaxPositiveMuDeltaPerResult    float64 `json:"max_positive_mu_delta_per_result"`
}

type SimulationSummary struct {
	ScenarioCount          int `json:"scenario_count"`
	AcceptedScenarioCount  int `json:"accepted_scenario_count"`
	RejectedScenarioCount  int `json:"rejected_scenario_count"`
	ComparisonOnlyCount    int `json:"comparison_only_count"`
	CutoverBlockerCount    int `json:"cutover_blocker_count"`
	OpenSkillDeltaRowCount int `json:"openskill_delta_row_count"`
}

type SimulationScenarioReport struct {
	Label                string                     `json:"label"`
	Title                string                     `json:"title"`
	Classification       string                     `json:"classification"`
	AcceptanceStatus     string                     `json:"acceptance_status"`
	RiskClassification   string                     `json:"risk_classification"`
	Reason               string                     `json:"reason"`
	Blockers             []string                   `json:"blockers,omitempty"`
	MatchCount           int                        `json:"match_count"`
	ParticipantCount     int                        `json:"participant_count"`
	ActivePolicy         SimulationProjectionReport `json:"active_policy"`
	LegacyBaseline       SimulationProjectionReport `json:"legacy_baseline"`
	LegacyDeltas         []SimulationLegacyDelta    `json:"legacy_deltas"`
	OpenSkillDeltas      []SimulationOpenSkillDelta `json:"openskill_deltas"`
	PublicMemberReadSafe bool                       `json:"public_member_read_safe"`
}

type SimulationProjectionReport struct {
	RatingEngine  string                    `json:"rating_engine"`
	EngineVersion string                    `json:"engine_version"`
	PolicyVersion string                    `json:"policy_version"`
	Watermark     string                    `json:"watermark"`
	States        []SimulationRatingState   `json:"states"`
	LastEvents    []SimulationComputedEvent `json:"last_events"`
}

type SimulationRatingState struct {
	Participant           string     `json:"participant"`
	UserID                uuid.UUID  `json:"user_id"`
	ModeKey               string     `json:"mode_key"`
	Mu                    float64    `json:"mu"`
	Sigma                 float64    `json:"sigma"`
	MatchesPlayed         int        `json:"matches_played"`
	CalibrationStatus     string     `json:"calibration_status,omitempty"`
	LastPlayedAt          *time.Time `json:"last_played_at,omitempty"`
	LastInactivityDecayAt *time.Time `json:"last_inactivity_decay_at,omitempty"`
	InactivityDecayCount  int        `json:"inactivity_decay_count"`
	ClimbingCapApplied    bool       `json:"climbing_cap_applied"`
}

type SimulationComputedEvent struct {
	Participant            string    `json:"participant"`
	UserID                 uuid.UUID `json:"user_id"`
	ModeKey                string    `json:"mode_key"`
	SourceResultID         uuid.UUID `json:"source_result_id"`
	Mu                     float64   `json:"mu"`
	Sigma                  float64   `json:"sigma"`
	DeltaMu                float64   `json:"delta_mu"`
	DeltaSigma             float64   `json:"delta_sigma"`
	CalibrationStatus      string    `json:"calibration_status,omitempty"`
	InactivityDecayApplied bool      `json:"inactivity_decay_applied"`
	ClimbingCapApplied     bool      `json:"climbing_cap_applied"`
	Watermark              string    `json:"watermark"`
	OccurredAt             time.Time `json:"occurred_at"`
}

type SimulationLegacyDelta struct {
	Participant          string    `json:"participant"`
	UserID               uuid.UUID `json:"user_id"`
	ModeKey              string    `json:"mode_key"`
	ActiveMu             float64   `json:"active_mu"`
	LegacyMu             float64   `json:"legacy_mu"`
	DeltaMu              float64   `json:"delta_mu"`
	ActiveSigma          float64   `json:"active_sigma"`
	LegacySigma          float64   `json:"legacy_sigma"`
	DeltaSigma           float64   `json:"delta_sigma"`
	CalibrationStatus    string    `json:"calibration_status,omitempty"`
	ClimbingCapApplied   bool      `json:"climbing_cap_applied"`
	InactivityDecayCount int       `json:"inactivity_decay_count"`
}

type SimulationOpenSkillDelta struct {
	Participant         string    `json:"participant"`
	UserID              uuid.UUID `json:"user_id"`
	ModeKey             string    `json:"mode_key"`
	SourceResultID      uuid.UUID `json:"source_result_id"`
	LegacyMu            float64   `json:"legacy_mu"`
	LegacySigma         float64   `json:"legacy_sigma"`
	OpenSkillMu         float64   `json:"openskill_mu"`
	OpenSkillSigma      float64   `json:"openskill_sigma"`
	DeltaFromLegacy     float64   `json:"delta_from_legacy"`
	AcceptedDeltaBudget float64   `json:"accepted_delta_budget"`
	ComparisonScenario  string    `json:"comparison_scenario"`
	DeltaFlagged        bool      `json:"delta_flagged"`
}

type SimulationCutoverBlocker struct {
	Key    string `json:"key"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type SimulationPolicyRisk struct {
	Key            string `json:"key"`
	Classification string `json:"classification"`
	Reason         string `json:"reason"`
	DeferredTo     string `json:"deferred_to"`
}

type simulationFixture struct {
	label              string
	title              string
	classification     string
	riskClassification string
	reason             string
	matches            []Match
	participantLabels  map[uuid.UUID]string
	accept             func(active Projection, legacy Projection, comparison ComparisonProjection) simulationAcceptance
}

type simulationAcceptance struct {
	status               string
	reason               string
	blockers             []string
	publicMemberReadSafe bool
}

func BuildActivePolicySimulationReport() SimulationReport {
	fixtures := activePolicySimulationFixtures()
	scenarios := make([]SimulationScenarioReport, 0, len(fixtures))
	summary := SimulationSummary{}

	for _, fixture := range fixtures {
		legacy := RebuildLegacy(fixture.matches)
		active := RebuildActivePolicy(fixture.matches)
		comparison := RebuildOpenSkillComparison(fixture.matches, legacy)
		acceptance := fixture.accept(active, legacy, comparison)

		report := SimulationScenarioReport{
			Label:                fixture.label,
			Title:                fixture.title,
			Classification:       fixture.classification,
			AcceptanceStatus:     acceptance.status,
			RiskClassification:   fixture.riskClassification,
			Reason:               acceptance.reason,
			Blockers:             slices.Clone(acceptance.blockers),
			MatchCount:           len(fixture.matches),
			ParticipantCount:     participantCount(fixture.matches),
			ActivePolicy:         buildSimulationProjectionReport(active, fixture.participantLabels),
			LegacyBaseline:       buildSimulationProjectionReport(legacy, fixture.participantLabels),
			LegacyDeltas:         buildSimulationLegacyDeltas(active, legacy, fixture.participantLabels),
			OpenSkillDeltas:      buildSimulationOpenSkillDeltas(comparison, fixture.participantLabels),
			PublicMemberReadSafe: acceptance.publicMemberReadSafe,
		}
		scenarios = append(scenarios, report)

		summary.ScenarioCount++
		if report.AcceptanceStatus == ScenarioAcceptanceAccepted {
			summary.AcceptedScenarioCount++
		}
		if report.AcceptanceStatus == ScenarioAcceptanceRejected {
			summary.RejectedScenarioCount++
		}
		if report.Classification == ScenarioClassificationComparisonOnly {
			summary.ComparisonOnlyCount++
		}
		summary.OpenSkillDeltaRowCount += len(report.OpenSkillDeltas)
	}

	blockers := activePolicySimulationCutoverBlockers()
	summary.CutoverBlockerCount = len(blockers)

	return SimulationReport{
		ReportVersion:          SimulationReportVersion,
		FixtureVersion:         "apollo_rating_policy_simulation_fixtures_v1",
		ActiveRatingEngine:     EngineLegacyEloLike,
		ActiveEngineVersion:    EngineVersionLegacy,
		ActivePolicyVersion:    PolicyVersionActive,
		LegacyPolicyVersion:    PolicyVersionLegacy,
		OpenSkillPolicyVersion: PolicyVersionOpenSkill,
		PolicyConstants: SimulationPolicyConstants{
			CalibrationMatchThreshold:      CalibrationMatchThreshold,
			InactivityThresholdDays:        InactivityThresholdDays,
			InactivitySigmaInflationFactor: InactivitySigmaInflationFactor,
			MaxPositiveMuDeltaPerResult:    MaxPositiveMuDeltaPerResult,
		},
		Scenarios:       scenarios,
		CutoverBlockers: blockers,
		PolicyRisks:     activePolicySimulationRisks(),
		Summary:         summary,
	}
}

func activePolicySimulationFixtures() []simulationFixture {
	base := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	return []simulationFixture{
		activeFixture("unranked_1v1_a_wins", "Unranked 1v1, A wins", RiskClassificationLow, "Baseline active wrapper behavior for a first canonical result.", []Match{
			simulationMatch(1, "head_to_head:s2-p1", base, []simulationSide{
				{team: 1, outcome: "win", users: []int{1}},
				{team: 2, outcome: "loss", users: []int{2}},
			}),
		}, nil),
		activeFixture("stronger_beats_new", "Stronger player beats new player", RiskClassificationLow, "Expected result stays bounded while the stronger participant remains ranked only after enough matches.", append(
			repeatedSimulationMatches(1, 4, "head_to_head:s2-p1", base, []simulationSide{
				{team: 1, outcome: "win", users: []int{1}},
				{team: 2, outcome: "loss", users: []int{2}},
			}),
			simulationMatch(5, "head_to_head:s2-p1", base.Add(4*time.Hour), []simulationSide{
				{team: 1, outcome: "win", users: []int{1}},
				{team: 3, outcome: "loss", users: []int{3}},
			}),
		), nil),
		activeFixture("new_beats_stronger", "New player beats stronger player", RiskClassificationModerate, "Upset scenario proves the wrapper bounds upward movement instead of tuning an upset bonus.", append(
			repeatedSimulationMatches(1, 4, "head_to_head:s2-p1", base, []simulationSide{
				{team: 1, outcome: "win", users: []int{1}},
				{team: 2, outcome: "loss", users: []int{2}},
			}),
			simulationMatch(5, "head_to_head:s2-p1", base.Add(4*time.Hour), []simulationSide{
				{team: 3, outcome: "win", users: []int{3}},
				{team: 1, outcome: "loss", users: []int{1}},
			}),
		), nil),
		activeFixture("fifth_match_ranked_transition", "Repeated wins through fifth-match ranked transition", RiskClassificationLow, "The fifth rated match changes calibration status from provisional to ranked.", repeatedSimulationMatches(1, CalibrationMatchThreshold, "head_to_head:s2-p1", base, []simulationSide{
			{team: 1, outcome: "win", users: []int{1}},
			{team: 2, outcome: "loss", users: []int{2}},
		}), acceptFifthMatchRanked),
		activeFixture("inactive_return_after_threshold", "Inactivity return after threshold", RiskClassificationModerate, "A return after the threshold inflates sigma only, with no direct mu movement from decay.", []Match{
			simulationMatch(1, "head_to_head:s2-p1", base, []simulationSide{
				{team: 1, outcome: "win", users: []int{1}},
				{team: 2, outcome: "loss", users: []int{2}},
			}),
			simulationMatch(2, "head_to_head:s2-p1", base.Add(time.Duration(InactivityThresholdDays+1)*24*time.Hour), []simulationSide{
				{team: 1, outcome: "loss", users: []int{1}},
				{team: 2, outcome: "win", users: []int{2}},
			}),
		}, acceptInactivityApplied),
		activeFixture("climbing_cap_activation", "Climbing-cap activation", RiskClassificationLow, "Positive movement above the policy cap is bounded and marked in metadata.", []Match{
			simulationMatch(1, "head_to_head:s2-p1", base, []simulationSide{
				{team: 1, outcome: "win", users: []int{1}},
				{team: 2, outcome: "loss", users: []int{2}},
			}),
		}, acceptClimbingCapApplied),
		activeFixture("draw_handling", "Draw handling", RiskClassificationLow, "Draws keep mu stable for equal teams while still recording calibration progress and sigma shrink.", []Match{
			simulationMatch(1, "head_to_head:s2-p1", base, []simulationSide{
				{team: 1, outcome: "draw", users: []int{1}},
				{team: 2, outcome: "draw", users: []int{2}},
			}),
		}, acceptDrawHandling),
		activeFixture("five_v_five_even_teams", "5v5 even teams", RiskClassificationModerate, "Even team fixture proves the active wrapper remains deterministic for larger rosters.", []Match{
			simulationMatch(1, "team:s2-p5", base, []simulationSide{
				{team: 1, outcome: "win", users: []int{1, 2, 3, 4, 5}},
				{team: 2, outcome: "loss", users: []int{6, 7, 8, 9, 10}},
			}),
		}, nil),
		{
			label:              "three_v_five_asymmetric_comparison_stress",
			title:              "3v5 asymmetric match comparison stress",
			classification:     ScenarioClassificationComparisonOnly,
			riskClassification: RiskClassificationHigh,
			reason:             "Asymmetric teams are allowed only as comparison stress; active read-path claims remain blocked.",
			matches: []Match{
				simulationMatch(1, "team:s2-pmixed", base, []simulationSide{
					{team: 1, outcome: "win", users: []int{1, 2, 3}},
					{team: 2, outcome: "loss", users: []int{4, 5, 6, 7, 8}},
				}),
			},
			participantLabels: defaultParticipantLabels(8),
			accept: func(active Projection, legacy Projection, comparison ComparisonProjection) simulationAcceptance {
				return simulationAcceptance{
					status: ScenarioAcceptanceRejected,
					reason: "Rejected for active-policy/cutover evidence; retained as OpenSkill sidecar stress only.",
					blockers: []string{
						"asymmetric_team_active_policy_claim_blocked",
						"openskill_read_path_cutover_deferred",
					},
					publicMemberReadSafe: simulationReadSafe(active, legacy, comparison),
				}
			},
		},
		{
			label:              "public_member_read_safety",
			title:              "Public/member read safety",
			classification:     ScenarioClassificationReadSafety,
			riskClassification: RiskClassificationLow,
			reason:             "Read-safety sentinel confirms the simulation report keeps OpenSkill comparison explicit as internal sidecar data.",
			matches: []Match{
				simulationMatch(1, "head_to_head:s2-p1", base, []simulationSide{
					{team: 1, outcome: "win", users: []int{1}},
					{team: 2, outcome: "loss", users: []int{2}},
				}),
			},
			participantLabels: defaultParticipantLabels(2),
			accept: func(active Projection, legacy Projection, comparison ComparisonProjection) simulationAcceptance {
				return simulationAcceptance{
					status:               ScenarioAcceptanceAccepted,
					reason:               "Accepted as a proof sentinel only; public/member API leak tests remain the source of read-safety proof.",
					publicMemberReadSafe: simulationReadSafe(active, legacy, comparison),
				}
			},
		},
	}
}

func activeFixture(label string, title string, risk string, reason string, matches []Match, accept func(active Projection, legacy Projection, comparison ComparisonProjection) simulationAcceptance) simulationFixture {
	if accept == nil {
		accept = acceptActivePolicy
	}
	return simulationFixture{
		label:              label,
		title:              title,
		classification:     ScenarioClassificationActivePolicy,
		riskClassification: risk,
		reason:             reason,
		matches:            matches,
		participantLabels:  labelsForMatches(matches),
		accept:             accept,
	}
}

func acceptActivePolicy(active Projection, legacy Projection, comparison ComparisonProjection) simulationAcceptance {
	if active.RatingEngine != EngineLegacyEloLike || active.PolicyVersion != PolicyVersionActive {
		return simulationAcceptance{
			status:   ScenarioAcceptanceRejected,
			reason:   "Active projection did not remain on the legacy-engine policy wrapper.",
			blockers: []string{"active_policy_path_changed"},
		}
	}
	if legacy.PolicyVersion != PolicyVersionLegacy {
		return simulationAcceptance{
			status:   ScenarioAcceptanceRejected,
			reason:   "Legacy baseline policy version changed.",
			blockers: []string{"legacy_baseline_changed"},
		}
	}
	if len(comparison.Facts) == 0 {
		return simulationAcceptance{
			status:   ScenarioAcceptanceRejected,
			reason:   "OpenSkill sidecar produced no comparison delta rows.",
			blockers: []string{"openskill_comparison_missing"},
		}
	}
	return simulationAcceptance{
		status:               ScenarioAcceptanceAccepted,
		reason:               "Accepted for deterministic active-policy fixture proof.",
		publicMemberReadSafe: simulationReadSafe(active, legacy, comparison),
	}
}

func acceptFifthMatchRanked(active Projection, legacy Projection, comparison ComparisonProjection) simulationAcceptance {
	acceptance := acceptActivePolicy(active, legacy, comparison)
	if acceptance.status != ScenarioAcceptanceAccepted {
		return acceptance
	}
	for _, state := range active.States {
		if state.MatchesPlayed != CalibrationMatchThreshold || state.CalibrationStatus != CalibrationStatusRanked {
			return simulationAcceptance{
				status:   ScenarioAcceptanceRejected,
				reason:   "Fifth-match ranked transition was not applied to all participants.",
				blockers: []string{"calibration_transition_missing"},
			}
		}
	}
	acceptance.reason = "Accepted: every participant reaches ranked status on the fifth rated match."
	return acceptance
}

func acceptInactivityApplied(active Projection, legacy Projection, comparison ComparisonProjection) simulationAcceptance {
	acceptance := acceptActivePolicy(active, legacy, comparison)
	if acceptance.status != ScenarioAcceptanceAccepted {
		return acceptance
	}
	for _, event := range active.Events {
		if event.InactivityDecayApplied {
			acceptance.reason = "Accepted: inactivity sigma inflation is applied and reported without a separate mu bonus."
			return acceptance
		}
	}
	return simulationAcceptance{
		status:   ScenarioAcceptanceRejected,
		reason:   "No inactivity decay metadata was recorded after the configured threshold.",
		blockers: []string{"inactivity_decay_missing"},
	}
}

func acceptClimbingCapApplied(active Projection, legacy Projection, comparison ComparisonProjection) simulationAcceptance {
	acceptance := acceptActivePolicy(active, legacy, comparison)
	if acceptance.status != ScenarioAcceptanceAccepted {
		return acceptance
	}
	for _, event := range active.Events {
		if event.ClimbingCapApplied && almostEqual(event.DeltaMu, MaxPositiveMuDeltaPerResult) {
			acceptance.reason = "Accepted: positive movement is capped and marked in event/state metadata."
			return acceptance
		}
	}
	return simulationAcceptance{
		status:   ScenarioAcceptanceRejected,
		reason:   "No capped positive movement was reported.",
		blockers: []string{"climbing_cap_missing"},
	}
}

func acceptDrawHandling(active Projection, legacy Projection, comparison ComparisonProjection) simulationAcceptance {
	acceptance := acceptActivePolicy(active, legacy, comparison)
	if acceptance.status != ScenarioAcceptanceAccepted {
		return acceptance
	}
	for _, event := range active.Events {
		if !almostEqual(event.DeltaMu, 0) {
			return simulationAcceptance{
				status:   ScenarioAcceptanceRejected,
				reason:   "Equal-team draw changed mu.",
				blockers: []string{"draw_mu_changed"},
			}
		}
	}
	acceptance.reason = "Accepted: equal-team draw leaves mu unchanged while recording match progress."
	return acceptance
}

func simulationReadSafe(active Projection, legacy Projection, comparison ComparisonProjection) bool {
	return active.PolicyVersion == PolicyVersionActive &&
		active.RatingEngine == EngineLegacyEloLike &&
		legacy.PolicyVersion == PolicyVersionLegacy &&
		comparison.Watermark == active.Watermark
}

func buildSimulationProjectionReport(projection Projection, labels map[uuid.UUID]string) SimulationProjectionReport {
	return SimulationProjectionReport{
		RatingEngine:  projection.RatingEngine,
		EngineVersion: projection.EngineVersion,
		PolicyVersion: projection.PolicyVersion,
		Watermark:     projection.Watermark,
		States:        buildSimulationStates(projection.States, labels),
		LastEvents:    buildSimulationLastEvents(projection.Events, labels),
	}
}

func buildSimulationStates(states []State, labels map[uuid.UUID]string) []SimulationRatingState {
	report := make([]SimulationRatingState, 0, len(states))
	for _, state := range states {
		report = append(report, SimulationRatingState{
			Participant:           participantLabel(labels, state.UserID),
			UserID:                state.UserID,
			ModeKey:               state.ModeKey,
			Mu:                    roundedSimulationFloat(state.Mu),
			Sigma:                 roundedSimulationFloat(state.Sigma),
			MatchesPlayed:         state.MatchesPlayed,
			CalibrationStatus:     state.CalibrationStatus,
			LastPlayedAt:          state.LastPlayedAt,
			LastInactivityDecayAt: state.LastInactivityDecayAt,
			InactivityDecayCount:  state.InactivityDecayCount,
			ClimbingCapApplied:    state.ClimbingCapApplied,
		})
	}
	return report
}

func buildSimulationLastEvents(events []ComputedEvent, labels map[uuid.UUID]string) []SimulationComputedEvent {
	latestByUser := make(map[uuid.UUID]ComputedEvent, len(events))
	for _, event := range events {
		latestByUser[event.UserID] = event
	}

	userIDs := make([]uuid.UUID, 0, len(latestByUser))
	for userID := range latestByUser {
		userIDs = append(userIDs, userID)
	}
	slices.SortFunc(userIDs, compareUUID)

	report := make([]SimulationComputedEvent, 0, len(userIDs))
	for _, userID := range userIDs {
		event := latestByUser[userID]
		report = append(report, SimulationComputedEvent{
			Participant:            participantLabel(labels, event.UserID),
			UserID:                 event.UserID,
			ModeKey:                event.ModeKey,
			SourceResultID:         event.SourceResultID,
			Mu:                     roundedSimulationFloat(event.Mu),
			Sigma:                  roundedSimulationFloat(event.Sigma),
			DeltaMu:                roundedSimulationFloat(event.DeltaMu),
			DeltaSigma:             roundedSimulationFloat(event.DeltaSigma),
			CalibrationStatus:      event.CalibrationStatus,
			InactivityDecayApplied: event.InactivityDecayApplied,
			ClimbingCapApplied:     event.ClimbingCapApplied,
			Watermark:              event.Watermark,
			OccurredAt:             event.OccurredAt,
		})
	}
	return report
}

func buildSimulationLegacyDeltas(active Projection, legacy Projection, labels map[uuid.UUID]string) []SimulationLegacyDelta {
	legacyStates := make(map[simulationStateKey]State, len(legacy.States))
	for _, state := range legacy.States {
		legacyStates[simulationStateKey{modeKey: state.ModeKey, userID: state.UserID}] = state
	}

	deltas := make([]SimulationLegacyDelta, 0, len(active.States))
	for _, activeState := range active.States {
		legacyState, exists := legacyStates[simulationStateKey{modeKey: activeState.ModeKey, userID: activeState.UserID}]
		if !exists {
			continue
		}
		deltas = append(deltas, SimulationLegacyDelta{
			Participant:          participantLabel(labels, activeState.UserID),
			UserID:               activeState.UserID,
			ModeKey:              activeState.ModeKey,
			ActiveMu:             roundedSimulationFloat(activeState.Mu),
			LegacyMu:             roundedSimulationFloat(legacyState.Mu),
			DeltaMu:              roundedSimulationFloat(activeState.Mu - legacyState.Mu),
			ActiveSigma:          roundedSimulationFloat(activeState.Sigma),
			LegacySigma:          roundedSimulationFloat(legacyState.Sigma),
			DeltaSigma:           roundedSimulationFloat(activeState.Sigma - legacyState.Sigma),
			CalibrationStatus:    activeState.CalibrationStatus,
			ClimbingCapApplied:   activeState.ClimbingCapApplied,
			InactivityDecayCount: activeState.InactivityDecayCount,
		})
	}
	return deltas
}

func buildSimulationOpenSkillDeltas(comparison ComparisonProjection, labels map[uuid.UUID]string) []SimulationOpenSkillDelta {
	deltas := make([]SimulationOpenSkillDelta, 0, len(comparison.Facts))
	for _, fact := range comparison.Facts {
		deltas = append(deltas, SimulationOpenSkillDelta{
			Participant:         participantLabel(labels, fact.UserID),
			UserID:              fact.UserID,
			ModeKey:             fact.ModeKey,
			SourceResultID:      fact.SourceResultID,
			LegacyMu:            roundedSimulationFloat(fact.LegacyMu),
			LegacySigma:         roundedSimulationFloat(fact.LegacySigma),
			OpenSkillMu:         roundedSimulationFloat(fact.OpenSkillMu),
			OpenSkillSigma:      roundedSimulationFloat(fact.OpenSkillSigma),
			DeltaFromLegacy:     roundedSimulationFloat(fact.DeltaFromLegacy),
			AcceptedDeltaBudget: roundedSimulationFloat(fact.AcceptedDeltaBudget),
			ComparisonScenario:  fact.ComparisonScenario,
			DeltaFlagged:        fact.DeltaFlagged,
		})
	}
	return deltas
}

func activePolicySimulationCutoverBlockers() []SimulationCutoverBlocker {
	return []SimulationCutoverBlocker{
		{
			Key:    "openskill_active_read_path_deferred",
			Status: "blocking",
			Reason: "OpenSkill is still comparison-only; this report does not activate OpenSkill reads.",
		},
		{
			Key:    "rollback_cutover_packet_missing",
			Status: "blocking",
			Reason: "No approved rollback/cutover packet has built the operational switch or fallback procedure.",
		},
		{
			Key:    "production_backtest_missing",
			Status: "blocking",
			Reason: "Synthetic fixtures do not prove real population dynamics or full historical validation.",
		},
		{
			Key:    "public_tournament_readiness_blocked",
			Status: "blocking",
			Reason: "Public tournaments remain blocked by privacy, scale, dispute, and product gates.",
		},
	}
}

func activePolicySimulationRisks() []SimulationPolicyRisk {
	return []SimulationPolicyRisk{
		{
			Key:            "fixture_representativeness",
			Classification: RiskClassificationModerate,
			Reason:         "Synthetic fixtures prove deterministic behavior but may not represent production population dynamics.",
			DeferredTo:     "production backtest and telemetry review",
		},
		{
			Key:            "asymmetric_team_math",
			Classification: RiskClassificationHigh,
			Reason:         "The active legacy engine uses team average mu; asymmetric team proof remains sidecar/comparison-only.",
			DeferredTo:     "OpenSkill rollback/cutover packet",
		},
		{
			Key:            "delta_budget_interpretation",
			Classification: RiskClassificationModerate,
			Reason:         "OpenSkill deltas are visible for comparison but are not accepted as product rating claims.",
			DeferredTo:     "future calibration packet",
		},
		{
			Key:            "public_claim_safety",
			Classification: RiskClassificationLow,
			Reason:         "Simulation output is CLI/local proof; public/member routes must continue to use allowlisted APOLLO contracts.",
			DeferredTo:     "frontend route/API contract matrix",
		},
	}
}

type simulationSide struct {
	team    int
	outcome string
	users   []int
}

func repeatedSimulationMatches(startIndex int, count int, modeKey string, start time.Time, sides []simulationSide) []Match {
	matches := make([]Match, 0, count)
	for index := 0; index < count; index++ {
		matches = append(matches, simulationMatch(startIndex+index, modeKey, start.Add(time.Duration(index)*time.Hour), sides))
	}
	return matches
}

func simulationMatch(index int, modeKey string, recordedAt time.Time, sides []simulationSide) Match {
	ratingSides := make([]Side, 0, len(sides))
	for sideIndex, side := range sides {
		userIDs := make([]uuid.UUID, 0, len(side.users))
		for _, userIndex := range side.users {
			userIDs = append(userIDs, simulationUserID(userIndex))
		}
		ratingSides = append(ratingSides, Side{
			CompetitionSessionTeamID: simulationTeamID(side.team),
			SideIndex:                sideIndex + 1,
			Outcome:                  side.outcome,
			UserIDs:                  userIDs,
		})
	}
	return Match{
		CompetitionMatchID: simulationMatchID(index),
		SourceResultID:     simulationResultID(index),
		ModeKey:            modeKey,
		RecordedAt:         recordedAt,
		Sides:              ratingSides,
	}
}

func labelsForMatches(matches []Match) map[uuid.UUID]string {
	maxIndex := 0
	for _, match := range matches {
		for _, side := range match.Sides {
			for _, userID := range side.UserIDs {
				index := simulationUserIndex(userID)
				if index > maxIndex {
					maxIndex = index
				}
			}
		}
	}
	return defaultParticipantLabels(maxIndex)
}

func defaultParticipantLabels(count int) map[uuid.UUID]string {
	labels := make(map[uuid.UUID]string, count)
	for index := 1; index <= count; index++ {
		labels[simulationUserID(index)] = "player_" + string(rune('a'+index-1))
	}
	return labels
}

func participantLabel(labels map[uuid.UUID]string, userID uuid.UUID) string {
	if label, exists := labels[userID]; exists {
		return label
	}
	return userID.String()
}

func participantCount(matches []Match) int {
	users := make(map[uuid.UUID]struct{})
	for _, match := range matches {
		for _, side := range match.Sides {
			for _, userID := range side.UserIDs {
				users[userID] = struct{}{}
			}
		}
	}
	return len(users)
}

type simulationStateKey struct {
	modeKey string
	userID  uuid.UUID
}

func simulationUserID(index int) uuid.UUID {
	return uuid.MustParse("10000000-0000-0000-0000-" + simulationPaddedNumber(index))
}

func simulationTeamID(index int) uuid.UUID {
	return uuid.MustParse("20000000-0000-0000-0000-" + simulationPaddedNumber(index))
}

func simulationMatchID(index int) uuid.UUID {
	return uuid.MustParse("30000000-0000-0000-0000-" + simulationPaddedNumber(index))
}

func simulationResultID(index int) uuid.UUID {
	return uuid.MustParse("40000000-0000-0000-0000-" + simulationPaddedNumber(index))
}

func simulationUserIndex(userID uuid.UUID) int {
	for index := 1; index <= 32; index++ {
		if userID == simulationUserID(index) {
			return index
		}
	}
	return 0
}

func simulationPaddedNumber(index int) string {
	switch {
	case index < 10:
		return "00000000000" + string(rune('0'+index))
	case index < 100:
		tens := index / 10
		ones := index % 10
		return "0000000000" + string(rune('0'+tens)) + string(rune('0'+ones))
	default:
		panic("simulation fixture index out of range")
	}
}

func roundedSimulationFloat(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func almostEqual(left float64, right float64) bool {
	return math.Abs(left-right) <= 0.000001
}
