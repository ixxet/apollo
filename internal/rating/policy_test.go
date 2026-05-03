package rating

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestActivePolicyFirstCalibrationMatchRemainsProvisional(t *testing.T) {
	playerA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	playerB := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	projection := RebuildActivePolicy([]Match{policyMatch(1, playerA, playerB, "win", "loss", time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC))})

	if projection.PolicyVersion != PolicyVersionActive {
		t.Fatalf("PolicyVersion = %q, want %q", projection.PolicyVersion, PolicyVersionActive)
	}
	if got, want := len(projection.States), 2; got != want {
		t.Fatalf("len(States) = %d, want %d", got, want)
	}
	if projection.States[0].CalibrationStatus != CalibrationStatusProvisional {
		t.Fatalf("CalibrationStatus = %q, want provisional", projection.States[0].CalibrationStatus)
	}
	if projection.Events[0].CalibrationStatus != CalibrationStatusProvisional {
		t.Fatalf("event CalibrationStatus = %q, want provisional", projection.Events[0].CalibrationStatus)
	}
}

func TestActivePolicyFifthCalibrationMatchBecomesRanked(t *testing.T) {
	playerA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	playerB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	startedAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	matches := make([]Match, 0, CalibrationMatchThreshold)
	for index := 0; index < CalibrationMatchThreshold; index++ {
		matches = append(matches, policyMatch(index+1, playerA, playerB, "win", "loss", startedAt.Add(time.Duration(index)*time.Hour)))
	}

	projection := RebuildActivePolicy(matches)

	if projection.States[0].MatchesPlayed != CalibrationMatchThreshold {
		t.Fatalf("MatchesPlayed = %d, want %d", projection.States[0].MatchesPlayed, CalibrationMatchThreshold)
	}
	if projection.States[0].CalibrationStatus != CalibrationStatusRanked {
		t.Fatalf("CalibrationStatus = %q, want ranked", projection.States[0].CalibrationStatus)
	}
	lastPlayerAEvent := projection.Events[len(projection.Events)-2]
	if lastPlayerAEvent.CalibrationStatus != CalibrationStatusRanked {
		t.Fatalf("fifth event CalibrationStatus = %q, want ranked", lastPlayerAEvent.CalibrationStatus)
	}
}

func TestActivePolicyInactivityInflatesSigmaOnly(t *testing.T) {
	playerA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	playerB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	startedAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	noGap := make([]Match, 0, 6)
	withGap := make([]Match, 0, 6)
	for index := 0; index < 5; index++ {
		noGap = append(noGap, policyMatch(index+1, playerA, playerB, "win", "loss", startedAt.Add(time.Duration(index)*time.Hour)))
		withGap = append(withGap, policyMatch(index+1, playerA, playerB, "win", "loss", startedAt.Add(time.Duration(index)*time.Hour)))
	}
	noGap = append(noGap, policyMatch(6, playerA, playerB, "loss", "win", startedAt.Add(5*time.Hour)))
	withGap = append(withGap, policyMatch(6, playerA, playerB, "loss", "win", startedAt.Add((InactivityThresholdDays+1)*24*time.Hour)))

	normalProjection := RebuildActivePolicy(noGap)
	decayedProjection := RebuildActivePolicy(withGap)

	if math.Abs(normalProjection.States[0].Mu-decayedProjection.States[0].Mu) > 0.000001 {
		t.Fatalf("inactive mu changed by decay: normal %.10f decayed %.10f", normalProjection.States[0].Mu, decayedProjection.States[0].Mu)
	}
	if decayedProjection.States[0].Sigma <= normalProjection.States[0].Sigma {
		t.Fatalf("decayed sigma = %.10f, want above normal %.10f", decayedProjection.States[0].Sigma, normalProjection.States[0].Sigma)
	}
	if decayedProjection.States[0].Sigma > InitialSigma {
		t.Fatalf("decayed sigma = %.10f, want capped at %.10f", decayedProjection.States[0].Sigma, InitialSigma)
	}
	if decayedProjection.States[0].LastInactivityDecayAt == nil || decayedProjection.States[0].InactivityDecayCount != 1 {
		t.Fatalf("decay metadata = at:%v count:%d, want one decay", decayedProjection.States[0].LastInactivityDecayAt, decayedProjection.States[0].InactivityDecayCount)
	}
	lastPlayerAEvent := decayedProjection.Events[len(decayedProjection.Events)-2]
	if !lastPlayerAEvent.InactivityDecayApplied {
		t.Fatalf("InactivityDecayApplied = false, want true")
	}
	if lastPlayerAEvent.DeltaMu != normalProjection.Events[len(normalProjection.Events)-2].DeltaMu {
		t.Fatalf("decay changed match delta mu = %.10f, want %.10f", lastPlayerAEvent.DeltaMu, normalProjection.Events[len(normalProjection.Events)-2].DeltaMu)
	}
}

