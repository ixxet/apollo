CREATE TABLE apollo.competition_safety_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_session_id UUID REFERENCES apollo.competition_sessions(id) ON DELETE RESTRICT,
    competition_match_id UUID REFERENCES apollo.competition_matches(id) ON DELETE RESTRICT,
    competition_session_team_id UUID REFERENCES apollo.competition_session_teams(id) ON DELETE RESTRICT,
    competition_tournament_id UUID REFERENCES apollo.competition_tournaments(id) ON DELETE RESTRICT,
    reporter_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    subject_user_id UUID REFERENCES apollo.users(id) ON DELETE RESTRICT,
    target_type TEXT NOT NULL,
    target_id UUID NOT NULL,
    reason_code TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    privacy_scope TEXT NOT NULL DEFAULT 'manager_internal',
    note TEXT,
    actor_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    actor_role TEXT NOT NULL,
    actor_session_id UUID NOT NULL REFERENCES apollo.sessions(id) ON DELETE RESTRICT,
    capability TEXT NOT NULL,
    trusted_surface_key TEXT NOT NULL,
    trusted_surface_label TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_safety_reports_target_type_allowed
        CHECK (target_type IN ('competition_session', 'competition_match', 'competition_team', 'competition_tournament', 'competition_member')),
    CONSTRAINT competition_safety_reports_reason_code_allowed
        CHECK (reason_code IN ('conduct', 'harassment', 'unsafe_play', 'no_show', 'reliability', 'eligibility', 'other')),
    CONSTRAINT competition_safety_reports_status_allowed
        CHECK (status = 'open'),
    CONSTRAINT competition_safety_reports_privacy_scope_internal
        CHECK (privacy_scope = 'manager_internal'),
    CONSTRAINT competition_safety_reports_actor_role_required
        CHECK (btrim(actor_role) <> ''),
    CONSTRAINT competition_safety_reports_capability_required
        CHECK (btrim(capability) <> ''),
    CONSTRAINT competition_safety_reports_trusted_surface_key_required
        CHECK (btrim(trusted_surface_key) <> ''),
    CONSTRAINT competition_safety_reports_note_not_blank
        CHECK (note IS NULL OR btrim(note) <> ''),
    CONSTRAINT competition_safety_reports_target_shape
        CHECK (
            (target_type = 'competition_session' AND competition_session_id IS NOT NULL AND competition_match_id IS NULL AND competition_session_team_id IS NULL AND competition_tournament_id IS NULL AND target_id = competition_session_id) OR
            (target_type = 'competition_match' AND competition_session_id IS NOT NULL AND competition_match_id IS NOT NULL AND competition_session_team_id IS NULL AND competition_tournament_id IS NULL AND target_id = competition_match_id) OR
            (target_type = 'competition_team' AND competition_session_id IS NOT NULL AND competition_match_id IS NULL AND competition_session_team_id IS NOT NULL AND competition_tournament_id IS NULL AND target_id = competition_session_team_id) OR
            (target_type = 'competition_tournament' AND competition_session_id IS NULL AND competition_match_id IS NULL AND competition_session_team_id IS NULL AND competition_tournament_id IS NOT NULL AND target_id = competition_tournament_id) OR
            (target_type = 'competition_member' AND competition_session_id IS NOT NULL AND competition_match_id IS NULL AND competition_session_team_id IS NULL AND competition_tournament_id IS NULL AND subject_user_id IS NOT NULL AND target_id = subject_user_id)
        )
);

CREATE INDEX idx_competition_safety_reports_session_occurred_at
    ON apollo.competition_safety_reports (competition_session_id, occurred_at DESC, id DESC)
    WHERE competition_session_id IS NOT NULL;

CREATE INDEX idx_competition_safety_reports_tournament_occurred_at
    ON apollo.competition_safety_reports (competition_tournament_id, occurred_at DESC, id DESC)
    WHERE competition_tournament_id IS NOT NULL;

