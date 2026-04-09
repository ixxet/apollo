ALTER TABLE apollo.competition_sessions
    DROP CONSTRAINT competition_sessions_status_allowed;

ALTER TABLE apollo.competition_sessions
    ADD COLUMN queue_version INTEGER NOT NULL DEFAULT 0,
    ADD CONSTRAINT competition_sessions_status_allowed
        CHECK (status IN ('draft', 'queue_open', 'assigned', 'in_progress', 'archived')),
    ADD CONSTRAINT competition_sessions_queue_version_nonnegative
        CHECK (queue_version >= 0);

CREATE TABLE apollo.competition_session_queue_members (
    competition_session_id UUID NOT NULL REFERENCES apollo.competition_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (competition_session_id, user_id)
);

CREATE INDEX idx_competition_session_queue_members_session_joined_at
    ON apollo.competition_session_queue_members (competition_session_id, joined_at ASC, user_id ASC);

ALTER TABLE apollo.competition_matches
    DROP CONSTRAINT competition_matches_status_allowed;

ALTER TABLE apollo.competition_matches
    ADD CONSTRAINT competition_matches_status_allowed
        CHECK (status IN ('draft', 'assigned', 'in_progress', 'archived'));
