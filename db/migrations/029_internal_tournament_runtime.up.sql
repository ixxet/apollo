CREATE TABLE apollo.competition_tournaments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    display_name TEXT NOT NULL,
    format TEXT NOT NULL,
    visibility TEXT NOT NULL DEFAULT 'internal',
    sport_key TEXT NOT NULL REFERENCES apollo.sports(sport_key) ON DELETE RESTRICT,
    facility_key TEXT NOT NULL REFERENCES apollo.facility_catalog_refs(facility_key) ON DELETE RESTRICT,
    zone_key TEXT,
    participants_per_side INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    tournament_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    CONSTRAINT competition_tournaments_display_name_required
        CHECK (btrim(display_name) <> ''),
    CONSTRAINT competition_tournaments_format_allowed
        CHECK (format IN ('single_elimination')),
    CONSTRAINT competition_tournaments_visibility_internal_only
        CHECK (visibility = 'internal'),
    CONSTRAINT competition_tournaments_participants_per_side_positive
        CHECK (participants_per_side > 0),
    CONSTRAINT competition_tournaments_status_allowed
        CHECK (status IN ('draft', 'seeded', 'locked', 'in_progress', 'completed', 'archived')),
    CONSTRAINT competition_tournaments_version_positive
        CHECK (tournament_version > 0),
    FOREIGN KEY (facility_key, zone_key)
        REFERENCES apollo.facility_zone_refs(facility_key, zone_key)
        ON DELETE RESTRICT
);

CREATE UNIQUE INDEX idx_competition_tournaments_owner_sport_name_active
    ON apollo.competition_tournaments (owner_user_id, sport_key, lower(display_name))
    WHERE status <> 'archived';

CREATE INDEX idx_competition_tournaments_created_at
    ON apollo.competition_tournaments (created_at DESC, id DESC);

CREATE TABLE apollo.competition_tournament_brackets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tournament_id UUID NOT NULL REFERENCES apollo.competition_tournaments(id) ON DELETE CASCADE,
    bracket_index INTEGER NOT NULL DEFAULT 1,
    format TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_tournament_brackets_index_positive
        CHECK (bracket_index > 0),
    CONSTRAINT competition_tournament_brackets_format_allowed
        CHECK (format IN ('single_elimination')),
    CONSTRAINT competition_tournament_brackets_status_allowed
        CHECK (status IN ('draft', 'seeded', 'locked', 'in_progress', 'completed', 'archived')),
    CONSTRAINT competition_tournament_brackets_tournament_id_id_unique
        UNIQUE (tournament_id, id),
    UNIQUE (tournament_id, bracket_index)
);

CREATE TABLE apollo.competition_tournament_seeds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tournament_id UUID NOT NULL,
    bracket_id UUID NOT NULL,
    seed INTEGER NOT NULL,
    competition_session_team_id UUID NOT NULL REFERENCES apollo.competition_session_teams(id) ON DELETE RESTRICT,
    seeded_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_tournament_seeds_seed_positive
        CHECK (seed > 0),
    CONSTRAINT competition_tournament_seeds_tournament_bracket_fkey
        FOREIGN KEY (tournament_id, bracket_id)
        REFERENCES apollo.competition_tournament_brackets(tournament_id, id)
        ON DELETE CASCADE,
    UNIQUE (bracket_id, seed),
    UNIQUE (bracket_id, competition_session_team_id)
);

CREATE TABLE apollo.competition_tournament_team_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tournament_id UUID NOT NULL,
    bracket_id UUID NOT NULL,
    tournament_seed_id UUID NOT NULL REFERENCES apollo.competition_tournament_seeds(id) ON DELETE RESTRICT,
    seed INTEGER NOT NULL,
    competition_session_id UUID NOT NULL,
    competition_session_team_id UUID NOT NULL,
    roster_hash TEXT NOT NULL,
    locked_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_tournament_team_snapshots_seed_positive
        CHECK (seed > 0),
    CONSTRAINT competition_tournament_team_snapshots_roster_hash_required
        CHECK (btrim(roster_hash) <> ''),
    CONSTRAINT competition_tournament_team_snapshots_tournament_bracket_fkey
        FOREIGN KEY (tournament_id, bracket_id)
        REFERENCES apollo.competition_tournament_brackets(tournament_id, id)
        ON DELETE CASCADE,
    CONSTRAINT competition_tournament_team_snapshots_session_team_fkey
        FOREIGN KEY (competition_session_id, competition_session_team_id)
        REFERENCES apollo.competition_session_teams(competition_session_id, id)
        ON DELETE RESTRICT,
    UNIQUE (tournament_seed_id),
    UNIQUE (bracket_id, seed),
    UNIQUE (bracket_id, competition_session_team_id)
);

