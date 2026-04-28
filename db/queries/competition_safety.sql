-- name: CreateCompetitionSafetyReport :one
INSERT INTO apollo.competition_safety_reports (
  competition_session_id,
  competition_match_id,
  competition_session_team_id,
  competition_tournament_id,
  reporter_user_id,
  subject_user_id,
  target_type,
  target_id,
  reason_code,
  note,
  actor_user_id,
  actor_role,
  actor_session_id,
  capability,
  trusted_surface_key,
  trusted_surface_label,
  occurred_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING id,
          competition_session_id,
          competition_match_id,
          competition_session_team_id,
          competition_tournament_id,
          reporter_user_id,
          subject_user_id,
          target_type,
          target_id,
          reason_code,
          status,
          privacy_scope,
          note,
          actor_user_id,
          actor_role,
          actor_session_id,
          capability,
          trusted_surface_key,
          trusted_surface_label,
          occurred_at,
          created_at;

-- name: CreateCompetitionSafetyBlock :one
INSERT INTO apollo.competition_safety_blocks (
  competition_session_id,
  competition_match_id,
  blocker_user_id,
  blocked_user_id,
  reason_code,
  actor_user_id,
  actor_role,
  actor_session_id,
  capability,
  trusted_surface_key,
  trusted_surface_label,
  occurred_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id,
          competition_session_id,
          competition_match_id,
          blocker_user_id,
          blocked_user_id,
          reason_code,
          status,
          privacy_scope,
          actor_user_id,
          actor_role,
          actor_session_id,
          capability,
          trusted_surface_key,
          trusted_surface_label,
          occurred_at,
          created_at;

-- name: CreateCompetitionReliabilityEvent :one
INSERT INTO apollo.competition_reliability_events (
  competition_session_id,
  competition_match_id,
  subject_user_id,
  reliability_type,
  severity,
  note,
  actor_user_id,
  actor_role,
  actor_session_id,
  capability,
  trusted_surface_key,
  trusted_surface_label,
  occurred_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id,
          competition_session_id,
          competition_match_id,
          subject_user_id,
          reliability_type,
          severity,
          privacy_scope,
          note,
          actor_user_id,
          actor_role,
          actor_session_id,
          capability,
          trusted_surface_key,
          trusted_surface_label,
          occurred_at,
          created_at;

-- name: CreateCompetitionSafetyEvent :one
INSERT INTO apollo.competition_safety_events (
  event_type,
  competition_session_id,
  competition_match_id,
  competition_session_team_id,
  competition_tournament_id,
  safety_report_id,
  safety_block_id,
  reliability_event_id,
  reporter_user_id,
  subject_user_id,
  blocker_user_id,
  blocked_user_id,
  target_type,
  target_id,
  reason_code,
  reliability_type,
  severity,
  actor_user_id,
  actor_role,
  actor_session_id,
  capability,
  trusted_surface_key,
  trusted_surface_label,
  occurred_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
RETURNING id,
          event_type,
          competition_session_id,
          competition_match_id,
          competition_session_team_id,
          competition_tournament_id,
          safety_report_id,
          safety_block_id,
          reliability_event_id,
          reporter_user_id,
          subject_user_id,
          blocker_user_id,
          blocked_user_id,
          target_type,
          target_id,
          reason_code,
          reliability_type,
          severity,
          privacy_scope,
          actor_user_id,
          actor_role,
          actor_session_id,
          capability,
          trusted_surface_key,
          trusted_surface_label,
          occurred_at,
          created_at;

-- name: GetCompetitionSafetyReviewSummary :one
SELECT
  (SELECT count(*) FROM apollo.competition_safety_reports) AS report_count,
  (SELECT count(*) FROM apollo.competition_safety_blocks WHERE status = 'active') AS block_count,
  (SELECT count(*) FROM apollo.competition_reliability_events) AS reliability_event_count,
  (SELECT count(*) FROM apollo.competition_safety_events) AS audit_event_count;

-- name: ListCompetitionSafetyReportsForReview :many
SELECT id,
       competition_session_id,
       competition_match_id,
       competition_session_team_id,
       competition_tournament_id,
       reporter_user_id,
       subject_user_id,
       target_type,
       target_id,
       reason_code,
       status,
       privacy_scope,
       note,
       actor_user_id,
       actor_role,
       actor_session_id,
       capability,
       trusted_surface_key,
       trusted_surface_label,
       occurred_at,
       created_at
FROM apollo.competition_safety_reports
ORDER BY occurred_at DESC, id DESC
LIMIT $1;

-- name: ListCompetitionSafetyBlocksForReview :many
SELECT id,
       competition_session_id,
       competition_match_id,
       blocker_user_id,
       blocked_user_id,
       reason_code,
       status,
       privacy_scope,
       actor_user_id,
       actor_role,
       actor_session_id,
       capability,
       trusted_surface_key,
       trusted_surface_label,
       occurred_at,
       created_at
FROM apollo.competition_safety_blocks
ORDER BY occurred_at DESC, id DESC
LIMIT $1;

-- name: ListCompetitionReliabilityEventsForReview :many
SELECT id,
       competition_session_id,
       competition_match_id,
       subject_user_id,
       reliability_type,
       severity,
       privacy_scope,
       note,
       actor_user_id,
       actor_role,
       actor_session_id,
       capability,
       trusted_surface_key,
       trusted_surface_label,
       occurred_at,
       created_at
FROM apollo.competition_reliability_events
ORDER BY occurred_at DESC, id DESC
LIMIT $1;

-- name: ListCompetitionSafetyAuditEventsForReview :many
SELECT id,
       event_type,
       competition_session_id,
       competition_match_id,
       competition_session_team_id,
       competition_tournament_id,
       safety_report_id,
       safety_block_id,
       reliability_event_id,
       reporter_user_id,
       subject_user_id,
       blocker_user_id,
       blocked_user_id,
       target_type,
       target_id,
       reason_code,
       reliability_type,
       severity,
       privacy_scope,
       actor_user_id,
       actor_role,
       actor_session_id,
       capability,
       trusted_surface_key,
       trusted_surface_label,
       occurred_at,
       created_at
FROM apollo.competition_safety_events
ORDER BY occurred_at DESC, id DESC
LIMIT $1;
