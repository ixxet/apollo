DROP INDEX IF EXISTS apollo.idx_competition_staff_action_attributions_actor_occurred_at;

DROP INDEX IF EXISTS apollo.idx_competition_staff_action_attributions_session_occurred_at;

DROP TABLE IF EXISTS apollo.competition_staff_action_attributions;

ALTER TABLE apollo.users
    DROP CONSTRAINT IF EXISTS users_role_allowed,
    DROP COLUMN IF EXISTS role;