CREATE TABLE apollo.competition_tournament_team_snapshot_members (
    team_snapshot_id UUID NOT NULL REFERENCES apollo.competition_tournament_team_snapshots(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    display_name TEXT NOT NULL,
    slot_index INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_snapshot_id, user_id),
    CONSTRAINT competition_tournament_team_snapshot_members_display_name_required
        CHECK (btrim(display_name) <> ''),
    CONSTRAINT competition_tournament_team_snapshot_members_slot_positive
        CHECK (slot_index > 0),
    UNIQUE (team_snapshot_id, slot_index)
);

CREATE TABLE apollo.competition_tournament_match_bindings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tournament_id UUID NOT NULL,
    bracket_id UUID NOT NULL,
    round INTEGER NOT NULL,
    match_number INTEGER NOT NULL,
    competition_match_id UUID NOT NULL REFERENCES apollo.competition_matches(id) ON DELETE RESTRICT,
    side_one_team_snapshot_id UUID NOT NULL REFERENCES apollo.competition_tournament_team_snapshots(id) ON DELETE RESTRICT,
    side_two_team_snapshot_id UUID NOT NULL REFERENCES apollo.competition_tournament_team_snapshots(id) ON DELETE RESTRICT,
    bound_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_tournament_match_bindings_round_positive
        CHECK (round > 0),
    CONSTRAINT competition_tournament_match_bindings_match_number_positive
        CHECK (match_number > 0),
    CONSTRAINT competition_tournament_match_bindings_distinct_snapshots
        CHECK (side_one_team_snapshot_id <> side_two_team_snapshot_id),
    CONSTRAINT competition_tournament_match_bindings_tournament_bracket_fkey
        FOREIGN KEY (tournament_id, bracket_id)
        REFERENCES apollo.competition_tournament_brackets(tournament_id, id)
        ON DELETE CASCADE,
    UNIQUE (bracket_id, round, match_number),
    UNIQUE (bracket_id, competition_match_id)
);

CREATE TABLE apollo.competition_tournament_advancements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tournament_id UUID NOT NULL,
    bracket_id UUID NOT NULL,
    match_binding_id UUID NOT NULL REFERENCES apollo.competition_tournament_match_bindings(id) ON DELETE RESTRICT,
    round INTEGER NOT NULL,
    winning_team_snapshot_id UUID NOT NULL REFERENCES apollo.competition_tournament_team_snapshots(id) ON DELETE RESTRICT,
    losing_team_snapshot_id UUID NOT NULL REFERENCES apollo.competition_tournament_team_snapshots(id) ON DELETE RESTRICT,
    competition_match_id UUID NOT NULL REFERENCES apollo.competition_matches(id) ON DELETE RESTRICT,
    canonical_result_id UUID NOT NULL REFERENCES apollo.competition_match_results(id) ON DELETE RESTRICT,
    advance_reason TEXT NOT NULL,
    advanced_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_tournament_advancements_round_positive
        CHECK (round > 0),
    CONSTRAINT competition_tournament_advancements_reason_allowed
        CHECK (advance_reason IN ('canonical_result_win')),
    CONSTRAINT competition_tournament_advancements_distinct_snapshots
        CHECK (winning_team_snapshot_id <> losing_team_snapshot_id),
    CONSTRAINT competition_tournament_advancements_tournament_bracket_fkey
        FOREIGN KEY (tournament_id, bracket_id)
        REFERENCES apollo.competition_tournament_brackets(tournament_id, id)
        ON DELETE CASCADE,
    UNIQUE (match_binding_id)
);

