package telemetry

import (
	"strings"
	"testing"
	"time"
)

func TestRenderPrometheusExportsMilestoneMetrics(t *testing.T) {
	RecordResultWriteAttempt("record")
	RecordResultWriteReject("stale version")
	RecordTrustedSurfaceFailure("missing trusted surface")
	RecordRatingRebuild(3, "apollo_legacy_rating_v1", 25*time.Millisecond, 2)
	ObserveGameIdentityProjection(10 * time.Millisecond)

	body := RenderPrometheus()
	required := []string{
		"competition_result_write_attempt_total",
		"competition_result_write_reject_total",
		"competition_consensus_vote_total",
		"competition_dispute_open_total",
		"competition_dispute_resolution_duration_seconds",
		"rating_update_duration_seconds",
		"rating_rebuild_rows_scanned_total",
		"rating_policy_delta_total",
		"ares_preview_total",
		"ares_queue_assignment_failure_total",
		"trusted_surface_failure_total",
		"booking_public_submit_total",
		"booking_idempotency_conflict_total",
		"booking_approval_conflict_total",
		"public_leak_test_failure_total",
		"tournament_advancement_total",
		"tournament_seed_lock_total",
		"safety_report_open_total",
		"safety_block_active_total",
		"reliability_event_total",
		"game_identity_projection_duration_seconds",
	}
	for _, metric := range required {
		if !strings.Contains(body, metric) {
			t.Fatalf("RenderPrometheus() missing %q in:\n%s", metric, body)
		}
	}
	if !strings.Contains(body, `competition_consensus_vote_total{tier="not_active",outcome="deferred"} 0`) {
		t.Fatalf("consensus vote deferred marker missing:\n%s", body)
	}
}
