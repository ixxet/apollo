package rating

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestOpenSkillComparisonGoldenSinglesWin(t *testing.T) {
	playerA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	playerB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	teamA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	teamB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	resultID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	recordedAt := time.Date(2026, 4, 27, 10, 30, 0, 0, time.UTC)

	matches := []Match{{
		CompetitionMatchID: uuid.MustParse("aaaaaaaa-1111-1111-1111-111111111111"),
		SourceResultID:     resultID,
		ModeKey:            "head_to_head:s2-p1",
		RecordedAt:         recordedAt,
		Sides: []Side{
			{CompetitionSessionTeamID: teamA, SideIndex: 1, Outcome: "win", UserIDs: []uuid.UUID{playerA}},
			{CompetitionSessionTeamID: teamB, SideIndex: 2, Outcome: "loss", UserIDs: []uuid.UUID{playerB}},
		},
	}}

	comparison := RebuildOpenSkillComparison(matches, RebuildLegacy(matches))
	if comparison.Watermark != "2026-04-27T10:30:00Z#99999999-9999-9999-9999-999999999999" {
		t.Fatalf("Watermark = %q", comparison.Watermark)
	}
	if got, want := len(comparison.Facts), 2; got != want {
		t.Fatalf("len(Facts) = %d, want %d", got, want)
	}

	assertComparisonFact(t, comparison.Facts[0], playerA, resultID, 27.0, 7.999968, 27.6353768, 8.0658698, 0.6353768, false)
	assertComparisonFact(t, comparison.Facts[1], playerB, resultID, 23.0, 7.999968, 22.3646232, 8.0658698, -0.6353768, false)
}

func TestOpenSkillComparisonDeltaBudgetFlagsExplicitly(t *testing.T) {
	playerA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	playerB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	resultID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	matches := []Match{{
		CompetitionMatchID: uuid.MustParse("aaaaaaaa-1111-1111-1111-111111111111"),
		SourceResultID:     resultID,
		ModeKey:            "head_to_head:s2-p1",
		RecordedAt:         time.Date(2026, 4, 27, 10, 30, 0, 0, time.UTC),
		Sides: []Side{
			{CompetitionSessionTeamID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), SideIndex: 1, Outcome: "win", UserIDs: []uuid.UUID{playerA}},
			{CompetitionSessionTeamID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), SideIndex: 2, Outcome: "loss", UserIDs: []uuid.UUID{playerB}},
		},
	}}

	comparison := RebuildOpenSkillComparisonWithBudget(matches, RebuildLegacy(matches), 0.5)
	for _, fact := range comparison.Facts {
		if !fact.DeltaFlagged {
			t.Fatalf("fact for %s DeltaFlagged = false, want true", fact.UserID)
		}
		if fact.AcceptedDeltaBudget != 0.5 {
			t.Fatalf("fact.AcceptedDeltaBudget = %.4f, want 0.5000", fact.AcceptedDeltaBudget)
		}
	}
}

func TestOpenSkillComparisonDeterministic(t *testing.T) {
	match := Match{
		CompetitionMatchID: uuid.MustParse("aaaaaaaa-1111-1111-1111-111111111111"),
		SourceResultID:     uuid.MustParse("99999999-9999-9999-9999-999999999999"),
		ModeKey:            "head_to_head:s2-p2",
		RecordedAt:         time.Date(2026, 4, 27, 10, 30, 0, 0, time.UTC),
		Sides: []Side{
			{
				CompetitionSessionTeamID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
				SideIndex:                1,
				Outcome:                  "draw",
				UserIDs: []uuid.UUID{
					uuid.MustParse("11111111-1111-1111-1111-111111111111"),
					uuid.MustParse("22222222-2222-2222-2222-222222222222"),
				},
			},
			{
				CompetitionSessionTeamID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
				SideIndex:                2,
				Outcome:                  "draw",
				UserIDs: []uuid.UUID{
					uuid.MustParse("33333333-3333-3333-3333-333333333333"),
					uuid.MustParse("44444444-4444-4444-4444-444444444444"),
				},
			},
		},
	}
	matches := []Match{match}
	legacy := RebuildLegacy(matches)

	first := RebuildOpenSkillComparison(matches, legacy)
	second := RebuildOpenSkillComparison(matches, legacy)
	if first.Watermark != second.Watermark || len(first.OpenSkillStates) != len(second.OpenSkillStates) || len(first.Facts) != len(second.Facts) {
		t.Fatalf("comparison shape changed: first=%#v second=%#v", first, second)
	}
	for index := range first.OpenSkillStates {
		assertState(t, second.OpenSkillStates[index], first.OpenSkillStates[index].UserID, first.OpenSkillStates[index].Mu, first.OpenSkillStates[index].Sigma, first.OpenSkillStates[index].MatchesPlayed, first.OpenSkillStates[index].SourceResultID)
	}
	for index := range first.Facts {
		assertComparisonFact(t, second.Facts[index], first.Facts[index].UserID, first.Facts[index].SourceResultID, first.Facts[index].LegacyMu, first.Facts[index].LegacySigma, first.Facts[index].OpenSkillMu, first.Facts[index].OpenSkillSigma, first.Facts[index].DeltaFromLegacy, first.Facts[index].DeltaFlagged)
	}
}

func assertComparisonFact(t *testing.T, fact ComparisonFact, userID uuid.UUID, sourceResultID uuid.UUID, legacyMu float64, legacySigma float64, openSkillMu float64, openSkillSigma float64, deltaFromLegacy float64, deltaFlagged bool) {
	t.Helper()

	if fact.UserID != userID {
		t.Fatalf("fact.UserID = %s, want %s", fact.UserID, userID)
	}
	if fact.SourceResultID != sourceResultID {
		t.Fatalf("fact.SourceResultID = %s, want %s", fact.SourceResultID, sourceResultID)
	}
	if math.Abs(fact.LegacyMu-legacyMu) > 0.000001 {
		t.Fatalf("fact.LegacyMu = %.10f, want %.10f", fact.LegacyMu, legacyMu)
	}
	if math.Abs(fact.LegacySigma-legacySigma) > 0.000001 {
		t.Fatalf("fact.LegacySigma = %.10f, want %.10f", fact.LegacySigma, legacySigma)
	}
	if math.Abs(fact.OpenSkillMu-openSkillMu) > 0.000001 {
		t.Fatalf("fact.OpenSkillMu = %.10f, want %.10f", fact.OpenSkillMu, openSkillMu)
	}
	if math.Abs(fact.OpenSkillSigma-openSkillSigma) > 0.000001 {
		t.Fatalf("fact.OpenSkillSigma = %.10f, want %.10f", fact.OpenSkillSigma, openSkillSigma)
	}
	if math.Abs(fact.DeltaFromLegacy-deltaFromLegacy) > 0.000001 {
		t.Fatalf("fact.DeltaFromLegacy = %.10f, want %.10f", fact.DeltaFromLegacy, deltaFromLegacy)
	}
	if fact.DeltaFlagged != deltaFlagged {
		t.Fatalf("fact.DeltaFlagged = %t, want %t", fact.DeltaFlagged, deltaFlagged)
	}
}
