package rating

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLegacyRatingGoldenSinglesWin(t *testing.T) {
	playerA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	playerB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	teamA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	teamB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	resultID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	recordedAt := time.Date(2026, 4, 27, 10, 30, 0, 0, time.UTC)

	projection := RebuildLegacy([]Match{{
		CompetitionMatchID: uuid.MustParse("aaaaaaaa-1111-1111-1111-111111111111"),
		SourceResultID:     resultID,
		ModeKey:            "head_to_head:s2-p1",
		RecordedAt:         recordedAt,
		Sides: []Side{
			{CompetitionSessionTeamID: teamA, SideIndex: 1, Outcome: "win", UserIDs: []uuid.UUID{playerA}},
			{CompetitionSessionTeamID: teamB, SideIndex: 2, Outcome: "loss", UserIDs: []uuid.UUID{playerB}},
		},
	}})

	if projection.Watermark != "2026-04-27T10:30:00Z#99999999-9999-9999-9999-999999999999" {
		t.Fatalf("Watermark = %q", projection.Watermark)
	}
	if got, want := len(projection.States), 2; got != want {
		t.Fatalf("len(States) = %d, want %d", got, want)
	}
	assertState(t, projection.States[0], playerA, 27.0, 7.999968, 1, resultID)
	assertState(t, projection.States[1], playerB, 23.0, 7.999968, 1, resultID)

	if got, want := len(projection.Events), 2; got != want {
		t.Fatalf("len(Events) = %d, want %d", got, want)
	}
	assertEvent(t, projection.Events[0], playerA, resultID, 27.0, 7.999968, 2.0, -0.333332)
	assertEvent(t, projection.Events[1], playerB, resultID, 23.0, 7.999968, -2.0, -0.333332)
}

func TestLegacyRatingGoldenComebackChain(t *testing.T) {
	playerA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	playerB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	teamA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	teamB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	firstResultID := uuid.MustParse("99999999-9999-9999-9999-999999999991")
	secondResultID := uuid.MustParse("99999999-9999-9999-9999-999999999992")

	projection := RebuildLegacy([]Match{
		{
			CompetitionMatchID: uuid.MustParse("aaaaaaaa-1111-1111-1111-111111111111"),
			SourceResultID:     firstResultID,
			ModeKey:            "head_to_head:s2-p1",
			RecordedAt:         time.Date(2026, 4, 27, 10, 30, 0, 0, time.UTC),
			Sides: []Side{
				{CompetitionSessionTeamID: teamA, SideIndex: 1, Outcome: "win", UserIDs: []uuid.UUID{playerA}},
				{CompetitionSessionTeamID: teamB, SideIndex: 2, Outcome: "loss", UserIDs: []uuid.UUID{playerB}},
			},
		},
		{
			CompetitionMatchID: uuid.MustParse("aaaaaaaa-2222-2222-2222-222222222222"),
			SourceResultID:     secondResultID,
			ModeKey:            "head_to_head:s2-p1",
			RecordedAt:         time.Date(2026, 4, 27, 11, 30, 0, 0, time.UTC),
			Sides: []Side{
				{CompetitionSessionTeamID: teamA, SideIndex: 1, Outcome: "loss", UserIDs: []uuid.UUID{playerA}},
				{CompetitionSessionTeamID: teamB, SideIndex: 2, Outcome: "win", UserIDs: []uuid.UUID{playerB}},
			},
		},
	})

	if projection.Watermark != "2026-04-27T11:30:00Z#99999999-9999-9999-9999-999999999992" {
		t.Fatalf("Watermark = %q", projection.Watermark)
	}
	assertState(t, projection.States[0], playerA, 24.5101627, 7.67996928, 2, secondResultID)
	assertState(t, projection.States[1], playerB, 25.4898373, 7.67996928, 2, secondResultID)
}

func TestLegacyRatingProjectionIsDeterministic(t *testing.T) {
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

	first := RebuildLegacy([]Match{match})
	second := RebuildLegacy([]Match{match})
	if first.Watermark != second.Watermark {
		t.Fatalf("watermark changed: first=%q second=%q", first.Watermark, second.Watermark)
	}
	if len(first.States) != len(second.States) || len(first.Events) != len(second.Events) {
		t.Fatalf("projection shape changed: first=%#v second=%#v", first, second)
	}
	for index := range first.States {
		assertState(t, second.States[index], first.States[index].UserID, first.States[index].Mu, first.States[index].Sigma, first.States[index].MatchesPlayed, first.States[index].SourceResultID)
	}
	for index := range first.Events {
		assertEvent(t, second.Events[index], first.Events[index].UserID, first.Events[index].SourceResultID, first.Events[index].Mu, first.Events[index].Sigma, first.Events[index].DeltaMu, first.Events[index].DeltaSigma)
	}
}

func assertState(t *testing.T, state State, userID uuid.UUID, mu float64, sigma float64, matchesPlayed int, sourceResultID uuid.UUID) {
	t.Helper()

	if state.UserID != userID {
		t.Fatalf("state.UserID = %s, want %s", state.UserID, userID)
	}
	if math.Abs(state.Mu-mu) > 0.000001 {
		t.Fatalf("state.Mu = %.10f, want %.10f", state.Mu, mu)
	}
	if math.Abs(state.Sigma-sigma) > 0.000001 {
		t.Fatalf("state.Sigma = %.10f, want %.10f", state.Sigma, sigma)
	}
	if state.MatchesPlayed != matchesPlayed {
		t.Fatalf("state.MatchesPlayed = %d, want %d", state.MatchesPlayed, matchesPlayed)
	}
	if state.SourceResultID != sourceResultID {
		t.Fatalf("state.SourceResultID = %s, want %s", state.SourceResultID, sourceResultID)
	}
}

func assertEvent(t *testing.T, event ComputedEvent, userID uuid.UUID, sourceResultID uuid.UUID, mu float64, sigma float64, deltaMu float64, deltaSigma float64) {
	t.Helper()

	if event.UserID != userID {
		t.Fatalf("event.UserID = %s, want %s", event.UserID, userID)
	}
	if event.SourceResultID != sourceResultID {
		t.Fatalf("event.SourceResultID = %s, want %s", event.SourceResultID, sourceResultID)
	}
	if math.Abs(event.Mu-mu) > 0.000001 {
		t.Fatalf("event.Mu = %.10f, want %.10f", event.Mu, mu)
	}
	if math.Abs(event.Sigma-sigma) > 0.000001 {
		t.Fatalf("event.Sigma = %.10f, want %.10f", event.Sigma, sigma)
	}
	if math.Abs(event.DeltaMu-deltaMu) > 0.000001 {
		t.Fatalf("event.DeltaMu = %.10f, want %.10f", event.DeltaMu, deltaMu)
	}
	if math.Abs(event.DeltaSigma-deltaSigma) > 0.000001 {
		t.Fatalf("event.DeltaSigma = %.10f, want %.10f", event.DeltaSigma, deltaSigma)
	}
}
