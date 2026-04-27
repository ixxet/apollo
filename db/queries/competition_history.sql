-- name: CompleteCompetitionMatchWithResult :one
UPDATE apollo.competition_matches
SET status = 'completed',
    canonical_result_id = $2,
    result_version = result_version + 1,
    updated_at = $4
WHERE id = $1
  AND status = 'in_progress'
  AND result_version = $3
RETURNING id,
          competition_session_id,
          match_index,
          status,
          result_version,
          canonical_result_id,
          created_at,
          updated_at,
          archived_at;

-- name: UpdateCompetitionMatchCanonicalResult :one
UPDATE apollo.competition_matches
SET canonical_result_id = $2,
    result_version = result_version + 1,
    updated_at = $4
WHERE id = $1
  AND result_version = $3
RETURNING id,
          competition_session_id,
          match_index,
          status,
          result_version,
          canonical_result_id,
          created_at,
          updated_at,
          archived_at;

-- name: CompleteCompetitionSession :one
UPDATE apollo.competition_sessions
SET status = 'completed',
    updated_at = $3
WHERE id = $1
  AND owner_user_id = $2
  AND status = 'in_progress'
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

-- name: CreateCompetitionMatchResult :one
INSERT INTO apollo.competition_match_results (
  competition_match_id,
  recorded_by_user_id,
  recorded_at,
  result_status,
  dispute_status,
  correction_id,
  supersedes_result_id,
  finalized_at,
  corrected_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id,
          competition_match_id,
          recorded_by_user_id,
          recorded_at,
          result_status,
          dispute_status,
          correction_id,
          supersedes_result_id,
          finalized_at,
          corrected_at;

-- name: CreateCompetitionMatchResultSide :one
INSERT INTO apollo.competition_match_result_sides (
  competition_match_result_id,
  competition_match_id,
  side_index,
  competition_session_team_id,
  outcome
)
VALUES ($1, $2, $3, $4, $5)
RETURNING competition_match_result_id,
          competition_match_id,
          side_index,
          competition_session_team_id,
          outcome;

-- name: GetCompetitionCanonicalMatchResultByMatchID :one
SELECT r.id,
       r.competition_match_id,
       recorded_by_user_id,
       recorded_at,
       result_status,
       dispute_status,
       correction_id,
       supersedes_result_id,
       finalized_at,
       corrected_at
FROM apollo.competition_matches AS m
INNER JOIN apollo.competition_match_results AS r
  ON r.id = m.canonical_result_id
WHERE m.id = $1
LIMIT 1;

-- name: ListCompetitionMatchResultRowsBySessionID :many
SELECT m.id AS competition_match_id,
       r.id AS competition_match_result_id,
       r.recorded_by_user_id,
       r.recorded_at,
       r.result_status,
       r.dispute_status,
       r.correction_id,
       r.supersedes_result_id,
       r.finalized_at,
       r.corrected_at,
       rs.side_index,
       rs.competition_session_team_id,
       rs.outcome
FROM apollo.competition_matches AS m
INNER JOIN apollo.competition_match_results AS r
  ON r.id = m.canonical_result_id
INNER JOIN apollo.competition_match_result_sides AS rs
  ON rs.competition_match_result_id = r.id
WHERE m.competition_session_id = $1
ORDER BY m.match_index ASC, rs.side_index ASC;

-- name: CountCompetitionIncompleteActiveMatchesBySessionID :one
SELECT count(*)
FROM apollo.competition_matches
WHERE competition_session_id = $1
  AND status NOT IN ('completed', 'archived');

-- name: ListCompetitionRatingParticipantsBySport :many
SELECT m.id AS competition_match_id,
       r.id AS competition_match_result_id,
       s.sport_key,
       sp.competition_mode,
       sp.sides_per_match,
       s.participants_per_side,
       r.recorded_at,
       rs.competition_session_team_id,
       rs.side_index,
       rs.outcome,
       rm.user_id
FROM apollo.competition_sessions AS s
INNER JOIN apollo.sports AS sp
  ON sp.sport_key = s.sport_key
INNER JOIN apollo.competition_matches AS m
  ON m.competition_session_id = s.id
INNER JOIN apollo.competition_match_results AS r
  ON r.id = m.canonical_result_id
INNER JOIN apollo.competition_match_result_sides AS rs
  ON rs.competition_match_result_id = r.id
INNER JOIN apollo.competition_team_roster_members AS rm
  ON rm.competition_session_id = s.id
 AND rm.competition_session_team_id = rs.competition_session_team_id
WHERE s.sport_key = $1
  AND m.status = 'completed'
  AND r.result_status IN ('finalized', 'corrected')
ORDER BY r.recorded_at ASC, m.id ASC, rs.side_index ASC, rm.user_id ASC;

-- name: DeleteCompetitionMemberRatingsBySportKey :execrows
DELETE FROM apollo.competition_member_ratings
WHERE sport_key = $1;

-- name: CreateCompetitionRatingEvent :one
INSERT INTO apollo.competition_rating_events (
  event_type,
  rating_engine,
  engine_version,
  policy_version,
  sport_key,
  mode_key,
  user_id,
  source_result_id,
  mu,
  sigma,
  delta_mu,
  delta_sigma,
  projection_watermark,
  occurred_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id,
          event_type,
          rating_engine,
          engine_version,
          policy_version,
          sport_key,
          mode_key,
          user_id,
          source_result_id,
          mu,
          sigma,
          delta_mu,
          delta_sigma,
          projection_watermark,
          occurred_at,
          created_at;

-- name: UpsertCompetitionLegacyRatingEvent :one
INSERT INTO apollo.competition_rating_events (
  event_type,
  rating_engine,
  engine_version,
  policy_version,
  sport_key,
  mode_key,
  user_id,
  source_result_id,
  mu,
  sigma,
  delta_mu,
  delta_sigma,
  projection_watermark,
  occurred_at
)
VALUES (
  'competition.rating.legacy_computed',
  $1,
  $2,
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
  $13
)
ON CONFLICT (
  rating_engine,
  engine_version,
  policy_version,
  sport_key,
  mode_key,
  source_result_id,
  user_id
)
WHERE event_type = 'competition.rating.legacy_computed'
DO UPDATE SET
  mu = EXCLUDED.mu,
  sigma = EXCLUDED.sigma,
  delta_mu = EXCLUDED.delta_mu,
  delta_sigma = EXCLUDED.delta_sigma,
  projection_watermark = EXCLUDED.projection_watermark,
  occurred_at = EXCLUDED.occurred_at
RETURNING id,
          event_type,
          rating_engine,
          engine_version,
          policy_version,
          sport_key,
          mode_key,
          user_id,
          source_result_id,
          mu,
          sigma,
          delta_mu,
          delta_sigma,
          projection_watermark,
          occurred_at,
          created_at;

-- name: UpsertCompetitionMemberRating :one
INSERT INTO apollo.competition_member_ratings (
  user_id,
  sport_key,
  mode_key,
  mu,
  sigma,
  matches_played,
  last_played,
  updated_at,
  rating_engine,
  engine_version,
  policy_version,
  source_result_id,
  rating_event_id,
  projection_watermark
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (user_id, sport_key, mode_key)
DO UPDATE SET
  mu = EXCLUDED.mu,
  sigma = EXCLUDED.sigma,
  matches_played = EXCLUDED.matches_played,
  last_played = EXCLUDED.last_played,
  updated_at = EXCLUDED.updated_at,
  rating_engine = EXCLUDED.rating_engine,
  engine_version = EXCLUDED.engine_version,
  policy_version = EXCLUDED.policy_version,
  source_result_id = EXCLUDED.source_result_id,
  rating_event_id = EXCLUDED.rating_event_id,
  projection_watermark = EXCLUDED.projection_watermark
RETURNING user_id,
          sport_key,
          mode_key,
          mu,
          sigma,
          matches_played,
          last_played,
          updated_at,
          rating_engine,
          engine_version,
          policy_version,
          source_result_id,
          rating_event_id,
          projection_watermark;

-- name: ListCompetitionMemberRatingsByUserID :many
SELECT user_id,
       sport_key,
       mode_key,
       mu,
       sigma,
       matches_played,
       last_played,
       updated_at
FROM apollo.competition_member_ratings
WHERE user_id = $1
ORDER BY sport_key ASC, mode_key ASC;

-- name: ListCompetitionMemberStatRowsByUserID :many
SELECT s.sport_key,
       sp.competition_mode,
       sp.sides_per_match,
       s.participants_per_side,
       r.recorded_at,
       rs.outcome
FROM apollo.competition_sessions AS s
INNER JOIN apollo.sports AS sp
  ON sp.sport_key = s.sport_key
INNER JOIN apollo.competition_matches AS m
  ON m.competition_session_id = s.id
INNER JOIN apollo.competition_match_results AS r
  ON r.id = m.canonical_result_id
INNER JOIN apollo.competition_match_result_sides AS rs
  ON rs.competition_match_result_id = r.id
INNER JOIN apollo.competition_team_roster_members AS rm
  ON rm.competition_session_id = s.id
 AND rm.competition_session_team_id = rs.competition_session_team_id
WHERE rm.user_id = $1
  AND m.status = 'completed'
  AND r.result_status IN ('finalized', 'corrected')
ORDER BY r.recorded_at ASC, m.id ASC;

-- name: ListCompetitionMemberHistoryByUserID :many
SELECT m.id AS competition_match_id,
       r.id AS competition_match_result_id,
       s.display_name,
       s.sport_key,
       s.facility_key,
       sp.competition_mode,
       sp.sides_per_match,
       s.participants_per_side,
       r.recorded_at,
       rs.outcome
FROM apollo.competition_sessions AS s
INNER JOIN apollo.sports AS sp
  ON sp.sport_key = s.sport_key
INNER JOIN apollo.competition_matches AS m
  ON m.competition_session_id = s.id
INNER JOIN apollo.competition_match_results AS r
  ON r.id = m.canonical_result_id
INNER JOIN apollo.competition_match_result_sides AS rs
  ON rs.competition_match_result_id = r.id
INNER JOIN apollo.competition_team_roster_members AS rm
  ON rm.competition_session_id = s.id
 AND rm.competition_session_team_id = rs.competition_session_team_id
WHERE rm.user_id = $1
  AND m.status = 'completed'
  AND r.result_status IN ('finalized', 'corrected')
ORDER BY r.recorded_at DESC, m.id DESC;

-- name: UpdateCompetitionMatchResultStatus :one
UPDATE apollo.competition_match_results
SET result_status = $2,
    dispute_status = $3,
    finalized_at = CASE
        WHEN $2 = 'finalized' THEN $5
        ELSE finalized_at
    END,
    corrected_at = CASE
        WHEN $2 = 'corrected' THEN $5
        ELSE corrected_at
    END
WHERE id = $1
  AND result_status = $4
RETURNING id,
          competition_match_id,
          recorded_by_user_id,
          recorded_at,
          result_status,
          dispute_status,
          correction_id,
          supersedes_result_id,
          finalized_at,
          corrected_at;

-- name: CreateCompetitionLifecycleEvent :one
INSERT INTO apollo.competition_lifecycle_events (
  competition_session_id,
  competition_match_id,
  competition_match_result_id,
  event_type,
  actor_user_id,
  actor_role,
  actor_session_id,
  capability,
  trusted_surface_key,
  trusted_surface_label,
  previous_result_status,
  result_status,
  dispute_status,
  correction_id,
  supersedes_result_id,
  occurred_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING id,
          competition_session_id,
          competition_match_id,
          competition_match_result_id,
          event_type,
          actor_user_id,
          actor_role,
          actor_session_id,
          capability,
          trusted_surface_key,
          trusted_surface_label,
          previous_result_status,
          result_status,
          dispute_status,
          correction_id,
          supersedes_result_id,
          occurred_at;
