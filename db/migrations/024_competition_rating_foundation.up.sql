CREATE TABLE apollo.competition_rating_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type TEXT NOT NULL,
    rating_engine TEXT NOT NULL,
    engine_version TEXT NOT NULL,
    policy_version TEXT NOT NULL,
    sport_key TEXT NOT NULL REFERENCES apollo.sports(sport_key) ON DELETE RESTRICT,
    mode_key TEXT,
    user_id UUID REFERENCES apollo.users(id) ON DELETE RESTRICT,
    source_result_id UUID REFERENCES apollo.competition_match_results(id) ON DELETE RESTRICT,
    mu NUMERIC(8,4),
    sigma NUMERIC(8,4),
    delta_mu NUMERIC(8,4),
    delta_sigma NUMERIC(8,4),
    projection_watermark TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_rating_events_event_type_allowed
        CHECK (event_type IN (
            'competition.rating.legacy_computed',
            'competition.rating.policy_selected',
            'competition.rating.projection_rebuilt'
        )),
    CONSTRAINT competition_rating_events_engine_required
        CHECK (btrim(rating_engine) <> ''),
    CONSTRAINT competition_rating_events_engine_version_required
        CHECK (btrim(engine_version) <> ''),
    CONSTRAINT competition_rating_events_policy_version_required
        CHECK (btrim(policy_version) <> ''),
    CONSTRAINT competition_rating_events_watermark_required
        CHECK (btrim(projection_watermark) <> ''),
    CONSTRAINT competition_rating_events_legacy_payload_required
        CHECK (
            event_type <> 'competition.rating.legacy_computed'
            OR (
                mode_key IS NOT NULL
                AND btrim(mode_key) <> ''
                AND user_id IS NOT NULL
                AND source_result_id IS NOT NULL
                AND mu IS NOT NULL
                AND sigma IS NOT NULL
                AND delta_mu IS NOT NULL
                AND delta_sigma IS NOT NULL
            )
        )
);

CREATE UNIQUE INDEX idx_competition_rating_events_legacy_unique
    ON apollo.competition_rating_events (
        rating_engine,
        engine_version,
        policy_version,
        sport_key,
        mode_key,
        source_result_id,
        user_id
    )
    WHERE event_type = 'competition.rating.legacy_computed';

CREATE INDEX idx_competition_rating_events_source_result
    ON apollo.competition_rating_events (source_result_id, event_type);

CREATE INDEX idx_competition_rating_events_sport_watermark
    ON apollo.competition_rating_events (sport_key, projection_watermark, event_type);

ALTER TABLE apollo.competition_member_ratings
    ADD COLUMN rating_engine TEXT NOT NULL DEFAULT 'legacy_elo_like',
    ADD COLUMN engine_version TEXT NOT NULL DEFAULT 'legacy_elo_like.v1',
    ADD COLUMN policy_version TEXT NOT NULL DEFAULT 'apollo_legacy_rating_v1',
    ADD COLUMN source_result_id UUID REFERENCES apollo.competition_match_results(id) ON DELETE SET NULL,
    ADD COLUMN rating_event_id UUID REFERENCES apollo.competition_rating_events(id) ON DELETE SET NULL,
    ADD COLUMN projection_watermark TEXT NOT NULL DEFAULT 'no_results',
    ADD CONSTRAINT competition_member_ratings_engine_required
        CHECK (btrim(rating_engine) <> ''),
    ADD CONSTRAINT competition_member_ratings_engine_version_required
        CHECK (btrim(engine_version) <> ''),
    ADD CONSTRAINT competition_member_ratings_policy_version_required
        CHECK (btrim(policy_version) <> ''),
    ADD CONSTRAINT competition_member_ratings_projection_watermark_required
        CHECK (btrim(projection_watermark) <> '');

CREATE INDEX idx_competition_member_ratings_source_result
    ON apollo.competition_member_ratings (source_result_id);

CREATE INDEX idx_competition_member_ratings_event
    ON apollo.competition_member_ratings (rating_event_id);