func TestActivePolicyClimbingCapBoundsPositiveMovement(t *testing.T) {
	playerA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	playerB := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	projection := RebuildActivePolicy([]Match{policyMatch(1, playerA, playerB, "win", "loss", time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC))})

	winnerEvent := projection.Events[0]
	if winnerEvent.DeltaMu != MaxPositiveMuDeltaPerResult {
		t.Fatalf("winner DeltaMu = %.10f, want cap %.10f", winnerEvent.DeltaMu, MaxPositiveMuDeltaPerResult)
	}
	if !winnerEvent.ClimbingCapApplied || !projection.States[0].ClimbingCapApplied {
		t.Fatalf("climbing cap metadata missing: event=%t state=%t", winnerEvent.ClimbingCapApplied, projection.States[0].ClimbingCapApplied)
	}
	loserEvent := projection.Events[1]
	if loserEvent.DeltaMu >= 0 || loserEvent.ClimbingCapApplied {
		t.Fatalf("loser event = %+v, want negative uncapped movement", loserEvent)
	}
}

func TestActivePolicyKeepsOpenSkillComparisonSeparate(t *testing.T) {
	playerA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	playerB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	matches := []Match{policyMatch(1, playerA, playerB, "win", "loss", time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC))}

	active := RebuildActivePolicy(matches)
	legacyBaseline := RebuildLegacy(matches)
	comparison := RebuildOpenSkillComparison(matches, legacyBaseline)

	if active.RatingEngine != EngineLegacyEloLike || active.PolicyVersion != PolicyVersionActive {
		t.Fatalf("active projection = %s/%s, want legacy engine with active policy", active.RatingEngine, active.PolicyVersion)
	}
	if legacyBaseline.PolicyVersion != PolicyVersionLegacy {
		t.Fatalf("legacy baseline PolicyVersion = %q, want %q", legacyBaseline.PolicyVersion, PolicyVersionLegacy)
	}
	if got, want := len(comparison.Facts), len(legacyBaseline.Events); got != want {
		t.Fatalf("comparison facts = %d, want %d sidecar facts", got, want)
	}
	if comparison.OpenSkillStates[0].Mu == active.States[0].Mu {
		t.Fatalf("OpenSkill state matched active state unexpectedly; comparison must remain sidecar")
	}
}

func policyMatch(index int, playerA uuid.UUID, playerB uuid.UUID, outcomeA string, outcomeB string, recordedAt time.Time) Match {
	return Match{
		CompetitionMatchID: uuid.MustParse("aaaaaaaa-1111-1111-1111-111111111111"),
		SourceResultID:     uuid.MustParse("99999999-9999-9999-9999-99999999999" + string(rune('0'+index))),
		ModeKey:            "head_to_head:s2-p1",
		RecordedAt:         recordedAt,
		Sides: []Side{
			{
				CompetitionSessionTeamID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
				SideIndex:                1,
				Outcome:                  outcomeA,
				UserIDs:                  []uuid.UUID{playerA},
			},
			{
				CompetitionSessionTeamID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
				SideIndex:                2,
				Outcome:                  outcomeB,
				UserIDs:                  []uuid.UUID{playerB},
			},
		},
	}
}
