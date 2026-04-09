UPDATE apollo.competition_matches
SET status = 'draft',
    archived_at = NULL,
    updated_at = NOW()
WHERE status IN ('assigned', 'in_progress');

ALTER TABLE apollo.competition_matches
    DROP CONSTRAINT competition_matches_status_allowed;

ALTER TABLE apollo.competition_matches
    ADD CONSTRAINT competition_matches_status_allowed
        CHECK (status IN ('draft', 'archived'));

DROP INDEX IF EXISTS apollo.idx_competition_session_queue_members_session_joined_at;
DROP TABLE IF EXISTS apollo.competition_session_queue_members;

UPDATE apollo.competition_sessions
SET status = 'draft',
    archived_at = NULL,
    queue_version = 0,
    updated_at = NOW()
WHERE status IN ('queue_open', 'assigned', 'in_progress');

ALTER TABLE apollo.competition_sessions
    DROP CONSTRAINT competition_sessions_queue_version_nonnegative,
    DROP CONSTRAINT competition_sessions_status_allowed;

ALTER TABLE apollo.competition_sessions
    ADD CONSTRAINT competition_sessions_status_allowed
        CHECK (status IN ('draft', 'archived'));

ALTER TABLE apollo.competition_sessions
    DROP COLUMN queue_version;
