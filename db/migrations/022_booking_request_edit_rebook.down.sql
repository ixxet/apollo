DROP INDEX IF EXISTS apollo.idx_booking_requests_replaces_request_id;

ALTER TABLE apollo.booking_requests
    DROP CONSTRAINT IF EXISTS booking_requests_replaces_not_self,
    DROP COLUMN IF EXISTS replaces_request_id;
