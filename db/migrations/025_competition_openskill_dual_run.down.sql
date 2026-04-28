DROP INDEX IF EXISTS apollo.idx_competition_rating_comparisons_delta_flagged;
DROP INDEX IF EXISTS apollo.idx_competition_rating_comparisons_sport_watermark;
DROP INDEX IF EXISTS apollo.idx_competition_rating_comparisons_unique;
DROP TABLE IF EXISTS apollo.competition_rating_comparisons;

DROP INDEX IF EXISTS apollo.idx_competition_rating_events_delta_flagged_unique;
DROP INDEX IF EXISTS apollo.idx_competition_rating_events_openskill_unique;

DELETE FROM apollo.competition_rating_events
WHERE event_type IN (
    'competition.rating.openskill_computed',
    'competition.rating.delta_flagged'
);

ALTER TABLE apollo.competition_rating_events
    DROP CONSTRAINT IF EXISTS competition_rating_events_delta_flagged_budget_required,
    DROP CONSTRAINT IF EXISTS competition_rating_events_openskill_payload_required,
    DROP CONSTRAINT IF EXISTS competition_rating_events_event_type_allowed,
    DROP COLUMN IF EXISTS comparison_scenario,
    DROP COLUMN IF EXISTS accepted_delta_budget,
    DROP COLUMN IF EXISTS delta_from_legacy,
    DROP COLUMN IF EXISTS openskill_sigma,
    DROP COLUMN IF EXISTS openskill_mu,
    DROP COLUMN IF EXISTS legacy_sigma,
    DROP COLUMN IF EXISTS legacy_mu,
    ADD CONSTRAINT competition_rating_events_event_type_allowed
        CHECK (event_type IN (
            'competition.rating.legacy_computed',
            'competition.rating.policy_selected',
            'competition.rating.projection_rebuilt'
        ));