CREATE INDEX idx_competition_safety_reports_subject_occurred_at
    ON apollo.competition_safety_reports (subject_user_id, occurred_at DESC, id DESC)
    WHERE subject_user_id IS NOT NULL;

CREATE TABLE apollo.competition_safety_blocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_session_id UUID NOT NULL REFERENCES apollo.competition_sessions(id) ON DELETE RESTRICT,
    competition_match_id UUID REFERENCES apollo.competition_matches(id) ON DELETE RESTRICT,
    blocker_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    blocked_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    reason_code TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    privacy_scope TEXT NOT NULL DEFAULT 'manager_internal',
    actor_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    actor_role TEXT NOT NULL,
    actor_session_id UUID NOT NULL REFERENCES apollo.sessions(id) ON DELETE RESTRICT,
    capability TEXT NOT NULL,
    trusted_surface_key TEXT NOT NULL,
    trusted_surface_label TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_safety_blocks_distinct_members
        CHECK (blocker_user_id <> blocked_user_id),
    CONSTRAINT competition_safety_blocks_reason_code_allowed
        CHECK (reason_code IN ('conduct', 'harassment', 'unsafe_play', 'no_show', 'reliability', 'eligibility', 'other')),
    CONSTRAINT competition_safety_blocks_status_allowed
        CHECK (status = 'active'),
    CONSTRAINT competition_safety_blocks_privacy_scope_internal
        CHECK (privacy_scope = 'manager_internal'),
    CONSTRAINT competition_safety_blocks_actor_role_required
        CHECK (btrim(actor_role) <> ''),
    CONSTRAINT competition_safety_blocks_capability_required
        CHECK (btrim(capability) <> ''),
    CONSTRAINT competition_safety_blocks_trusted_surface_key_required
        CHECK (btrim(trusted_surface_key) <> '')
);

CREATE UNIQUE INDEX idx_competition_safety_blocks_active_pair
    ON apollo.competition_safety_blocks (competition_session_id, blocker_user_id, blocked_user_id)
    WHERE status = 'active';

CREATE INDEX idx_competition_safety_blocks_blocked_occurred_at
    ON apollo.competition_safety_blocks (blocked_user_id, occurred_at DESC, id DESC);

CREATE TABLE apollo.competition_reliability_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_session_id UUID NOT NULL REFERENCES apollo.competition_sessions(id) ON DELETE RESTRICT,
    competition_match_id UUID REFERENCES apollo.competition_matches(id) ON DELETE RESTRICT,
    subject_user_id UUID REFERENCES apollo.users(id) ON DELETE RESTRICT,
    reliability_type TEXT NOT NULL,
    severity TEXT NOT NULL,
    privacy_scope TEXT NOT NULL DEFAULT 'manager_internal',
    note TEXT,
    actor_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    actor_role TEXT NOT NULL,
    actor_session_id UUID NOT NULL REFERENCES apollo.sessions(id) ON DELETE RESTRICT,
    capability TEXT NOT NULL,
    trusted_surface_key TEXT NOT NULL,
    trusted_surface_label TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_reliability_events_type_allowed
        CHECK (reliability_type IN ('late_arrival', 'no_show', 'forfeit', 'disconnect', 'ops_delay', 'equipment_issue', 'unsafe_interruption', 'other')),
    CONSTRAINT competition_reliability_events_severity_allowed
        CHECK (severity IN ('info', 'warning', 'critical')),
    CONSTRAINT competition_reliability_events_privacy_scope_internal
        CHECK (privacy_scope = 'manager_internal'),
    CONSTRAINT competition_reliability_events_actor_role_required
        CHECK (btrim(actor_role) <> ''),
    CONSTRAINT competition_reliability_events_capability_required
        CHECK (btrim(capability) <> ''),
    CONSTRAINT competition_reliability_events_trusted_surface_key_required
        CHECK (btrim(trusted_surface_key) <> ''),
    CONSTRAINT competition_reliability_events_note_not_blank
        CHECK (note IS NULL OR btrim(note) <> '')
);

