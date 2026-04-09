ALTER TABLE apollo.competition_sessions
    DROP CONSTRAINT competition_sessions_status_allowed;

ALTER TABLE apollo.competition_sessions
    ADD CONSTRAINT competition_sessions_status_allowed
        CHECK (status IN ('draft', 'queue_open', 'assigned', 'in_progress', 'completed', 'archived'));

ALTER TABLE apollo.competition_matches
    DROP CONSTRAINT competition_matches_status_allowed;

ALTER TABLE apollo.competition_matches
    ADD CONSTRAINT competition_matches_status_allowed
        CHECK (status IN ('draft', 'assigned', 'in_progress', 'completed', 'archived'));

CREATE TABLE apollo.competition_match_results (
    competition_match_id UUID PRIMARY KEY REFERENCES apollo.competition_matches(id) ON DELETE CASCADE,
    recorded_by_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    recorded_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE apollo.competition_match_result_sides (
    competition_match_id UUID NOT NULL,
    side_index INTEGER NOT NULL,
    competition_session_team_id UUID NOT NULL,
    outcome TEXT NOT NULL,
    PRIMARY KEY (competition_match_id, side_index),
    CONSTRAINT competition_match_result_sides_side_index_positive
        CHECK (side_index > 0),
    CONSTRAINT competition_match_result_sides_outcome_allowed
        CHECK (outcome IN ('win', 'loss', 'draw')),
    CONSTRAINT competition_match_result_sides_match_team_unique
        UNIQUE (competition_match_id, competition_session_team_id),
    FOREIGN KEY (competition_match_id)
        REFERENCES apollo.competition_match_results(competition_match_id)
        ON DELETE CASCADE,
    FOREIGN KEY (competition_match_id, competition_session_team_id)
        REFERENCES apollo.competition_match_side_slots(competition_match_id, competition_session_team_id)
        ON DELETE RESTRICT
);

CREATE TABLE apollo.competition_member_ratings (
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    sport_key TEXT NOT NULL REFERENCES apollo.sports(sport_key) ON DELETE RESTRICT,
    mode_key TEXT NOT NULL,
    mu NUMERIC(8,4) NOT NULL,
    sigma NUMERIC(8,4) NOT NULL,
    matches_played INTEGER NOT NULL DEFAULT 0,
    last_played TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, sport_key, mode_key),
    CONSTRAINT competition_member_ratings_mode_key_required
        CHECK (btrim(mode_key) <> ''),
    CONSTRAINT competition_member_ratings_matches_played_nonnegative
        CHECK (matches_played >= 0)
);

CREATE INDEX idx_competition_member_ratings_sport_mode
    ON apollo.competition_member_ratings (sport_key, mode_key, user_id);
