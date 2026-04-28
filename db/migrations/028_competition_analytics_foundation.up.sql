CREATE TABLE apollo.competition_analytics_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type TEXT NOT NULL,
    projection_version TEXT NOT NULL,
    projection_watermark TEXT NOT NULL,
    user_id UUID REFERENCES apollo.users(id) ON DELETE RESTRICT,
    sport_key TEXT NOT NULL REFERENCES apollo.sports(sport_key) ON DELETE RESTRICT,
    facility_key TEXT NOT NULL DEFAULT 'all',
    mode_key TEXT NOT NULL DEFAULT 'all',
    team_scope TEXT NOT NULL DEFAULT 'all',
    stat_type TEXT NOT NULL,
    stat_value NUMERIC(12,4) NOT NULL,
    source_match_id UUID REFERENCES apollo.competition_matches(id) ON DELETE SET NULL,
    source_result_id UUID REFERENCES apollo.competition_match_results(id) ON DELETE SET NULL,
    sample_size INTEGER NOT NULL,
    confidence NUMERIC(5,4) NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_analytics_events_type_allowed
        CHECK (event_type IN (
            'competition.analytics.stat_computed',
            'competition.analytics.projection_rebuilt'
        )),
    CONSTRAINT competition_analytics_events_projection_version_required
        CHECK (btrim(projection_version) <> ''),
    CONSTRAINT competition_analytics_events_watermark_required
        CHECK (btrim(projection_watermark) <> ''),
    CONSTRAINT competition_analytics_events_facility_key_required
        CHECK (btrim(facility_key) <> ''),
    CONSTRAINT competition_analytics_events_mode_key_required
        CHECK (btrim(mode_key) <> ''),
    CONSTRAINT competition_analytics_events_team_scope_allowed
        CHECK (team_scope IN ('all', 'solo', 'team')),
    CONSTRAINT competition_analytics_events_stat_type_allowed
        CHECK (stat_type IN (
            'matches_played',
            'wins',
            'losses',
            'draws',
            'current_streak',
            'rating_movement',
            'opponent_strength',
            'team_vs_solo_delta',
            'projection_rebuilt'
        )),
    CONSTRAINT competition_analytics_events_sample_size_nonnegative
        CHECK (sample_size >= 0),
    CONSTRAINT competition_analytics_events_confidence_range
        CHECK (confidence >= 0 AND confidence <= 1),
    CONSTRAINT competition_analytics_events_stat_payload_required
        CHECK (
            event_type <> 'competition.analytics.stat_computed'
            OR (
                user_id IS NOT NULL
                AND source_match_id IS NOT NULL
                AND source_result_id IS NOT NULL
                AND stat_type <> 'projection_rebuilt'
                AND sample_size > 0
            )
        ),
    CONSTRAINT competition_analytics_events_rebuild_payload_required
        CHECK (
            event_type <> 'competition.analytics.projection_rebuilt'
            OR (
                user_id IS NULL
                AND facility_key = 'all'
                AND mode_key = 'all'
                AND team_scope = 'all'
                AND stat_type = 'projection_rebuilt'
            )
        )
);

CREATE INDEX idx_competition_analytics_events_source_result
    ON apollo.competition_analytics_events (source_result_id, event_type, stat_type);

CREATE INDEX idx_competition_analytics_events_sport_watermark
    ON apollo.competition_analytics_events (sport_key, projection_version, projection_watermark, event_type);

CREATE TABLE apollo.competition_analytics_projections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    sport_key TEXT NOT NULL REFERENCES apollo.sports(sport_key) ON DELETE RESTRICT,
    facility_key TEXT NOT NULL DEFAULT 'all',
    mode_key TEXT NOT NULL DEFAULT 'all',
    team_scope TEXT NOT NULL DEFAULT 'all',
    stat_type TEXT NOT NULL,
    stat_value NUMERIC(12,4) NOT NULL,
    source_match_id UUID REFERENCES apollo.competition_matches(id) ON DELETE SET NULL,
    source_result_id UUID REFERENCES apollo.competition_match_results(id) ON DELETE SET NULL,
    sample_size INTEGER NOT NULL,
    confidence NUMERIC(5,4) NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL,
    projection_version TEXT NOT NULL,
    projection_watermark TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_analytics_projections_projection_version_required
        CHECK (btrim(projection_version) <> ''),
    CONSTRAINT competition_analytics_projections_watermark_required
        CHECK (btrim(projection_watermark) <> ''),
    CONSTRAINT competition_analytics_projections_facility_key_required
        CHECK (btrim(facility_key) <> ''),
    CONSTRAINT competition_analytics_projections_mode_key_required
        CHECK (btrim(mode_key) <> ''),
    CONSTRAINT competition_analytics_projections_team_scope_allowed
        CHECK (team_scope IN ('all', 'solo', 'team')),
    CONSTRAINT competition_analytics_projections_stat_type_allowed
        CHECK (stat_type IN (
            'matches_played',
            'wins',
            'losses',
            'draws',
            'current_streak',
            'rating_movement',
            'opponent_strength',
            'team_vs_solo_delta'
        )),
    CONSTRAINT competition_analytics_projections_sample_size_nonnegative
        CHECK (sample_size >= 0),
    CONSTRAINT competition_analytics_projections_confidence_range
        CHECK (confidence >= 0 AND confidence <= 1)
);

CREATE UNIQUE INDEX idx_competition_analytics_projections_unique
    ON apollo.competition_analytics_projections (
        projection_version,
        user_id,
        sport_key,
        facility_key,
        mode_key,
        team_scope,
        stat_type
    );

CREATE INDEX idx_competition_analytics_projections_user_sport
    ON apollo.competition_analytics_projections (user_id, sport_key, mode_key, facility_key, team_scope);

CREATE INDEX idx_competition_analytics_projections_source_result
    ON apollo.competition_analytics_projections (source_result_id, stat_type);
