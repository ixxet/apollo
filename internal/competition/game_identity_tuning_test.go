package competition

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestGameIdentityPolicyTuningReportCoversCurrentPoliciesWithoutRetuning(t *testing.T) {
	report := BuildGameIdentityPolicyTuningReport()

	if report.ReportVersion != GameIdentityPolicyTuningReportVersion {
		t.Fatalf("ReportVersion = %q, want %q", report.ReportVersion, GameIdentityPolicyTuningReportVersion)
	}
	if report.CPPolicyVersion != gameIdentityCPPolicyVersion ||
		report.BadgePolicyVersion != gameIdentityBadgePolicyVersion ||
		report.RivalryPolicyVersion != gameIdentityRivalryPolicyVersion ||
		report.SquadPolicyVersion != gameIdentitySquadPolicyVersion {
		t.Fatalf("policy versions = %+v, want active game identity policy versions", report)
	}
	if report.ActiveBehaviorChange {
		t.Fatal("ActiveBehaviorChange = true, want tuning report to leave active output unchanged")
	}
	if report.Summary.ScenarioCount != 6 || report.Summary.RejectedScenarioCount != 0 {
		t.Fatalf("summary scenarios = %+v, want six accepted deterministic scenarios", report.Summary)
	}
	if report.Summary.AcceptedFindingCount != 4 || report.Summary.RejectedFindingCount != 2 {
		t.Fatalf("summary findings = %+v, want accepted and rejected policy findings", report.Summary)
	}
	if report.PolicyConstants.RivalryActivation.ActiveCPGapMax != 50 {
		t.Fatalf("ActiveCPGapMax = %d, want 50", report.PolicyConstants.RivalryActivation.ActiveCPGapMax)
	}
	if got, want := report.PolicyConstants.CPWeights, []GameIdentityCPWeight{
		{Metric: analyticsStatMatchesPlayed, PointsPerUnit: gameIdentityCPMatchesPlayedPoints},
		{Metric: analyticsStatWins, PointsPerUnit: gameIdentityCPWinPoints},
		{Metric: analyticsStatDraws, PointsPerUnit: gameIdentityCPDrawPoints},
		{Metric: analyticsStatLosses, PointsPerUnit: gameIdentityCPLossPoints},
	}; !slices.Equal(got, want) {
		t.Fatalf("CPWeights = %+v, want %+v", got, want)
	}

	scenarios := gameIdentityScenariosByLabel(report)
	cpScenario, ok := scenarios["cp_weight_balance"]
	if !ok {
		t.Fatalf("missing cp_weight_balance scenario in %+v", report.Scenarios)
	}
	if len(cpScenario.CP) != 2 || cpScenario.CP[0].CP != 125 || cpScenario.RivalryStates[0].State != "active" || cpScenario.SquadIdentities[0].CPTotal != 205 {
		t.Fatalf("cp_weight_balance scenario = %+v, want deterministic CP/rivalry/squad proof", cpScenario)
	}
	if scenarios["rivalry_active_gap_boundary"].RivalryStates[0].CPGap != 50 ||
		scenarios["rivalry_active_gap_boundary"].RivalryStates[0].State != "active" {
		t.Fatalf("rivalry boundary scenario = %+v, want inclusive active gap", scenarios["rivalry_active_gap_boundary"])
	}
	if scenarios["rivalry_emerging_gap"].RivalryStates[0].State != "emerging" {
		t.Fatalf("rivalry emerging scenario = %+v, want emerging state", scenarios["rivalry_emerging_gap"])
	}

	foundRetuneRejection := false
	for _, finding := range report.Findings {
		if finding.Key == "retune_active_constants_now" && finding.Status == gameIdentityPolicyFindingRejected && !finding.BehaviorChange {
			foundRetuneRejection = true
		}
	}
	if !foundRetuneRejection {
		t.Fatalf("findings = %+v, want rejected no-retune finding", report.Findings)
	}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal(report) error = %v", err)
	}
	body := strings.ToLower(string(raw))
	for _, forbidden := range []string{
		"user_id",
		"source_result_id",
		"canonical_result_id",
		"openskill",
		"trusted_surface",
		"command_readiness",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("tuning report leaked %q: %s", forbidden, body)
		}
	}
}

func TestGameIdentityPolicyTuningReportCanIncludeDBBackedRows(t *testing.T) {
	computedAt := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	var captured GameIdentityProjectionInput
	svc := NewService(stubStore{
		gameIdentityRows: func(_ context.Context, input GameIdentityProjectionInput) ([]gameIdentityProjectionRowRecord, error) {
			captured = input
			return []gameIdentityProjectionRowRecord{
				{
					UserID:        gameIdentityTuningUUID(30),
					SportKey:      "badminton",
					ModeKey:       "head_to_head:s2-p1",
					FacilityKey:   "ashtonbee",
					TeamScope:     analyticsDimensionAll,
					MatchesPlayed: 5,
					Wins:          2,
					Losses:        3,
					ComputedAt:    computedAt,
				},
				{
					UserID:        gameIdentityTuningUUID(31),
					SportKey:      "badminton",
					ModeKey:       "head_to_head:s2-p1",
					FacilityKey:   "ashtonbee",
					TeamScope:     analyticsDimensionAll,
					MatchesPlayed: 2,
					Wins:          2,
					ComputedAt:    computedAt.Add(time.Minute),
				},
			}, nil
		},
	})

	report, err := svc.GameIdentityPolicyTuningReport(context.Background(), PublicGameIdentityInput{
		TeamScope: "private_scope",
		Limit:     1000,
	})
	if err != nil {
		t.Fatalf("GameIdentityPolicyTuningReport() error = %v", err)
	}
	if captured.TeamScope != analyticsDimensionAll || captured.Limit != maxGameIdentityLimit {
		t.Fatalf("captured input = %+v, want normalized public-safe scope and limit", captured)
	}
	if !report.Summary.DBBackedAnalysis || report.DatabaseAnalysis.Status != gameIdentityDatabaseAnalysisIncluded {
		t.Fatalf("database analysis = %+v, want included DB-backed analysis", report.DatabaseAnalysis)
	}
	if report.DatabaseAnalysis.RowCount != 2 || report.DatabaseAnalysis.CPMax != 125 || report.DatabaseAnalysis.CPMin != 80 {
		t.Fatalf("database analysis = %+v, want aggregate CP evidence", report.DatabaseAnalysis)
	}
	if report.DatabaseAnalysis.ActiveRivalryCount != 1 || report.DatabaseAnalysis.BadgeAwardCount != 5 {
		t.Fatalf("database analysis = %+v, want active rivalry and badge counts", report.DatabaseAnalysis)
	}
	if got, want := len(report.DatabaseAnalysis.Findings), 2; got != want {
		t.Fatalf("len(database findings) = %d, want %d", got, want)
	}
}

func gameIdentityScenariosByLabel(report GameIdentityPolicyTuningReport) map[string]GameIdentityTuningScenario {
	scenarios := make(map[string]GameIdentityTuningScenario, len(report.Scenarios))
	for _, scenario := range report.Scenarios {
		scenarios[scenario.Label] = scenario
	}
	return scenarios
}
