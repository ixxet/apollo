-- name: GetActiveUserByTagHash :one
SELECT u.*
FROM apollo.claimed_tags AS ct
JOIN apollo.users AS u ON u.id = ct.user_id
WHERE ct.tag_hash = $1
  AND ct.is_active = TRUE
LIMIT 1;

-- name: GetVisitBySourceEventID :one
SELECT v.*
FROM apollo.visits AS v
WHERE source_event_id = $1
LIMIT 1;

-- name: GetOpenVisitByUserAndFacility :one
SELECT v.*
FROM apollo.visits AS v
WHERE user_id = $1
  AND facility_key = $2
  AND departed_at IS NULL
ORDER BY arrived_at DESC
LIMIT 1;

-- name: CreateVisit :one
INSERT INTO apollo.visits (
  user_id,
  facility_key,
  zone_key,
  source_event_id,
  arrived_at
)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, facility_key, zone_key, source_event_id, arrived_at, departed_at, metadata;

-- name: CloseVisit :one
UPDATE apollo.visits
SET departed_at = $2,
    departure_source_event_id = $3
WHERE id = $1
  AND departed_at IS NULL
RETURNING id, user_id, facility_key, zone_key, source_event_id, departure_source_event_id, arrived_at, departed_at, metadata;

-- name: ListVisitsByStudentID :many
SELECT v.*
FROM apollo.visits AS v
JOIN apollo.users AS u ON u.id = v.user_id
WHERE u.student_id = $1
ORDER BY v.arrived_at DESC;

-- name: GetVisitByDepartureSourceEventID :one
SELECT v.*
FROM apollo.visits AS v
WHERE departure_source_event_id = $1
LIMIT 1;
