package telemetry

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var registry = newRegistry()

type counterVec struct {
	name       string
	help       string
	labelNames []string
	defaults   [][]string

	mu     sync.Mutex
	values map[string]float64
}

type summaryVec struct {
	name       string
	help       string
	labelNames []string
	defaults   [][]string

	mu     sync.Mutex
	values map[string]summaryValue
}

type summaryValue struct {
	count uint64
	sum   float64
}

type telemetryRegistry struct {
	counters  []*counterVec
	summaries []*summaryVec

	competitionResultWriteAttemptTotal *counterVec
	competitionResultWriteRejectTotal  *counterVec
	competitionConsensusVoteTotal      *counterVec
	competitionDisputeOpenTotal        *counterVec
	ratingRebuildRowsScannedTotal      *counterVec
	ratingPolicyDeltaTotal             *counterVec
	aresPreviewTotal                   *counterVec
	aresQueueAssignmentFailureTotal    *counterVec
	trustedSurfaceFailureTotal         *counterVec
	bookingPublicSubmitTotal           *counterVec
	bookingIdempotencyConflictTotal    *counterVec
	bookingApprovalConflictTotal       *counterVec
	bookingPublicReceiptNotFoundTotal  *counterVec
	publicLeakTestFailureTotal         *counterVec
	tournamentAdvancementTotal         *counterVec
	tournamentSeedLockTotal            *counterVec
	safetyReportOpenTotal              *counterVec
	safetyBlockActiveTotal             *counterVec
	reliabilityEventTotal              *counterVec

	disputeResolutionDurationSeconds      *summaryVec
	ratingUpdateDurationSeconds           *summaryVec
	gameIdentityProjectionDurationSeconds *summaryVec
}

func newRegistry() *telemetryRegistry {
	r := &telemetryRegistry{}
	r.competitionResultWriteAttemptTotal = r.counter("competition_result_write_attempt_total", "Competition result write attempts observed by APOLLO.", []string{"operation"}, [][]string{{"record"}, {"finalize"}, {"dispute"}, {"correct"}, {"void"}})
	r.competitionResultWriteRejectTotal = r.counter("competition_result_write_reject_total", "Competition result writes rejected by APOLLO.", []string{"reason"}, [][]string{{"validation"}, {"stale_version"}, {"unauthorized"}, {"runtime"}})
	r.competitionConsensusVoteTotal = r.counter("competition_consensus_vote_total", "Competition consensus vote telemetry. Consensus voting is deferred in Milestone 3.0.", []string{"tier", "outcome"}, [][]string{{"not_active", "deferred"}})
	r.competitionDisputeOpenTotal = r.counter("competition_dispute_open_total", "Competition disputes opened.", []string{"tier", "reason"}, [][]string{{"unscoped", "result_disputed"}})
	r.ratingRebuildRowsScannedTotal = r.counter("rating_rebuild_rows_scanned_total", "Rows scanned by APOLLO rating rebuilds.", nil, nil)
	r.ratingPolicyDeltaTotal = r.counter("rating_policy_delta_total", "Rating policy comparison deltas emitted by APOLLO.", []string{"legacy", "openskill"}, [][]string{{"legacy_elo_like", "openskill"}})
	r.aresPreviewTotal = r.counter("ares_preview_total", "ARES competition preview requests.", []string{"sport", "mode", "tier"}, [][]string{{"unknown", "unknown", "unknown"}})
	r.aresQueueAssignmentFailureTotal = r.counter("ares_queue_assignment_failure_total", "ARES queue assignment failures.", []string{"reason"}, [][]string{{"runtime"}})
	r.trustedSurfaceFailureTotal = r.counter("trusted_surface_failure_total", "Trusted-surface verification failures.", []string{"reason"}, [][]string{{"missing"}, {"invalid"}, {"capability_denied"}})
	r.bookingPublicSubmitTotal = r.counter("booking_public_submit_total", "Public booking submit attempts.", nil, nil)
	r.bookingIdempotencyConflictTotal = r.counter("booking_idempotency_conflict_total", "Booking idempotency conflicts.", nil, nil)
	r.bookingApprovalConflictTotal = r.counter("booking_approval_conflict_total", "Booking approval conflicts.", nil, nil)
	r.bookingPublicReceiptNotFoundTotal = r.counter("booking_public_receipt_not_found_total", "Public booking receipt not-found responses.", nil, nil)
	r.publicLeakTestFailureTotal = r.counter("public_leak_test_failure_total", "Public leak test failures detected by APOLLO smoke probes.", nil, nil)
	r.tournamentAdvancementTotal = r.counter("tournament_advancement_total", "Internal tournament advancements.", []string{"format"}, [][]string{{"single_elimination"}})
	r.tournamentSeedLockTotal = r.counter("tournament_seed_lock_total", "Internal tournament seed or team-lock operations.", nil, nil)
	r.safetyReportOpenTotal = r.counter("safety_report_open_total", "Competition safety reports opened.", []string{"category"}, [][]string{{"uncategorized"}})
	r.safetyBlockActiveTotal = r.counter("safety_block_active_total", "Competition safety blocks opened.", nil, nil)
	r.reliabilityEventTotal = r.counter("reliability_event_total", "Competition reliability events recorded.", []string{"kind"}, [][]string{{"uncategorized"}})

	r.disputeResolutionDurationSeconds = r.summary("competition_dispute_resolution_duration_seconds", "Competition dispute resolution duration in seconds.", nil, nil)
	r.ratingUpdateDurationSeconds = r.summary("rating_update_duration_seconds", "APOLLO rating update duration in seconds.", []string{"policy_version"}, [][]string{{"apollo_rating_policy_wrapper_v1"}})
	r.gameIdentityProjectionDurationSeconds = r.summary("game_identity_projection_duration_seconds", "Game identity projection duration in seconds.", nil, nil)
	return r
}

