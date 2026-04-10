-- name: GetActiveClaimedTagByHash :one
SELECT ct.*
FROM apollo.claimed_tags AS ct
WHERE ct.tag_hash = $1
  AND ct.is_active = TRUE
LIMIT 1;

-- name: InsertVisitTapLink :execrows
INSERT INTO apollo.visit_tap_links (
  visit_id,
  claimed_tag_id
)
SELECT $1, ct.id
FROM apollo.claimed_tags AS ct
WHERE ct.id = $2
  AND ct.user_id = $3
  AND ct.is_active = TRUE
ON CONFLICT (visit_id) DO NOTHING;

-- name: GetVisitTapLinkByVisitID :one
SELECT visit_id, claimed_tag_id, linked_at
FROM apollo.visit_tap_links
WHERE visit_id = $1
LIMIT 1;

-- name: GetMemberPresenceStreakByUserIDAndFacilityKey :one
SELECT user_id, facility_key, current_count, current_start_day, last_credited_day, last_linked_visit_id, updated_at
FROM apollo.member_presence_streaks
WHERE user_id = $1
  AND facility_key = $2
LIMIT 1;

-- name: GetMemberPresenceStreakByUserIDAndFacilityKeyForUpdate :one
SELECT user_id, facility_key, current_count, current_start_day, last_credited_day, last_linked_visit_id, updated_at
FROM apollo.member_presence_streaks
WHERE user_id = $1
  AND facility_key = $2
LIMIT 1
FOR UPDATE;

-- name: InsertMemberPresenceStreakEvent :execrows
INSERT INTO apollo.member_presence_streak_events (
  user_id,
  facility_key,
  event_kind,
  count_before,
  count_after,
  streak_day,
  visit_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (user_id, facility_key, streak_day) DO NOTHING;

-- name: UpsertMemberPresenceStreak :one
INSERT INTO apollo.member_presence_streaks (
  user_id,
  facility_key,
  current_count,
  current_start_day,
  last_credited_day,
  last_linked_visit_id,
  updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (user_id, facility_key) DO UPDATE
SET current_count = EXCLUDED.current_count,
    current_start_day = EXCLUDED.current_start_day,
    last_credited_day = EXCLUDED.last_credited_day,
    last_linked_visit_id = EXCLUDED.last_linked_visit_id,
    updated_at = EXCLUDED.updated_at
RETURNING user_id, facility_key, current_count, current_start_day, last_credited_day, last_linked_visit_id, updated_at;

-- name: ListFacilityPresenceStreaksByUserID :many
SELECT user_id, facility_key, current_count, current_start_day, last_credited_day, last_linked_visit_id, updated_at
FROM apollo.member_presence_streaks
WHERE user_id = $1
ORDER BY facility_key ASC;

-- name: ListLatestMemberPresenceStreakEventsByUserID :many
SELECT DISTINCT ON (facility_key)
  id,
  user_id,
  facility_key,
  event_kind,
  count_before,
  count_after,
  streak_day,
  visit_id,
  created_at
FROM apollo.member_presence_streak_events
WHERE user_id = $1
ORDER BY facility_key ASC, streak_day DESC, created_at DESC, id DESC;

-- name: ListLinkedVisitsByUserID :many
SELECT v.id, v.user_id, v.facility_key, v.zone_key, v.source_event_id, v.arrived_at, v.departed_at, v.metadata, v.departure_source_event_id
FROM apollo.visits AS v
JOIN apollo.visit_tap_links AS vtl ON vtl.visit_id = v.id
WHERE v.user_id = $1
ORDER BY v.facility_key ASC, v.arrived_at DESC, v.id DESC;

-- name: ListVisitTapLinksByUserID :many
SELECT vtl.visit_id, vtl.claimed_tag_id, vtl.linked_at
FROM apollo.visit_tap_links AS vtl
JOIN apollo.visits AS v ON v.id = vtl.visit_id
WHERE v.user_id = $1
ORDER BY v.facility_key ASC, v.arrived_at DESC, v.id DESC;
