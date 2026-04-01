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
2. email verification for account activation
3. signed `HTTPOnly`, `Secure`, `SameSite=Strict` session cookies for ongoing auth

OAuth or SSO may be added later if a real provider becomes available.

## Important Boundary

Authentication and identity linkage are not the same thing:

- auth proves who owns the APOLLO account
- claimed tags or similar identifiers link ATHENA presence to that account

## Consequences

- APOLLO can operate entirely on self-hosted infrastructure
- member auth stays simple enough for the first implementation wave
- the platform avoids premature dependence on third-party or institutional auth