CREATE TABLE apollo.competition_tournament_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tournament_id UUID NOT NULL REFERENCES apollo.competition_tournaments(id) ON DELETE CASCADE,
    bracket_id UUID REFERENCES apollo.competition_tournament_brackets(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    tournament_seed_id UUID REFERENCES apollo.competition_tournament_seeds(id) ON DELETE SET NULL,
    team_snapshot_id UUID REFERENCES apollo.competition_tournament_team_snapshots(id) ON DELETE SET NULL,
    match_binding_id UUID REFERENCES apollo.competition_tournament_match_bindings(id) ON DELETE SET NULL,
    round INTEGER,
    seed INTEGER,
    advance_reason TEXT,
    competition_session_team_id UUID REFERENCES apollo.competition_session_teams(id) ON DELETE SET NULL,
    competition_match_id UUID REFERENCES apollo.competition_matches(id) ON DELETE SET NULL,
    canonical_result_id UUID REFERENCES apollo.competition_match_results(id) ON DELETE SET NULL,
    actor_user_id UUID REFERENCES apollo.users(id) ON DELETE SET NULL,
    actor_role TEXT,
    actor_session_id UUID REFERENCES apollo.sessions(id) ON DELETE SET NULL,
    capability TEXT,
    trusted_surface_key TEXT,
    trusted_surface_label TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_tournament_events_type_allowed
        CHECK (event_type IN (
            'competition.tournament.created',
            'competition.tournament.seeded',
            'competition.tournament.team_locked',
            'competition.tournament.match_bound',
            'competition.tournament.round_advanced'
        )),
    CONSTRAINT competition_tournament_events_round_positive
        CHECK (round IS NULL OR round > 0),
    CONSTRAINT competition_tournament_events_seed_positive
        CHECK (seed IS NULL OR seed > 0),
    CONSTRAINT competition_tournament_events_advance_reason_allowed
        CHECK (advance_reason IS NULL OR advance_reason IN ('canonical_result_win'))
);

CREATE INDEX idx_competition_tournament_events_tournament_occurred_at
    ON apollo.competition_tournament_events (tournament_id, occurred_at DESC, id DESC);

CREATE INDEX idx_competition_tournament_events_match_binding
    ON apollo.competition_tournament_events (match_binding_id, event_type);

CREATE OR REPLACE FUNCTION apollo.reject_competition_tournament_fact_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'competition tournament facts are immutable once recorded';
END;
$$;

CREATE TRIGGER competition_tournament_seeds_immutable
    BEFORE UPDATE ON apollo.competition_tournament_seeds
    FOR EACH ROW
    EXECUTE FUNCTION apollo.reject_competition_tournament_fact_update();

CREATE TRIGGER competition_tournament_team_snapshots_immutable
    BEFORE UPDATE ON apollo.competition_tournament_team_snapshots
    FOR EACH ROW
    EXECUTE FUNCTION apollo.reject_competition_tournament_fact_update();

CREATE TRIGGER competition_tournament_team_snapshot_members_immutable
    BEFORE UPDATE ON apollo.competition_tournament_team_snapshot_members
    FOR EACH ROW
    EXECUTE FUNCTION apollo.reject_competition_tournament_fact_update();

CREATE TRIGGER competition_tournament_match_bindings_immutable
    BEFORE UPDATE ON apollo.competition_tournament_match_bindings
    FOR EACH ROW
    EXECUTE FUNCTION apollo.reject_competition_tournament_fact_update();

CREATE TRIGGER competition_tournament_advancements_immutable
    BEFORE UPDATE ON apollo.competition_tournament_advancements
    FOR EACH ROW
    EXECUTE FUNCTION apollo.reject_competition_tournament_fact_update();

CREATE TRIGGER competition_tournament_events_immutable
    BEFORE UPDATE ON apollo.competition_tournament_events
    FOR EACH ROW
    EXECUTE FUNCTION apollo.reject_competition_tournament_fact_update();
