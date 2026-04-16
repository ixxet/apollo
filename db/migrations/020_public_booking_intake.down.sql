DROP INDEX IF EXISTS apollo.idx_booking_request_idempotency_request_id;

DROP TABLE IF EXISTS apollo.booking_request_idempotency_keys;

ALTER TABLE apollo.booking_requests
    DROP CONSTRAINT IF EXISTS booking_requests_public_created_staff_empty,
    DROP CONSTRAINT IF EXISTS booking_requests_staff_attribution_required,
    DROP CONSTRAINT IF EXISTS booking_requests_intake_channel_allowed,
    DROP CONSTRAINT IF EXISTS booking_requests_request_source_allowed,
    ALTER COLUMN created_by_user_id SET NOT NULL,
    ALTER COLUMN created_by_session_id SET NOT NULL,
    ALTER COLUMN created_by_role SET NOT NULL,
    ALTER COLUMN created_by_capability SET NOT NULL,
    ALTER COLUMN created_trusted_surface_key SET NOT NULL,
    ALTER COLUMN updated_by_user_id SET NOT NULL,
    ALTER COLUMN updated_by_session_id SET NOT NULL,
    ALTER COLUMN updated_by_role SET NOT NULL,
    ALTER COLUMN updated_by_capability SET NOT NULL,
    ALTER COLUMN updated_trusted_surface_key SET NOT NULL,
    DROP COLUMN intake_channel,
    DROP COLUMN request_source;

DROP INDEX IF EXISTS apollo.idx_schedule_resources_public_option_id;

ALTER TABLE apollo.schedule_resources
    DROP COLUMN public_option_id;