CREATE INDEX idx_competition_reliability_events_session_occurred_at
    ON apollo.competition_reliability_events (competition_session_id, occurred_at DESC, id DESC);

CREATE INDEX idx_competition_reliability_events_subject_occurred_at
    ON apollo.competition_reliability_events (subject_user_id, occurred_at DESC, id DESC)
    WHERE subject_user_id IS NOT NULL;

CREATE TABLE apollo.competition_safety_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type TEXT NOT NULL,
    competition_session_id UUID REFERENCES apollo.competition_sessions(id) ON DELETE RESTRICT,
    competition_match_id UUID REFERENCES apollo.competition_matches(id) ON DELETE RESTRICT,
    competition_session_team_id UUID REFERENCES apollo.competition_session_teams(id) ON DELETE RESTRICT,
    competition_tournament_id UUID REFERENCES apollo.competition_tournaments(id) ON DELETE RESTRICT,
    safety_report_id UUID REFERENCES apollo.competition_safety_reports(id) ON DELETE RESTRICT,
    safety_block_id UUID REFERENCES apollo.competition_safety_blocks(id) ON DELETE RESTRICT,
    reliability_event_id UUID REFERENCES apollo.competition_reliability_events(id) ON DELETE RESTRICT,
    reporter_user_id UUID REFERENCES apollo.users(id) ON DELETE RESTRICT,
    subject_user_id UUID REFERENCES apollo.users(id) ON DELETE RESTRICT,
    blocker_user_id UUID REFERENCES apollo.users(id) ON DELETE RESTRICT,
    blocked_user_id UUID REFERENCES apollo.users(id) ON DELETE RESTRICT,
    target_type TEXT,
    target_id UUID,
    reason_code TEXT,
    reliability_type TEXT,
    severity TEXT,
    privacy_scope TEXT NOT NULL DEFAULT 'manager_internal',
    actor_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    actor_role TEXT NOT NULL,
    actor_session_id UUID NOT NULL REFERENCES apollo.sessions(id) ON DELETE RESTRICT,
    capability TEXT NOT NULL,
    trusted_surface_key TEXT NOT NULL,
    trusted_surface_label TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT competition_safety_events_type_allowed
        CHECK (event_type IN (
            'competition.safety.report_recorded',
            'competition.safety.block_recorded',
            'competition.reliability.event_recorded'
        )),
    CONSTRAINT competition_safety_events_privacy_scope_internal
        CHECK (privacy_scope = 'manager_internal'),
    CONSTRAINT competition_safety_events_actor_role_required
        CHECK (btrim(actor_role) <> ''),
    CONSTRAINT competition_safety_events_capability_required
        CHECK (btrim(capability) <> ''),
    CONSTRAINT competition_safety_events_trusted_surface_key_required
        CHECK (btrim(trusted_surface_key) <> ''),
    CONSTRAINT competition_safety_events_shape
        CHECK (
            (event_type = 'competition.safety.report_recorded' AND safety_report_id IS NOT NULL AND safety_block_id IS NULL AND reliability_event_id IS NULL) OR
            (event_type = 'competition.safety.block_recorded' AND safety_report_id IS NULL AND safety_block_id IS NOT NULL AND reliability_event_id IS NULL) OR
            (event_type = 'competition.reliability.event_recorded' AND safety_report_id IS NULL AND safety_block_id IS NULL AND reliability_event_id IS NOT NULL)
        )
);

CREATE INDEX idx_competition_safety_events_session_occurred_at
    ON apollo.competition_safety_events (competition_session_id, occurred_at DESC, id DESC)
    WHERE competition_session_id IS NOT NULL;

CREATE INDEX idx_competition_safety_events_tournament_occurred_at
    ON apollo.competition_safety_events (competition_tournament_id, occurred_at DESC, id DESC)
    WHERE competition_tournament_id IS NOT NULL;

CREATE INDEX idx_competition_safety_events_occurred_at
    ON apollo.competition_safety_events (occurred_at DESC, id DESC);

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
