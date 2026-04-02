DROP INDEX IF EXISTS apollo.idx_sessions_user_created_at;
DROP TABLE IF EXISTS apollo.sessions;

DROP INDEX IF EXISTS apollo.idx_email_verification_tokens_user_created_at;
DROP TABLE IF EXISTS apollo.email_verification_tokens;

ALTER TABLE apollo.users
    DROP COLUMN IF EXISTS email_verified_at;
