ALTER TABLE apollo.competition_match_results
    ADD COLUMN id UUID DEFAULT gen_random_uuid(),
    ADD COLUMN result_status TEXT,
    ADD COLUMN dispute_status TEXT,
    ADD COLUMN correction_id UUID,
    ADD COLUMN supersedes_result_id UUID,
    ADD COLUMN finalized_at TIMESTAMPTZ,
    ADD COLUMN corrected_at TIMESTAMPTZ;

UPDATE apollo.competition_match_results
SET result_status = 'finalized',
    dispute_status = 'none',
    finalized_at = recorded_at
WHERE result_status IS NULL;

ALTER TABLE apollo.competition_match_results
    ALTER COLUMN id SET NOT NULL,
    ALTER COLUMN result_status SET NOT NULL,
    ALTER COLUMN dispute_status SET NOT NULL,
    ADD CONSTRAINT competition_match_results_result_status_allowed
        CHECK (result_status IN ('recorded', 'finalized', 'disputed', 'corrected', 'voided')),
    ADD CONSTRAINT competition_match_results_dispute_status_allowed
        CHECK (dispute_status IN ('none', 'disputed', 'resolved'));

ALTER TABLE apollo.competition_match_result_sides
    ADD COLUMN competition_match_result_id UUID;

UPDATE apollo.competition_match_result_sides AS side_row
SET competition_match_result_id = result_row.id
FROM apollo.competition_match_results AS result_row
WHERE result_row.competition_match_id = side_row.competition_match_id;

ALTER TABLE apollo.competition_match_result_sides
    ALTER COLUMN competition_match_result_id SET NOT NULL,
    DROP CONSTRAINT competition_match_result_sides_competition_match_id_fkey,
    DROP CONSTRAINT competition_match_result_sides_match_team_unique,
    DROP CONSTRAINT competition_match_result_sides_pkey;

ALTER TABLE apollo.competition_match_results
    DROP CONSTRAINT competition_match_results_pkey,
    ADD CONSTRAINT competition_match_results_pkey PRIMARY KEY (id),
    ADD CONSTRAINT competition_match_results_match_id_id_unique
        UNIQUE (competition_match_id, id),
    ADD CONSTRAINT competition_match_results_supersedes_result_id_fkey
        FOREIGN KEY (supersedes_result_id)
        REFERENCES apollo.competition_match_results(id)
        ON DELETE RESTRICT;

ALTER TABLE apollo.competition_match_result_sides
    ADD CONSTRAINT competition_match_result_sides_pkey
        PRIMARY KEY (competition_match_result_id, side_index),
    ADD CONSTRAINT competition_match_result_sides_result_team_unique
        UNIQUE (competition_match_result_id, competition_session_team_id),
    ADD CONSTRAINT competition_match_result_sides_result_id_fkey
        FOREIGN KEY (competition_match_result_id)
        REFERENCES apollo.competition_match_results(id)
        ON DELETE CASCADE;

ALTER TABLE apollo.competition_matches
    ADD COLUMN result_version INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN canonical_result_id UUID,
    ADD CONSTRAINT competition_matches_result_version_nonnegative
        CHECK (result_version >= 0);

UPDATE apollo.competition_matches AS match_row
SET canonical_result_id = result_row.id,
    result_version = 1
FROM apollo.competition_match_results AS result_row
WHERE result_row.competition_match_id = match_row.id;

ALTER TABLE apollo.competition_matches
    ADD CONSTRAINT competition_matches_canonical_result_id_fkey
        FOREIGN KEY (canonical_result_id)
        REFERENCES apollo.competition_match_results(id)
        ON DELETE SET NULL;

CREATE TABLE apollo.competition_lifecycle_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_session_id UUID NOT NULL REFERENCES apollo.competition_sessions(id) ON DELETE CASCADE,
    competition_match_id UUID REFERENCES apollo.competition_matches(id) ON DELETE CASCADE,
    competition_match_result_id UUID REFERENCES apollo.competition_match_results(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    actor_user_id UUID REFERENCES apollo.users(id) ON DELETE SET NULL,
    actor_role TEXT,
    actor_session_id UUID REFERENCES apollo.sessions(id) ON DELETE SET NULL,
    capability TEXT,
    trusted_surface_key TEXT,
    trusted_surface_label TEXT,
    previous_result_status TEXT,
    result_status TEXT,
    dispute_status TEXT,
    correction_id UUID,
    supersedes_result_id UUID,
    occurred_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT competition_lifecycle_events_event_type_allowed
        CHECK (event_type IN (
            'competition.match.started',
            'competition.result.recorded',
            'competition.result.finalized',
            'competition.result.disputed',
            'competition.result.corrected',
            'competition.result.voided'
        )),
    CONSTRAINT competition_lifecycle_events_result_status_allowed
        CHECK (result_status IS NULL OR result_status IN ('recorded', 'finalized', 'disputed', 'corrected', 'voided')),
    CONSTRAINT competition_lifecycle_events_dispute_status_allowed
        CHECK (dispute_status IS NULL OR dispute_status IN ('none', 'disputed', 'resolved'))
);

CREATE INDEX idx_competition_lifecycle_events_match_occurred_at
    ON apollo.competition_lifecycle_events (competition_match_id, occurred_at DESC, id DESC);

CREATE INDEX idx_competition_lifecycle_events_result_occurred_at
    ON apollo.competition_lifecycle_events (competition_match_result_id, occurred_at DESC, id DESC);

CREATE INDEX idx_competition_lifecycle_events_session_occurred_at
    ON apollo.competition_lifecycle_events (competition_session_id, occurred_at DESC, id DESC);

INSERT INTO apollo.competition_lifecycle_events (
    competition_session_id,
    competition_match_id,
    competition_match_result_id,
    event_type,
    actor_user_id,
    previous_result_status,
    result_status,
    dispute_status,
    occurred_at
)
SELECT match_row.competition_session_id,
       result_row.competition_match_id,
       result_row.id,
       'competition.result.finalized',
       result_row.recorded_by_user_id,
       NULL,
       result_row.result_status,
       result_row.dispute_status,
       result_row.finalized_at
FROM apollo.competition_match_results AS result_row
INNER JOIN apollo.competition_matches AS match_row
  ON match_row.id = result_row.competition_match_id
WHERE result_row.result_status = 'finalized';
