-- name: GetLobbyMembershipByUserID :one
SELECT user_id, status, joined_at, left_at, created_at, updated_at
FROM apollo.lobby_memberships
WHERE user_id = $1
LIMIT 1;

-- name: UpsertLobbyMembershipJoin :one
INSERT INTO apollo.lobby_memberships (
  user_id,
  status,
  joined_at,
  left_at,
  updated_at
)
VALUES ($1, 'joined', $2, NULL, $2)
ON CONFLICT (user_id) DO UPDATE
SET status = 'joined',
    joined_at = EXCLUDED.joined_at,
    left_at = NULL,
    updated_at = EXCLUDED.updated_at
WHERE apollo.lobby_memberships.status <> 'joined'
RETURNING user_id, status, joined_at, left_at, created_at, updated_at;

-- name: LeaveLobbyMembership :one
UPDATE apollo.lobby_memberships
SET status = 'not_joined',
    left_at = $2,
    updated_at = $2
WHERE user_id = $1
  AND status = 'joined'
RETURNING user_id, status, joined_at, left_at, created_at, updated_at;
