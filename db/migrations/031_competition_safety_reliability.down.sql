DROP TRIGGER IF EXISTS competition_safety_events_immutable ON apollo.competition_safety_events;
DROP TRIGGER IF EXISTS competition_reliability_events_immutable ON apollo.competition_reliability_events;
DROP TRIGGER IF EXISTS competition_safety_blocks_immutable ON apollo.competition_safety_blocks;
DROP TRIGGER IF EXISTS competition_safety_reports_immutable ON apollo.competition_safety_reports;

DROP FUNCTION IF EXISTS apollo.reject_competition_safety_fact_update();

DROP TABLE IF EXISTS apollo.competition_safety_events;
DROP TABLE IF EXISTS apollo.competition_reliability_events;
DROP TABLE IF EXISTS apollo.competition_safety_blocks;
DROP TABLE IF EXISTS apollo.competition_safety_reports;