func (r *telemetryRegistry) counter(name string, help string, labelNames []string, defaults [][]string) *counterVec {
	vec := &counterVec{name: name, help: help, labelNames: labelNames, defaults: defaults, values: make(map[string]float64)}
	r.counters = append(r.counters, vec)
	return vec
}

func (r *telemetryRegistry) summary(name string, help string, labelNames []string, defaults [][]string) *summaryVec {
	vec := &summaryVec{name: name, help: help, labelNames: labelNames, defaults: defaults, values: make(map[string]summaryValue)}
	r.summaries = append(r.summaries, vec)
	return vec
}

func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(RenderPrometheus()))
	}
}

func RenderPrometheus() string {
	var buffer bytes.Buffer
	for _, counter := range registry.counters {
		counter.writePrometheus(&buffer)
	}
	for _, summary := range registry.summaries {
		summary.writePrometheus(&buffer)
	}
	return buffer.String()
}

func RecordResultWriteAttempt(operation string) {
	registry.competitionResultWriteAttemptTotal.add(1, normalizeLabel(operation))
}

func RecordResultWriteReject(reason string) {
	registry.competitionResultWriteRejectTotal.add(1, normalizeReason(reason))
}

func RecordDisputeOpened(reason string) {
	registry.competitionDisputeOpenTotal.add(1, "unscoped", normalizeReason(reason))
}

func RecordRatingRebuild(rowsScanned int, policyVersion string, duration time.Duration, deltaCount int) {
	if rowsScanned > 0 {
		registry.ratingRebuildRowsScannedTotal.add(float64(rowsScanned))
	}
	registry.ratingUpdateDurationSeconds.observe(duration.Seconds(), normalizeLabel(policyVersion))
	if deltaCount > 0 {
		registry.ratingPolicyDeltaTotal.add(float64(deltaCount), "legacy_elo_like", "openskill")
	}
}

func RecordARESPreview(sport string, mode string, tier string) {
	registry.aresPreviewTotal.add(1, normalizeLabel(sport), normalizeLabel(mode), normalizeLabel(tier))
}

func RecordARESQueueAssignmentFailure(reason string) {
	registry.aresQueueAssignmentFailureTotal.add(1, normalizeReason(reason))
}

func RecordTrustedSurfaceFailure(reason string) {
	registry.trustedSurfaceFailureTotal.add(1, normalizeReason(reason))
}

func RecordPublicBookingSubmit() {
	registry.bookingPublicSubmitTotal.add(1)
}

func RecordBookingIdempotencyConflict() {
	registry.bookingIdempotencyConflictTotal.add(1)
}

func RecordBookingApprovalConflict() {
	registry.bookingApprovalConflictTotal.add(1)
}

func RecordPublicReceiptNotFound() {
	registry.bookingPublicReceiptNotFoundTotal.add(1)
}

func RecordPublicLeakTestFailure() {
	registry.publicLeakTestFailureTotal.add(1)
}

func RecordTournamentAdvancement(format string) {
	registry.tournamentAdvancementTotal.add(1, normalizeLabel(format))
}

func RecordTournamentSeedLock() {
	registry.tournamentSeedLockTotal.add(1)
}

func RecordSafetyReport(category string) {
	registry.safetyReportOpenTotal.add(1, normalizeLabel(category))
}

func RecordSafetyBlock() {
	registry.safetyBlockActiveTotal.add(1)
}

func RecordReliabilityEvent(kind string) {
	registry.reliabilityEventTotal.add(1, normalizeLabel(kind))
}

