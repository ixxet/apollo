package presence

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildAthenaPresenceGateReportClassifiesAcceptedEvidenceAndDailyReadiness(t *testing.T) {
	base := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	report := BuildAthenaPresenceGateReport(AthenaIngressBridgeReport{
		FacilityID: "ashtonbee",
		ZoneID:     "gym-floor",
		Since:      base.Add(-time.Hour),
		Until:      base.Add(4 * time.Hour),
		Evidence: []AthenaIngressBridgeEvidence{
			bridgeEvidence("evidence-001", "evt-pass-in", "identity-001", "ashtonbee", "in", "pass", base, true, true, "", AthenaIngressEligibility{
				CoPresenceProof:         GateSignal{Eligible: true},
				PrivateDailyPresence:    GateSignal{Eligible: true},
				ReliabilityVerification: GateSignal{Eligible: true},
			}),
			bridgeEvidence("evidence-002", "evt-pass-in-again", "identity-001", "ashtonbee", "in", "pass", base.Add(30*time.Minute), true, true, "", AthenaIngressEligibility{
				CoPresenceProof:         GateSignal{Eligible: true},
				PrivateDailyPresence:    GateSignal{Eligible: true},
				ReliabilityVerification: GateSignal{Eligible: true},
			}),
			bridgeEvidence("evidence-003", "evt-policy", "identity-002", "ashtonbee", "in", "fail", base.Add(time.Hour), true, false, "policy", AthenaIngressEligibility{
				CoPresenceProof:         GateSignal{Eligible: true},
				PrivateDailyPresence:    GateSignal{Eligible: true},
				ReliabilityVerification: GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonAcceptedPresenceWithoutSourcePassSession}},
			}),
		},
	})

	if report.Contract.PhysicalTruthOwner != "athena" || report.Contract.ProductTruthOwner != "apollo" {
		t.Fatalf("contract owners = %#v, want athena/apollo boundary", report.Contract)
	}
	if report.Summary.EligibleCoPresenceEvidence != 3 {
		t.Fatalf("EligibleCoPresenceEvidence = %d, want 3", report.Summary.EligibleCoPresenceEvidence)
	}
	if report.Summary.DailyPresenceReadyCredits != 2 {
		t.Fatalf("DailyPresenceReadyCredits = %d, want 2", report.Summary.DailyPresenceReadyCredits)
	}
	if report.Summary.DailyPresenceAlreadyCounted != 1 {
		t.Fatalf("DailyPresenceAlreadyCounted = %d, want 1", report.Summary.DailyPresenceAlreadyCounted)
	}
	if len(report.DailyPresenceReady) != 2 {
		t.Fatalf("len(DailyPresenceReady) = %d, want 2", len(report.DailyPresenceReady))
	}
	if report.DailyPresenceReady[0].CreditKey != "ashtonbee:identity-001:2026-04-10" {
		t.Fatalf("first credit key = %q, want facility identity UTC day key", report.DailyPresenceReady[0].CreditKey)
	}

	replay := findGateEvidence(t, report, "evt-pass-in-again")
	if replay.PrivateDailyPresence.Eligible {
		t.Fatal("second same-day evidence private daily presence eligible = true, want one report-local credit")
	}
	assertGateReason(t, replay.PrivateDailyPresence.ReasonCodes, GateReasonDailyPresenceAlreadyCounted)
	if !replay.CoPresence.Eligible {
		t.Fatal("second same-day evidence co-presence eligible = false, want true")
	}

	policy := findGateEvidence(t, report, "evt-policy")
	if !policy.CoPresence.Eligible || !policy.PrivateDailyPresence.Eligible {
		t.Fatalf("policy accepted fail co/daily eligibility = %+v %+v, want eligible", policy.CoPresence, policy.PrivateDailyPresence)
	}
	if policy.SourcePassSession.Eligible {
		t.Fatal("policy accepted fail source-pass session eligible = true, want false")
	}
	assertGateReason(t, policy.SourcePassSession.ReasonCodes, AthenaReasonAcceptedPresenceWithoutSourcePassSession)
}

