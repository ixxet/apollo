ALTER TABLE apollo.competition_rating_events
    DROP CONSTRAINT IF EXISTS competition_rating_events_event_type_allowed,
    ADD COLUMN legacy_mu NUMERIC(8,4),
    ADD COLUMN legacy_sigma NUMERIC(8,4),
    ADD COLUMN openskill_mu NUMERIC(8,4),
    ADD COLUMN openskill_sigma NUMERIC(8,4),
    ADD COLUMN delta_from_legacy NUMERIC(8,4),
    ADD COLUMN accepted_delta_budget NUMERIC(8,4),
    ADD COLUMN comparison_scenario TEXT,
    ADD CONSTRAINT competition_rating_events_event_type_allowed
        CHECK (event_type IN (
            'competition.rating.legacy_computed',
            'competition.rating.openskill_computed',
            'competition.rating.delta_flagged',
            'competition.rating.policy_selected',
            'competition.rating.projection_rebuilt'
        )),
    ADD CONSTRAINT competition_rating_events_openskill_payload_required
        CHECK (
            event_type NOT IN (
                'competition.rating.openskill_computed',
                'competition.rating.delta_flagged'
            )
            OR (
                mode_key IS NOT NULL
                AND btrim(mode_key) <> ''
                AND user_id IS NOT NULL
                AND source_result_id IS NOT NULL
                AND legacy_mu IS NOT NULL
                AND legacy_sigma IS NOT NULL
                AND openskill_mu IS NOT NULL
                AND openskill_sigma IS NOT NULL
                AND delta_from_legacy IS NOT NULL
                AND accepted_delta_budget IS NOT NULL
                AND accepted_delta_budget >= 0
                AND comparison_scenario IS NOT NULL
                AND btrim(comparison_scenario) <> ''
            )
        ),
    ADD CONSTRAINT competition_rating_events_delta_flagged_budget_required
        CHECK (
            event_type <> 'competition.rating.delta_flagged'
            OR abs(delta_from_legacy) > accepted_delta_budget
        );

CREATE UNIQUE INDEX idx_competition_rating_events_openskill_unique
    ON apollo.competition_rating_events (
        rating_engine,
        engine_version,
        policy_version,
        sport_key,
        mode_key,
        source_result_id,
        user_id,
        comparison_scenario
    )
    WHERE event_type = 'competition.rating.openskill_computed';

CREATE UNIQUE INDEX idx_competition_rating_events_delta_flagged_unique
    ON apollo.competition_rating_events (
        rating_engine,
        engine_version,
        policy_version,
        sport_key,
        mode_key,
        source_result_id,
        user_id,
        comparison_scenario
    )
    WHERE event_type = 'competition.rating.delta_flagged';

CREATE TABLE apollo.competition_rating_comparisons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sport_key TEXT NOT NULL REFERENCES apollo.sports(sport_key) ON DELETE RESTRICT,
    mode_key TEXT NOT NULL,
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    source_result_id UUID NOT NULL REFERENCES apollo.competition_match_results(id) ON DELETE RESTRICT,
    legacy_rating_engine TEXT NOT NULL,
    legacy_engine_version TEXT NOT NULL,
    legacy_policy_version TEXT NOT NULL,
    openskill_rating_engine TEXT NOT NULL,
    openskill_engine_version TEXT NOT NULL,
    openskill_policy_version TEXT NOT NULL,
    legacy_mu NUMERIC(8,4) NOT NULL,
    legacy_sigma NUMERIC(8,4) NOT NULL,
    openskill_mu NUMERIC(8,4) NOT NULL,
    openskill_sigma NUMERIC(8,4) NOT NULL,
    delta_from_legacy NUMERIC(8,4) NOT NULL,
    accepted_delta_budget NUMERIC(8,4) NOT NULL,
    comparison_scenario TEXT NOT NULL,
    delta_flagged BOOLEAN NOT NULL,
    projection_watermark TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_rating_comparisons_mode_key_required
        CHECK (btrim(mode_key) <> ''),
    CONSTRAINT competition_rating_comparisons_legacy_engine_required
        CHECK (btrim(legacy_rating_engine) <> ''),
    CONSTRAINT competition_rating_comparisons_legacy_engine_version_required
        CHECK (btrim(legacy_engine_version) <> ''),
    CONSTRAINT competition_rating_comparisons_legacy_policy_version_required
        CHECK (btrim(legacy_policy_version) <> ''),
    CONSTRAINT competition_rating_comparisons_openskill_engine_required
        CHECK (btrim(openskill_rating_engine) <> ''),
    CONSTRAINT competition_rating_comparisons_openskill_engine_version_required
        CHECK (btrim(openskill_engine_version) <> ''),
    CONSTRAINT competition_rating_comparisons_openskill_policy_version_required
        CHECK (btrim(openskill_policy_version) <> ''),
    CONSTRAINT competition_rating_comparisons_delta_budget_nonnegative
        CHECK (accepted_delta_budget >= 0),
    CONSTRAINT competition_rating_comparisons_scenario_required
        CHECK (btrim(comparison_scenario) <> ''),
    CONSTRAINT competition_rating_comparisons_watermark_required
        CHECK (btrim(projection_watermark) <> ''),
    CONSTRAINT competition_rating_comparisons_flag_matches_budget
        CHECK (delta_flagged = (abs(delta_from_legacy) > accepted_delta_budget))
);

CREATE UNIQUE INDEX idx_competition_rating_comparisons_unique
    ON apollo.competition_rating_comparisons (
        openskill_rating_engine,
        openskill_engine_version,
        openskill_policy_version,
        sport_key,
        mode_key,
        source_result_id,
        user_id,
        comparison_scenario
    );

CREATE INDEX idx_competition_rating_comparisons_sport_watermark
    ON apollo.competition_rating_comparisons (sport_key, projection_watermark);

CREATE INDEX idx_competition_rating_comparisons_delta_flagged
    ON apollo.competition_rating_comparisons (sport_key, delta_flagged, comparison_scenario);
