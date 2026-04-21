ALTER TABLE apollo.booking_requests
    ADD COLUMN replaces_request_id UUID REFERENCES apollo.booking_requests(id) ON DELETE RESTRICT;

ALTER TABLE apollo.booking_requests
    ADD CONSTRAINT booking_requests_replaces_not_self
        CHECK (replaces_request_id IS NULL OR replaces_request_id <> id);

CREATE INDEX idx_booking_requests_replaces_request_id
    ON apollo.booking_requests (replaces_request_id, created_at DESC)
    WHERE replaces_request_id IS NOT NULL;
