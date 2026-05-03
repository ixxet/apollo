package rating

import "testing"

func TestActivePolicySimulationReportCoversRequiredScenarios(t *testing.T) {
	report := BuildActivePolicySimulationReport()

	if report.ReportVersion != SimulationReportVersion {
		t.Fatalf("ReportVersion = %q, want %q", report.ReportVersion, SimulationReportVersion)
	}
	if report.ActiveRatingEngine != EngineLegacyEloLike || report.ActivePolicyVersion != PolicyVersionActive {
		t.Fatalf("active policy report = %s/%s, want legacy engine active wrapper", report.ActiveRatingEngine, report.ActivePolicyVersion)
	}
	if report.LegacyPolicyVersion != PolicyVersionLegacy || report.OpenSkillPolicyVersion != PolicyVersionOpenSkill {
		t.Fatalf("sidecar versions = legacy:%s openskill:%s", report.LegacyPolicyVersion, report.OpenSkillPolicyVersion)
	}
	if report.PolicyConstants.CalibrationMatchThreshold != CalibrationMatchThreshold ||
		report.PolicyConstants.InactivityThresholdDays != InactivityThresholdDays ||
		report.PolicyConstants.InactivitySigmaInflationFactor != InactivitySigmaInflationFactor ||
		report.PolicyConstants.MaxPositiveMuDeltaPerResult != MaxPositiveMuDeltaPerResult {
		t.Fatalf("policy constants changed in simulation report: %+v", report.PolicyConstants)
	}

	required := []string{
		"unranked_1v1_a_wins",
		"stronger_beats_new",
		"new_beats_stronger",
		"fifth_match_ranked_transition",
		"inactive_return_after_threshold",
		"climbing_cap_activation",
		"draw_handling",
		"five_v_five_even_teams",
		"three_v_five_asymmetric_comparison_stress",
		"public_member_read_safety",
	}
	scenarios := scenariosByLabel(report)
	for _, label := range required {
		scenario, exists := scenarios[label]
		if !exists {
			t.Fatalf("missing simulation scenario %q", label)
		}
		if scenario.ActivePolicy.PolicyVersion != PolicyVersionActive {
			t.Fatalf("%s active policy version = %q, want %q", label, scenario.ActivePolicy.PolicyVersion, PolicyVersionActive)
		}
		if scenario.LegacyBaseline.PolicyVersion != PolicyVersionLegacy {
			t.Fatalf("%s legacy policy version = %q, want %q", label, scenario.LegacyBaseline.PolicyVersion, PolicyVersionLegacy)
		}
		if len(scenario.OpenSkillDeltas) == 0 {
			t.Fatalf("%s has no OpenSkill sidecar delta rows", label)
		}
	}
	if got, want := report.Summary.ScenarioCount, len(required); got != want {
		t.Fatalf("ScenarioCount = %d, want %d", got, want)
	}
	if got, want := report.Summary.AcceptedScenarioCount, 9; got != want {
		t.Fatalf("AcceptedScenarioCount = %d, want %d", got, want)
	}
	if got, want := report.Summary.RejectedScenarioCount, 1; got != want {
		t.Fatalf("RejectedScenarioCount = %d, want %d", got, want)
	}
	if got, want := report.Summary.ComparisonOnlyCount, 1; got != want {
		t.Fatalf("ComparisonOnlyCount = %d, want %d", got, want)
	}
	if got, want := report.Summary.OpenSkillDeltaRowCount, 60; got != want {
		t.Fatalf("OpenSkillDeltaRowCount = %d, want %d", got, want)
	}
	if len(report.CutoverBlockers) == 0 || len(report.PolicyRisks) == 0 {
		t.Fatalf("cutover blockers or policy risks missing: blockers=%d risks=%d", len(report.CutoverBlockers), len(report.PolicyRisks))
	}
}

