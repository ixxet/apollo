ALTER TABLE apollo.users
    ADD COLUMN role TEXT NOT NULL DEFAULT 'member',
    ADD CONSTRAINT users_role_allowed
        CHECK (role IN ('member', 'supervisor', 'manager', 'owner'));

CREATE TABLE apollo.competition_staff_action_attributions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE RESTRICT,
    actor_role TEXT NOT NULL,
    session_id UUID NOT NULL REFERENCES apollo.sessions(id) ON DELETE RESTRICT,
    capability TEXT NOT NULL,
    trusted_surface_key TEXT NOT NULL,
    trusted_surface_label TEXT,
    action TEXT NOT NULL,
    competition_session_id UUID,
    competition_session_team_id UUID,
    competition_match_id UUID,
    subject_user_id UUID,
    occurred_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT competition_staff_action_attributions_actor_role_required
        CHECK (btrim(actor_role) <> ''),
    CONSTRAINT competition_staff_action_attributions_capability_required
        CHECK (btrim(capability) <> ''),
    CONSTRAINT competition_staff_action_attributions_trusted_surface_key_required
        CHECK (btrim(trusted_surface_key) <> ''),
    CONSTRAINT competition_staff_action_attributions_action_required
        CHECK (btrim(action) <> '')
);

CREATE INDEX idx_competition_staff_action_attributions_session_occurred_at
    ON apollo.competition_staff_action_attributions (competition_session_id, occurred_at DESC, id DESC);

CREATE INDEX idx_competition_staff_action_attributions_actor_occurred_at
    ON apollo.competition_staff_action_attributions (actor_user_id, occurred_at DESC, id DESC);
