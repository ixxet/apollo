-- name: CreateBookingRequest :one
INSERT INTO apollo.booking_requests (
    facility_key,
    zone_key,
    resource_key,
    scope,
    requested_start_at,
    requested_end_at,
    contact_name,
    contact_email,
    contact_phone,
    organization,
    purpose,
    attendee_count,
    internal_notes,
    replaces_request_id,
    request_source,
    intake_channel,
    status,
    version,
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
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 'staff', 'themis', 'requested', 1, $15, $16, $17, $18, $19, $20, $15, $16, $17, $18, $19, $20, $21, $21)
RETURNING id,
          facility_key,
          zone_key,
          resource_key,
          scope,
          requested_start_at,
          requested_end_at,
          contact_name,
          contact_email,
          contact_phone,
          organization,
          purpose,
          attendee_count,
          internal_notes,
          status,
          version,
          schedule_block_id,
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
          request_source,
          intake_channel,
          replaces_request_id;

-- name: CreatePublicBookingRequest :one
INSERT INTO apollo.booking_requests (
    id,
    facility_key,
    zone_key,
    resource_key,
    scope,
    requested_start_at,
    requested_end_at,
    contact_name,
    contact_email,
    contact_phone,
    organization,
    purpose,
    attendee_count,
    internal_notes,
    replaces_request_id,
    request_source,
    intake_channel,
    status,
    version,
    created_at,
    updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NULL, NULL, 'public', $14, 'requested', 1, $15, $15)
RETURNING id,
          facility_key,
          zone_key,
          resource_key,
          scope,
          requested_start_at,
          requested_end_at,
          contact_name,
          contact_email,
          contact_phone,
          organization,
          purpose,
          attendee_count,
          internal_notes,
          status,
          version,
          schedule_block_id,
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
          request_source,
          intake_channel,
          replaces_request_id;

-- name: ListBookingRequests :many
SELECT id,
       facility_key,
       zone_key,
       resource_key,
       scope,
       requested_start_at,
       requested_end_at,
       contact_name,
       contact_email,
       contact_phone,
       organization,
       purpose,
       attendee_count,
       internal_notes,
       status,
       version,
       schedule_block_id,
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
       request_source,
       intake_channel,
       replaces_request_id
FROM apollo.booking_requests
ORDER BY updated_at DESC, id DESC;

-- name: ListBookingRequestsByFacilityKey :many
SELECT id,
       facility_key,
       zone_key,
       resource_key,
       scope,
       requested_start_at,
       requested_end_at,
       contact_name,
       contact_email,
       contact_phone,
       organization,
       purpose,
       attendee_count,
       internal_notes,
       status,
       version,
       schedule_block_id,
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
       request_source,
       intake_channel,
       replaces_request_id
FROM apollo.booking_requests
WHERE facility_key = $1
ORDER BY updated_at DESC, id DESC;

-- name: GetBookingRequestByID :one
SELECT id,
       facility_key,
       zone_key,
       resource_key,
       scope,
       requested_start_at,
       requested_end_at,
       contact_name,
       contact_email,
       contact_phone,
       organization,
       purpose,
       attendee_count,
       internal_notes,
       status,
       version,
       schedule_block_id,
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
       request_source,
       intake_channel,
       replaces_request_id
FROM apollo.booking_requests
WHERE id = $1
LIMIT 1;

-- name: GetBookingRequestByIDForUpdate :one
SELECT id,
       facility_key,
       zone_key,
       resource_key,
       scope,
       requested_start_at,
       requested_end_at,
       contact_name,
       contact_email,
       contact_phone,
       organization,
       purpose,
       attendee_count,
       internal_notes,
       status,
       version,
       schedule_block_id,
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
       request_source,
       intake_channel,
       replaces_request_id
FROM apollo.booking_requests
WHERE id = $1
LIMIT 1
FOR UPDATE;

-- name: UpdateBookingRequestStatus :one
UPDATE apollo.booking_requests
SET status = $2,
    version = version + 1,
    schedule_block_id = $3,
    internal_notes = COALESCE($4, internal_notes),
    updated_by_user_id = $5,
    updated_by_session_id = $6,
    updated_by_role = $7,
    updated_by_capability = $8,
    updated_trusted_surface_key = $9,
    updated_trusted_surface_label = $10,
    updated_at = $11
WHERE id = $1
  AND version = $12
RETURNING id,
          facility_key,
          zone_key,
          resource_key,
          scope,
          requested_start_at,
          requested_end_at,
          contact_name,
          contact_email,
          contact_phone,
          organization,
          purpose,
          attendee_count,
          internal_notes,
          status,
          version,
          schedule_block_id,
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
          request_source,
          intake_channel,
          replaces_request_id;

