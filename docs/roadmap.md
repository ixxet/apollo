# apollo Roadmap

## Objective

Keep APOLLO moving through narrow member-facing tracers instead of broad app
builds.

## Current Narrow Slice

- member registration and passwordless sign-in start from student ID + email
- email verification tokens are stored hashed, expired, and single-use
- successful verification issues a signed server-backed session cookie
- authenticated profile reads and writes persist `visibility_mode` and
  `availability_mode`
- visit history remains separate from auth and profile state
- workouts, recommendations, and matchmaking stay deferred

## Boundaries

- no full PWA build in this tracer
- no workout runtime in this tracer
- no recommendation engine until workout data exists
- no automatic lobby entry from tap-in events
- no matchmaking lobby until user state and activity data are stable

## Exit Criteria

- one member can start verification with student ID + email
- one member can verify ownership through a real token lifecycle
- one member can receive and use a signed session cookie
- one member can read and update persisted privacy and availability settings
- visit recording still stays separate from workouts and matchmaking intent

## Tracer Ownership

- `Tracer 2`: consume ATHENA-backed presence to create visit records
- `Tracer 3`: member auth -> profile -> privacy and availability state
- `Tracer 4`: lobby eligibility from explicit availability, not tap-in

## Current State

Tracer 3 now owns the first real member-account and profile-state slice:

- `POST /api/v1/auth/verification/start` creates or reuses the correct member
  record without touching tag linkage
- verification tokens are persisted hashed in Postgres, expire cleanly, and are
  rejected after use
- `GET/POST /api/v1/auth/verify` verifies ownership and issues a signed
  `HTTPOnly`, `Secure`, `SameSite=Strict` session cookie backed by Postgres
- `GET/PATCH /api/v1/profile` now persists `visibility_mode` and
  `availability_mode` through `users.preferences`
- visit recording is still separate from auth, profile state, workouts, and
  matchmaking intent
- visit closing, workouts, recommendations, and lobby behavior remain deferred
