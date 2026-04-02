# ADR 002: Member Authentication Strategy

## Status

Accepted.

## Context

APOLLO needs a low-friction member auth path for a student-facing PWA. The
project does not have a reliable institutional OAuth provider, and adding
third-party auth early would create external dependency and onboarding friction.

## Decision

Use:

1. student ID and email for registration
2. email verification for account activation and passwordless sign-in
3. hashed verification-token persistence with expiry and single-use semantics
4. server-side session rows in Postgres
5. signed `HTTPOnly`, `Secure`, `SameSite=Strict` cookies that carry the
   session identifier, not a standalone JWT

Local development can surface verification tokens through explicit dev logging.
This tracer does not add SMTP dependency or a public token-inspection API.

OAuth or SSO may be added later if a real provider becomes available.

## Important Boundary

Authentication and identity linkage are not the same thing:

- auth proves who owns the APOLLO account
- claimed tags or similar identifiers link ATHENA presence to that account

## Consequences

- APOLLO can operate entirely on self-hosted infrastructure
- member auth stays simple enough for the first implementation wave while still
  being real
- session invalidation stays straightforward because active auth state is
  server-side
- the platform avoids premature dependence on third-party or institutional auth
- local smoke needs explicit handling for `Secure` cookies when testing over
  plain HTTP
