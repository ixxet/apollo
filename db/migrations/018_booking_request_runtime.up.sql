CREATE TABLE apollo.booking_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    facility_key TEXT NOT NULL REFERENCES apollo.facility_catalog_refs(facility_key) ON DELETE RESTRICT,
    zone_key TEXT,
    resource_key TEXT,
    scope TEXT NOT NULL,
    requested_start_at TIMESTAMPTZ NOT NULL,
    requested_end_at TIMESTAMPTZ NOT NULL,
    contact_name TEXT NOT NULL,
    contact_email TEXT,
    contact_phone TEXT,
    organization TEXT,
    purpose TEXT,
    attendee_count INTEGER,
    internal_notes TEXT,
    status TEXT NOT NULL DEFAULT 'requested',
    version INTEGER NOT NULL DEFAULT 1,
    schedule_block_id UUID UNIQUE REFERENCES apollo.schedule_blocks(id) ON DELETE RESTRICT,
    created_by_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    created_by_session_id UUID NOT NULL REFERENCES apollo.sessions(id) ON DELETE RESTRICT,
    created_by_role TEXT NOT NULL,
    created_by_capability TEXT NOT NULL,
    created_trusted_surface_key TEXT NOT NULL,
    created_trusted_surface_label TEXT,
    updated_by_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    updated_by_session_id UUID NOT NULL REFERENCES apollo.sessions(id) ON DELETE RESTRICT,
    updated_by_role TEXT NOT NULL,
    updated_by_capability TEXT NOT NULL,
    updated_trusted_surface_key TEXT NOT NULL,
    updated_trusted_surface_label TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT booking_requests_scope_allowed
        CHECK (scope IN ('facility', 'zone', 'resource')),
    CONSTRAINT booking_requests_status_allowed
        CHECK (status IN ('requested', 'under_review', 'needs_changes', 'approved', 'rejected', 'cancelled')),
    CONSTRAINT booking_requests_version_positive
        CHECK (version > 0),
    CONSTRAINT booking_requests_window_valid
        CHECK (requested_start_at < requested_end_at),
    CONSTRAINT booking_requests_contact_name_required
        CHECK (btrim(contact_name) <> ''),
    CONSTRAINT booking_requests_contact_required
        CHECK (
            (contact_email IS NOT NULL AND btrim(contact_email) <> '')
            OR (contact_phone IS NOT NULL AND btrim(contact_phone) <> '')
        ),
    CONSTRAINT booking_requests_optional_text_required
        CHECK (
            (organization IS NULL OR btrim(organization) <> '')
            AND (purpose IS NULL OR btrim(purpose) <> '')
            AND (internal_notes IS NULL OR btrim(internal_notes) <> '')
        ),
    CONSTRAINT booking_requests_attendee_count_positive
        CHECK (attendee_count IS NULL OR attendee_count > 0),
    CONSTRAINT booking_requests_scope_shape
        CHECK (
            (scope = 'facility' AND zone_key IS NULL AND resource_key IS NULL)
            OR (scope = 'zone' AND zone_key IS NOT NULL AND resource_key IS NULL)
            OR (scope = 'resource' AND resource_key IS NOT NULL)
        ),
    CONSTRAINT booking_requests_approved_block_required
        CHECK (
            (status = 'approved' AND schedule_block_id IS NOT NULL)
            OR (status <> 'approved' AND schedule_block_id IS NULL)
        )
);

ALTER TABLE apollo.booking_requests
    ADD CONSTRAINT booking_requests_zone_ref
    FOREIGN KEY (facility_key, zone_key)
    REFERENCES apollo.facility_zone_refs(facility_key, zone_key)
    ON DELETE RESTRICT;

ALTER TABLE apollo.booking_requests
    ADD CONSTRAINT booking_requests_resource_ref
    FOREIGN KEY (resource_key)
    REFERENCES apollo.schedule_resources(resource_key)
    ON DELETE RESTRICT;

CREATE INDEX idx_booking_requests_facility_status_updated
    ON apollo.booking_requests (facility_key, status, updated_at DESC, id DESC);

CREATE INDEX idx_booking_requests_status_updated
    ON apollo.booking_requests (status, updated_at DESC, id DESC);
