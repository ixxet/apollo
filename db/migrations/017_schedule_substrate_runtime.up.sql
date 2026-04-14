CREATE TABLE apollo.schedule_resources (
    resource_key TEXT PRIMARY KEY,
    facility_key TEXT NOT NULL REFERENCES apollo.facility_catalog_refs(facility_key) ON DELETE RESTRICT,
    zone_key TEXT,
    resource_type TEXT NOT NULL,
    display_name TEXT NOT NULL,
    public_label TEXT,
    bookable BOOLEAN NOT NULL DEFAULT TRUE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT schedule_resources_resource_key_format
        CHECK (resource_key ~ '^[a-z0-9][a-z0-9-]*$'),
    CONSTRAINT schedule_resources_resource_type_required
        CHECK (btrim(resource_type) <> ''),
    CONSTRAINT schedule_resources_display_name_required
        CHECK (btrim(display_name) <> ''),
    CONSTRAINT schedule_resources_public_label_required
        CHECK (public_label IS NULL OR btrim(public_label) <> '')
);

ALTER TABLE apollo.schedule_resources
    ADD CONSTRAINT schedule_resources_zone_ref
    FOREIGN KEY (facility_key, zone_key)
    REFERENCES apollo.facility_zone_refs(facility_key, zone_key)
    ON DELETE RESTRICT;

CREATE INDEX idx_schedule_resources_facility_key
    ON apollo.schedule_resources (facility_key, zone_key, resource_key);

CREATE TABLE apollo.schedule_resource_edges (
    resource_key TEXT NOT NULL REFERENCES apollo.schedule_resources(resource_key) ON DELETE CASCADE,
    related_resource_key TEXT NOT NULL REFERENCES apollo.schedule_resources(resource_key) ON DELETE CASCADE,
    edge_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (resource_key, related_resource_key, edge_type),
    CONSTRAINT schedule_resource_edges_type_allowed
        CHECK (edge_type IN ('contains', 'composes', 'exclusive_with')),
    CONSTRAINT schedule_resource_edges_no_self_reference
        CHECK (resource_key <> related_resource_key),
    CONSTRAINT schedule_resource_edges_exclusive_order
        CHECK (edge_type <> 'exclusive_with' OR resource_key < related_resource_key)
);

CREATE INDEX idx_schedule_resource_edges_related_resource_key
    ON apollo.schedule_resource_edges (related_resource_key, edge_type);

CREATE TABLE apollo.schedule_blocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    facility_key TEXT NOT NULL REFERENCES apollo.facility_catalog_refs(facility_key) ON DELETE RESTRICT,
    zone_key TEXT,
    resource_key TEXT,
    scope TEXT NOT NULL,
    schedule_type TEXT NOT NULL,
    kind TEXT NOT NULL,
    effect TEXT NOT NULL,
    visibility TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'scheduled',
    version INTEGER NOT NULL DEFAULT 1,
    weekday SMALLINT,
    start_time TIME,
    end_time TIME,
    timezone TEXT,
    recurrence_start_date DATE,
    recurrence_end_date DATE,
    start_at TIMESTAMPTZ,
    end_at TIMESTAMPTZ,
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
    cancelled_at TIMESTAMPTZ,
    cancelled_by_user_id UUID REFERENCES apollo.users(id) ON DELETE RESTRICT,
    cancelled_by_session_id UUID REFERENCES apollo.sessions(id) ON DELETE RESTRICT,
    cancelled_by_role TEXT,
    cancelled_by_capability TEXT,
    cancelled_trusted_surface_key TEXT,
    cancelled_trusted_surface_label TEXT,
    CONSTRAINT schedule_blocks_scope_allowed
        CHECK (scope IN ('facility', 'zone', 'resource')),
    CONSTRAINT schedule_blocks_schedule_type_allowed
        CHECK (schedule_type IN ('one_off', 'weekly')),
    CONSTRAINT schedule_blocks_kind_allowed
        CHECK (kind IN ('operating_hours', 'closure', 'event', 'hold', 'reservation')),
    CONSTRAINT schedule_blocks_effect_allowed
        CHECK (effect IN ('informational', 'soft_hold', 'hard_reserve', 'closed')),
    CONSTRAINT schedule_blocks_visibility_allowed
        CHECK (visibility IN ('internal', 'public_busy', 'public_labeled')),
    CONSTRAINT schedule_blocks_status_allowed
        CHECK (status IN ('scheduled', 'cancelled')),
    CONSTRAINT schedule_blocks_weekday_range
        CHECK (weekday IS NULL OR weekday BETWEEN 1 AND 7),
    CONSTRAINT schedule_blocks_scope_shape
        CHECK (
            (scope = 'facility' AND zone_key IS NULL AND resource_key IS NULL)
            OR (scope = 'zone' AND zone_key IS NOT NULL AND resource_key IS NULL)
            OR (scope = 'resource' AND resource_key IS NOT NULL)
        ),
    CONSTRAINT schedule_blocks_one_off_shape
        CHECK (
            (schedule_type = 'one_off'
                AND start_at IS NOT NULL
                AND end_at IS NOT NULL
                AND weekday IS NULL
                AND start_time IS NULL
                AND end_time IS NULL
                AND timezone IS NULL
                AND recurrence_start_date IS NULL
                AND recurrence_end_date IS NULL)
            OR
            (schedule_type = 'weekly'
                AND start_at IS NULL
                AND end_at IS NULL
                AND weekday IS NOT NULL
                AND start_time IS NOT NULL
                AND end_time IS NOT NULL
                AND timezone IS NOT NULL
                AND recurrence_start_date IS NOT NULL
                AND recurrence_end_date IS NOT NULL)
        )
);

ALTER TABLE apollo.schedule_blocks
    ADD CONSTRAINT schedule_blocks_zone_ref
    FOREIGN KEY (facility_key, zone_key)
    REFERENCES apollo.facility_zone_refs(facility_key, zone_key)
    ON DELETE RESTRICT;

ALTER TABLE apollo.schedule_blocks
    ADD CONSTRAINT schedule_blocks_resource_ref
    FOREIGN KEY (resource_key)
    REFERENCES apollo.schedule_resources(resource_key)
    ON DELETE RESTRICT;

CREATE INDEX idx_schedule_blocks_facility_scope_status
    ON apollo.schedule_blocks (facility_key, scope, status, schedule_type, kind, updated_at DESC, id DESC);

CREATE INDEX idx_schedule_blocks_resource_key
    ON apollo.schedule_blocks (resource_key, status, schedule_type, updated_at DESC, id DESC);

CREATE TABLE apollo.schedule_block_exceptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_block_id UUID NOT NULL REFERENCES apollo.schedule_blocks(id) ON DELETE CASCADE,
    exception_date DATE NOT NULL,
    created_by_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    created_by_session_id UUID NOT NULL REFERENCES apollo.sessions(id) ON DELETE RESTRICT,
    created_by_role TEXT NOT NULL,
    created_by_capability TEXT NOT NULL,
    created_trusted_surface_key TEXT NOT NULL,
    created_trusted_surface_label TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT schedule_block_exceptions_unique
        UNIQUE (schedule_block_id, exception_date)
);

CREATE INDEX idx_schedule_block_exceptions_block_id_date
    ON apollo.schedule_block_exceptions (schedule_block_id, exception_date);
