-- name: ListCompetitionSessionsByOwner :many
SELECT id,
       owner_user_id,
       display_name,
       sport_key,
       facility_key,
       zone_key,
       participants_per_side,
       status,
       created_at,
       updated_at,
       archived_at
FROM apollo.competition_sessions
WHERE owner_user_id = $1
ORDER BY created_at DESC, id DESC;

-- name: GetCompetitionSessionByIDForOwner :one
SELECT id,
       owner_user_id,
       display_name,
       sport_key,
       facility_key,
       zone_key,
       participants_per_side,
       status,
       created_at,
       updated_at,
       archived_at
FROM apollo.competition_sessions
WHERE id = $1
  AND owner_user_id = $2
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
          status,
          created_at,
          updated_at,
          archived_at;

-- name: ArchiveCompetitionSession :one
UPDATE apollo.competition_sessions
SET status = 'archived',
    archived_at = $3,
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
          status,
          created_at,
          updated_at,
          archived_at;

-- name: CountDraftCompetitionMatchesBySessionID :one
SELECT count(*)
FROM apollo.competition_matches
WHERE competition_session_id = $1
  AND status = 'draft';

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
VALUES ($1, $2, 'draft', NOW())
RETURNING id,
          competition_session_id,
          match_index,
          status,
          created_at,
          updated_at,
          archived_at;

-- name: ArchiveCompetitionMatch :one
UPDATE apollo.competition_matches
SET status = 'archived',
    archived_at = $2,
    updated_at = $2
WHERE id = $1
  AND status = 'draft'
RETURNING id,
          competition_session_id,
          match_index,
          status,
          created_at,
          updated_at,
          archived_at;

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
