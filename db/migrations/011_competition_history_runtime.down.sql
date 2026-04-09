DROP INDEX IF EXISTS apollo.idx_competition_member_ratings_sport_mode;

DROP TABLE IF EXISTS apollo.competition_member_ratings;
DROP TABLE IF EXISTS apollo.competition_match_result_sides;
DROP TABLE IF EXISTS apollo.competition_match_results;

UPDATE apollo.competition_matches
SET status = 'archived'
WHERE status = 'completed';

ALTER TABLE apollo.competition_matches
    DROP CONSTRAINT competition_matches_status_allowed;

ALTER TABLE apollo.competition_matches
    ADD CONSTRAINT competition_matches_status_allowed
        CHECK (status IN ('draft', 'assigned', 'in_progress', 'archived'));

UPDATE apollo.competition_sessions
SET status = 'archived'
WHERE status = 'completed';

ALTER TABLE apollo.competition_sessions
    DROP CONSTRAINT competition_sessions_status_allowed;

ALTER TABLE apollo.competition_sessions
    ADD CONSTRAINT competition_sessions_status_allowed
        CHECK (status IN ('draft', 'queue_open', 'assigned', 'in_progress', 'archived'));
