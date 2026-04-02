package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ixxet/apollo/internal/eligibility"
)

func decodeEligibilityResponse(t *testing.T, recorder *httptest.ResponseRecorder) eligibility.LobbyEligibility {
	t.Helper()

	var memberEligibility eligibility.LobbyEligibility
	if err := json.Unmarshal(recorder.Body.Bytes(), &memberEligibility); err != nil {
		t.Fatalf("json.Unmarshal(eligibility) error = %v", err)
	}

	return memberEligibility
}

func assertEligibility(t *testing.T, got eligibility.LobbyEligibility, wantEligible bool, wantReason string, wantVisibilityMode string, wantAvailabilityMode string) {
	t.Helper()

	if got.Eligible != wantEligible {
		t.Fatalf("Eligible = %v, want %v", got.Eligible, wantEligible)
	}
	if got.Reason != wantReason {
		t.Fatalf("Reason = %q, want %q", got.Reason, wantReason)
	}
	if got.VisibilityMode != wantVisibilityMode {
		t.Fatalf("VisibilityMode = %q, want %q", got.VisibilityMode, wantVisibilityMode)
	}
	if got.AvailabilityMode != wantAvailabilityMode {
		t.Fatalf("AvailabilityMode = %q, want %q", got.AvailabilityMode, wantAvailabilityMode)
	}
}