func TestActivePolicySimulationGoldenScenarioOutputs(t *testing.T) {
	report := BuildActivePolicySimulationReport()
	scenarios := scenariosByLabel(report)

	unranked := scenarios["unranked_1v1_a_wins"]
	if unranked.AcceptanceStatus != ScenarioAcceptanceAccepted {
		t.Fatalf("unranked acceptance = %q", unranked.AcceptanceStatus)
	}
	assertSimulationState(t, unranked.ActivePolicy.States[0], "player_a", 26.5, 8, 1, CalibrationStatusProvisional, true)
	assertSimulationState(t, unranked.ActivePolicy.States[1], "player_b", 23, 8, 1, CalibrationStatusProvisional, false)
	assertSimulationLegacyDelta(t, unranked.LegacyDeltas[0], "player_a", -0.5, 0, true)
	assertSimulationOpenSkillDelta(t, unranked.OpenSkillDeltas[0], "player_a", 0.6354, false)

	stronger := scenarios["stronger_beats_new"]
	if stronger.AcceptanceStatus != ScenarioAcceptanceAccepted || stronger.ParticipantCount != 3 {
		t.Fatalf("stronger scenario = %+v, want accepted three-player proof", stronger)
	}
	if stronger.ActivePolicy.States[0].MatchesPlayed != CalibrationMatchThreshold || stronger.ActivePolicy.States[0].CalibrationStatus != CalibrationStatusRanked {
		t.Fatalf("stronger player state = %+v, want ranked after fifth match", stronger.ActivePolicy.States[0])
	}

	upset := scenarios["new_beats_stronger"]
	if upset.AcceptanceStatus != ScenarioAcceptanceAccepted {
		t.Fatalf("upset acceptance = %q", upset.AcceptanceStatus)
	}
	assertSimulationEvent(t, upset.ActivePolicy.LastEvents[2], "player_c", MaxPositiveMuDeltaPerResult, true)

	fifth := scenarios["fifth_match_ranked_transition"]
	for _, state := range fifth.ActivePolicy.States {
		if state.MatchesPlayed != CalibrationMatchThreshold || state.CalibrationStatus != CalibrationStatusRanked {
			t.Fatalf("fifth-match state = %+v, want ranked at threshold", state)
		}
	}

	inactive := scenarios["inactive_return_after_threshold"]
	assertSimulationState(t, inactive.ActivePolicy.States[0], "player_a", 24.0693, 8, 2, CalibrationStatusProvisional, false)
	if inactive.ActivePolicy.States[0].InactivityDecayCount != 1 || inactive.ActivePolicy.States[0].LastInactivityDecayAt == nil {
		t.Fatalf("inactive state = %+v, want one decay metadata row", inactive.ActivePolicy.States[0])
	}
	if !inactive.ActivePolicy.LastEvents[0].InactivityDecayApplied {
		t.Fatalf("inactive last event = %+v, want decay applied", inactive.ActivePolicy.LastEvents[0])
	}

	draw := scenarios["draw_handling"]
	assertSimulationEvent(t, draw.ActivePolicy.LastEvents[0], "player_a", 0, false)
	assertSimulationState(t, draw.ActivePolicy.States[0], "player_a", 25, 8, 1, CalibrationStatusProvisional, false)

	fiveVFive := scenarios["five_v_five_even_teams"]
	if fiveVFive.ParticipantCount != 10 || len(fiveVFive.ActivePolicy.States) != 10 {
		t.Fatalf("five_v_five shape = participants:%d states:%d", fiveVFive.ParticipantCount, len(fiveVFive.ActivePolicy.States))
	}
	assertSimulationState(t, fiveVFive.ActivePolicy.States[0], "player_a", 26.5, 8, 1, CalibrationStatusProvisional, true)
	assertSimulationState(t, fiveVFive.ActivePolicy.States[9], "player_j", 23, 8, 1, CalibrationStatusProvisional, false)

	asymmetric := scenarios["three_v_five_asymmetric_comparison_stress"]
	if asymmetric.AcceptanceStatus != ScenarioAcceptanceRejected || asymmetric.Classification != ScenarioClassificationComparisonOnly {
		t.Fatalf("asymmetric scenario = %+v, want rejected comparison-only stress", asymmetric)
	}
	if len(asymmetric.OpenSkillDeltas) != 8 || len(asymmetric.Blockers) == 0 {
		t.Fatalf("asymmetric sidecar rows/blockers = %d/%d", len(asymmetric.OpenSkillDeltas), len(asymmetric.Blockers))
	}
}

func TestActivePolicySimulationKeepsOpenSkillComparisonSidecarOnly(t *testing.T) {
	report := BuildActivePolicySimulationReport()

	for _, scenario := range report.Scenarios {
		if scenario.ActivePolicy.RatingEngine == EngineOpenSkill || scenario.ActivePolicy.PolicyVersion == PolicyVersionOpenSkill {
			t.Fatalf("%s active policy switched to OpenSkill: %+v", scenario.Label, scenario.ActivePolicy)
		}
		if scenario.LegacyBaseline.PolicyVersion != PolicyVersionLegacy {
			t.Fatalf("%s legacy baseline changed: %+v", scenario.Label, scenario.LegacyBaseline)
		}
		for _, delta := range scenario.OpenSkillDeltas {
			if delta.AcceptedDeltaBudget != AcceptedOpenSkillDeltaBudget {
				t.Fatalf("%s delta budget = %.4f, want %.4f", scenario.Label, delta.AcceptedDeltaBudget, AcceptedOpenSkillDeltaBudget)
			}
			if delta.OpenSkillMu == 0 || delta.OpenSkillSigma == 0 {
				t.Fatalf("%s OpenSkill sidecar delta missing values: %+v", scenario.Label, delta)
			}
		}
	}
}

