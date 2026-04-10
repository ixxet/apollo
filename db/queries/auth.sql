-- name: CreateEmailVerificationToken :one
INSERT INTO apollo.email_verification_tokens (
  user_id,
  email,
  token_hash,
  expires_at
)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, email, token_hash, expires_at, used_at, created_at;

-- name: CreateSession :one
INSERT INTO apollo.sessions (
  user_id,
  expires_at
)
VALUES ($1, $2)
RETURNING id, user_id, expires_at, revoked_at, created_at;

-- name: SetUserRole :one
UPDATE apollo.users
SET role = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING id, student_id, display_name, email, role, preferences, created_at, updated_at, email_verified_at;

-- name: CreateUser :one
INSERT INTO apollo.users (
  student_id,
  display_name,
  email
)
VALUES ($1, $2, $3)
RETURNING id, student_id, display_name, email, role, preferences, created_at, updated_at, email_verified_at;

-- name: DeletePendingEmailVerificationTokensByUserID :exec
DELETE FROM apollo.email_verification_tokens
WHERE user_id = $1
  AND used_at IS NULL;

-- name: GetEmailVerificationTokenByHash :one
SELECT id, user_id, email, token_hash, expires_at, used_at, created_at
FROM apollo.email_verification_tokens
WHERE token_hash = $1
LIMIT 1;

-- name: GetSessionByID :one
SELECT id, user_id, expires_at, revoked_at, created_at
FROM apollo.sessions
WHERE id = $1
LIMIT 1;

-- name: GetSessionPrincipalByID :one
SELECT s.id,
       s.user_id,
       u.role,
       s.expires_at,
       s.revoked_at,
       s.created_at
FROM apollo.sessions AS s
INNER JOIN apollo.users AS u
  ON u.id = s.user_id
WHERE s.id = $1
LIMIT 1;

-- name: GetUserByEmail :one
SELECT id, student_id, display_name, email, role, preferences, created_at, updated_at, email_verified_at
FROM apollo.users
WHERE email = $1
LIMIT 1;

-- name: GetUserByID :one
SELECT id, student_id, display_name, email, role, preferences, created_at, updated_at, email_verified_at
FROM apollo.users
WHERE id = $1
LIMIT 1;

-- name: GetUserByStudentID :one
SELECT id, student_id, display_name, email, role, preferences, created_at, updated_at, email_verified_at
FROM apollo.users
WHERE student_id = $1
LIMIT 1;

-- name: MarkEmailVerificationTokenUsed :one
UPDATE apollo.email_verification_tokens
SET used_at = $2
WHERE token_hash = $1
  AND used_at IS NULL
  AND expires_at > $2
RETURNING id, user_id, email, token_hash, expires_at, used_at, created_at;

-- name: MarkUserEmailVerified :one
UPDATE apollo.users
SET email_verified_at = COALESCE(email_verified_at, $2),
    updated_at = $2
WHERE id = $1
RETURNING id, student_id, display_name, email, role, preferences, created_at, updated_at, email_verified_at;

-- name: RevokeSession :exec
UPDATE apollo.sessions
SET revoked_at = $2
WHERE id = $1
  AND revoked_at IS NULL;

-- name: UpdateUserPreferences :one
UPDATE apollo.users
SET preferences = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING id, student_id, display_name, email, role, preferences, created_at, updated_at, email_verified_at;
