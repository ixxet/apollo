CREATE TABLE apollo.visit_tap_links (
    visit_id UUID PRIMARY KEY REFERENCES apollo.visits(id) ON DELETE CASCADE,
    claimed_tag_id UUID NOT NULL REFERENCES apollo.claimed_tags(id) ON DELETE RESTRICT,
    linked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_visit_tap_links_claimed_tag_id
    ON apollo.visit_tap_links (claimed_tag_id);

CREATE TABLE apollo.member_presence_streaks (
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    facility_key TEXT NOT NULL,
    current_count INTEGER NOT NULL CHECK (current_count > 0),
    current_start_day DATE NOT NULL,
    last_credited_day DATE NOT NULL,
    last_linked_visit_id UUID NOT NULL REFERENCES apollo.visits(id) ON DELETE RESTRICT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, facility_key)
);

CREATE TABLE apollo.member_presence_streak_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    facility_key TEXT NOT NULL,
    event_kind TEXT NOT NULL CHECK (event_kind IN ('started', 'continued', 'reset')),
    count_before INTEGER NOT NULL CHECK (count_before >= 0),
    count_after INTEGER NOT NULL CHECK (count_after > 0),
    streak_day DATE NOT NULL,
    visit_id UUID NOT NULL REFERENCES apollo.visits(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, facility_key, streak_day),
    UNIQUE (visit_id)
);

CREATE INDEX idx_member_presence_streak_events_user_day
    ON apollo.member_presence_streak_events (user_id, facility_key, streak_day DESC);
