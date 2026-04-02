DROP INDEX IF EXISTS apollo.idx_visits_departure_source_event_id;

ALTER TABLE apollo.visits
DROP COLUMN IF EXISTS departure_source_event_id;
