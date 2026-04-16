ALTER TABLE apollo.schedule_resources
    ADD COLUMN public_option_id UUID NOT NULL DEFAULT gen_random_uuid();

CREATE UNIQUE INDEX idx_schedule_resources_public_option_id
    ON apollo.schedule_resources (public_option_id);

ALTER TABLE apollo.booking_requests
    ADD COLUMN request_source TEXT NOT NULL DEFAULT 'staff',
    ADD COLUMN intake_channel TEXT NOT NULL DEFAULT 'themis',
    ALTER COLUMN created_by_user_id DROP NOT NULL,
    ALTER COLUMN created_by_session_id DROP NOT NULL,
    ALTER COLUMN created_by_role DROP NOT NULL,
    ALTER COLUMN created_by_capability DROP NOT NULL,
    ALTER COLUMN created_trusted_surface_key DROP NOT NULL,
    ALTER COLUMN updated_by_user_id DROP NOT NULL,
    ALTER COLUMN updated_by_session_id DROP NOT NULL,
    ALTER COLUMN updated_by_role DROP NOT NULL,
    ALTER COLUMN updated_by_capability DROP NOT NULL,
    ALTER COLUMN updated_trusted_surface_key DROP NOT NULL;

ALTER TABLE apollo.booking_requests
    ADD CONSTRAINT booking_requests_request_source_allowed
        CHECK (request_source IN ('staff', 'public')),
    ADD CONSTRAINT booking_requests_intake_channel_allowed
        CHECK (intake_channel IN ('themis', 'public_web', 'public_api')),
    ADD CONSTRAINT booking_requests_staff_attribution_required
        CHECK (
            request_source <> 'staff'
            OR (
                created_by_user_id IS NOT NULL
                AND created_by_session_id IS NOT NULL
                AND created_by_role IS NOT NULL
                AND btrim(created_by_role) <> ''
                AND created_by_capability IS NOT NULL
                AND btrim(created_by_capability) <> ''
                AND created_trusted_surface_key IS NOT NULL
                AND btrim(created_trusted_surface_key) <> ''
                AND updated_by_user_id IS NOT NULL
                AND updated_by_session_id IS NOT NULL
                AND updated_by_role IS NOT NULL
                AND btrim(updated_by_role) <> ''
                AND updated_by_capability IS NOT NULL
                AND btrim(updated_by_capability) <> ''
                AND updated_trusted_surface_key IS NOT NULL
                AND btrim(updated_trusted_surface_key) <> ''
            )
        ),
    ADD CONSTRAINT booking_requests_public_created_staff_empty
        CHECK (
            request_source <> 'public'
            OR (
                created_by_user_id IS NULL
                AND created_by_session_id IS NULL
                AND created_by_role IS NULL
                AND created_by_capability IS NULL
                AND created_trusted_surface_key IS NULL
                AND created_trusted_surface_label IS NULL
            )
        );

CREATE TABLE apollo.booking_request_idempotency_keys (
    key_hash TEXT PRIMARY KEY,
    payload_hash TEXT NOT NULL,
    booking_request_id UUID NOT NULL REFERENCES apollo.booking_requests(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT booking_request_idempotency_key_hash_required
        CHECK (btrim(key_hash) <> ''),
    CONSTRAINT booking_request_idempotency_payload_hash_required
        CHECK (btrim(payload_hash) <> '')
);

CREATE INDEX idx_booking_request_idempotency_request_id
    ON apollo.booking_request_idempotency_keys (booking_request_id);
