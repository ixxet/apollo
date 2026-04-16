ALTER TABLE apollo.booking_requests
    DROP CONSTRAINT booking_requests_approved_block_required;

ALTER TABLE apollo.booking_requests
    ADD CONSTRAINT booking_requests_schedule_block_lifecycle
    CHECK (
        (status = 'approved' AND schedule_block_id IS NOT NULL)
        OR status = 'cancelled'
        OR (status NOT IN ('approved', 'cancelled') AND schedule_block_id IS NULL)
    );
