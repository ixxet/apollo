DROP INDEX IF EXISTS apollo.idx_competition_member_ratings_event;
DROP INDEX IF EXISTS apollo.idx_competition_member_ratings_source_result;

ALTER TABLE apollo.competition_member_ratings
    DROP CONSTRAINT IF EXISTS competition_member_ratings_projection_watermark_required,
    DROP CONSTRAINT IF EXISTS competition_member_ratings_policy_version_required,
    DROP CONSTRAINT IF EXISTS competition_member_ratings_engine_version_required,
    DROP CONSTRAINT IF EXISTS competition_member_ratings_engine_required,
    DROP COLUMN IF EXISTS projection_watermark,
    DROP COLUMN IF EXISTS rating_event_id,
    DROP COLUMN IF EXISTS source_result_id,
    DROP COLUMN IF EXISTS policy_version,
    DROP COLUMN IF EXISTS engine_version,
    DROP COLUMN IF EXISTS rating_engine;

DROP INDEX IF EXISTS apollo.idx_competition_rating_events_sport_watermark;
DROP INDEX IF EXISTS apollo.idx_competition_rating_events_source_result;
DROP INDEX IF EXISTS apollo.idx_competition_rating_events_legacy_unique;
DROP TABLE IF EXISTS apollo.competition_rating_events;
