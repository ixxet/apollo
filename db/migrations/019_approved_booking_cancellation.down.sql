ALTER TABLE apollo.booking_requests
    DROP CONSTRAINT booking_requests_schedule_block_lifecycle;

UPDATE apollo.booking_requests
SET schedule_block_id = NULL
WHERE status = 'cancelled';

ALTER TABLE apollo.booking_requests
    ADD CONSTRAINT booking_requests_approved_block_required
    CHECK (
        (status = 'approved' AND schedule_block_id IS NOT NULL)
        OR (status <> 'approved' AND schedule_block_id IS NULL)
    );
