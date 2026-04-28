package competition

import "testing"

func TestTournamentBracketHelpersEnforceDeterministicSingleElimination(t *testing.T) {
	tests := []struct {
		name       string
		seedCount  int
		wantRounds int
		matches    map[int]int
	}{
		{
			name:       "two seeds",
			seedCount:  2,
			wantRounds: 1,
			matches:    map[int]int{1: 1, 2: 0},
		},
		{
			name:       "four seeds",
			seedCount:  4,
			wantRounds: 2,
			matches:    map[int]int{1: 2, 2: 1, 3: 0},
		},
		{
			name:       "eight seeds",
			seedCount:  8,
			wantRounds: 3,
			matches:    map[int]int{1: 4, 2: 2, 3: 1, 4: 0},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := tournamentTotalRounds(test.seedCount); got != test.wantRounds {
				t.Fatalf("tournamentTotalRounds(%d) = %d, want %d", test.seedCount, got, test.wantRounds)
			}
			for round, want := range test.matches {
				if got := tournamentMatchesInRound(test.seedCount, round); got != want {
					t.Fatalf("tournamentMatchesInRound(%d, %d) = %d, want %d", test.seedCount, round, got, want)
				}
			}
		})
	}
}

func TestTournamentFirstRoundPairingUsesSeedMirror(t *testing.T) {
	if !validFirstRoundPairing(4, 1, 1, 4) {
		t.Fatal("seed 1 should pair with seed 4 in four-seed match 1")
	}
	if !validFirstRoundPairing(4, 1, 4, 1) {
		t.Fatal("seed 4 should pair with seed 1 in four-seed match 1")
	}
	if !validFirstRoundPairing(4, 2, 2, 3) {
		t.Fatal("seed 2 should pair with seed 3 in four-seed match 2")
	}
	if validFirstRoundPairing(4, 1, 1, 2) {
		t.Fatal("seed 1 should not pair with seed 2 in four-seed match 1")
	}
	if validFirstRoundPairing(4, 3, 1, 4) {
		t.Fatal("four-seed bracket should not have a third first-round match")
	}
}

func TestTournamentPreviousRoundMatchNumbersFeedCurrentMatch(t *testing.T) {
	first, second := previousRoundMatchNumbers(1)
	if first != 1 || second != 2 {
		t.Fatalf("previousRoundMatchNumbers(1) = (%d, %d), want (1, 2)", first, second)
	}

	first, second = previousRoundMatchNumbers(2)
	if first != 3 || second != 4 {
		t.Fatalf("previousRoundMatchNumbers(2) = (%d, %d), want (3, 4)", first, second)
	}
}