func TestBuildAthenaPresenceGateReportRejectsUnsafeOrIneligibleEvidence(t *testing.T) {
	base := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	rawHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	report := BuildAthenaPresenceGateReport(AthenaIngressBridgeReport{
		FacilityID: "ashtonbee",
		Since:      base.Add(-time.Hour),
		Until:      base.Add(4 * time.Hour),
		Evidence: []AthenaIngressBridgeEvidence{
			bridgeEvidence("evidence-accepted-false", "evt-accepted-false", "identity-001", "ashtonbee", "in", "pass", base, false, false, "", AthenaIngressEligibility{
				CoPresenceProof:      GateSignal{Eligible: true},
				PrivateDailyPresence: GateSignal{Eligible: true},
			}),
			bridgeEvidence("evidence-source-fail", "evt-source-fail", "identity-002", "ashtonbee", "in", "fail", base.Add(time.Minute), false, false, "", AthenaIngressEligibility{
				CoPresenceProof:      GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonSourceFailWithoutAcceptedPresence}},
				PrivateDailyPresence: GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonSourceFailWithoutAcceptedPresence}},
			}),
			bridgeEvidence("evidence-anonymous", "evt-anonymous", "", "ashtonbee", "in", "pass", base.Add(2*time.Minute), false, false, "", AthenaIngressEligibility{
				CoPresenceProof:      GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonMissingIdentity}},
				PrivateDailyPresence: GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonMissingIdentity}},
			}),
			bridgeEvidence("evidence-unknown", "evt-unknown", "identity-003", "ashtonbee", "in", "pass", base.Add(3*time.Minute), true, true, "", AthenaIngressEligibility{
				CoPresenceProof:      GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonUnknownIdentity}},
				PrivateDailyPresence: GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonUnknownIdentity}},
			}, AthenaReasonUnknownIdentity),
			bridgeEvidence("evidence-stale", "evt-stale", "identity-004", "ashtonbee", "in", "pass", base.Add(4*time.Minute), true, true, "", AthenaIngressEligibility{
				CoPresenceProof:      GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonStaleEvent}},
				PrivateDailyPresence: GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonStaleEvent}},
			}, AthenaReasonStaleEvent),
			bridgeEvidence("evidence-duplicate", "evt-duplicate", "identity-005", "ashtonbee", "in", "pass", base.Add(5*time.Minute), true, true, "", AthenaIngressEligibility{
				CoPresenceProof:      GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonDuplicateReplay}},
				PrivateDailyPresence: GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonDuplicateReplay}},
			}, AthenaReasonDuplicateReplay),
			bridgeEvidence("evidence-out-of-order", "evt-out-of-order", "identity-006", "ashtonbee", "out", "pass", base.Add(6*time.Minute), true, true, "", AthenaIngressEligibility{
				CoPresenceProof:      GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonOutOfOrderLifecycle}},
				PrivateDailyPresence: GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonOutOfOrderLifecycle, AthenaReasonNotArrival}},
			}, AthenaReasonOutOfOrderLifecycle),
			bridgeEvidence("evidence-missing-facility", "evt-missing-facility", "identity-007", "", "in", "pass", base.Add(7*time.Minute), true, true, "", AthenaIngressEligibility{
				CoPresenceProof:      GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonMissingFacility}},
				PrivateDailyPresence: GateSignal{Eligible: false, ReasonCodes: []string{AthenaReasonMissingFacility}},
			}),
			bridgeEvidence("evidence-unsafe-identity", "evt-unsafe-identity", rawHash, "ashtonbee", "in", "pass", base.Add(8*time.Minute), true, true, "", AthenaIngressEligibility{
				CoPresenceProof:      GateSignal{Eligible: true},
				PrivateDailyPresence: GateSignal{Eligible: true},
			}),
		},
	})

	if report.Summary.EligibleCoPresenceEvidence != 0 || report.Summary.DailyPresenceReadyCredits != 0 {
		t.Fatalf("eligible summary = %+v, want no ineligible evidence admitted", report.Summary)
	}

	assertGateReason(t, findGateEvidence(t, report, "evt-accepted-false").ReasonCodes, GateReasonAthenaEvidenceNotAccepted)
	assertGateReason(t, findGateEvidence(t, report, "evt-source-fail").ReasonCodes, AthenaReasonSourceFailWithoutAcceptedPresence)
	assertGateReason(t, findGateEvidence(t, report, "evt-anonymous").ReasonCodes, AthenaReasonMissingIdentity)
	assertGateReason(t, findGateEvidence(t, report, "evt-unknown").ReasonCodes, AthenaReasonUnknownIdentity)
	assertGateReason(t, findGateEvidence(t, report, "evt-stale").ReasonCodes, AthenaReasonStaleEvent)
	assertGateReason(t, findGateEvidence(t, report, "evt-duplicate").ReasonCodes, AthenaReasonDuplicateReplay)
	assertGateReason(t, findGateEvidence(t, report, "evt-out-of-order").ReasonCodes, AthenaReasonOutOfOrderLifecycle)
	assertGateReason(t, findGateEvidence(t, report, "evt-missing-facility").ReasonCodes, AthenaReasonMissingFacility)
	unsafe := findGateEvidence(t, report, "evt-unsafe-identity")
	if unsafe.IdentityRef != "" {
		t.Fatalf("unsafe IdentityRef = %q, want redacted", unsafe.IdentityRef)
	}
	assertGateReason(t, unsafe.ReasonCodes, GateReasonUnsafeIdentityRef)
}

