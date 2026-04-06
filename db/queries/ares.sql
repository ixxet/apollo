-- name: ListJoinedLobbyMatchPreviewCandidates :many
SELECT
  apollo.users.id AS user_id,
  apollo.users.preferences,
  apollo.users.updated_at AS user_updated_at,
  apollo.lobby_memberships.joined_at,
  apollo.lobby_memberships.updated_at AS membership_updated_at
FROM apollo.lobby_memberships
JOIN apollo.users
  ON apollo.users.id = apollo.lobby_memberships.user_id
WHERE apollo.lobby_memberships.status = 'joined'
ORDER BY user_id ASC;
