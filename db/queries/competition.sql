-- name: ListCompetitionSessions :many
SELECT id,
       owner_user_id,
       display_name,
       sport_key,
       facility_key,
       zone_key,
       participants_per_side,
       queue_version,
       status,
       created_at,
       updated_at,
       archived_at
FROM apollo.competition_sessions
ORDER BY created_at DESC, id DESC;

-- name: GetCompetitionSessionByID :one
SELECT id,
       owner_user_id,
       display_name,
       sport_key,
       facility_key,
       zone_key,
       participants_per_side,
       queue_version,
       status,
       created_at,
       updated_at,
       archived_at
FROM apollo.competition_sessions
WHERE id = $1
LIMIT 1;

-- name: CreateCompetitionSession :one
INSERT INTO apollo.competition_sessions (
  owner_user_id,
  display_name,
  sport_key,
  facility_key,
  zone_key,
  participants_per_side,
  status,
  updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, 'draft', NOW())
RETURNING id,
          owner_user_id,
          display_name,
          sport_key,
          facility_key,
          zone_key,
          participants_per_side,
          queue_version,
          status,
          created_at,
          updated_at,
          archived_at;

-- name: CreateCompetitionStaffActionAttribution :one
INSERT INTO apollo.competition_staff_action_attributions (
  actor_user_id,
  actor_role,
  session_id,
  capability,
  trusted_surface_key,
  trusted_surface_label,
  action,
  competition_session_id,
  competition_session_team_id,
  competition_match_id,
  subject_user_id,
  occurred_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id,
          actor_user_id,
          actor_role,
          session_id,
          capability,
          trusted_surface_key,
          trusted_surface_label,
          action,
          competition_session_id,
          competition_session_team_id,
          competition_match_id,
          subject_user_id,
          occurred_at;

-- name: OpenCompetitionSessionQueue :one
UPDATE apollo.competition_sessions
SET status = 'queue_open',
    queue_version = 1,
    updated_at = $3
WHERE id = $1
  AND owner_user_id = $2
  AND status = 'draft'
RETURNING id,
          owner_user_id,
          display_name,
          sport_key,
          facility_key,
          zone_key,
          participants_per_side,
          queue_version,
          status,
          created_at,
          updated_at,
          archived_at;

-- name: UpdateCompetitionSessionStatus :one
UPDATE apollo.competition_sessions
SET status = $3,
    updated_at = $4,
    archived_at = CASE
        WHEN $3 = 'archived' THEN $4
        ELSE archived_at
    END
WHERE id = $1
  AND owner_user_id = $2
  AND status = $5
RETURNING id,
          owner_user_id,
          display_name,
          sport_key,
          facility_key,
          zone_key,
          participants_per_side,
          queue_version,
          status,
          created_at,
          updated_at,
          archived_at;

-- name: BumpCompetitionSessionQueueVersion :one
UPDATE apollo.competition_sessions
SET queue_version = queue_version + 1,
    updated_at = $3
WHERE id = $1
  AND owner_user_id = $2
  AND status = 'queue_open'
RETURNING id,
          owner_user_id,
          display_name,
          sport_key,
          facility_key,
          zone_key,
          participants_per_side,
          queue_version,
          status,
          created_at,
          updated_at,
          archived_at;

-- name: AssignCompetitionSessionFromQueue :one
UPDATE apollo.competition_sessions
SET status = 'assigned',
    queue_version = queue_version + 1,
    updated_at = $4
WHERE id = $1
  AND owner_user_id = $2
  AND status = 'queue_open'
  AND queue_version = $3
RETURNING id,
          owner_user_id,
          display_name,
          sport_key,
          facility_key,
          zone_key,
          participants_per_side,
          queue_version,
          status,
          created_at,
          updated_at,
          archived_at;

-- name: CountDraftCompetitionMatchesBySessionID :one
SELECT count(*)
FROM apollo.competition_matches
WHERE competition_session_id = $1
  AND status = 'draft';

-- name: CountCompetitionSessionQueueMembersBySessionID :one
SELECT count(*)
FROM apollo.competition_session_queue_members
WHERE competition_session_id = $1;

-- name: ListCompetitionSessionQueueMembersBySessionID :many
SELECT q.competition_session_id,
       q.user_id,
       q.joined_at,
       u.display_name,
       u.preferences,
       COALESCE(lm.status, 'not_joined') AS lobby_membership_status
FROM apollo.competition_session_queue_members AS q
INNER JOIN apollo.users AS u
  ON u.id = q.user_id
LEFT JOIN apollo.lobby_memberships AS lm
  ON lm.user_id = q.user_id
WHERE q.competition_session_id = $1
ORDER BY q.joined_at ASC, q.user_id ASC;

-- name: CreateCompetitionSessionQueueMember :one
INSERT INTO apollo.competition_session_queue_members (
  competition_session_id,
  user_id,
  joined_at
)
VALUES ($1, $2, $3)
RETURNING competition_session_id,
          user_id,
          joined_at;

-- name: UpsertCompetitionQueueIntent :one
INSERT INTO apollo.competition_queue_intents (
  competition_session_id,
  user_id,
  facility_key,
  sport_key,
  mode_key,
  tier,
  status,
  updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (competition_session_id, user_id)
DO UPDATE SET
  facility_key = EXCLUDED.facility_key,
  sport_key = EXCLUDED.sport_key,
  mode_key = EXCLUDED.mode_key,
  tier = EXCLUDED.tier,
  status = EXCLUDED.status,
  updated_at = EXCLUDED.updated_at
RETURNING id,
          competition_session_id,
          user_id,
          facility_key,
          sport_key,
          mode_key,
          tier,
          status,
          created_at,
          updated_at;

-- name: UpdateCompetitionQueueIntentTier :one
UPDATE apollo.competition_queue_intents
SET tier = $3,
    status = 'active',
    updated_at = $4
WHERE competition_session_id = $1
  AND user_id = $2
  AND status = 'active'
RETURNING id,
          competition_session_id,
          user_id,
          facility_key,
          sport_key,
          mode_key,
          tier,
          status,
          created_at,
          updated_at;

-- name: UpdateCompetitionQueueIntentStatus :one
UPDATE apollo.competition_queue_intents
SET status = $3,
    updated_at = $4
WHERE competition_session_id = $1
  AND user_id = $2
RETURNING id,
          competition_session_id,
          user_id,
          facility_key,
          sport_key,
          mode_key,
          tier,
          status,
          created_at,
          updated_at;

-- name: CreateCompetitionQueueIntentEvent :one
INSERT INTO apollo.competition_queue_intent_events (
  competition_queue_intent_id,
  competition_session_id,
  user_id,
  event_type,
  facility_key,
  sport_key,
  mode_key,
  tier,
  status,
  actor_user_id,
  actor_role,
  actor_session_id,
  capability,
  trusted_surface_key,
  trusted_surface_label,
  occurred_at
)
VALUES (
  $1,
  $2,
  $3,
  'competition.queue_intent.updated',
  $4,
  $5,
  $6,
  $7,
  $8,
  $9,
  $10,
  $11,
  $12,
  $13,
  $14,
  $15
)
RETURNING id,
          competition_queue_intent_id,
          competition_session_id,
          user_id,
          event_type,
          facility_key,
          sport_key,
          mode_key,
          tier,
          status,
          actor_user_id,
          actor_role,
          actor_session_id,
          capability,
          trusted_surface_key,
          trusted_surface_label,
          occurred_at,
          created_at;

-- name: DeleteCompetitionSessionQueueMember :execrows
DELETE FROM apollo.competition_session_queue_members
WHERE competition_session_id = $1
  AND user_id = $2;

-- name: DeleteCompetitionSessionQueueMembersBySessionID :execrows
DELETE FROM apollo.competition_session_queue_members
WHERE competition_session_id = $1;

-- name: ListCompetitionMatchPreviewCandidatesBySessionID :many
SELECT qi.id AS competition_queue_intent_id,
       q.competition_session_id,
       q.user_id,
       u.display_name,
       u.preferences,
       q.joined_at,
       qi.facility_key,
       qi.sport_key,
       qi.mode_key,
       qi.tier,
       qi.updated_at AS queue_intent_updated_at,
       r.mu,
       r.sigma,
       r.matches_played,
       r.updated_at AS rating_updated_at
FROM apollo.competition_session_queue_members AS q
INNER JOIN apollo.users AS u
  ON u.id = q.user_id
INNER JOIN apollo.competition_queue_intents AS qi
  ON qi.competition_session_id = q.competition_session_id
 AND qi.user_id = q.user_id
 AND qi.status = 'active'
LEFT JOIN apollo.competition_member_ratings AS r
  ON r.user_id = q.user_id
 AND r.sport_key = qi.sport_key
 AND r.mode_key = qi.mode_key
WHERE q.competition_session_id = $1
ORDER BY q.joined_at ASC, q.user_id ASC;

-- name: UpsertCompetitionMatchPreview :one
INSERT INTO apollo.competition_match_previews (
  competition_session_id,
  queue_version,
  proposal_index,
  preview_version,
  policy_version,
  rating_engine,
  rating_policy_version,
  facility_key,
  sport_key,
  mode_key,
  tier,
  match_quality,
  predicted_win_probability,
  explanation_code,
  input_watermark,
  updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
ON CONFLICT (
  competition_session_id,
  queue_version,
  proposal_index,
  preview_version,
  policy_version
)
DO UPDATE SET
  rating_engine = EXCLUDED.rating_engine,
  rating_policy_version = EXCLUDED.rating_policy_version,
  facility_key = EXCLUDED.facility_key,
  sport_key = EXCLUDED.sport_key,
  mode_key = EXCLUDED.mode_key,
  tier = EXCLUDED.tier,
  match_quality = EXCLUDED.match_quality,
  predicted_win_probability = EXCLUDED.predicted_win_probability,
  explanation_code = EXCLUDED.explanation_code,
  input_watermark = EXCLUDED.input_watermark,
  updated_at = EXCLUDED.updated_at
RETURNING id,
          competition_session_id,
          queue_version,
          proposal_index,
          preview_version,
          policy_version,
          rating_engine,
          rating_policy_version,
          facility_key,
          sport_key,
          mode_key,
          tier,
          match_quality,
          predicted_win_probability,
          explanation_code,
          input_watermark,
          created_at,
          updated_at;

-- name: DeleteCompetitionMatchPreviewMembersByPreviewID :execrows
DELETE FROM apollo.competition_match_preview_members
WHERE competition_match_preview_id = $1;

-- name: CreateCompetitionMatchPreviewMember :one
INSERT INTO apollo.competition_match_preview_members (
  competition_match_preview_id,
  side_index,
  slot_index,
  competition_queue_intent_id,
  user_id,
  rating_mu,
  rating_sigma,
  rating_matches_played,
  rating_source,
  tier
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING competition_match_preview_id,
          side_index,
          slot_index,
          competition_queue_intent_id,
          user_id,
          rating_mu,
          rating_sigma,
          rating_matches_played,
          rating_source,
          tier;

-- name: UpsertCompetitionMatchPreviewGeneratedEvent :one
INSERT INTO apollo.competition_match_preview_events (
  competition_match_preview_id,
  competition_session_id,
  event_type,
  queue_version,
  preview_version,
  policy_version,
  rating_engine,
  rating_policy_version,
  facility_key,
  sport_key,
  mode_key,
  tier,
  match_quality,
  predicted_win_probability,
  explanation_code,
  actor_user_id,
  actor_role,
  actor_session_id,
  capability,
  trusted_surface_key,
  trusted_surface_label,
  occurred_at
)
VALUES (
  $1,
  $2,
  'competition.match_preview.generated',
  $3,
  $4,
  $5,
  $6,
  $7,
  $8,
  $9,
  $10,
  $11,
  $12,
  $13,
  $14,
  $15,
  $16,
  $17,
  $18,
  $19,
  $20,
  $21
)
ON CONFLICT (competition_match_preview_id, event_type)
DO UPDATE SET
  queue_version = EXCLUDED.queue_version,
  preview_version = EXCLUDED.preview_version,
  policy_version = EXCLUDED.policy_version,
  rating_engine = EXCLUDED.rating_engine,
  rating_policy_version = EXCLUDED.rating_policy_version,
  facility_key = EXCLUDED.facility_key,
  sport_key = EXCLUDED.sport_key,
  mode_key = EXCLUDED.mode_key,
  tier = EXCLUDED.tier,
  match_quality = EXCLUDED.match_quality,
  predicted_win_probability = EXCLUDED.predicted_win_probability,
  explanation_code = EXCLUDED.explanation_code,
  actor_user_id = EXCLUDED.actor_user_id,
  actor_role = EXCLUDED.actor_role,
  actor_session_id = EXCLUDED.actor_session_id,
  capability = EXCLUDED.capability,
  trusted_surface_key = EXCLUDED.trusted_surface_key,
  trusted_surface_label = EXCLUDED.trusted_surface_label,
  occurred_at = EXCLUDED.occurred_at
RETURNING id,
          competition_match_preview_id,
          competition_session_id,
          event_type,
          queue_version,
          preview_version,
          policy_version,
          rating_engine,
          rating_policy_version,
          facility_key,
          sport_key,
          mode_key,
          tier,
          match_quality,
          predicted_win_probability,
          explanation_code,
          actor_user_id,
          actor_role,
          actor_session_id,
          capability,
          trusted_surface_key,
          trusted_surface_label,
          occurred_at,
          created_at;

-- name: ListCompetitionSessionTeamsBySessionID :many
SELECT id,
       competition_session_id,
       side_index,
       created_at
FROM apollo.competition_session_teams
WHERE competition_session_id = $1
ORDER BY side_index ASC, id ASC;

-- name: GetCompetitionSessionTeamByID :one
SELECT id,
       competition_session_id,
       side_index,
       created_at
FROM apollo.competition_session_teams
WHERE id = $1
LIMIT 1;

-- name: CreateCompetitionSessionTeam :one
INSERT INTO apollo.competition_session_teams (
  competition_session_id,
  side_index
)
VALUES ($1, $2)
RETURNING id,
          competition_session_id,
          side_index,
          created_at;

-- name: DeleteCompetitionSessionTeam :execrows
DELETE FROM apollo.competition_session_teams
WHERE id = $1;

-- name: CountCompetitionRosterMembersByTeamID :one
SELECT count(*)
FROM apollo.competition_team_roster_members
WHERE competition_session_team_id = $1;

-- name: CompetitionSessionHasRosterMemberUser :one
SELECT EXISTS (
    SELECT 1
    FROM apollo.competition_team_roster_members
    WHERE competition_session_id = $1
      AND user_id = $2
);

-- name: ListCompetitionTeamRosterMembersBySessionID :many
SELECT r.competition_session_team_id,
       r.user_id,
       r.slot_index,
       r.created_at,
       u.display_name
FROM apollo.competition_team_roster_members AS r
INNER JOIN apollo.competition_session_teams AS t
  ON t.id = r.competition_session_team_id
INNER JOIN apollo.users AS u
  ON u.id = r.user_id
WHERE t.competition_session_id = $1
ORDER BY t.side_index ASC, r.slot_index ASC, r.user_id ASC;

-- name: CreateCompetitionTeamRosterMember :one
INSERT INTO apollo.competition_team_roster_members (
  competition_session_id,
  competition_session_team_id,
  user_id,
  slot_index
)
VALUES ($1, $2, $3, $4)
RETURNING competition_session_id,
          competition_session_team_id,
          user_id,
          slot_index,
          created_at;

-- name: DeleteCompetitionTeamRosterMember :execrows
DELETE FROM apollo.competition_team_roster_members
WHERE competition_session_team_id = $1
  AND user_id = $2;

-- name: CompetitionTeamHasMatchReference :one
SELECT EXISTS (
    SELECT 1
    FROM apollo.competition_match_side_slots AS s
    WHERE s.competition_session_team_id = $1
);

-- name: ListCompetitionMatchesBySessionID :many
SELECT id,
       competition_session_id,
       match_index,
       status,
       result_version,
       canonical_result_id,
       created_at,
       updated_at,
       archived_at
FROM apollo.competition_matches
WHERE competition_session_id = $1
ORDER BY match_index ASC, id ASC;

-- name: GetCompetitionMatchByID :one
SELECT id,
       competition_session_id,
       match_index,
       status,
       result_version,
       canonical_result_id,
       created_at,
       updated_at,
       archived_at
FROM apollo.competition_matches
WHERE id = $1
LIMIT 1;

-- name: CreateCompetitionMatch :one
INSERT INTO apollo.competition_matches (
  competition_session_id,
  match_index,
  status,
  updated_at
)
VALUES ($1, $2, $3, NOW())
RETURNING id,
          competition_session_id,
          match_index,
          status,
          result_version,
          canonical_result_id,
          created_at,
          updated_at,
          archived_at;

-- name: ArchiveCompetitionMatch :one
UPDATE apollo.competition_matches
SET status = 'archived',
    archived_at = $2,
    updated_at = $2
WHERE id = $1
  AND status IN ('draft', 'assigned', 'in_progress')
RETURNING id,
          competition_session_id,
          match_index,
          status,
          result_version,
          canonical_result_id,
          created_at,
          updated_at,
          archived_at;

-- name: UpdateCompetitionMatchStatusBySessionID :execrows
UPDATE apollo.competition_matches
SET status = $2,
    updated_at = $4,
    archived_at = CASE
        WHEN $2 = 'archived' THEN $4
        ELSE archived_at
    END
WHERE competition_session_id = $1
  AND status = $3;

-- name: CreateCompetitionMatchSideSlot :one
INSERT INTO apollo.competition_match_side_slots (
  competition_match_id,
  competition_session_team_id,
  side_index
)
VALUES ($1, $2, $3)
RETURNING competition_match_id,
          competition_session_team_id,
          side_index,
          created_at;

-- name: ListCompetitionMatchSideSlotsBySessionID :many
SELECT s.competition_match_id,
       s.competition_session_team_id,
       s.side_index,
       s.created_at
FROM apollo.competition_match_side_slots AS s
INNER JOIN apollo.competition_matches AS m
  ON m.id = s.competition_match_id
WHERE m.competition_session_id = $1
ORDER BY m.match_index ASC, s.side_index ASC, s.competition_session_team_id ASC;
