package competition

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	GameIdentityPolicyTuningReportVersion = "apollo_game_identity_policy_tuning_v1"

	gameIdentityPolicyTuningFixtureVersion = "apollo_game_identity_policy_tuning_fixtures_v1"

	gameIdentityTuningClassificationActivePolicy = "active_policy"
	gameIdentityTuningClassificationPrivacyGuard = "privacy_guard"

	gameIdentityPolicyFindingAccepted = "accepted"
	gameIdentityPolicyFindingRejected = "rejected"

	gameIdentityPolicyRiskLow      = "low"
	gameIdentityPolicyRiskModerate = "moderate"
	gameIdentityPolicyRiskHigh     = "high"

	gameIdentityDatabaseAnalysisNotRequested = "not_requested"
	gameIdentityDatabaseAnalysisIncluded     = "included"
)

type GameIdentityPolicyTuningReport struct {
	ReportVersion        string                          `json:"report_version"`
	FixtureVersion       string                          `json:"fixture_version"`
	ContractVersion      string                          `json:"contract_version"`
	ProjectionVersion    string                          `json:"projection_version"`
	CPPolicyVersion      string                          `json:"cp_policy_version"`
	BadgePolicyVersion   string                          `json:"badge_policy_version"`
	RivalryPolicyVersion string                          `json:"rivalry_policy_version"`
	SquadPolicyVersion   string                          `json:"squad_policy_version"`
	ActiveBehaviorChange bool                            `json:"active_behavior_change"`
	PolicyConstants      GameIdentityPolicyConstants     `json:"policy_constants"`
	Scenarios            []GameIdentityTuningScenario    `json:"scenarios"`
	Findings             []GameIdentityPolicyFinding     `json:"findings"`
	PolicyRisks          []GameIdentityPolicyRisk        `json:"policy_risks"`
	Blockers             []GameIdentityPolicyBlocker     `json:"blockers"`
	DatabaseAnalysis     GameIdentityDatabaseAnalysis    `json:"database_analysis"`
	Summary              GameIdentityPolicyTuningSummary `json:"summary"`
	RuntimeTruth         string                          `json:"runtime_truth"`
	DeployedTruth        string                          `json:"deployed_truth"`
}

type GameIdentityPolicyConstants struct {
	CPWeights         []GameIdentityCPWeight        `json:"cp_weights"`
	BadgeThresholds   []GameIdentityBadgeThreshold  `json:"badge_thresholds"`
	RivalryActivation GameIdentityRivalryActivation `json:"rivalry_activation"`
	SquadAggregation  GameIdentitySquadAggregation  `json:"squad_aggregation"`
}

type GameIdentityCPWeight struct {
	Metric        string `json:"metric"`
	PointsPerUnit int    `json:"points_per_unit"`
}

type GameIdentityBadgeThreshold struct {
	BadgeKey  string  `json:"badge_key"`
	Metric    string  `json:"metric"`
	Threshold float64 `json:"threshold"`
}

type GameIdentityRivalryActivation struct {
	CandidateRule  string `json:"candidate_rule"`
	ActiveCPGapMax int    `json:"active_cp_gap_max"`
	EmergingRule   string `json:"emerging_rule"`
}

type GameIdentitySquadAggregation struct {
	GroupingFields []string `json:"grouping_fields"`
	Metrics        []string `json:"metrics"`
}

type GameIdentityTuningScenario struct {
	Label              string                           `json:"label"`
	Title              string                           `json:"title"`
	Classification     string                           `json:"classification"`
	AcceptanceStatus   string                           `json:"acceptance_status"`
	RiskClassification string                           `json:"risk_classification"`
	Reason             string                           `json:"reason"`
	Blockers           []string                         `json:"blockers,omitempty"`
	InputRows          int                              `json:"input_rows"`
	CP                 []GameIdentityCPProjection       `json:"cp"`
	BadgeAwards        []GameIdentityBadgeAward         `json:"badge_awards"`
	RivalryStates      []GameIdentityRivalryState       `json:"rivalry_states"`
	SquadIdentities    []GameIdentitySquadIdentity      `json:"squad_identities"`
	Evaluations        []GameIdentityScenarioEvaluation `json:"evaluations"`
}

type GameIdentityScenarioEvaluation struct {
	PolicyArea string `json:"policy_area"`
	Expected   string `json:"expected"`
	Actual     string `json:"actual"`
	Status     string `json:"status"`
}