func TestBuildAthenaPresenceGateReportDoesNotEmitRawAthenaIdentityFields(t *testing.T) {
	rawBridge := []byte(`{
		"facility_id":"ashtonbee",
		"since":"2026-04-10T08:00:00Z",
		"until":"2026-04-10T12:00:00Z",
		"evidence":[{
			"evidence_id":"evidence-raw",
			"event_id":"evt-raw",
			"identity_present":true,
			"identity_ref":"identity-001",
			"external_identity_hash":"raw-hash-should-not-emit",
			"account_raw":"raw-account-should-not-emit",
			"name":"Raw Name Should Not Emit",
			"facility_id":"ashtonbee",
			"direction":"in",
			"source_result":"pass",
			"observed_at":"2026-04-10T09:00:00Z",
			"source_committed":true,
			"accepted_presence":true,
			"eligibility":{
				"co_presence_proof":{"eligible":true},
				"private_daily_presence":{"eligible":true},
				"reliability_verification":{"eligible":true}
			}
		}]
	}`)
	var bridge AthenaIngressBridgeReport
	if err := json.Unmarshal(rawBridge, &bridge); err != nil {
		t.Fatalf("json.Unmarshal(rawBridge) error = %v", err)
	}

	output, err := json.Marshal(BuildAthenaPresenceGateReport(bridge))
	if err != nil {
		t.Fatalf("json.Marshal(gate report) error = %v", err)
	}
	for _, forbidden := range []string{
		"external_identity_hash",
		"raw-hash-should-not-emit",
		"account_raw",
		"raw-account-should-not-emit",
		"Raw Name Should Not Emit",
	} {
		if strings.Contains(string(output), forbidden) {
			t.Fatalf("gate output leaked %q: %s", forbidden, output)
		}
	}
}

func bridgeEvidence(evidenceID, eventID, identityRef, facilityID, direction, sourceResult string, observedAt time.Time, accepted bool, sourceCommitted bool, acceptancePath string, eligibility AthenaIngressEligibility, reasonCodes ...string) AthenaIngressBridgeEvidence {
	return AthenaIngressBridgeEvidence{
		EvidenceID:       evidenceID,
		EventID:          eventID,
		IdentityPresent:  identityRef != "",
		IdentityRef:      identityRef,
		FacilityID:       facilityID,
		Direction:        direction,
		SourceResult:     sourceResult,
		ObservedAt:       &observedAt,
		SourceCommitted:  sourceCommitted,
		AcceptedPresence: accepted,
		AcceptancePath:   acceptancePath,
		Eligibility:      eligibility,
		ReasonCodes:      reasonCodes,
	}
}

func findGateEvidence(t *testing.T, report AthenaPresenceGateReport, eventID string) AthenaPresenceGateEvidence {
	t.Helper()
	for _, evidence := range report.Evidence {
		if evidence.EventID == eventID {
			return evidence
		}
	}
	t.Fatalf("event %q not found in gate report", eventID)
	return AthenaPresenceGateEvidence{}
}

func assertGateReason(t *testing.T, reasons []string, want string) {
	t.Helper()
	for _, reason := range reasons {
		if reason == want {
			return
		}
	}
	t.Fatalf("reasons = %v, want %q", reasons, want)
}
