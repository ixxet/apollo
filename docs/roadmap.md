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
- authenticated lobby eligibility reads derive `eligible` and a machine-readable
  `reason` from persisted `visibility_mode` and `availability_mode`
- visit history now supports deterministic open and close behavior while
  remaining separate from auth and profile state
- workouts, recommendations, lobby membership, and matchmaking stay deferred

## Boundaries

- no full PWA build in this tracer
- no workout runtime in this tracer
- no recommendation engine until workout data exists
- no automatic lobby entry from tap-in events
- no lobby membership persistence or matchmaking until user state and activity
  data are stable

## Exit Criteria

- one member can start verification with student ID + email
- one member can verify ownership through a real token lifecycle
- one member can receive and use a signed session cookie
- one member can read and update persisted privacy and availability settings
- one authenticated member can read deterministic lobby eligibility derived from
  those persisted settings
- visit recording still stays separate from workouts and lobby intent

## Tracer Ownership

- `Tracer 2`: consume ATHENA-backed presence to create visit records
- `Tracer 3`: member auth -> profile -> privacy and availability state
- `Tracer 4`: lobby eligibility from explicit availability, not tap-in
- `Tracer 5`: close the correct open visit from ATHENA departure truth

## Current State

Tracer 5 now completes the first full APOLLO visit lifecycle slice:

- `POST /api/v1/auth/verification/start` creates or reuses the correct member
  record without touching tag linkage
- verification tokens are persisted hashed in Postgres, expire cleanly, and are
  rejected after use
- `GET/POST /api/v1/auth/verify` verifies ownership and issues a signed
  `HTTPOnly`, `Secure`, `SameSite=Strict` session cookie backed by Postgres
- `GET/PATCH /api/v1/profile` now persists `visibility_mode` and
  `availability_mode` through `users.preferences`
- `GET /api/v1/lobby/eligibility` now derives open-lobby eligibility from
  explicit member state only and never from tap-in or visit history
- Ghost Mode is explicit: `ghost + available_now` remains ineligible for the
  open lobby
- departures close the matching open visit for the same member and facility
  without reopening history, creating workouts, or mutating member intent
- duplicate and no-open departures resolve deterministically
- visit lifecycle is still separate from auth, profile state, workouts, and
  lobby or matchmaking intent
- Milestone 1.5 now proves the bounded live deployment can bootstrap APOLLO,
  consume the identified arrival subject in-cluster, and persist the visit
  without widening into broader product runtime
- workouts, recommendations, lobby membership, and matchmaking remain deferred
