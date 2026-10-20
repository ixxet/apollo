package presence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	AthenaBridgeVersion          = "athena_ingress_bridge_v2"
	AthenaBridgeMaxBytes         = 2 << 20
	AthenaBridgeMaxObservations  = 1000
	AthenaBridgeMaxSessions      = 250
	AthenaBridgeMaxWindow        = 24 * time.Hour
	AthenaReasonIncompleteReplay = "incomplete_replay_context"

	AthenaReasonMissingIdentity                          = "missing_identity"
	AthenaReasonUnknownIdentity                          = "unknown_identity"
	AthenaReasonSourceFailWithoutAcceptedPresence        = "source_fail_without_accepted_presence"
	AthenaReasonStaleEvent                               = "stale_event"
	AthenaReasonDuplicateReplay                          = "duplicate_replay"
	AthenaReasonOutOfOrderLifecycle                      = "out_of_order_lifecycle"
	AthenaReasonMissingFacility                          = "missing_facility"
	AthenaReasonMissingTimestamp                         = "missing_timestamp"
	AthenaReasonIncompleteLifecycle                      = "incomplete_lifecycle"
	AthenaReasonAcceptedPresenceWithoutSourcePassSession = "accepted_presence_without_source_pass_session"
	AthenaReasonNotArrival                               = "not_arrival"

	GateReasonAthenaEvidenceNotAccepted      = "athena_evidence_not_accepted"
	GateReasonAthenaCoPresenceNotEligible    = "athena_co_presence_not_eligible"
	GateReasonAthenaDailyPresenceNotEligible = "athena_private_daily_presence_not_eligible"
	GateReasonDailyPresenceAlreadyCounted    = "daily_presence_already_counted"
	GateReasonUnsafeIdentityRef              = "unsafe_identity_ref"
)

type AthenaIngressBridgeReport struct {
	FacilityID   string                          `json:"facility_id"`
	ZoneID       string                          `json:"zone_id,omitempty"`
	NodeID       string                          `json:"node_id,omitempty"`
	Since        time.Time                       `json:"since"`
	Until        time.Time                       `json:"until"`
	Completeness AthenaIngressBridgeCompleteness `json:"completeness"`
	Contract     AthenaIngressBridgeContract     `json:"contract"`
	Summary      AthenaIngressBridgeSummary      `json:"summary"`
	Evidence     []AthenaIngressBridgeEvidence   `json:"evidence"`
	Sessions     []AthenaIngressBridgeSession    `json:"sessions"`
}

type AthenaIngressBridgeCompleteness struct {
	Observations  bool     `json:"observations"`
	Sessions      bool     `json:"sessions"`
	ReplayContext bool     `json:"replay_context"`
	ReasonCodes   []string `json:"reason_codes"`
}

type AthenaIngressBridgeContract struct {
	Version               string `json:"version"`
	ReplayOrder           string `json:"replay_order"`
	IdentityScope         string `json:"identity_scope"`
	Scope                 string `json:"scope"`
	SourceTruth           string `json:"source_truth"`
	AcceptedPresenceTruth string `json:"accepted_presence_truth"`
	SessionTruth          string `json:"session_truth"`
	IdentityOutput        string `json:"identity_output"`
	UnknownIdentityScope  string `json:"unknown_identity_scope"`
}

type AthenaIngressBridgeSummary struct {
	TotalEvidence                   int               `json:"total_evidence"`
	TotalSessions                   int               `json:"total_sessions"`
	SourcePass                      int               `json:"source_pass"`
	SourceFail                      int               `json:"source_fail"`
	AcceptedSourcePass              int               `json:"accepted_source_pass"`
	AcceptedPolicy                  int               `json:"accepted_policy"`
	EligibleCoPresence              int               `json:"eligible_co_presence"`
	EligibleDailyPresence           int               `json:"eligible_daily_presence"`
	EligibleReliabilityVerification int               `json:"eligible_reliability_verification"`
	NoEligibleSignals               int               `json:"no_eligible_signals"`
	ReasonCounts                    []GateReasonCount `json:"reason_counts"`
}

