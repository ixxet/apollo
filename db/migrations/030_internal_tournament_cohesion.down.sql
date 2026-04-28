DROP TRIGGER IF EXISTS competition_tournament_advancements_coherence
    ON apollo.competition_tournament_advancements;

DROP FUNCTION IF EXISTS apollo.validate_competition_tournament_advancement_coherence();

ALTER TABLE apollo.competition_tournament_advancements
    DROP CONSTRAINT IF EXISTS competition_tournament_advancements_losing_snapshot_bracket_fkey,
    DROP CONSTRAINT IF EXISTS competition_tournament_advancements_winning_snapshot_bracket_fkey;

ALTER TABLE apollo.competition_tournament_match_bindings
    DROP CONSTRAINT IF EXISTS competition_tournament_match_bindings_side_two_snapshot_bracket_fkey,
    DROP CONSTRAINT IF EXISTS competition_tournament_match_bindings_side_one_snapshot_bracket_fkey;

ALTER TABLE apollo.competition_tournament_team_snapshots
    DROP CONSTRAINT IF EXISTS competition_tournament_team_snapshots_bracket_id_id_unique;
