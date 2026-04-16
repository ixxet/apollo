-- name: ListScheduleResourcesByFacilityKey :many
SELECT resource_key,
       facility_key,
       zone_key,
       resource_type,
       display_name,
       public_label,
       bookable,
       active,
       created_at,
       updated_at
FROM apollo.schedule_resources
WHERE facility_key = $1
ORDER BY resource_key;

-- name: GetScheduleResourceByKey :one
SELECT resource_key,
       facility_key,
       zone_key,
       resource_type,
       display_name,
       public_label,
       bookable,
       active,
       created_at,
       updated_at
FROM apollo.schedule_resources
WHERE resource_key = $1
LIMIT 1;

-- name: UpsertScheduleResource :one
INSERT INTO apollo.schedule_resources (
    resource_key,
    facility_key,
    zone_key,
    resource_type,
    display_name,
    public_label,
    bookable,
    active,
    updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (resource_key)
DO UPDATE SET
    facility_key = EXCLUDED.facility_key,
    zone_key = EXCLUDED.zone_key,
    resource_type = EXCLUDED.resource_type,
    display_name = EXCLUDED.display_name,
    public_label = EXCLUDED.public_label,
    bookable = EXCLUDED.bookable,
    active = EXCLUDED.active,
    updated_at = EXCLUDED.updated_at
RETURNING resource_key,
          facility_key,
          zone_key,
          resource_type,
          display_name,
          public_label,
          bookable,
          active,
          created_at,
          updated_at;

-- name: ListScheduleResourceEdgesByFacilityKey :many
SELECT edge.resource_key,
       edge.related_resource_key,
       edge.edge_type,
       edge.created_at,
       edge.updated_at
FROM apollo.schedule_resource_edges AS edge
JOIN apollo.schedule_resources AS source_resource
  ON source_resource.resource_key = edge.resource_key
JOIN apollo.schedule_resources AS related_resource
  ON related_resource.resource_key = edge.related_resource_key
WHERE source_resource.facility_key = $1
  AND related_resource.facility_key = $1
ORDER BY edge.edge_type, edge.resource_key, edge.related_resource_key;

-- name: UpsertScheduleResourceEdge :one
INSERT INTO apollo.schedule_resource_edges (
    resource_key,
    related_resource_key,
    edge_type,
    updated_at
)
VALUES ($1, $2, $3, $4)
ON CONFLICT (resource_key, related_resource_key, edge_type)
DO UPDATE SET updated_at = EXCLUDED.updated_at
RETURNING resource_key,
          related_resource_key,
          edge_type,
          created_at,
          updated_at;

-- name: FacilityZoneRefExists :one
SELECT EXISTS (
    SELECT 1
    FROM apollo.facility_zone_refs
    WHERE facility_key = $1
      AND zone_key = $2
);

-- name: ListScheduleBlocksByFacilityKey :many
SELECT id,
       facility_key,
       zone_key,
       resource_key,
       scope,
       schedule_type,
       kind,
       effect,
       visibility,
       status,
       version,
       weekday,
       start_time,
       end_time,
       timezone,
       recurrence_start_date,
       recurrence_end_date,
       start_at,
       end_at,
       created_by_user_id,
       created_by_session_id,
       created_by_role,
       created_by_capability,
       created_trusted_surface_key,
       created_trusted_surface_label,
       updated_by_user_id,
       updated_by_session_id,
       updated_by_role,
       updated_by_capability,
       updated_trusted_surface_key,
       updated_trusted_surface_label,
       created_at,
       updated_at,
       cancelled_at,
       cancelled_by_user_id,
       cancelled_by_session_id,
       cancelled_by_role,
       cancelled_by_capability,
       cancelled_trusted_surface_key,
       cancelled_trusted_surface_label
FROM apollo.schedule_blocks
WHERE facility_key = $1
ORDER BY updated_at DESC, id DESC;

-- name: GetScheduleBlockByID :one
SELECT id,
       facility_key,
       zone_key,
       resource_key,
       scope,
       schedule_type,
       kind,
       effect,
       visibility,
       status,
       version,
       weekday,
       start_time,
       end_time,
       timezone,
       recurrence_start_date,
       recurrence_end_date,
       start_at,
       end_at,
       created_by_user_id,
       created_by_session_id,
       created_by_role,
       created_by_capability,
       created_trusted_surface_key,
       created_trusted_surface_label,
       updated_by_user_id,
       updated_by_session_id,
       updated_by_role,
       updated_by_capability,
       updated_trusted_surface_key,
       updated_trusted_surface_label,
       created_at,
       updated_at,
       cancelled_at,
       cancelled_by_user_id,
       cancelled_by_session_id,
       cancelled_by_role,
       cancelled_by_capability,
       cancelled_trusted_surface_key,
       cancelled_trusted_surface_label
FROM apollo.schedule_blocks
WHERE id = $1
LIMIT 1;

-- name: GetScheduleBlockByIDForUpdate :one
SELECT id,
       facility_key,
       zone_key,
       resource_key,
       scope,
       schedule_type,
       kind,
       effect,
       visibility,
       status,
       version,
       weekday,
       start_time,
       end_time,
       timezone,
       recurrence_start_date,
       recurrence_end_date,
       start_at,
       end_at,
       created_by_user_id,
       created_by_session_id,
       created_by_role,
       created_by_capability,
       created_trusted_surface_key,
       created_trusted_surface_label,
       updated_by_user_id,
       updated_by_session_id,
       updated_by_role,
       updated_by_capability,
       updated_trusted_surface_key,
       updated_trusted_surface_label,
       created_at,
       updated_at,
       cancelled_at,
       cancelled_by_user_id,
       cancelled_by_session_id,
       cancelled_by_role,
       cancelled_by_capability,
       cancelled_trusted_surface_key,
       cancelled_trusted_surface_label
FROM apollo.schedule_blocks
WHERE id = $1
LIMIT 1
FOR UPDATE;

-- name: CreateScheduleBlock :one
INSERT INTO apollo.schedule_blocks (
    facility_key,
    zone_key,
    resource_key,
    scope,
    schedule_type,
    kind,
    effect,
    visibility,
    status,
    version,
    weekday,
    start_time,
    end_time,
    timezone,
    recurrence_start_date,
    recurrence_end_date,
    start_at,
    end_at,
    created_by_user_id,
    created_by_session_id,
    created_by_role,
    created_by_capability,
    created_trusted_surface_key,
    created_trusted_surface_label,
    updated_by_user_id,
    updated_by_session_id,
    updated_by_role,
    updated_by_capability,
    updated_trusted_surface_key,
    updated_trusted_surface_label,
    created_at,
    updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'scheduled', 1, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30)
RETURNING id,
          facility_key,
          zone_key,
          resource_key,
          scope,
          schedule_type,
          kind,
          effect,
          visibility,
          status,
          version,
          weekday,
          start_time,
          end_time,
          timezone,
          recurrence_start_date,
          recurrence_end_date,
          start_at,
          end_at,
          created_by_user_id,
          created_by_session_id,
          created_by_role,
          created_by_capability,
          created_trusted_surface_key,
          created_trusted_surface_label,
          updated_by_user_id,
          updated_by_session_id,
          updated_by_role,
          updated_by_capability,
          updated_trusted_surface_key,
          updated_trusted_surface_label,
          created_at,
          updated_at,
          cancelled_at,
          cancelled_by_user_id,
          cancelled_by_session_id,
          cancelled_by_role,
          cancelled_by_capability,
          cancelled_trusted_surface_key,
          cancelled_trusted_surface_label;

-- name: BumpScheduleBlockVersion :one
UPDATE apollo.schedule_blocks
SET version = version + 1,
    updated_by_user_id = $3,
    updated_by_session_id = $4,
    updated_by_role = $5,
    updated_by_capability = $6,
    updated_trusted_surface_key = $7,
    updated_trusted_surface_label = $8,
    updated_at = $9
WHERE id = $1
  AND version = $2
  AND status = 'scheduled'
RETURNING id,
          facility_key,
          zone_key,
          resource_key,
          scope,
          schedule_type,
          kind,
          effect,
          visibility,
          status,
          version,
          weekday,
          start_time,
          end_time,
          timezone,
          recurrence_start_date,
          recurrence_end_date,
          start_at,
          end_at,
          created_by_user_id,
          created_by_session_id,
          created_by_role,
          created_by_capability,
          created_trusted_surface_key,
          created_trusted_surface_label,
          updated_by_user_id,
          updated_by_session_id,
          updated_by_role,
          updated_by_capability,
          updated_trusted_surface_key,
          updated_trusted_surface_label,
          created_at,
          updated_at,
          cancelled_at,
          cancelled_by_user_id,
          cancelled_by_session_id,
          cancelled_by_role,
          cancelled_by_capability,
          cancelled_trusted_surface_key,
          cancelled_trusted_surface_label;

-- name: CancelScheduleBlock :one
UPDATE apollo.schedule_blocks
SET status = 'cancelled',
    version = version + 1,
    updated_by_user_id = $3,
    updated_by_session_id = $4,
    updated_by_role = $5,
    updated_by_capability = $6,
    updated_trusted_surface_key = $7,
    updated_trusted_surface_label = $8,
    cancelled_at = $9,
    cancelled_by_user_id = $3,
    cancelled_by_session_id = $4,
    cancelled_by_role = $5,
    cancelled_by_capability = $6,
    cancelled_trusted_surface_key = $7,
    cancelled_trusted_surface_label = $8,
    updated_at = $9
WHERE id = $1
  AND version = $2
  AND status = 'scheduled'
RETURNING id,
          facility_key,
          zone_key,
          resource_key,
          scope,
          schedule_type,
          kind,
          effect,
          visibility,
          status,
          version,
          weekday,
          start_time,
          end_time,
          timezone,
          recurrence_start_date,
          recurrence_end_date,
          start_at,
          end_at,
          created_by_user_id,
          created_by_session_id,
          created_by_role,
          created_by_capability,
          created_trusted_surface_key,
          created_trusted_surface_label,
          updated_by_user_id,
          updated_by_session_id,
          updated_by_role,
          updated_by_capability,
          updated_trusted_surface_key,
          updated_trusted_surface_label,
          created_at,
          updated_at,
          cancelled_at,
          cancelled_by_user_id,
          cancelled_by_session_id,
          cancelled_by_role,
          cancelled_by_capability,
          cancelled_trusted_surface_key,
          cancelled_trusted_surface_label;

-- name: InsertScheduleBlockException :one
INSERT INTO apollo.schedule_block_exceptions (
    schedule_block_id,
    exception_date,
    created_by_user_id,
    created_by_session_id,
    created_by_role,
    created_by_capability,
    created_trusted_surface_key,
    created_trusted_surface_label,
    created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id,
          schedule_block_id,
          exception_date,
          created_by_user_id,
          created_by_session_id,
          created_by_role,
          created_by_capability,
          created_trusted_surface_key,
          created_trusted_surface_label,
          created_at;

-- name: ListScheduleBlockExceptionsByBlockIDs :many
SELECT id,
       schedule_block_id,
       exception_date,
       created_by_user_id,
       created_by_session_id,
       created_by_role,
       created_by_capability,
       created_trusted_surface_key,
       created_trusted_surface_label,
       created_at
FROM apollo.schedule_block_exceptions
WHERE schedule_block_id = ANY($1::uuid[])
ORDER BY schedule_block_id, exception_date;
