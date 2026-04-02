ALTER TABLE apollo.visits
ADD COLUMN departure_source_event_id TEXT;

CREATE UNIQUE INDEX idx_visits_departure_source_event_id
    ON apollo.visits (departure_source_event_id)
    WHERE departure_source_event_id IS NOT NULL;