type AthenaIngressBridgeEvidence struct {
	EvidenceID         string                   `json:"evidence_id"`
	EventID            string                   `json:"event_id,omitempty"`
	IdentityPresent    bool                     `json:"identity_present"`
	IdentityRef        string                   `json:"identity_ref,omitempty"`
	FacilityID         string                   `json:"facility_id,omitempty"`
	ZoneID             string                   `json:"zone_id,omitempty"`
	NodeID             string                   `json:"node_id,omitempty"`
	Direction          string                   `json:"direction,omitempty"`
	SourceResult       string                   `json:"source_result,omitempty"`
	ObservedAt         *time.Time               `json:"observed_at,omitempty"`
	SourceCommitted    bool                     `json:"source_committed"`
	AcceptedPresence   bool                     `json:"accepted_presence"`
	AcceptancePath     string                   `json:"acceptance_path,omitempty"`
	AcceptedReasonCode string                   `json:"accepted_reason_code,omitempty"`
	SessionState       string                   `json:"session_state,omitempty"`
	Eligibility        AthenaIngressEligibility `json:"eligibility"`
	ReasonCodes        []string                 `json:"reason_codes,omitempty"`
}

type AthenaIngressBridgeSession struct {
	SessionID       string                   `json:"session_id"`
	IdentityPresent bool                     `json:"identity_present"`
	IdentityRef     string                   `json:"identity_ref,omitempty"`
	State           string                   `json:"state"`
	EntryEventID    string                   `json:"entry_event_id,omitempty"`
	EntryNodeID     string                   `json:"entry_node_id,omitempty"`
	EntryAt         *time.Time               `json:"entry_at,omitempty"`
	ExitEventID     string                   `json:"exit_event_id,omitempty"`
	ExitNodeID      string                   `json:"exit_node_id,omitempty"`
	ExitAt          *time.Time               `json:"exit_at,omitempty"`
	Eligibility     AthenaIngressEligibility `json:"eligibility"`
	ReasonCodes     []string                 `json:"reason_codes,omitempty"`
}

type AthenaIngressEligibility struct {
	CoPresenceProof         GateSignal `json:"co_presence_proof"`
	PrivateDailyPresence    GateSignal `json:"private_daily_presence"`
	ReliabilityVerification GateSignal `json:"reliability_verification"`
}

type AthenaPresenceGateReport struct {
	Completeness       AthenaIngressBridgeCompleteness `json:"completeness"`
	FacilityID         string                          `json:"facility_id"`
	ZoneID             string                          `json:"zone_id,omitempty"`
	NodeID             string                          `json:"node_id,omitempty"`
	Since              time.Time                       `json:"since"`
	Until              time.Time                       `json:"until"`
	Contract           AthenaPresenceGateContract      `json:"contract"`
	Summary            AthenaPresenceGateSummary       `json:"summary"`
	Evidence           []AthenaPresenceGateEvidence    `json:"evidence"`
	DailyPresenceReady []DailyPresenceReady            `json:"daily_presence_ready"`
}

type AthenaPresenceGateContract struct {
	Scope                  string `json:"scope"`
	PhysicalTruthOwner     string `json:"physical_truth_owner"`
	ProductTruthOwner      string `json:"product_truth_owner"`
	CoPresence             string `json:"co_presence"`
	PrivateDailyPresence   string `json:"private_daily_presence"`
	SourcePassSessionTruth string `json:"source_pass_session_truth"`
	IdentityOutput         string `json:"identity_output"`
	MutationBoundary       string `json:"mutation_boundary"`
}

type AthenaPresenceGateSummary struct {
	TotalEvidence                 int               `json:"total_evidence"`
	AcceptedEvidence              int               `json:"accepted_evidence"`
	EligibleCoPresenceEvidence    int               `json:"eligible_co_presence_evidence"`
	EligibleDailyPresenceEvidence int               `json:"eligible_daily_presence_evidence"`
	DailyPresenceReadyCredits     int               `json:"daily_presence_ready_credits"`
	DailyPresenceAlreadyCounted   int               `json:"daily_presence_already_counted"`
	NoEligibleSignals             int               `json:"no_eligible_signals"`
	ReasonCounts                  []GateReasonCount `json:"reason_counts"`
}