-- name: UpdateBookingRequestDetails :one
UPDATE apollo.booking_requests
SET facility_key = $2,
    zone_key = $3,
    resource_key = $4,
    scope = $5,
    requested_start_at = $6,
    requested_end_at = $7,
    contact_name = $8,
    contact_email = $9,
    contact_phone = $10,
    organization = $11,
    purpose = $12,
    attendee_count = $13,
    internal_notes = $14,
    version = version + 1,
    updated_by_user_id = $15,
    updated_by_session_id = $16,
    updated_by_role = $17,
    updated_by_capability = $18,
    updated_trusted_surface_key = $19,
    updated_trusted_surface_label = $20,
    updated_at = $21
WHERE id = $1
  AND version = $22
RETURNING id,
          facility_key,
          zone_key,
          resource_key,
          scope,
          requested_start_at,
          requested_end_at,
          contact_name,
          contact_email,
          contact_phone,
          organization,
          purpose,
          attendee_count,
          internal_notes,
          status,
          version,
          schedule_block_id,
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
          request_source,
          intake_channel,
          replaces_request_id;

-- name: GetBookingRequestIdempotencyByKeyHashForUpdate :one
SELECT key_hash,
       payload_hash,
       booking_request_id,
       created_at
FROM apollo.booking_request_idempotency_keys
WHERE key_hash = $1
LIMIT 1
FOR UPDATE;

-- name: LockBookingRequestIdempotencyKey :exec
SELECT pg_advisory_xact_lock(hashtextextended($1, 0));

-- name: CreateBookingRequestIdempotencyKey :one
INSERT INTO apollo.booking_request_idempotency_keys (
    key_hash,
    payload_hash,
    booking_request_id,
    created_at
)
VALUES ($1, $2, $3, $4)
RETURNING key_hash,
          payload_hash,
          booking_request_id,
          created_at;

-- name: CreatePublicBookingReceipt :one
INSERT INTO apollo.public_booking_receipts (
    receipt_code,
    booking_request_id,
    created_at,
    updated_at
)
VALUES ($1, $2, $3, $3)
RETURNING receipt_code,
          booking_request_id,
          public_message,
          created_at,
          updated_at;

-- name: GetPublicBookingReceiptByBookingRequestID :one
SELECT receipt_code,
       booking_request_id,
       public_message,
       created_at,
       updated_at
FROM apollo.public_booking_receipts
WHERE booking_request_id = $1
LIMIT 1;

-- name: GetPublicBookingStatusByReceiptCode :one
SELECT receipt.receipt_code,
       request.status,
       receipt.public_message,
       request.requested_start_at,
       request.requested_end_at,
       GREATEST(request.updated_at, receipt.updated_at)::timestamptz AS updated_at
FROM apollo.public_booking_receipts AS receipt
JOIN apollo.booking_requests AS request
  ON request.id = receipt.booking_request_id
WHERE receipt.receipt_code = $1
LIMIT 1;

-- name: GetPublicBookingStatusByRequestID :one
SELECT receipt.receipt_code,
       request.status,
       receipt.public_message,
       request.requested_start_at,
       request.requested_end_at,
       GREATEST(request.updated_at, receipt.updated_at)::timestamptz AS updated_at
FROM apollo.public_booking_receipts AS receipt
JOIN apollo.booking_requests AS request
  ON request.id = receipt.booking_request_id
WHERE request.id = $1
LIMIT 1;

-- name: UpdatePublicBookingReceiptMessage :one
UPDATE apollo.public_booking_receipts
SET public_message = $2,
    updated_at = $3
WHERE booking_request_id = $1
RETURNING receipt_code,
          booking_request_id,
          public_message,
          created_at,
          updated_at;

-- name: TouchBookingRequestPublicMessage :one
UPDATE apollo.booking_requests
SET version = version + 1,
    updated_by_user_id = $2,
    updated_by_session_id = $3,
    updated_by_role = $4,
    updated_by_capability = $5,
    updated_trusted_surface_key = $6,
    updated_trusted_surface_label = $7,
    updated_at = $8
WHERE id = $1
  AND version = $9
RETURNING id,
          facility_key,
          zone_key,
          resource_key,
          scope,
          requested_start_at,
          requested_end_at,
          contact_name,
          contact_email,
          contact_phone,
          organization,
          purpose,
          attendee_count,
          internal_notes,
          status,
          version,
          schedule_block_id,
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
          request_source,
          intake_channel,
          replaces_request_id;
