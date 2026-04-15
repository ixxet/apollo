-- name: CompleteCompetitionMatch :one
UPDATE apollo.competition_matches
SET status = 'completed',
    updated_at = $2
WHERE id = $1
  AND status = 'in_progress'
RETURNING id,
          competition_session_id,
          match_index,
          status,
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
  recorded_at
)
VALUES ($1, $2, $3)
RETURNING competition_match_id,
          recorded_by_user_id,
          recorded_at;

-- name: CreateCompetitionMatchResultSide :one
INSERT INTO apollo.competition_match_result_sides (
  competition_match_id,
  side_index,
  competition_session_team_id,
  outcome
)
VALUES ($1, $2, $3, $4)
RETURNING competition_match_id,
          side_index,
          competition_session_team_id,
          outcome;

-- name: GetCompetitionMatchResultByMatchID :one
SELECT competition_match_id,
       recorded_by_user_id,
       recorded_at
FROM apollo.competition_match_results
WHERE competition_match_id = $1
LIMIT 1;

-- name: ListCompetitionMatchResultRowsBySessionID :many
SELECT m.id AS competition_match_id,
       r.recorded_by_user_id,
       r.recorded_at,
       rs.side_index,
       rs.competition_session_team_id,
       rs.outcome
FROM apollo.competition_matches AS m
INNER JOIN apollo.competition_match_results AS r
  ON r.competition_match_id = m.id
INNER JOIN apollo.competition_match_result_sides AS rs
  ON rs.competition_match_id = m.id
WHERE m.competition_session_id = $1
ORDER BY m.match_index ASC, rs.side_index ASC;

-- name: CountCompetitionIncompleteActiveMatchesBySessionID :one
SELECT count(*)
FROM apollo.competition_matches
WHERE competition_session_id = $1
  AND status NOT IN ('completed', 'archived');

-- name: ListCompetitionRatingParticipantsBySport :many
SELECT m.id AS competition_match_id,
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
  ON r.competition_match_id = m.id
INNER JOIN apollo.competition_match_result_sides AS rs
  ON rs.competition_match_id = m.id
INNER JOIN apollo.competition_team_roster_members AS rm
  ON rm.competition_session_id = s.id
 AND rm.competition_session_team_id = rs.competition_session_team_id
WHERE s.sport_key = $1
  AND m.status = 'completed'
ORDER BY r.recorded_at ASC, m.id ASC, rs.side_index ASC, rm.user_id ASC;

-- name: DeleteCompetitionMemberRatingsBySportKey :execrows
DELETE FROM apollo.competition_member_ratings
WHERE sport_key = $1;

-- name: UpsertCompetitionMemberRating :one
INSERT INTO apollo.competition_member_ratings (
  user_id,
  sport_key,
  mode_key,
  mu,
  sigma,
  matches_played,
  last_played,
  updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (user_id, sport_key, mode_key)
DO UPDATE SET
  mu = EXCLUDED.mu,
  sigma = EXCLUDED.sigma,
  matches_played = EXCLUDED.matches_played,
  last_played = EXCLUDED.last_played,
  updated_at = EXCLUDED.updated_at
RETURNING user_id,
          sport_key,
          mode_key,
          mu,
          sigma,
          matches_played,
          last_played,
          updated_at;

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
  ON r.competition_match_id = m.id
INNER JOIN apollo.competition_match_result_sides AS rs
  ON rs.competition_match_id = m.id
INNER JOIN apollo.competition_team_roster_members AS rm
  ON rm.competition_session_id = s.id
 AND rm.competition_session_team_id = rs.competition_session_team_id
WHERE rm.user_id = $1
  AND m.status = 'completed'
ORDER BY r.recorded_at ASC, m.id ASC;

-- name: ListCompetitionMemberHistoryByUserID :many
SELECT m.id AS competition_match_id,
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
  ON r.competition_match_id = m.id
INNER JOIN apollo.competition_match_result_sides AS rs
  ON rs.competition_match_id = m.id
INNER JOIN apollo.competition_team_roster_members AS rm
  ON rm.competition_session_id = s.id
 AND rm.competition_session_team_id = rs.competition_session_team_id
WHERE rm.user_id = $1
  AND m.status = 'completed'
ORDER BY r.recorded_at DESC, m.id DESC;