type AthenaPresenceGateEvidence struct {
	EvidenceID              string     `json:"evidence_id"`
	EventID                 string     `json:"event_id,omitempty"`
	IdentityRef             string     `json:"identity_ref,omitempty"`
	FacilityID              string     `json:"facility_id,omitempty"`
	ZoneID                  string     `json:"zone_id,omitempty"`
	NodeID                  string     `json:"node_id,omitempty"`
	ObservedAt              *time.Time `json:"observed_at,omitempty"`
	AcceptedPresence        bool       `json:"accepted_presence"`
	AcceptancePath          string     `json:"acceptance_path,omitempty"`
	SourceResult            string     `json:"source_result,omitempty"`
	SourcePassSession       GateSignal `json:"source_pass_session"`
	CoPresence              GateSignal `json:"co_presence"`
	PrivateDailyPresence    GateSignal `json:"private_daily_presence"`
	PrivateDailyPresenceDay string     `json:"private_daily_presence_day,omitempty"`
	ReasonCodes             []string   `json:"reason_codes,omitempty"`
}

type GateSignal struct {
	Eligible    bool     `json:"eligible"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
}

type DailyPresenceReady struct {
	CreditKey   string    `json:"credit_key"`
	FacilityID  string    `json:"facility_id"`
	Day         string    `json:"day"`
	IdentityRef string    `json:"identity_ref"`
	EvidenceID  string    `json:"evidence_id"`
	EventID     string    `json:"event_id,omitempty"`
	ZoneID      string    `json:"zone_id,omitempty"`
	NodeID      string    `json:"node_id,omitempty"`
	ObservedAt  time.Time `json:"observed_at"`
}

type GateReasonCount struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

func BuildAthenaPresenceGateReport(input AthenaIngressBridgeReport) AthenaPresenceGateReport {
	report := AthenaPresenceGateReport{
		Completeness: input.Completeness,
		FacilityID:   strings.TrimSpace(input.FacilityID),
		ZoneID:       strings.TrimSpace(input.ZoneID),
		NodeID:       strings.TrimSpace(input.NodeID),
		Since:        input.Since.UTC(),
		Until:        input.Until.UTC(),
		Contract: AthenaPresenceGateContract{
			Scope:                  "repo_local_runtime_proof",
			PhysicalTruthOwner:     "athena",
			ProductTruthOwner:      "apollo",
			CoPresence:             "eligible only from explicit accepted ATHENA evidence; no team, lobby, matchmaking, or social intent is inferred",
			PrivateDailyPresence:   "facility-scoped UTC-day readiness, at most one report-local credit per identity/day; no XP ledger or streak mutation is created",
			SourcePassSessionTruth: "source-pass session truth stays separate; policy accepted presence does not become a source-pass session",
			IdentityOutput:         "report-local identity_ref only; raw ATHENA identity hashes, account ids, and names are not emitted",
			MutationBoundary:       "read-only report; APOLLO visits, tap-links, streaks, teams, XP, and reliability state are not mutated",
		},
	}

	ordered := append([]AthenaIngressBridgeEvidence(nil), input.Evidence...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return athenaBridgeEvidenceLess(ordered[i], ordered[j])
	})

	dailySeen := make(map[string]struct{})
	reasonCounts := make(map[string]int)
	report.Evidence = make([]AthenaPresenceGateEvidence, 0, len(ordered))
	for _, evidence := range ordered {
		if input.Contract.Version != AthenaBridgeVersion || !input.Completeness.Observations || !input.Completeness.Sessions || !input.Completeness.ReplayContext {
			evidence.ReasonCodes = appendReasonCode(evidence.ReasonCodes, AthenaReasonIncompleteReplay)
		}
		classified := classifyAthenaPresenceGateEvidence(evidence)
		if classified.PrivateDailyPresence.Eligible {
			key := dailyPresenceCreditKey(classified)
			if _, exists := dailySeen[key]; exists {
				classified.PrivateDailyPresence.Eligible = false
				classified.PrivateDailyPresence.ReasonCodes = appendReasonCode(classified.PrivateDailyPresence.ReasonCodes, GateReasonDailyPresenceAlreadyCounted)
				classified.ReasonCodes = appendReasonCode(classified.ReasonCodes, GateReasonDailyPresenceAlreadyCounted)
				report.Summary.DailyPresenceAlreadyCounted++
			} else {
				dailySeen[key] = struct{}{}
				report.DailyPresenceReady = append(report.DailyPresenceReady, DailyPresenceReady{
					CreditKey:   key,
					FacilityID:  classified.FacilityID,
					Day:         classified.PrivateDailyPresenceDay,
					IdentityRef: classified.IdentityRef,
					EvidenceID:  classified.EvidenceID,
					EventID:     classified.EventID,
					ZoneID:      classified.ZoneID,
					NodeID:      classified.NodeID,
					ObservedAt:  classified.ObservedAt.UTC(),
				})
			}
		}

		report.Evidence = append(report.Evidence, classified)
		accumulateAthenaPresenceGateSummary(&report.Summary, classified, reasonCounts)
	}

	report.Summary.ReasonCounts = sortedGateReasonCounts(reasonCounts)
	return report
}

func classifyAthenaPresenceGateEvidence(input AthenaIngressBridgeEvidence) AthenaPresenceGateEvidence {
	structuralReasons := structuralGateReasons(input)
	acceptedReasons := acceptedGateReasons(input)
	identityRef := safeGateIdentityRef(input.IdentityRef)
	observedAt := copyGateTime(input.ObservedAt)

	coPresenceReasons := mergeReasonCodes(structuralReasons, acceptedReasons, input.Eligibility.CoPresenceProof.ReasonCodes)
	if !input.Eligibility.CoPresenceProof.Eligible {
		coPresenceReasons = appendReasonCode(coPresenceReasons, GateReasonAthenaCoPresenceNotEligible)
	}
	coPresenceEligible := input.AcceptedPresence && input.Eligibility.CoPresenceProof.Eligible && len(coPresenceReasons) == 0

	dailyReasons := mergeReasonCodes(structuralReasons, acceptedReasons, input.Eligibility.PrivateDailyPresence.ReasonCodes)
	if strings.TrimSpace(input.Direction) != "in" {
		dailyReasons = appendReasonCode(dailyReasons, AthenaReasonNotArrival)
	}
	if !input.Eligibility.PrivateDailyPresence.Eligible {
		dailyReasons = appendReasonCode(dailyReasons, GateReasonAthenaDailyPresenceNotEligible)
	}
	dailyDay := ""
	if observedAt != nil {
		dailyDay = formatGateDay(*observedAt)
	}
	dailyEligible := input.AcceptedPresence && input.Eligibility.PrivateDailyPresence.Eligible && len(dailyReasons) == 0

	sourcePassReasons := append([]string(nil), input.Eligibility.ReliabilityVerification.ReasonCodes...)
	if len(sourcePassReasons) == 0 && !input.Eligibility.ReliabilityVerification.Eligible {
		if strings.TrimSpace(input.SourceResult) == "fail" && input.AcceptedPresence {
			sourcePassReasons = appendReasonCode(sourcePassReasons, AthenaReasonAcceptedPresenceWithoutSourcePassSession)
		} else {
			sourcePassReasons = appendReasonCode(sourcePassReasons, AthenaReasonIncompleteLifecycle)
		}
	}

	classified := AthenaPresenceGateEvidence{
		EvidenceID:              strings.TrimSpace(input.EvidenceID),
		EventID:                 strings.TrimSpace(input.EventID),
		IdentityRef:             identityRef,
		FacilityID:              strings.TrimSpace(input.FacilityID),
		ZoneID:                  strings.TrimSpace(input.ZoneID),
		NodeID:                  strings.TrimSpace(input.NodeID),
		ObservedAt:              observedAt,
		AcceptedPresence:        input.AcceptedPresence,
		AcceptancePath:          strings.TrimSpace(input.AcceptancePath),
		SourceResult:            strings.TrimSpace(input.SourceResult),
		SourcePassSession:       GateSignal{Eligible: input.Eligibility.ReliabilityVerification.Eligible && input.AcceptedPresence && input.SourceCommitted && input.SourceResult == "pass" && input.SessionState == "closed" && len(structuralReasons) == 0, ReasonCodes: mergeReasonCodes(sourcePassReasons, structuralReasons)},
		CoPresence:              GateSignal{Eligible: coPresenceEligible, ReasonCodes: reasonCodesIfBlocked(coPresenceEligible, coPresenceReasons)},
		PrivateDailyPresence:    GateSignal{Eligible: dailyEligible, ReasonCodes: reasonCodesIfBlocked(dailyEligible, dailyReasons)},
		PrivateDailyPresenceDay: dailyDay,
	}
	classified.ReasonCodes = mergeReasonCodes(classified.CoPresence.ReasonCodes, classified.PrivateDailyPresence.ReasonCodes, classified.SourcePassSession.ReasonCodes)
	return classified
}

func structuralGateReasons(input AthenaIngressBridgeEvidence) []string {
	reasons := make([]string, 0)
	if input.Direction != "in" && input.Direction != "out" {
		reasons = appendReasonCode(reasons, "unsupported_direction")
	}
	if input.SourceResult != "pass" && input.SourceResult != "fail" {
		reasons = appendReasonCode(reasons, "unsupported_source_result")
	}
	if input.SourceResult == "pass" && !input.SourceCommitted {
		reasons = appendReasonCode(reasons, "source_pass_not_committed")
	}
	if !input.IdentityPresent || safeGateIdentityRef(input.IdentityRef) == "" {
		reasons = appendReasonCode(reasons, AthenaReasonMissingIdentity)
	}
	if strings.TrimSpace(input.IdentityRef) != "" && safeGateIdentityRef(input.IdentityRef) == "" {
		reasons = appendReasonCode(reasons, GateReasonUnsafeIdentityRef)
	}
	if strings.TrimSpace(input.FacilityID) == "" {
		reasons = appendReasonCode(reasons, AthenaReasonMissingFacility)
	}
	if input.ObservedAt == nil || input.ObservedAt.IsZero() {
		reasons = appendReasonCode(reasons, AthenaReasonMissingTimestamp)
	}
	if strings.TrimSpace(input.SourceResult) == "fail" && !input.AcceptedPresence {
		reasons = appendReasonCode(reasons, AthenaReasonSourceFailWithoutAcceptedPresence)
	}
	for _, reason := range input.ReasonCodes {
		switch reason {
		case AthenaReasonIncompleteLifecycle, AthenaReasonNotArrival, AthenaReasonAcceptedPresenceWithoutSourcePassSession:
			// These describe a separate signal, rather than invalidating accepted presence.
		default:
			reasons = appendReasonCode(reasons, reason)
		}
	}

	return reasons
}

func acceptedGateReasons(input AthenaIngressBridgeEvidence) []string {
	if input.AcceptedPresence {
		return nil
	}
	return []string{GateReasonAthenaEvidenceNotAccepted}
}

func accumulateAthenaPresenceGateSummary(summary *AthenaPresenceGateSummary, evidence AthenaPresenceGateEvidence, reasonCounts map[string]int) {
	summary.TotalEvidence++
	if evidence.AcceptedPresence {
		summary.AcceptedEvidence++
	}
	if evidence.CoPresence.Eligible {
		summary.EligibleCoPresenceEvidence++
	}
	if evidence.PrivateDailyPresence.Eligible {
		summary.EligibleDailyPresenceEvidence++
		summary.DailyPresenceReadyCredits++
	}
	if !evidence.CoPresence.Eligible && !evidence.PrivateDailyPresence.Eligible {
		summary.NoEligibleSignals++
	}
	for _, reason := range evidence.ReasonCodes {
		reasonCounts[reason]++
	}
}

func dailyPresenceCreditKey(evidence AthenaPresenceGateEvidence) string {
	return fmt.Sprintf("%s:%s:%s", evidence.FacilityID, evidence.IdentityRef, evidence.PrivateDailyPresenceDay)
}

func athenaBridgeEvidenceLess(left, right AthenaIngressBridgeEvidence) bool {
	leftAt := gateSortTime(left.ObservedAt)
	rightAt := gateSortTime(right.ObservedAt)
	if !leftAt.Equal(rightAt) {
		return leftAt.Before(rightAt)
	}
	leftID := strings.TrimSpace(left.EvidenceID)
	rightID := strings.TrimSpace(right.EvidenceID)
	if leftID != rightID {
		return leftID < rightID
	}
	return strings.TrimSpace(left.EventID) < strings.TrimSpace(right.EventID)
}

func gateSortTime(value *time.Time) time.Time {
	if value == nil || value.IsZero() {
		return time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	}
	return value.UTC()
}

func copyGateTime(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	copied := value.UTC()
	return &copied
}

func formatGateDay(value time.Time) string {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

var bridgeIdentityPattern = regexp.MustCompile(`^identity-[0-9]{3,}$`)

func safeGateIdentityRef(value string) string {
	if len(value) > 64 || !bridgeIdentityPattern.MatchString(value) {
		return ""
	}
	return value
}

func reasonCodesIfBlocked(eligible bool, reasons []string) []string {
	if eligible {
		return nil
	}
	return append([]string(nil), reasons...)
}

func mergeReasonCodes(groups ...[]string) []string {
	reasons := make([]string, 0)
	for _, group := range groups {
		for _, reason := range group {
			reasons = appendReasonCode(reasons, reason)
		}
	}
	return reasons
}

func appendReasonCode(reasons []string, reason string) []string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return reasons
	}
	for _, existing := range reasons {
		if existing == trimmed {
			return reasons
		}
	}
	return append(reasons, trimmed)
}

func sortedGateReasonCounts(counts map[string]int) []GateReasonCount {
	reasons := make([]string, 0, len(counts))
	for reason := range counts {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)

	result := make([]GateReasonCount, 0, len(reasons))
	for _, reason := range reasons {
		result = append(result, GateReasonCount{Code: reason, Count: counts[reason]})
	}
	return result
}

// DecodeAthenaIngressBridgeReport is shared by HTTP and file consumers. Legacy
// reports must be regenerated because missing safety metadata cannot be inferred.
func DecodeAthenaIngressBridgeReport(reader io.Reader) (AthenaIngressBridgeReport, error) {
	var report AthenaIngressBridgeReport
	payload, err := io.ReadAll(io.LimitReader(reader, AthenaBridgeMaxBytes+1))
	if err != nil {
		return report, fmt.Errorf("read bridge report: %w", err)
	}
	if len(payload) > AthenaBridgeMaxBytes {
		return report, fmt.Errorf("bridge report exceeds 2 MiB")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return report, fmt.Errorf("decode bridge report: %w", err)
	}
	if err := requireBridgeFields(envelope, "facility_id", "since", "until", "contract", "completeness", "summary", "evidence", "sessions"); err != nil {
		return report, err
	}
	if err := json.Unmarshal(payload, &report); err != nil {
		return report, fmt.Errorf("decode bridge report: %w", err)
	}
	for field, names := range map[string][]string{
		"contract":     {"version", "replay_order", "identity_scope", "scope", "source_truth", "accepted_presence_truth", "session_truth", "identity_output", "unknown_identity_scope"},
		"completeness": {"observations", "sessions", "replay_context", "reason_codes"},
		"summary":      {"total_evidence", "total_sessions", "source_pass", "source_fail", "accepted_source_pass", "accepted_policy", "eligible_co_presence", "eligible_daily_presence", "eligible_reliability_verification", "no_eligible_signals", "reason_counts"},
	} {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(envelope[field], &object); err != nil {
			return report, err
		}
		if err := requireBridgeFields(object, names...); err != nil {
			return report, err
		}
	}
	var evidenceObjects []map[string]json.RawMessage
	var sessionObjects []map[string]json.RawMessage
	if err := json.Unmarshal(envelope["evidence"], &evidenceObjects); err != nil {
		return report, err
	}
	if err := json.Unmarshal(envelope["sessions"], &sessionObjects); err != nil {
		return report, err
	}
	for _, object := range evidenceObjects {
		if err := requireBridgeFields(object, "evidence_id", "event_id", "identity_present", "facility_id", "direction", "source_result", "observed_at", "source_committed", "accepted_presence", "eligibility"); err != nil {
			return report, err
		}
		if err := requireBridgeSignals(object["eligibility"]); err != nil {
			return report, err
		}
	}
	for _, object := range sessionObjects {
		if err := requireBridgeFields(object, "session_id", "identity_present", "state", "eligibility"); err != nil {
			return report, err
		}
		if err := requireBridgeSignals(object["eligibility"]); err != nil {
			return report, err
		}
	}
	if err := ValidateAthenaIngressBridgeReport(report); err != nil {
		return report, err
	}
	return report, nil
}

func requireBridgeFields(object map[string]json.RawMessage, names ...string) error {
	for _, name := range names {
		value, ok := object[name]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("bridge report missing %s; regenerate with ATHENA v2", name)
		}
	}
	return nil
}

func requireBridgeSignals(value json.RawMessage) error {
	var signals map[string]json.RawMessage
	if err := json.Unmarshal(value, &signals); err != nil {
		return err
	}
	for _, name := range []string{"co_presence_proof", "private_daily_presence", "reliability_verification"} {
		var signal map[string]json.RawMessage
		if err := json.Unmarshal(signals[name], &signal); err != nil {
			return err
		}
		if err := requireBridgeFields(signal, "eligible"); err != nil {
			return err
		}
	}
	return nil
}

func ValidateAthenaIngressBridgeReport(report AthenaIngressBridgeReport) error {
	invalid := func() error { return fmt.Errorf("ATHENA bridge envelope is inconsistent or unsupported") }
	if report.Contract.Version != AthenaBridgeVersion || report.Contract.Scope != "repo_local_runtime_proof" || report.Contract.IdentityScope != "report_local" || report.Contract.ReplayOrder != "stored_at,observed_at,event_id,observation_id" {
		return invalid()
	}
	for _, text := range []string{report.Contract.SourceTruth, report.Contract.AcceptedPresenceTruth, report.Contract.SessionTruth, report.Contract.IdentityOutput, report.Contract.UnknownIdentityScope} {
		if strings.TrimSpace(text) == "" {
			return invalid()
		}
	}
	if strings.TrimSpace(report.FacilityID) == "" || len(report.FacilityID) > 128 || report.Since.IsZero() || report.Until.IsZero() || report.Until.Before(report.Since) || report.Until.Sub(report.Since) > AthenaBridgeMaxWindow {
		return invalid()
	}
	if report.Evidence == nil || report.Sessions == nil || len(report.Evidence) > AthenaBridgeMaxObservations || len(report.Sessions) > AthenaBridgeMaxSessions || report.Summary.TotalEvidence != len(report.Evidence) || report.Summary.TotalSessions != len(report.Sessions) {
		return invalid()
	}
	complete := report.Completeness.Observations && report.Completeness.Sessions && report.Completeness.ReplayContext
	if complete && len(report.Completeness.ReasonCodes) > 0 || !complete && len(report.Completeness.ReasonCodes) == 0 {
		return invalid()
	}
	evidenceIDs := map[string]bool{}
	sessionIDs := map[string]bool{}
	var expected AthenaIngressBridgeSummary
	countSignals := func(eligibility AthenaIngressEligibility) error {
		for _, signal := range []GateSignal{eligibility.CoPresenceProof, eligibility.PrivateDailyPresence, eligibility.ReliabilityVerification} {
			if signal.Eligible && (!complete || len(signal.ReasonCodes) > 0) {
				return invalid()
			}
		}
		if eligibility.CoPresenceProof.Eligible {
			expected.EligibleCoPresence++
		}
		if eligibility.PrivateDailyPresence.Eligible {
			expected.EligibleDailyPresence++
		}
		if eligibility.ReliabilityVerification.Eligible {
			expected.EligibleReliabilityVerification++
		}
		if !eligibility.CoPresenceProof.Eligible && !eligibility.PrivateDailyPresence.Eligible && !eligibility.ReliabilityVerification.Eligible {
			expected.NoEligibleSignals++
		}
		return nil
	}
	for _, evidence := range report.Evidence {
		if evidence.EvidenceID == "" || evidenceIDs[evidence.EvidenceID] || evidence.EventID == "" || evidence.FacilityID != report.FacilityID || report.ZoneID != "" && evidence.ZoneID != report.ZoneID || report.NodeID != "" && evidence.NodeID != report.NodeID {
			return invalid()
		}
		evidenceIDs[evidence.EvidenceID] = true
		if evidence.ObservedAt == nil || evidence.ObservedAt.IsZero() || evidence.ObservedAt.Before(report.Since) || evidence.ObservedAt.After(report.Until) || evidence.Direction != "in" && evidence.Direction != "out" || evidence.SourceResult != "pass" && evidence.SourceResult != "fail" {
			return invalid()
		}
		if evidence.IdentityPresent != (safeGateIdentityRef(evidence.IdentityRef) != "") || !evidence.IdentityPresent && evidence.IdentityRef != "" {
			return invalid()
		}
		if err := countSignals(evidence.Eligibility); err != nil {
			return err
		}
		signals := evidence.Eligibility
		if signals.CoPresenceProof.Eligible || signals.PrivateDailyPresence.Eligible || signals.ReliabilityVerification.Eligible {
			if len(structuralGateReasons(evidence)) > 0 || !evidence.AcceptedPresence {
				return invalid()
			}
		}
		if signals.PrivateDailyPresence.Eligible && evidence.Direction != "in" || signals.ReliabilityVerification.Eligible && (evidence.SourceResult != "pass" || !evidence.SourceCommitted || evidence.SessionState != "closed") {
			return invalid()
		}
		if evidence.SourceResult == "pass" {
			expected.SourcePass++
		} else {
			expected.SourceFail++
		}
		if evidence.AcceptedPresence {
			if evidence.AcceptancePath == "touchnet_pass" {
				expected.AcceptedSourcePass++
			} else if evidence.AcceptancePath == "always_admit" || evidence.AcceptancePath == "grace_until" || evidence.AcceptancePath == "facility_window" {
				expected.AcceptedPolicy++
			}
			if evidence.SourceResult == "pass" && !evidence.SourceCommitted || evidence.SourceResult == "fail" && evidence.AcceptancePath != "always_admit" && evidence.AcceptancePath != "grace_until" && evidence.AcceptancePath != "facility_window" {
				return invalid()
			}
		}
	}
	for _, session := range report.Sessions {
		if session.SessionID == "" || sessionIDs[session.SessionID] || session.IdentityPresent != (safeGateIdentityRef(session.IdentityRef) != "") || !session.IdentityPresent && session.IdentityRef != "" {
			return invalid()
		}
		sessionIDs[session.SessionID] = true
		if err := countSignals(session.Eligibility); err != nil {
			return err
		}
		if session.EntryAt != nil && (session.EntryAt.IsZero() || session.EntryAt.After(report.Until) || session.EntryEventID == "") || session.ExitAt != nil && (session.ExitAt.IsZero() || session.ExitAt.Before(report.Since) || session.ExitAt.After(report.Until) || session.ExitEventID == "") {
			return invalid()
		}
		if session.EntryAt != nil && session.ExitAt != nil && session.ExitAt.Before(*session.EntryAt) {
			return invalid()
		}
		switch session.State {
		case "closed":
			if session.EntryAt == nil || session.ExitAt == nil {
				return invalid()
			}
		case "open":
			if session.EntryAt == nil || session.ExitAt != nil || session.Eligibility.ReliabilityVerification.Eligible {
				return invalid()
			}
		case "unmatched_exit":
			if session.EntryAt != nil || session.ExitAt == nil || session.Eligibility.CoPresenceProof.Eligible || session.Eligibility.PrivateDailyPresence.Eligible || session.Eligibility.ReliabilityVerification.Eligible {
				return invalid()
			}
		default:
			return invalid()
		}
		if !session.IdentityPresent && (session.Eligibility.CoPresenceProof.Eligible || session.Eligibility.PrivateDailyPresence.Eligible || session.Eligibility.ReliabilityVerification.Eligible) {
			return invalid()
		}
		if report.NodeID != "" && session.EntryNodeID != report.NodeID && session.ExitNodeID != report.NodeID {
			return invalid()
		}
		for _, evidence := range report.Evidence {
			var at *time.Time
			direction := ""
			if evidence.EventID == session.EntryEventID {
				at = session.EntryAt
				direction = "in"
			}
			if evidence.EventID == session.ExitEventID {
				at = session.ExitAt
				direction = "out"
			}
			if at != nil && (evidence.IdentityRef != session.IdentityRef || !evidence.ObservedAt.Equal(*at) || evidence.Direction != direction || evidence.SourceResult != "pass") {
				return invalid()
			}
		}
	}
	for _, evidence := range report.Evidence {
		if !evidence.Eligibility.ReliabilityVerification.Eligible {
			continue
		}
		found := false
		for _, session := range report.Sessions {
			if session.State == "closed" && session.IdentityRef == evidence.IdentityRef && (session.EntryEventID == evidence.EventID || session.ExitEventID == evidence.EventID) && session.Eligibility.ReliabilityVerification.Eligible {
				found = true
			}
		}
		if !found {
			return invalid()
		}
	}
	if expected.SourcePass != report.Summary.SourcePass || expected.SourceFail != report.Summary.SourceFail || expected.AcceptedSourcePass != report.Summary.AcceptedSourcePass || expected.AcceptedPolicy != report.Summary.AcceptedPolicy || expected.EligibleCoPresence != report.Summary.EligibleCoPresence || expected.EligibleDailyPresence != report.Summary.EligibleDailyPresence || expected.EligibleReliabilityVerification != report.Summary.EligibleReliabilityVerification || expected.NoEligibleSignals != report.Summary.NoEligibleSignals {
		return invalid()
	}
	return nil
}
