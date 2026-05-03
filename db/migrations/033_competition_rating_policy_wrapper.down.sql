DROP INDEX IF EXISTS apollo.idx_competition_member_ratings_calibration_status;

DELETE FROM apollo.competition_rating_events
WHERE policy_version = 'apollo_rating_policy_wrapper_v1';

UPDATE apollo.competition_member_ratings
SET policy_version = 'apollo_legacy_rating_v1'
WHERE policy_version = 'apollo_rating_policy_wrapper_v1';

ALTER TABLE apollo.competition_rating_events
    DROP CONSTRAINT IF EXISTS competition_rating_events_policy_wrapper_payload_required,
    DROP CONSTRAINT IF EXISTS competition_rating_events_calibration_status_allowed,
    DROP COLUMN IF EXISTS climbing_cap_applied,
    DROP COLUMN IF EXISTS inactivity_decay_applied,
    DROP COLUMN IF EXISTS calibration_status;

ALTER TABLE apollo.competition_member_ratings
    DROP CONSTRAINT IF EXISTS competition_member_ratings_inactivity_decay_count_nonnegative,
    DROP CONSTRAINT IF EXISTS competition_member_ratings_calibration_status_allowed,
    DROP COLUMN IF EXISTS climbing_cap_applied,
    DROP COLUMN IF EXISTS inactivity_decay_count,
    DROP COLUMN IF EXISTS last_inactivity_decay_at,
    DROP COLUMN IF EXISTS calibration_status;
