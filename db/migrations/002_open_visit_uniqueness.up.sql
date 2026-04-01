CREATE UNIQUE INDEX idx_visits_open_user_facility
    ON apollo.visits (user_id, facility_key)
    WHERE departed_at IS NULL;