func scenariosByLabel(report SimulationReport) map[string]SimulationScenarioReport {
	scenarios := make(map[string]SimulationScenarioReport, len(report.Scenarios))
	for _, scenario := range report.Scenarios {
		scenarios[scenario.Label] = scenario
	}
	return scenarios
}

func assertSimulationState(t *testing.T, state SimulationRatingState, participant string, mu float64, sigma float64, matchesPlayed int, calibrationStatus string, climbingCapApplied bool) {
	t.Helper()

	if state.Participant != participant {
		t.Fatalf("state.Participant = %q, want %q", state.Participant, participant)
	}
	if state.Mu != mu {
		t.Fatalf("%s state.Mu = %.4f, want %.4f", participant, state.Mu, mu)
	}
	if state.Sigma != sigma {
		t.Fatalf("%s state.Sigma = %.4f, want %.4f", participant, state.Sigma, sigma)
	}
	if state.MatchesPlayed != matchesPlayed {
		t.Fatalf("%s state.MatchesPlayed = %d, want %d", participant, state.MatchesPlayed, matchesPlayed)
	}
	if state.CalibrationStatus != calibrationStatus {
		t.Fatalf("%s state.CalibrationStatus = %q, want %q", participant, state.CalibrationStatus, calibrationStatus)
	}
	if state.ClimbingCapApplied != climbingCapApplied {
		t.Fatalf("%s state.ClimbingCapApplied = %t, want %t", participant, state.ClimbingCapApplied, climbingCapApplied)
	}
}

func assertSimulationEvent(t *testing.T, event SimulationComputedEvent, participant string, deltaMu float64, climbingCapApplied bool) {
	t.Helper()

	if event.Participant != participant {
		t.Fatalf("event.Participant = %q, want %q", event.Participant, participant)
	}
	if event.DeltaMu != deltaMu {
		t.Fatalf("%s event.DeltaMu = %.4f, want %.4f", participant, event.DeltaMu, deltaMu)
	}
	if event.ClimbingCapApplied != climbingCapApplied {
		t.Fatalf("%s event.ClimbingCapApplied = %t, want %t", participant, event.ClimbingCapApplied, climbingCapApplied)
	}
}

func assertSimulationLegacyDelta(t *testing.T, delta SimulationLegacyDelta, participant string, deltaMu float64, deltaSigma float64, climbingCapApplied bool) {
	t.Helper()

	if delta.Participant != participant {
		t.Fatalf("delta.Participant = %q, want %q", delta.Participant, participant)
	}
	if delta.DeltaMu != deltaMu {
		t.Fatalf("%s legacy DeltaMu = %.4f, want %.4f", participant, delta.DeltaMu, deltaMu)
	}
	if delta.DeltaSigma != deltaSigma {
		t.Fatalf("%s legacy DeltaSigma = %.4f, want %.4f", participant, delta.DeltaSigma, deltaSigma)
	}
	if delta.ClimbingCapApplied != climbingCapApplied {
		t.Fatalf("%s legacy ClimbingCapApplied = %t, want %t", participant, delta.ClimbingCapApplied, climbingCapApplied)
	}
}

func assertSimulationOpenSkillDelta(t *testing.T, delta SimulationOpenSkillDelta, participant string, deltaFromLegacy float64, deltaFlagged bool) {
	t.Helper()

	if delta.Participant != participant {
		t.Fatalf("delta.Participant = %q, want %q", delta.Participant, participant)
	}
	if delta.DeltaFromLegacy != deltaFromLegacy {
		t.Fatalf("%s OpenSkill DeltaFromLegacy = %.4f, want %.4f", participant, delta.DeltaFromLegacy, deltaFromLegacy)
	}
	if delta.DeltaFlagged != deltaFlagged {
		t.Fatalf("%s OpenSkill DeltaFlagged = %t, want %t", participant, delta.DeltaFlagged, deltaFlagged)
	}
}
