ALTER TABLE apollo.competition_member_ratings
    ADD COLUMN calibration_status TEXT,
    ADD COLUMN last_inactivity_decay_at TIMESTAMPTZ,
    ADD COLUMN inactivity_decay_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN climbing_cap_applied BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE apollo.competition_member_ratings
SET calibration_status = CASE
    WHEN matches_played >= 5 THEN 'ranked'
    ELSE 'provisional'
END
WHERE calibration_status IS NULL;

ALTER TABLE apollo.competition_member_ratings
    ALTER COLUMN calibration_status SET NOT NULL,
    ALTER COLUMN calibration_status SET DEFAULT 'provisional',
    ADD CONSTRAINT competition_member_ratings_calibration_status_allowed
        CHECK (calibration_status IN ('provisional', 'ranked')),
    ADD CONSTRAINT competition_member_ratings_inactivity_decay_count_nonnegative
        CHECK (inactivity_decay_count >= 0);

ALTER TABLE apollo.competition_rating_events
    ADD COLUMN calibration_status TEXT,
    ADD COLUMN inactivity_decay_applied BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN climbing_cap_applied BOOLEAN NOT NULL DEFAULT FALSE,
    ADD CONSTRAINT competition_rating_events_calibration_status_allowed
        CHECK (calibration_status IS NULL OR calibration_status IN ('provisional', 'ranked')),
    ADD CONSTRAINT competition_rating_events_policy_wrapper_payload_required
        CHECK (
            event_type <> 'competition.rating.legacy_computed'
            OR policy_version <> 'apollo_rating_policy_wrapper_v1'
            OR calibration_status IS NOT NULL
        );

CREATE INDEX idx_competition_member_ratings_calibration_status
    ON apollo.competition_member_ratings (sport_key, mode_key, calibration_status);