func ObserveGameIdentityProjection(duration time.Duration) {
	registry.gameIdentityProjectionDurationSeconds.observe(duration.Seconds())
}

func (c *counterVec) add(delta float64, labelValues ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[c.key(labelValues)] += delta
}

func (c *counterVec) writePrometheus(buffer *bytes.Buffer) {
	c.mu.Lock()
	values := make(map[string]float64, len(c.values)+len(c.defaults)+1)
	for key, value := range c.values {
		values[key] = value
	}
	for _, defaults := range c.defaults {
		key := c.key(defaults)
		if _, ok := values[key]; !ok {
			values[key] = 0
		}
	}
	if len(c.labelNames) == 0 {
		if _, ok := values[""]; !ok {
			values[""] = 0
		}
	}
	c.mu.Unlock()

	writeHelpAndType(buffer, c.name, c.help, "counter")
	keys := sortedKeys(values)
	for _, key := range keys {
		fmt.Fprintf(buffer, "%s%s %s\n", c.name, labelsFromKey(c.labelNames, key), formatFloat(values[key]))
	}
}

func (c *counterVec) key(labelValues []string) string {
	return keyFor(c.labelNames, labelValues)
}

func (s *summaryVec) observe(value float64, labelValues ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.key(labelValues)
	current := s.values[key]
	current.count++
	current.sum += value
	s.values[key] = current
}

func (s *summaryVec) writePrometheus(buffer *bytes.Buffer) {
	s.mu.Lock()
	values := make(map[string]summaryValue, len(s.values)+len(s.defaults)+1)
	for key, value := range s.values {
		values[key] = value
	}
	for _, defaults := range s.defaults {
		key := s.key(defaults)
		if _, ok := values[key]; !ok {
			values[key] = summaryValue{}
		}
	}
	if len(s.labelNames) == 0 {
		if _, ok := values[""]; !ok {
			values[""] = summaryValue{}
		}
	}
	s.mu.Unlock()

	writeHelpAndType(buffer, s.name, s.help, "summary")
	keys := sortedKeys(values)
	for _, key := range keys {
		labels := labelsFromKey(s.labelNames, key)
		value := values[key]
		fmt.Fprintf(buffer, "%s_count%s %d\n", s.name, labels, value.count)
		fmt.Fprintf(buffer, "%s_sum%s %s\n", s.name, labels, formatFloat(value.sum))
	}
}

func (s *summaryVec) key(labelValues []string) string {
	return keyFor(s.labelNames, labelValues)
}

func writeHelpAndType(buffer *bytes.Buffer, name string, help string, metricType string) {
	fmt.Fprintf(buffer, "# HELP %s %s\n", name, escapeHelp(help))
	fmt.Fprintf(buffer, "# TYPE %s %s\n", name, metricType)
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func keyFor(labelNames []string, labelValues []string) string {
	if len(labelNames) == 0 {
		return ""
	}
	values := make([]string, len(labelNames))
	for i := range labelNames {
		if i < len(labelValues) {
			values[i] = normalizeLabel(labelValues[i])
		}
		if values[i] == "" {
			values[i] = "unknown"
		}
	}
	return strings.Join(values, "\xff")
}

func labelsFromKey(labelNames []string, key string) string {
	if len(labelNames) == 0 {
		return ""
	}
	values := strings.Split(key, "\xff")
	parts := make([]string, len(labelNames))
	for i, name := range labelNames {
		value := "unknown"
		if i < len(values) && values[i] != "" {
			value = values[i]
		}
		parts[i] = fmt.Sprintf(`%s="%s"`, name, escapeLabel(value))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func normalizeReason(value string) string {
	normalized := normalizeLabel(value)
	switch {
	case strings.Contains(normalized, "trusted_surface") || strings.Contains(normalized, "trusted-surface"):
		return "trusted_surface"
	case strings.Contains(normalized, "capability") || strings.Contains(normalized, "denied") || strings.Contains(normalized, "forbidden"):
		return "capability_denied"
	case strings.Contains(normalized, "stale") || strings.Contains(normalized, "version"):
		return "stale_version"
	case strings.Contains(normalized, "missing"):
		return "missing"
	case strings.Contains(normalized, "invalid"):
		return "invalid"
	case normalized == "":
		return "unknown"
	default:
		return normalized
	}
}

func normalizeLabel(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '_' || r == '-':
			builder.WriteRune('_')
		default:
			builder.WriteRune('_')
		}
	}
	return strings.Trim(builder.String(), "_")
}

func escapeHelp(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, "\n", `\n`)
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return strings.ReplaceAll(value, "\n", `\n`)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
