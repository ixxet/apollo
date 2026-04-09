CREATE TABLE apollo.competition_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL,
    sport_key TEXT NOT NULL REFERENCES apollo.sports(sport_key) ON DELETE RESTRICT,
    facility_key TEXT NOT NULL REFERENCES apollo.facility_catalog_refs(facility_key) ON DELETE RESTRICT,
    zone_key TEXT,
    participants_per_side INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    CONSTRAINT competition_sessions_display_name_required
        CHECK (btrim(display_name) <> ''),
    CONSTRAINT competition_sessions_participants_per_side_positive
        CHECK (participants_per_side > 0),
    CONSTRAINT competition_sessions_status_allowed
        CHECK (status IN ('draft', 'archived')),
    FOREIGN KEY (facility_key, zone_key)
        REFERENCES apollo.facility_zone_refs(facility_key, zone_key)
        ON DELETE RESTRICT
);

CREATE UNIQUE INDEX idx_competition_sessions_owner_sport_name_active
    ON apollo.competition_sessions (owner_user_id, sport_key, lower(display_name))
    WHERE status <> 'archived';

CREATE INDEX idx_competition_sessions_owner_created_at
    ON apollo.competition_sessions (owner_user_id, created_at DESC, id DESC);

CREATE TABLE apollo.competition_session_teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_session_id UUID NOT NULL REFERENCES apollo.competition_sessions(id) ON DELETE CASCADE,
    side_index INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_session_teams_side_index_positive
        CHECK (side_index > 0),
    CONSTRAINT competition_session_teams_session_id_id_unique
        UNIQUE (competition_session_id, id),
    UNIQUE (competition_session_id, side_index)
);

CREATE INDEX idx_competition_session_teams_session_created_at
    ON apollo.competition_session_teams (competition_session_id, created_at ASC, id ASC);

CREATE TABLE apollo.competition_team_roster_members (
    competition_session_id UUID NOT NULL,
    competition_session_team_id UUID NOT NULL REFERENCES apollo.competition_session_teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    slot_index INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (competition_session_team_id, user_id),
    CONSTRAINT competition_team_roster_members_slot_index_positive
        CHECK (slot_index > 0),
    CONSTRAINT competition_team_roster_members_team_slot_unique
        UNIQUE (competition_session_team_id, slot_index),
    CONSTRAINT competition_team_roster_members_session_user_unique
        UNIQUE (competition_session_id, user_id),
    CONSTRAINT competition_team_roster_members_session_team_fkey
        FOREIGN KEY (competition_session_id, competition_session_team_id)
        REFERENCES apollo.competition_session_teams (competition_session_id, id)
        ON DELETE CASCADE
);

CREATE TABLE apollo.competition_matches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_session_id UUID NOT NULL REFERENCES apollo.competition_sessions(id) ON DELETE CASCADE,
    match_index INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    CONSTRAINT competition_matches_match_index_positive
        CHECK (match_index > 0),
    CONSTRAINT competition_matches_status_allowed
        CHECK (status IN ('draft', 'archived')),
    UNIQUE (competition_session_id, match_index)
);

CREATE INDEX idx_competition_matches_session_created_at
    ON apollo.competition_matches (competition_session_id, created_at ASC, id ASC);

CREATE TABLE apollo.competition_match_side_slots (
    competition_match_id UUID NOT NULL REFERENCES apollo.competition_matches(id) ON DELETE CASCADE,
    competition_session_team_id UUID NOT NULL REFERENCES apollo.competition_session_teams(id) ON DELETE RESTRICT,
    side_index INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (competition_match_id, competition_session_team_id),
    CONSTRAINT competition_match_side_slots_side_index_positive
        CHECK (side_index > 0),
    UNIQUE (competition_match_id, side_index)
);
