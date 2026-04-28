-- name: ListCompetitionTournaments :many
SELECT id,
       owner_user_id,
       display_name,
       format,
       visibility,
       sport_key,
       facility_key,
       zone_key,
       participants_per_side,
       status,
       tournament_version,
       created_at,
       updated_at,
       archived_at
FROM apollo.competition_tournaments
ORDER BY created_at DESC, id DESC;

-- name: GetCompetitionTournamentByID :one
SELECT id,
       owner_user_id,
       display_name,
       format,
       visibility,
       sport_key,
       facility_key,
       zone_key,
       participants_per_side,
       status,
       tournament_version,
       created_at,
       updated_at,
       archived_at
FROM apollo.competition_tournaments
WHERE id = $1
LIMIT 1;

-- name: CreateCompetitionTournament :one
INSERT INTO apollo.competition_tournaments (
  owner_user_id,
  display_name,
  format,
  sport_key,
  facility_key,
  zone_key,
  participants_per_side,
  updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id,
          owner_user_id,
          display_name,
          format,
          visibility,
          sport_key,
          facility_key,
          zone_key,
          participants_per_side,
          status,
          tournament_version,
          created_at,
          updated_at,
          archived_at;

-- name: UpdateCompetitionTournamentStatusWithVersion :one
UPDATE apollo.competition_tournaments
SET status = $3,
    tournament_version = tournament_version + 1,
    updated_at = $4
WHERE id = $1
  AND tournament_version = $2
RETURNING id,
          owner_user_id,
          display_name,
          format,
          visibility,
          sport_key,
          facility_key,
          zone_key,
          participants_per_side,
          status,
          tournament_version,
          created_at,
          updated_at,
          archived_at;

-- name: CreateCompetitionTournamentBracket :one
INSERT INTO apollo.competition_tournament_brackets (
  tournament_id,
  bracket_index,
  format,
  status,
  updated_at
)
VALUES ($1, $2, $3, $4, $5)
RETURNING id,
          tournament_id,
          bracket_index,
          format,
          status,
          created_at,
          updated_at;

-- name: UpdateCompetitionTournamentBracketStatus :one
UPDATE apollo.competition_tournament_brackets
SET status = $3,
    updated_at = $4
WHERE id = $1
  AND tournament_id = $2
RETURNING id,
          tournament_id,
          bracket_index,
          format,
          status,
          created_at,
          updated_at;

-- name: ListCompetitionTournamentBracketsByTournamentID :many
SELECT id,
       tournament_id,
       bracket_index,
       format,
       status,
       created_at,
       updated_at
FROM apollo.competition_tournament_brackets
WHERE tournament_id = $1
ORDER BY bracket_index ASC, id ASC;

-- name: CreateCompetitionTournamentSeed :one
INSERT INTO apollo.competition_tournament_seeds (
  tournament_id,
  bracket_id,
  seed,
  competition_session_team_id,
  seeded_at
)
VALUES ($1, $2, $3, $4, $5)
RETURNING id,
          tournament_id,
          bracket_id,
          seed,
          competition_session_team_id,
          seeded_at,
          created_at;

-- name: ListCompetitionTournamentSeedsByBracketID :many
SELECT id,
       tournament_id,
       bracket_id,
       seed,
       competition_session_team_id,
       seeded_at,
       created_at
FROM apollo.competition_tournament_seeds
WHERE bracket_id = $1
ORDER BY seed ASC, id ASC;

-- name: GetCompetitionTournamentSeedByBracketSeed :one
SELECT id,
       tournament_id,
       bracket_id,
       seed,
       competition_session_team_id,
       seeded_at,
       created_at
FROM apollo.competition_tournament_seeds
WHERE bracket_id = $1
  AND seed = $2
LIMIT 1;

-- name: CountCompetitionTournamentSeedsByBracketID :one
SELECT count(*)
FROM apollo.competition_tournament_seeds
WHERE bracket_id = $1;

-- name: CreateCompetitionTournamentTeamSnapshot :one
INSERT INTO apollo.competition_tournament_team_snapshots (
  tournament_id,
  bracket_id,
  tournament_seed_id,
  seed,
  competition_session_id,
  competition_session_team_id,
  roster_hash,
  locked_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id,
          tournament_id,
          bracket_id,
          tournament_seed_id,
          seed,
          competition_session_id,
          competition_session_team_id,
          roster_hash,
          locked_at,
          created_at;

-- name: CreateCompetitionTournamentTeamSnapshotMember :one
INSERT INTO apollo.competition_tournament_team_snapshot_members (
  team_snapshot_id,
  user_id,
  display_name,
  slot_index
)
VALUES ($1, $2, $3, $4)
RETURNING team_snapshot_id,
          user_id,
          display_name,
          slot_index,
          created_at;

-- name: ListCompetitionTournamentTeamSnapshotsByBracketID :many
SELECT id,
       tournament_id,
       bracket_id,
       tournament_seed_id,
       seed,
       competition_session_id,
       competition_session_team_id,
       roster_hash,
       locked_at,
       created_at
FROM apollo.competition_tournament_team_snapshots
WHERE bracket_id = $1
ORDER BY seed ASC, id ASC;

-- name: GetCompetitionTournamentTeamSnapshotByID :one
SELECT id,
       tournament_id,
       bracket_id,
       tournament_seed_id,
       seed,
       competition_session_id,
       competition_session_team_id,
       roster_hash,
       locked_at,
       created_at
FROM apollo.competition_tournament_team_snapshots
WHERE id = $1
LIMIT 1;

-- name: ListCompetitionTournamentTeamSnapshotMembersByBracketID :many
SELECT m.team_snapshot_id,
       m.user_id,
       m.display_name,
       m.slot_index,
       m.created_at
FROM apollo.competition_tournament_team_snapshot_members AS m
INNER JOIN apollo.competition_tournament_team_snapshots AS s
  ON s.id = m.team_snapshot_id
WHERE s.bracket_id = $1
ORDER BY s.seed ASC, m.slot_index ASC, m.user_id ASC;

-- name: CountCompetitionTournamentTeamSnapshotsByBracketID :one
SELECT count(*)
FROM apollo.competition_tournament_team_snapshots
WHERE bracket_id = $1;

-- name: CreateCompetitionTournamentMatchBinding :one
INSERT INTO apollo.competition_tournament_match_bindings (
  tournament_id,
  bracket_id,
  round,
  match_number,
  competition_match_id,
  side_one_team_snapshot_id,
  side_two_team_snapshot_id,
  bound_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id,
          tournament_id,
          bracket_id,
          round,
          match_number,
          competition_match_id,
          side_one_team_snapshot_id,
          side_two_team_snapshot_id,
          bound_at,
          created_at;

-- name: ListCompetitionTournamentMatchBindingsByBracketID :many
SELECT id,
       tournament_id,
       bracket_id,
       round,
       match_number,
       competition_match_id,
       side_one_team_snapshot_id,
       side_two_team_snapshot_id,
       bound_at,
       created_at
FROM apollo.competition_tournament_match_bindings
WHERE bracket_id = $1
ORDER BY round ASC, match_number ASC, id ASC;

-- name: GetCompetitionTournamentMatchBindingByID :one
SELECT id,
       tournament_id,
       bracket_id,
       round,
       match_number,
       competition_match_id,
       side_one_team_snapshot_id,
       side_two_team_snapshot_id,
       bound_at,
       created_at
FROM apollo.competition_tournament_match_bindings
WHERE id = $1
LIMIT 1;

-- name: CreateCompetitionTournamentAdvancement :one
INSERT INTO apollo.competition_tournament_advancements (
  tournament_id,
  bracket_id,
  match_binding_id,
  round,
  winning_team_snapshot_id,
  losing_team_snapshot_id,
  competition_match_id,
  canonical_result_id,
  advance_reason,
  advanced_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id,
          tournament_id,
          bracket_id,
          match_binding_id,
          round,
          winning_team_snapshot_id,
          losing_team_snapshot_id,
          competition_match_id,
          canonical_result_id,
          advance_reason,
          advanced_at,
          created_at;

-- name: ListCompetitionTournamentAdvancementsByBracketID :many
SELECT id,
       tournament_id,
       bracket_id,
       match_binding_id,
       round,
       winning_team_snapshot_id,
       losing_team_snapshot_id,
       competition_match_id,
       canonical_result_id,
       advance_reason,
       advanced_at,
       created_at
FROM apollo.competition_tournament_advancements
WHERE bracket_id = $1
ORDER BY round ASC, advanced_at ASC, id ASC;

-- name: ListCompetitionMatchResultSidesByResultID :many
SELECT competition_match_result_id,
       competition_match_id,
       side_index,
       competition_session_team_id,
       outcome
FROM apollo.competition_match_result_sides
WHERE competition_match_result_id = $1
ORDER BY side_index ASC;

-- name: CreateCompetitionTournamentEvent :one
INSERT INTO apollo.competition_tournament_events (
  tournament_id,
  bracket_id,
  event_type,
  tournament_seed_id,
  team_snapshot_id,
  match_binding_id,
  round,
  seed,
  advance_reason,
  competition_session_team_id,
  competition_match_id,
  canonical_result_id,
  actor_user_id,
  actor_role,
  actor_session_id,
  capability,
  trusted_surface_key,
  trusted_surface_label,
  occurred_at
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
  $11, $12, $13, $14, $15, $16, $17, $18, $19
)
RETURNING id,
          tournament_id,
          bracket_id,
          event_type,
          tournament_seed_id,
          team_snapshot_id,
          match_binding_id,
          round,
          seed,
          advance_reason,
          competition_session_team_id,
          competition_match_id,
          canonical_result_id,
          actor_user_id,
          actor_role,
          actor_session_id,
          capability,
          trusted_surface_key,
          trusted_surface_label,
          occurred_at,
          created_at;