type GameIdentityPolicyFinding struct {
	Key            string `json:"key"`
	PolicyArea     string `json:"policy_area"`
	Status         string `json:"status"`
	Reason         string `json:"reason"`
	Evidence       string `json:"evidence"`
	BehaviorChange bool   `json:"behavior_change"`
}

type GameIdentityPolicyRisk struct {
	Key             string `json:"key"`
	PolicyArea      string `json:"policy_area"`
	Classification  string `json:"classification"`
	Reason          string `json:"reason"`
	CurrentHandling string `json:"current_handling"`
}

type GameIdentityPolicyBlocker struct {
	Key    string `json:"key"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type GameIdentityDatabaseAnalysis struct {
	Status               string                      `json:"status"`
	EvidenceLevel        string                      `json:"evidence_level"`
	Reason               string                      `json:"reason,omitempty"`
	Input                GameIdentityDatabaseInput   `json:"input"`
	RowCount             int                         `json:"row_count"`
	ContextCount         int                         `json:"context_count"`
	CPMin                int                         `json:"cp_min"`
	CPMax                int                         `json:"cp_max"`
	CPAverage            float64                     `json:"cp_average"`
	BadgeAwardCount      int                         `json:"badge_award_count"`
	RivalryCount         int                         `json:"rivalry_count"`
	ActiveRivalryCount   int                         `json:"active_rivalry_count"`
	EmergingRivalryCount int                         `json:"emerging_rivalry_count"`
	SquadCount           int                         `json:"squad_count"`
	Findings             []GameIdentityPolicyFinding `json:"findings"`
}

type GameIdentityDatabaseInput struct {
	SportKey    string `json:"sport_key"`
	ModeKey     string `json:"mode_key"`
	FacilityKey string `json:"facility_key"`
	TeamScope   string `json:"team_scope"`
	Limit       int    `json:"limit"`
}

type GameIdentityPolicyTuningSummary struct {
	ScenarioCount         int  `json:"scenario_count"`
	AcceptedScenarioCount int  `json:"accepted_scenario_count"`
	RejectedScenarioCount int  `json:"rejected_scenario_count"`
	FindingCount          int  `json:"finding_count"`
	AcceptedFindingCount  int  `json:"accepted_finding_count"`
	RejectedFindingCount  int  `json:"rejected_finding_count"`
	PolicyRiskCount       int  `json:"policy_risk_count"`
	BlockerCount          int  `json:"blocker_count"`
	DBBackedAnalysis      bool `json:"db_backed_analysis"`
	ActiveBehaviorChange  bool `json:"active_behavior_change"`
}

type gameIdentityTuningFixture struct {
	label              string
	title              string
	classification     string
	riskClassification string
	reason             string
	rows               []gameIdentityProjectionRowRecord
	evaluate           func(GameIdentityProjection) []GameIdentityScenarioEvaluation
}

func BuildGameIdentityPolicyTuningReport() GameIdentityPolicyTuningReport {
	return buildGameIdentityPolicyTuningReport(nil, nil, gameIdentityDatabaseAnalysisNotRequested, "DB-backed analysis was not requested.")
}

func (s *Service) GameIdentityPolicyTuningReport(ctx context.Context, input PublicGameIdentityInput) (GameIdentityPolicyTuningReport, error) {
	normalized := normalizeGameIdentityInput(input)
	rows, err := s.repository.ListGameIdentityProjectionRows(ctx, GameIdentityProjectionInput{
		SportKey:    normalized.SportKey,
		ModeKey:     normalized.ModeKey,
		FacilityKey: normalized.FacilityKey,
		TeamScope:   normalized.TeamScope,
		Limit:       normalized.Limit,
	})
	if err != nil {
		return GameIdentityPolicyTuningReport{}, err
	}

	return buildGameIdentityPolicyTuningReport(rows, &normalized, gameIdentityDatabaseAnalysisIncluded, ""), nil
}

func buildGameIdentityPolicyTuningReport(dbRows []gameIdentityProjectionRowRecord, dbInput *PublicGameIdentityInput, dbStatus string, dbReason string) GameIdentityPolicyTuningReport {
	scenarios := buildGameIdentityTuningScenarios()
	databaseAnalysis := buildGameIdentityDatabaseAnalysis(dbRows, dbInput, dbStatus, dbReason)
	findings := gameIdentityPolicyFindings(databaseAnalysis)
	risks := gameIdentityPolicyRisks()
	blockers := gameIdentityPolicyBlockers(databaseAnalysis)
	summary := GameIdentityPolicyTuningSummary{
		ScenarioCount:        len(scenarios),
		FindingCount:         len(findings),
		PolicyRiskCount:      len(risks),
		BlockerCount:         len(blockers),
		DBBackedAnalysis:     databaseAnalysis.Status == gameIdentityDatabaseAnalysisIncluded,
		ActiveBehaviorChange: false,
	}
	for _, scenario := range scenarios {
		if scenario.AcceptanceStatus == gameIdentityPolicyFindingAccepted {
			summary.AcceptedScenarioCount++
		}
		if scenario.AcceptanceStatus == gameIdentityPolicyFindingRejected {
			summary.RejectedScenarioCount++
		}
	}
	for _, finding := range findings {
		if finding.Status == gameIdentityPolicyFindingAccepted {
			summary.AcceptedFindingCount++
		}
		if finding.Status == gameIdentityPolicyFindingRejected {
			summary.RejectedFindingCount++
		}
	}

	return GameIdentityPolicyTuningReport{
		ReportVersion:        GameIdentityPolicyTuningReportVersion,
		FixtureVersion:       gameIdentityPolicyTuningFixtureVersion,
		ContractVersion:      gameIdentityContractVersion,
		ProjectionVersion:    gameIdentityProjectionVersion,
		CPPolicyVersion:      gameIdentityCPPolicyVersion,
		BadgePolicyVersion:   gameIdentityBadgePolicyVersion,
		RivalryPolicyVersion: gameIdentityRivalryPolicyVersion,
		SquadPolicyVersion:   gameIdentitySquadPolicyVersion,
		ActiveBehaviorChange: false,
		PolicyConstants:      gameIdentityPolicyConstants(),
		Scenarios:            scenarios,
		Findings:             findings,
		PolicyRisks:          risks,
		Blockers:             blockers,
		DatabaseAnalysis:     databaseAnalysis,
		Summary:              summary,
		RuntimeTruth:         "repo/local APOLLO proof only",
		DeployedTruth:        "unchanged",
	}
}

func gameIdentityPolicyConstants() GameIdentityPolicyConstants {
	return GameIdentityPolicyConstants{
		CPWeights: []GameIdentityCPWeight{
			{Metric: analyticsStatMatchesPlayed, PointsPerUnit: gameIdentityCPMatchesPlayedPoints},
			{Metric: analyticsStatWins, PointsPerUnit: gameIdentityCPWinPoints},
			{Metric: analyticsStatDraws, PointsPerUnit: gameIdentityCPDrawPoints},
			{Metric: analyticsStatLosses, PointsPerUnit: gameIdentityCPLossPoints},
		},
		BadgeThresholds: []GameIdentityBadgeThreshold{
			{BadgeKey: "first_match", Metric: analyticsStatMatchesPlayed, Threshold: 1},
			{BadgeKey: "first_win", Metric: analyticsStatWins, Threshold: 1},
			{BadgeKey: "regular_competitor", Metric: analyticsStatMatchesPlayed, Threshold: 5},
		},
		RivalryActivation: GameIdentityRivalryActivation{
			CandidateRule:  "top_two_cp_rows_in_same_sport_mode_facility_team_scope",
			ActiveCPGapMax: 50,
			EmergingRule:   "same-context top-two rivalry with CP gap greater than active_cp_gap_max",
		},
		SquadAggregation: GameIdentitySquadAggregation{
			GroupingFields: []string{"sport_key", "mode_key", "facility_key", "team_scope"},
			Metrics:        []string{"participant_count", "cp_total"},
		},
	}
}

func buildGameIdentityTuningScenarios() []GameIdentityTuningScenario {
	fixtures := gameIdentityTuningFixtures()
	scenarios := make([]GameIdentityTuningScenario, 0, len(fixtures))
	for _, fixture := range fixtures {
		projection := buildGameIdentityProjectionFromRows(cloneGameIdentityRows(fixture.rows), maxGameIdentityLimit, "participant")
		evaluations := fixture.evaluate(projection)
		acceptanceStatus := gameIdentityPolicyFindingAccepted
		blockers := []string(nil)
		for _, evaluation := range evaluations {
			if evaluation.Status == gameIdentityPolicyFindingRejected {
				acceptanceStatus = gameIdentityPolicyFindingRejected
				blockers = append(blockers, evaluation.PolicyArea+": "+evaluation.Expected)
			}
		}
		scenarios = append(scenarios, GameIdentityTuningScenario{
			Label:              fixture.label,
			Title:              fixture.title,
			Classification:     fixture.classification,
			AcceptanceStatus:   acceptanceStatus,
			RiskClassification: fixture.riskClassification,
			Reason:             fixture.reason,
			Blockers:           blockers,
			InputRows:          len(fixture.rows),
			CP:                 projection.CP,
			BadgeAwards:        projection.BadgeAwards,
			RivalryStates:      projection.RivalryStates,
			SquadIdentities:    projection.SquadIdentities,
			Evaluations:        evaluations,
		})
	}
	return scenarios
}

func gameIdentityTuningFixtures() []gameIdentityTuningFixture {
	return []gameIdentityTuningFixture{
		{
			label:              "cp_weight_balance",
			title:              "CP weight balance",
			classification:     gameIdentityTuningClassificationActivePolicy,
			riskClassification: gameIdentityPolicyRiskLow,
			reason:             "Proves the active CP formula and ordering using wins, losses, and matches played.",
			rows: []gameIdentityProjectionRowRecord{
				gameIdentityTuningRow(1, "badminton", "head_to_head:s2-p1", "ashtonbee", analyticsDimensionAll, 5, 2, 3, 0, 1),
				gameIdentityTuningRow(2, "badminton", "head_to_head:s2-p1", "ashtonbee", analyticsDimensionAll, 2, 2, 0, 0, 2),
			},
			evaluate: func(projection GameIdentityProjection) []GameIdentityScenarioEvaluation {
				return []GameIdentityScenarioEvaluation{
					gameIdentityEvaluation("cp", "top CP is 125", gameIdentityCPAt(projection, 0), 125),
					gameIdentityEvaluation("cp", "second CP is 80", gameIdentityCPAt(projection, 1), 80),
					gameIdentityEvaluation("badge", "five badge awards from first-match/first-win/regular thresholds", len(projection.BadgeAwards), 5),
					gameIdentityEvaluation("rivalry", "CP gap 45 activates rivalry", gameIdentityRivalryStateSummary(projection), "active:45"),
					gameIdentityEvaluation("squad", "single cohort has 2 participants and 205 CP", gameIdentitySquadSummary(projection, analyticsDimensionAll), "2:205"),
				}
			},
		},
		{
			label:              "badge_thresholds",
			title:              "Badge threshold boundaries",
			classification:     gameIdentityTuningClassificationActivePolicy,
			riskClassification: gameIdentityPolicyRiskLow,
			reason:             "Proves first-match, first-win, and regular-competitor thresholds exactly at their current boundaries.",
			rows: []gameIdentityProjectionRowRecord{
				gameIdentityTuningRow(3, "badminton", "head_to_head:s2-p1", "ashtonbee", analyticsDimensionAll, 1, 0, 1, 0, 3),
				gameIdentityTuningRow(4, "badminton", "head_to_head:s2-p1", "ashtonbee", analyticsDimensionAll, 1, 1, 0, 0, 4),
				gameIdentityTuningRow(5, "badminton", "head_to_head:s2-p1", "ashtonbee", analyticsDimensionAll, 5, 0, 5, 0, 5),
			},
			evaluate: func(projection GameIdentityProjection) []GameIdentityScenarioEvaluation {
				return []GameIdentityScenarioEvaluation{
					gameIdentityEvaluation("badge", "three first_match awards", gameIdentityBadgeCount(projection, "first_match"), 3),
					gameIdentityEvaluation("badge", "one first_win award", gameIdentityBadgeCount(projection, "first_win"), 1),
					gameIdentityEvaluation("badge", "one regular_competitor award", gameIdentityBadgeCount(projection, "regular_competitor"), 1),
				}
			},
		},
		{
			label:              "rivalry_active_gap_boundary",
			title:              "Rivalry active gap boundary",
			classification:     gameIdentityTuningClassificationActivePolicy,
			riskClassification: gameIdentityPolicyRiskModerate,
			reason:             "Proves the active rivalry threshold is inclusive at a 50 CP gap.",
			rows: []gameIdentityProjectionRowRecord{
				gameIdentityTuningRow(6, "badminton", "head_to_head:s2-p1", "ashtonbee", analyticsDimensionAll, 8, 0, 0, 0, 6),
				gameIdentityTuningRow(7, "badminton", "head_to_head:s2-p1", "ashtonbee", analyticsDimensionAll, 3, 0, 0, 0, 7),
			},
			evaluate: func(projection GameIdentityProjection) []GameIdentityScenarioEvaluation {
				return []GameIdentityScenarioEvaluation{
					gameIdentityEvaluation("rivalry", "gap 50 is active", gameIdentityRivalryStateSummary(projection), "active:50"),
				}
			},
		},
		{
			label:              "rivalry_emerging_gap",
			title:              "Rivalry emerging gap",
			classification:     gameIdentityTuningClassificationActivePolicy,
			riskClassification: gameIdentityPolicyRiskModerate,
			reason:             "Proves same-context top-two candidates remain emerging when the CP gap is above 50.",
			rows: []gameIdentityProjectionRowRecord{
				gameIdentityTuningRow(8, "badminton", "head_to_head:s2-p1", "ashtonbee", analyticsDimensionAll, 8, 2, 0, 0, 8),
				gameIdentityTuningRow(9, "badminton", "head_to_head:s2-p1", "ashtonbee", analyticsDimensionAll, 4, 0, 0, 0, 9),
			},
			evaluate: func(projection GameIdentityProjection) []GameIdentityScenarioEvaluation {
				return []GameIdentityScenarioEvaluation{
					gameIdentityEvaluation("rivalry", "gap above 50 is emerging", gameIdentityRivalryStateSummary(projection), "emerging:100"),
				}
			},
		},
		{
			label:              "context_scoped_labels",
			title:              "Context-scoped rivalry and labels",
			classification:     gameIdentityTuningClassificationPrivacyGuard,
			riskClassification: gameIdentityPolicyRiskLow,
			reason:             "Proves rivalry and labels stay scoped to the exact sport/mode/facility/team-scope projection row context.",
			rows: []gameIdentityProjectionRowRecord{
				gameIdentityTuningRow(10, "badminton", "head_to_head:s2-p1", "ashtonbee", analyticsDimensionAll, 6, 5, 1, 0, 10),
				gameIdentityTuningRow(11, "basketball", "three_on_three:s2-p3", "court-a", analyticsDimensionAll, 5, 4, 1, 0, 11),
				gameIdentityTuningRow(12, "badminton", "head_to_head:s2-p1", "ashtonbee", analyticsDimensionAll, 3, 2, 1, 0, 12),
			},
			evaluate: func(projection GameIdentityProjection) []GameIdentityScenarioEvaluation {
				return []GameIdentityScenarioEvaluation{
					gameIdentityEvaluation("rivalry", "only one same-context rivalry is emitted", len(projection.RivalryStates), 1),
					gameIdentityEvaluation("rivalry", "badminton rivalry excludes the basketball row label", strings.Contains(gameIdentityRivalryParticipants(projection), "participant_2"), false),
				}
			},
		},
		{
			label:              "squad_aggregation",
			title:              "Squad aggregation by context",
			classification:     gameIdentityTuningClassificationActivePolicy,
			riskClassification: gameIdentityPolicyRiskModerate,
			reason:             "Proves squad identity remains projection-only and aggregates participant count plus CP total by context.",
			rows: []gameIdentityProjectionRowRecord{
				gameIdentityTuningRow(13, "badminton", "head_to_head:s2-p1", "ashtonbee", analyticsTeamScopeSolo, 4, 1, 3, 0, 13),
				gameIdentityTuningRow(14, "badminton", "head_to_head:s2-p1", "ashtonbee", analyticsTeamScopeSolo, 1, 1, 0, 0, 14),
				gameIdentityTuningRow(15, "badminton", "head_to_head:s2-p2", "ashtonbee", analyticsTeamScopeTeam, 5, 2, 3, 0, 15),
			},
			evaluate: func(projection GameIdentityProjection) []GameIdentityScenarioEvaluation {
				return []GameIdentityScenarioEvaluation{
					gameIdentityEvaluation("squad", "solo squad has 2 participants and 125 CP", gameIdentitySquadSummary(projection, analyticsTeamScopeSolo), "2:125"),
					gameIdentityEvaluation("squad", "team squad has 1 participant and 125 CP", gameIdentitySquadSummary(projection, analyticsTeamScopeTeam), "1:125"),
					gameIdentityEvaluation("squad", "two squad contexts are emitted", len(projection.SquadIdentities), 2),
				}
			},
		},
	}
}

func gameIdentityEvaluation(policyArea string, expected string, actual any, want any) GameIdentityScenarioEvaluation {
	status := gameIdentityPolicyFindingAccepted
	if fmt.Sprint(actual) != fmt.Sprint(want) {
		status = gameIdentityPolicyFindingRejected
	}
	return GameIdentityScenarioEvaluation{
		PolicyArea: policyArea,
		Expected:   expected,
		Actual:     fmt.Sprint(actual),
		Status:     status,
	}
}

func buildGameIdentityDatabaseAnalysis(rows []gameIdentityProjectionRowRecord, input *PublicGameIdentityInput, status string, reason string) GameIdentityDatabaseAnalysis {
	analysis := GameIdentityDatabaseAnalysis{
		Status:        status,
		EvidenceLevel: "none",
		Reason:        reason,
	}
	if input != nil {
		analysis.Input = GameIdentityDatabaseInput{
			SportKey:    input.SportKey,
			ModeKey:     input.ModeKey,
			FacilityKey: input.FacilityKey,
			TeamScope:   input.TeamScope,
			Limit:       input.Limit,
		}
	}
	if status != gameIdentityDatabaseAnalysisIncluded {
		return analysis
	}

	analysis.RowCount = len(rows)
	if len(rows) == 0 {
		analysis.EvidenceLevel = "no_projection_rows"
		analysis.Reason = "Local APOLLO DB analysis ran, but no game identity projection rows matched the filter."
		analysis.Findings = []GameIdentityPolicyFinding{{
			Key:            "db_backed_retune_rejected_no_rows",
			PolicyArea:     "all",
			Status:         gameIdentityPolicyFindingRejected,
			Reason:         "No matched DB rows are available as evidence for active policy retuning.",
			Evidence:       "row_count=0",
			BehaviorChange: false,
		}}
		return analysis
	}

	projection := buildGameIdentityProjectionFromRows(cloneGameIdentityRows(rows), maxGameIdentityLimit, "participant")
	contexts := make(map[gameIdentitySquadKey]struct{})
	for _, row := range rows {
		contexts[gameIdentityRowContextKey(row)] = struct{}{}
	}
	analysis.EvidenceLevel = "local_projection_rows"
	analysis.ContextCount = len(contexts)
	analysis.BadgeAwardCount = len(projection.BadgeAwards)
	analysis.RivalryCount = len(projection.RivalryStates)
	analysis.SquadCount = len(projection.SquadIdentities)
	for index, row := range projection.CP {
		if index == 0 || row.CP < analysis.CPMin {
			analysis.CPMin = row.CP
		}
		if index == 0 || row.CP > analysis.CPMax {
			analysis.CPMax = row.CP
		}
		analysis.CPAverage += float64(row.CP)
	}
	if len(projection.CP) > 0 {
		analysis.CPAverage = roundGameIdentityFloat(analysis.CPAverage / float64(len(projection.CP)))
	}
	for _, rivalry := range projection.RivalryStates {
		switch rivalry.State {
		case "active":
			analysis.ActiveRivalryCount++
		case "emerging":
			analysis.EmergingRivalryCount++
		}
	}
	analysis.Findings = []GameIdentityPolicyFinding{
		{
			Key:            "db_projection_rows_reviewed",
			PolicyArea:     "all",
			Status:         gameIdentityPolicyFindingAccepted,
			Reason:         "Local DB projection rows were evaluated through the active APOLLO game identity policy path.",
			Evidence:       fmt.Sprintf("rows=%d contexts=%d cp_min=%d cp_max=%d", analysis.RowCount, analysis.ContextCount, analysis.CPMin, analysis.CPMax),
			BehaviorChange: false,
		},
		{
			Key:            "db_backed_active_retune_rejected",
			PolicyArea:     "all",
			Status:         gameIdentityPolicyFindingRejected,
			Reason:         "Local rows can seed candidate review, but this report does not prove production population behavior or justify changing active public/member output.",
			Evidence:       "candidate-only DB analysis",
			BehaviorChange: false,
		},
	}
	return analysis
}

func gameIdentityPolicyFindings(databaseAnalysis GameIdentityDatabaseAnalysis) []GameIdentityPolicyFinding {
	dbEvidence := "deterministic fixtures"
	if databaseAnalysis.Status == gameIdentityDatabaseAnalysisIncluded {
		dbEvidence = fmt.Sprintf("deterministic fixtures plus local DB rows=%d", databaseAnalysis.RowCount)
	}
	return []GameIdentityPolicyFinding{
		{
			Key:            "keep_cp_v1_weights",
			PolicyArea:     "cp",
			Status:         gameIdentityPolicyFindingAccepted,
			Reason:         "The current CP weights remain deterministic, explainable, and compatible with existing public/member contracts.",
			Evidence:       dbEvidence,
			BehaviorChange: false,
		},
		{
			Key:            "keep_badge_awards_v1_thresholds",
			PolicyArea:     "badge",
			Status:         gameIdentityPolicyFindingAccepted,
			Reason:         "The current first-match, first-win, and regular-competitor thresholds are low-risk first-version awards over public-safe rows.",
			Evidence:       "badge_thresholds fixture",
			BehaviorChange: false,
		},
		{
			Key:            "keep_rivalry_state_v1_context_guard",
			PolicyArea:     "rivalry",
			Status:         gameIdentityPolicyFindingAccepted,
			Reason:         "Rivalry remains bounded to top-two rows inside the same projection context and does not create a broader social graph.",
			Evidence:       "context_scoped_labels fixture",
			BehaviorChange: false,
		},
		{
			Key:            "keep_squad_identity_v1_projection_only",
			PolicyArea:     "squad",
			Status:         gameIdentityPolicyFindingAccepted,
			Reason:         "Squad identity remains an aggregate projection, not persistent membership or guild truth.",
			Evidence:       "squad_aggregation fixture",
			BehaviorChange: false,
		},
		{
			Key:            "retune_active_constants_now",
			PolicyArea:     "all",
			Status:         gameIdentityPolicyFindingRejected,
			Reason:         "No production population backtest or deployed evidence in this packet justifies changing active public/member behavior.",
			Evidence:       dbEvidence,
			BehaviorChange: false,
		},
		{
			Key:            "add_broader_identity_social_surface",
			PolicyArea:     "all",
			Status:         gameIdentityPolicyFindingRejected,
			Reason:         "Messaging, broad social graph behavior, public tournaments, persistent squads, and public profiles remain out of scope.",
			Evidence:       "packet hard stops",
			BehaviorChange: false,
		},
	}
}

func gameIdentityPolicyRisks() []GameIdentityPolicyRisk {
	return []GameIdentityPolicyRisk{
		{
			Key:             "cp_rewards_volume_and_losses",
			PolicyArea:      "cp",
			Classification:  gameIdentityPolicyRiskModerate,
			Reason:          "Losses still add CP through activity credit, which is explainable but can reward volume.",
			CurrentHandling: "Keep CP separate from rating truth and require production distribution review before retuning.",
		},
		{
			Key:             "badges_are_first_version_low_rarity",
			PolicyArea:      "badge",
			Classification:  gameIdentityPolicyRiskLow,
			Reason:          "Current awards are intentionally simple and may become too common as population grows.",
			CurrentHandling: "Keep thresholds unchanged and treat richer criteria registry work as future scope.",
		},
		{
			Key:             "rivalry_uses_cp_not_head_to_head",
			PolicyArea:      "rivalry",
			Classification:  gameIdentityPolicyRiskHigh,
			Reason:          "The first rivalry state is top-two CP proximity, not a direct head-to-head rivalry engine.",
			CurrentHandling: "Keep the output redacted, context-scoped, and projection-only; block broader social rivalry.",
		},
		{
			Key:             "squad_cp_total_favors_large_contexts",
			PolicyArea:      "squad",
			Classification:  gameIdentityPolicyRiskModerate,
			Reason:          "CP totals scale with participant count and are not normalized squad strength.",
			CurrentHandling: "Keep squad identity as count plus CP total only; require a later packet for normalized squad policy.",
		},
		{
			Key:             "fixture_data_is_not_population_backtest",
			PolicyArea:      "all",
			Classification:  gameIdentityPolicyRiskModerate,
			Reason:          "Deterministic fixtures prove behavior, but not production distribution or incentive effects.",
			CurrentHandling: "Expose optional DB-backed local analysis and document deployed truth as unchanged.",
		},
	}
}

func gameIdentityPolicyBlockers(databaseAnalysis GameIdentityDatabaseAnalysis) []GameIdentityPolicyBlocker {
	dbStatus := "open"
	dbReason := "No DB-backed local projection analysis was included."
	if databaseAnalysis.Status == gameIdentityDatabaseAnalysisIncluded && databaseAnalysis.RowCount > 0 {
		dbStatus = "partial_local_only"
		dbReason = "Local projection rows were analyzed, but deployed and production-load evidence remain absent."
	}
	if databaseAnalysis.Status == gameIdentityDatabaseAnalysisIncluded && databaseAnalysis.RowCount == 0 {
		dbReason = "DB-backed analysis ran but returned no matched projection rows."
	}
	return []GameIdentityPolicyBlocker{
		{
			Key:    "production_population_backtest",
			Status: dbStatus,
			Reason: dbReason,
		},
		{
			Key:    "policy_version_bump_required_for_active_retune",
			Status: "blocked_until_evidence",
			Reason: "Any CP, badge, rivalry, or squad constant change must bump its policy version and prove compatibility.",
		},
		{
			Key:    "frontend_formula_ownership",
			Status: "blocked",
			Reason: "Hestia and Themis remain consumers only and must not compute CP, badges, rivalry, or squad truth.",
		},
		{
			Key:    "broader_identity_social_surface",
			Status: "blocked",
			Reason: "Public tournaments, messaging/chat, broad public social graph, public profiles, and persistent guilds remain out of scope.",
		},
	}
}

func gameIdentityTuningRow(user int, sportKey string, modeKey string, facilityKey string, teamScope string, matches float64, wins float64, losses float64, draws float64, minuteOffset int) gameIdentityProjectionRowRecord {
	computedAt := time.Date(2026, 5, 3, 12, minuteOffset, 0, 0, time.UTC)
	lastResultAt := computedAt.Add(-30 * time.Minute)
	return gameIdentityProjectionRowRecord{
		UserID:        gameIdentityTuningUUID(user),
		SportKey:      sportKey,
		ModeKey:       modeKey,
		FacilityKey:   facilityKey,
		TeamScope:     teamScope,
		MatchesPlayed: matches,
		Wins:          wins,
		Losses:        losses,
		Draws:         draws,
		LastResultAt:  &lastResultAt,
		ComputedAt:    computedAt,
	}
}

func gameIdentityTuningUUID(value int) uuid.UUID {
	var id uuid.UUID
	id[0] = 6
	binary.BigEndian.PutUint64(id[8:], uint64(value))
	return id
}

func cloneGameIdentityRows(rows []gameIdentityProjectionRowRecord) []gameIdentityProjectionRowRecord {
	cloned := make([]gameIdentityProjectionRowRecord, len(rows))
	copy(cloned, rows)
	return cloned
}

func gameIdentityCPAt(projection GameIdentityProjection, index int) int {
	if index < 0 || index >= len(projection.CP) {
		return 0
	}
	return projection.CP[index].CP
}

func gameIdentityBadgeCount(projection GameIdentityProjection, badgeKey string) int {
	count := 0
	for _, award := range projection.BadgeAwards {
		if award.BadgeKey == badgeKey {
			count++
		}
	}
	return count
}

func gameIdentityRivalryStateSummary(projection GameIdentityProjection) string {
	if len(projection.RivalryStates) == 0 {
		return ""
	}
	rivalry := projection.RivalryStates[0]
	return fmt.Sprintf("%s:%d", rivalry.State, rivalry.CPGap)
}

func gameIdentityRivalryParticipants(projection GameIdentityProjection) string {
	participants := make([]string, 0)
	for _, rivalry := range projection.RivalryStates {
		participants = append(participants, rivalry.Participants...)
	}
	return strings.Join(participants, ",")
}

func gameIdentitySquadSummary(projection GameIdentityProjection, teamScope string) string {
	for _, squad := range projection.SquadIdentities {
		if squad.TeamScope == teamScope {
			return fmt.Sprintf("%d:%d", squad.ParticipantCount, squad.CPTotal)
		}
	}
	return ""
}

func roundGameIdentityFloat(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
