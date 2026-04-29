DROP TRIGGER IF EXISTS competition_safety_events_immutable ON apollo.competition_safety_events;
DROP TRIGGER IF EXISTS competition_reliability_events_immutable ON apollo.competition_reliability_events;
DROP TRIGGER IF EXISTS competition_safety_blocks_immutable ON apollo.competition_safety_blocks;
DROP TRIGGER IF EXISTS competition_safety_reports_immutable ON apollo.competition_safety_reports;

DROP FUNCTION IF EXISTS apollo.reject_competition_safety_fact_change();

CREATE OR REPLACE FUNCTION apollo.reject_competition_safety_fact_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'competition safety and reliability facts are immutable once recorded';
END;
$$;

CREATE TRIGGER competition_safety_reports_immutable
    BEFORE UPDATE ON apollo.competition_safety_reports
    FOR EACH ROW
    EXECUTE FUNCTION apollo.reject_competition_safety_fact_update();

CREATE TRIGGER competition_safety_blocks_immutable
    BEFORE UPDATE ON apollo.competition_safety_blocks
    FOR EACH ROW
    EXECUTE FUNCTION apollo.reject_competition_safety_fact_update();

CREATE TRIGGER competition_reliability_events_immutable
    BEFORE UPDATE ON apollo.competition_reliability_events
    FOR EACH ROW
    EXECUTE FUNCTION apollo.reject_competition_safety_fact_update();

CREATE TRIGGER competition_safety_events_immutable
    BEFORE UPDATE ON apollo.competition_safety_events
    FOR EACH ROW
    EXECUTE FUNCTION apollo.reject_competition_safety_fact_update();
