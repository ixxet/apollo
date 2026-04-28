CREATE TABLE apollo.competition_queue_intents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_session_id UUID NOT NULL REFERENCES apollo.competition_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    facility_key TEXT NOT NULL,
    sport_key TEXT NOT NULL REFERENCES apollo.sports(sport_key) ON DELETE RESTRICT,
    mode_key TEXT NOT NULL,
    tier TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_queue_intents_facility_key_required
        CHECK (btrim(facility_key) <> ''),
    CONSTRAINT competition_queue_intents_mode_key_required
        CHECK (btrim(mode_key) <> ''),
    CONSTRAINT competition_queue_intents_tier_required
        CHECK (btrim(tier) <> ''),
    CONSTRAINT competition_queue_intents_status_allowed
        CHECK (status IN ('active', 'withdrawn')),
    CONSTRAINT competition_queue_intents_session_user_unique
        UNIQUE (competition_session_id, user_id)
);

CREATE INDEX idx_competition_queue_intents_session_status
    ON apollo.competition_queue_intents (competition_session_id, status, updated_at DESC);

INSERT INTO apollo.competition_queue_intents (
    competition_session_id,
    user_id,
    facility_key,
    sport_key,
    mode_key,
    tier,
    status,
    created_at,
    updated_at
)
SELECT q.competition_session_id,
       q.user_id,
       s.facility_key,
       s.sport_key,
       sp.competition_mode || ':s' || sp.sides_per_match::TEXT || '-p' || s.participants_per_side::TEXT,
       'open',
       'active',
       q.joined_at,
       q.joined_at
FROM apollo.competition_session_queue_members AS q
INNER JOIN apollo.competition_sessions AS s
  ON s.id = q.competition_session_id
INNER JOIN apollo.sports AS sp
  ON sp.sport_key = s.sport_key
ON CONFLICT (competition_session_id, user_id) DO NOTHING;

CREATE TABLE apollo.competition_queue_intent_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_queue_intent_id UUID NOT NULL REFERENCES apollo.competition_queue_intents(id) ON DELETE CASCADE,
    competition_session_id UUID NOT NULL REFERENCES apollo.competition_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    event_type TEXT NOT NULL,
    facility_key TEXT NOT NULL,
    sport_key TEXT NOT NULL REFERENCES apollo.sports(sport_key) ON DELETE RESTRICT,
    mode_key TEXT NOT NULL,
    tier TEXT NOT NULL,
    status TEXT NOT NULL,
    actor_user_id UUID REFERENCES apollo.users(id) ON DELETE SET NULL,
    actor_role TEXT,
    actor_session_id UUID REFERENCES apollo.sessions(id) ON DELETE SET NULL,
    capability TEXT,
    trusted_surface_key TEXT,
    trusted_surface_label TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_queue_intent_events_type_allowed
        CHECK (event_type = 'competition.queue_intent.updated'),
    CONSTRAINT competition_queue_intent_events_facility_key_required
        CHECK (btrim(facility_key) <> ''),
    CONSTRAINT competition_queue_intent_events_mode_key_required
        CHECK (btrim(mode_key) <> ''),
    CONSTRAINT competition_queue_intent_events_tier_required
        CHECK (btrim(tier) <> ''),
    CONSTRAINT competition_queue_intent_events_status_allowed
        CHECK (status IN ('active', 'withdrawn'))
);

CREATE INDEX idx_competition_queue_intent_events_session_occurred_at
    ON apollo.competition_queue_intent_events (competition_session_id, occurred_at DESC, id DESC);

CREATE TABLE apollo.competition_match_previews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_session_id UUID NOT NULL REFERENCES apollo.competition_sessions(id) ON DELETE CASCADE,
    queue_version INTEGER NOT NULL,
    proposal_index INTEGER NOT NULL DEFAULT 1,
    preview_version TEXT NOT NULL,
    policy_version TEXT NOT NULL,
    rating_engine TEXT NOT NULL,
    rating_policy_version TEXT NOT NULL,
    facility_key TEXT NOT NULL,
    sport_key TEXT NOT NULL REFERENCES apollo.sports(sport_key) ON DELETE RESTRICT,
    mode_key TEXT NOT NULL,
    tier TEXT NOT NULL,
    match_quality NUMERIC(5,4) NOT NULL,
    predicted_win_probability NUMERIC(5,4) NOT NULL,
    explanation_code TEXT NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_match_previews_queue_version_positive
        CHECK (queue_version > 0),
    CONSTRAINT competition_match_previews_proposal_index_positive
        CHECK (proposal_index > 0),
    CONSTRAINT competition_match_previews_preview_version_required
        CHECK (btrim(preview_version) <> ''),
    CONSTRAINT competition_match_previews_policy_version_required
        CHECK (btrim(policy_version) <> ''),
    CONSTRAINT competition_match_previews_rating_engine_required
        CHECK (btrim(rating_engine) <> ''),
    CONSTRAINT competition_match_previews_rating_policy_version_required
        CHECK (btrim(rating_policy_version) <> ''),
    CONSTRAINT competition_match_previews_facility_key_required
        CHECK (btrim(facility_key) <> ''),
    CONSTRAINT competition_match_previews_mode_key_required
        CHECK (btrim(mode_key) <> ''),
    CONSTRAINT competition_match_previews_tier_required
        CHECK (btrim(tier) <> ''),
    CONSTRAINT competition_match_previews_match_quality_range
        CHECK (match_quality >= 0 AND match_quality <= 1),
    CONSTRAINT competition_match_previews_win_probability_range
        CHECK (predicted_win_probability >= 0 AND predicted_win_probability <= 1),
    CONSTRAINT competition_match_previews_explanation_code_required
        CHECK (btrim(explanation_code) <> ''),
    CONSTRAINT competition_match_previews_unique
        UNIQUE (
            competition_session_id,
            queue_version,
            proposal_index,
            preview_version,
            policy_version
        )
);

