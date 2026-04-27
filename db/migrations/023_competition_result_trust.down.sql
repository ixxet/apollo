DROP INDEX IF EXISTS apollo.idx_competition_lifecycle_events_session_occurred_at;
DROP INDEX IF EXISTS apollo.idx_competition_lifecycle_events_result_occurred_at;
DROP INDEX IF EXISTS apollo.idx_competition_lifecycle_events_match_occurred_at;
DROP TABLE IF EXISTS apollo.competition_lifecycle_events;

ALTER TABLE apollo.competition_matches
    DROP CONSTRAINT IF EXISTS competition_matches_canonical_result_id_fkey,
    DROP CONSTRAINT IF EXISTS competition_matches_result_version_nonnegative,
    DROP COLUMN IF EXISTS canonical_result_id,
    DROP COLUMN IF EXISTS result_version;

ALTER TABLE apollo.competition_match_result_sides
    DROP CONSTRAINT IF EXISTS competition_match_result_sides_result_id_fkey,
    DROP CONSTRAINT IF EXISTS competition_match_result_sides_result_team_unique,
    DROP CONSTRAINT IF EXISTS competition_match_result_sides_pkey;

ALTER TABLE apollo.competition_match_results
    DROP CONSTRAINT IF EXISTS competition_match_results_supersedes_result_id_fkey;

DELETE FROM apollo.competition_match_result_sides
WHERE competition_match_result_id IN (
    SELECT id
    FROM apollo.competition_match_results
    WHERE supersedes_result_id IS NOT NULL
       OR result_status IN ('recorded', 'disputed', 'corrected', 'voided')
);

DELETE FROM apollo.competition_match_results
WHERE supersedes_result_id IS NOT NULL
   OR result_status IN ('recorded', 'disputed', 'corrected', 'voided');

ALTER TABLE apollo.competition_match_results
    DROP CONSTRAINT IF EXISTS competition_match_results_match_id_id_unique,
    DROP CONSTRAINT IF EXISTS competition_match_results_pkey,
    ADD CONSTRAINT competition_match_results_pkey PRIMARY KEY (competition_match_id);

ALTER TABLE apollo.competition_match_result_sides
    ADD CONSTRAINT competition_match_result_sides_pkey
        PRIMARY KEY (competition_match_id, side_index),
    ADD CONSTRAINT competition_match_result_sides_match_team_unique
        UNIQUE (competition_match_id, competition_session_team_id),
    ADD CONSTRAINT competition_match_result_sides_competition_match_id_fkey
        FOREIGN KEY (competition_match_id)
        REFERENCES apollo.competition_match_results(competition_match_id)
        ON DELETE CASCADE;

ALTER TABLE apollo.competition_match_result_sides
    DROP COLUMN IF EXISTS competition_match_result_id;

ALTER TABLE apollo.competition_match_results
    DROP CONSTRAINT IF EXISTS competition_match_results_dispute_status_allowed,
    DROP CONSTRAINT IF EXISTS competition_match_results_result_status_allowed,
    DROP COLUMN IF EXISTS corrected_at,
    DROP COLUMN IF EXISTS finalized_at,
    DROP COLUMN IF EXISTS supersedes_result_id,
    DROP COLUMN IF EXISTS correction_id,
    DROP COLUMN IF EXISTS dispute_status,
    DROP COLUMN IF EXISTS result_status,
    DROP COLUMN IF EXISTS id;
