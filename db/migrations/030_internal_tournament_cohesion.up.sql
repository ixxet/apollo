ALTER TABLE apollo.competition_tournament_team_snapshots
    ADD CONSTRAINT competition_tournament_team_snapshots_bracket_id_id_unique
        UNIQUE (bracket_id, id);

ALTER TABLE apollo.competition_tournament_match_bindings
    ADD CONSTRAINT competition_tournament_match_bindings_side_one_snapshot_bracket_fkey
        FOREIGN KEY (bracket_id, side_one_team_snapshot_id)
        REFERENCES apollo.competition_tournament_team_snapshots(bracket_id, id)
        ON DELETE RESTRICT,
    ADD CONSTRAINT competition_tournament_match_bindings_side_two_snapshot_bracket_fkey
        FOREIGN KEY (bracket_id, side_two_team_snapshot_id)
        REFERENCES apollo.competition_tournament_team_snapshots(bracket_id, id)
        ON DELETE RESTRICT;

ALTER TABLE apollo.competition_tournament_advancements
    ADD CONSTRAINT competition_tournament_advancements_winning_snapshot_bracket_fkey
        FOREIGN KEY (bracket_id, winning_team_snapshot_id)
        REFERENCES apollo.competition_tournament_team_snapshots(bracket_id, id)
        ON DELETE RESTRICT,
    ADD CONSTRAINT competition_tournament_advancements_losing_snapshot_bracket_fkey
        FOREIGN KEY (bracket_id, losing_team_snapshot_id)
        REFERENCES apollo.competition_tournament_team_snapshots(bracket_id, id)
        ON DELETE RESTRICT;

CREATE OR REPLACE FUNCTION apollo.validate_competition_tournament_advancement_coherence()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    binding apollo.competition_tournament_match_bindings%ROWTYPE;
    result_match_id UUID;
    result_status TEXT;
    canonical_result_id UUID;
BEGIN
    SELECT *
    INTO binding
    FROM apollo.competition_tournament_match_bindings
    WHERE id = NEW.match_binding_id;

    IF binding.id IS NULL THEN
        RAISE EXCEPTION 'competition tournament advancement binding does not exist';
    END IF;

    IF binding.tournament_id <> NEW.tournament_id OR binding.bracket_id <> NEW.bracket_id THEN
        RAISE EXCEPTION 'competition tournament advancement binding does not match tournament bracket';
    END IF;

    IF binding.round <> NEW.round OR binding.competition_match_id <> NEW.competition_match_id THEN
        RAISE EXCEPTION 'competition tournament advancement does not match bound competition match';
    END IF;

    IF NOT (
        (NEW.winning_team_snapshot_id = binding.side_one_team_snapshot_id AND NEW.losing_team_snapshot_id = binding.side_two_team_snapshot_id) OR
        (NEW.winning_team_snapshot_id = binding.side_two_team_snapshot_id AND NEW.losing_team_snapshot_id = binding.side_one_team_snapshot_id)
    ) THEN
        RAISE EXCEPTION 'competition tournament advancement snapshots do not match bound sides';
    END IF;

    SELECT r.competition_match_id,
           r.result_status,
           m.canonical_result_id
    INTO result_match_id,
         result_status,
         canonical_result_id
    FROM apollo.competition_match_results AS r
    INNER JOIN apollo.competition_matches AS m
        ON m.id = r.competition_match_id
    WHERE r.id = NEW.canonical_result_id;

    IF result_match_id IS NULL OR result_match_id <> NEW.competition_match_id THEN
        RAISE EXCEPTION 'competition tournament advancement result does not match bound competition match';
    END IF;

    IF canonical_result_id IS DISTINCT FROM NEW.canonical_result_id THEN
        RAISE EXCEPTION 'competition tournament advancement result is not canonical';
    END IF;

    IF result_status NOT IN ('finalized', 'corrected') THEN
        RAISE EXCEPTION 'competition tournament advancement result is not finalized or corrected';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER competition_tournament_advancements_coherence
    BEFORE INSERT OR UPDATE ON apollo.competition_tournament_advancements
    FOR EACH ROW
    EXECUTE FUNCTION apollo.validate_competition_tournament_advancement_coherence();