CREATE INDEX idx_competition_match_previews_session_generated_at
    ON apollo.competition_match_previews (competition_session_id, generated_at DESC, id DESC);

CREATE TABLE apollo.competition_match_preview_members (
    competition_match_preview_id UUID NOT NULL REFERENCES apollo.competition_match_previews(id) ON DELETE CASCADE,
    side_index INTEGER NOT NULL,
    slot_index INTEGER NOT NULL,
    competition_queue_intent_id UUID NOT NULL REFERENCES apollo.competition_queue_intents(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    rating_mu NUMERIC(8,4) NOT NULL,
    rating_sigma NUMERIC(8,4) NOT NULL,
    rating_matches_played INTEGER NOT NULL,
    rating_source TEXT NOT NULL,
    tier TEXT NOT NULL,
    PRIMARY KEY (competition_match_preview_id, side_index, slot_index),
    CONSTRAINT competition_match_preview_members_side_index_positive
        CHECK (side_index > 0),
    CONSTRAINT competition_match_preview_members_slot_index_positive
        CHECK (slot_index > 0),
    CONSTRAINT competition_match_preview_members_rating_matches_nonnegative
        CHECK (rating_matches_played >= 0),
    CONSTRAINT competition_match_preview_members_rating_source_allowed
        CHECK (rating_source IN ('legacy_projection', 'initial_rating')),
    CONSTRAINT competition_match_preview_members_tier_required
        CHECK (btrim(tier) <> ''),
    CONSTRAINT competition_match_preview_members_user_unique
        UNIQUE (competition_match_preview_id, user_id),
    CONSTRAINT competition_match_preview_members_queue_intent_unique
        UNIQUE (competition_match_preview_id, competition_queue_intent_id)
);

CREATE INDEX idx_competition_match_preview_members_intent
    ON apollo.competition_match_preview_members (competition_queue_intent_id);

CREATE TABLE apollo.competition_match_preview_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_match_preview_id UUID NOT NULL REFERENCES apollo.competition_match_previews(id) ON DELETE CASCADE,
    competition_session_id UUID NOT NULL REFERENCES apollo.competition_sessions(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    queue_version INTEGER NOT NULL,
    preview_version TEXT NOT NULL,
    policy_version TEXT NOT NULL,
    rating_engine TEXT NOT NULL,
    rating_policy_version TEXT NOT NULL,
    facility_key TEXT NOT NULL,
    sport_key TEXT NOT NULL REFERENCES apollo.sports(sport_key) ON DELETE RESTRICT,
    mode_key TEXT NOT NULL,
    tier TEXT NOT NULL,
    match_quality NUMERIC(5,4) NOT NULL,
    predicted_win_probability NUMERIC(5,4) NOT NULL,
    explanation_code TEXT NOT NULL,
    actor_user_id UUID REFERENCES apollo.users(id) ON DELETE SET NULL,
    actor_role TEXT,
    actor_session_id UUID REFERENCES apollo.sessions(id) ON DELETE SET NULL,
    capability TEXT,
    trusted_surface_key TEXT,
    trusted_surface_label TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_match_preview_events_type_allowed
        CHECK (event_type = 'competition.match_preview.generated'),
    CONSTRAINT competition_match_preview_events_queue_version_positive
        CHECK (queue_version > 0),
    CONSTRAINT competition_match_preview_events_preview_version_required
        CHECK (btrim(preview_version) <> ''),
    CONSTRAINT competition_match_preview_events_policy_version_required
        CHECK (btrim(policy_version) <> ''),
    CONSTRAINT competition_match_preview_events_rating_engine_required
        CHECK (btrim(rating_engine) <> ''),
    CONSTRAINT competition_match_preview_events_rating_policy_version_required
        CHECK (btrim(rating_policy_version) <> ''),
    CONSTRAINT competition_match_preview_events_facility_key_required
        CHECK (btrim(facility_key) <> ''),
    CONSTRAINT competition_match_preview_events_mode_key_required
        CHECK (btrim(mode_key) <> ''),
    CONSTRAINT competition_match_preview_events_tier_required
        CHECK (btrim(tier) <> ''),
    CONSTRAINT competition_match_preview_events_match_quality_range
        CHECK (match_quality >= 0 AND match_quality <= 1),
    CONSTRAINT competition_match_preview_events_win_probability_range
        CHECK (predicted_win_probability >= 0 AND predicted_win_probability <= 1),
    CONSTRAINT competition_match_preview_events_explanation_code_required
        CHECK (btrim(explanation_code) <> ''),
    CONSTRAINT competition_match_preview_events_preview_type_unique
        UNIQUE (competition_match_preview_id, event_type)
);

CREATE INDEX idx_competition_match_preview_events_session_occurred_at
    ON apollo.competition_match_preview_events (competition_session_id, occurred_at DESC, id DESC);
