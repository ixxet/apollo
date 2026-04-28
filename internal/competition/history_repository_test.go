package competition

import "testing"

func TestStoredDeltaFlaggedMatchesComparisonNumericScale(t *testing.T) {
	tests := []struct {
		name        string
		delta       float64
		budget      float64
		wantFlagged bool
	}{
		{name: "positive raw over budget rounds in budget", delta: 0.75004, budget: 0.75, wantFlagged: false},
		{name: "negative raw over budget rounds in budget", delta: -0.75004, budget: 0.75, wantFlagged: false},
		{name: "positive stored over budget", delta: 0.7501, budget: 0.75, wantFlagged: true},
		{name: "negative stored over budget", delta: -0.7501, budget: 0.75, wantFlagged: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := storedDeltaFlagged(test.delta, test.budget)
			if err != nil {
				t.Fatalf("storedDeltaFlagged error = %v", err)
			}
			if got != test.wantFlagged {
				t.Fatalf("storedDeltaFlagged(%f, %f) = %t, want %t", test.delta, test.budget, got, test.wantFlagged)
			}
		})
	}
}
