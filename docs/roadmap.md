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
- authenticated workout create, update, finish, detail, and history reads are
  now real while staying separate from visits and member intent
- recommendations, lobby membership, and matchmaking stay deferred

## Boundaries

- no full PWA build in this tracer
- no recommendation engine until workout data exists
- no workout inference from arrivals, departures, or visits
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
- one authenticated member can create, update, finish, read, and list workout
  history without touching visits or profile intent
- visit recording still stays separate from workouts and lobby intent

## Tracer Ownership

- `Tracer 2`: consume ATHENA-backed presence to create visit records
- `Tracer 3`: member auth -> profile -> privacy and availability state
- `Tracer 4`: lobby eligibility from explicit availability, not tap-in
- `Tracer 5`: close the correct open visit from ATHENA departure truth
- `Tracer 6`: explicit workout runtime without visit-derived workout inference

## Current State

Tracer 6 now completes APOLLO's first workout-history runtime slice:

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
- `POST /api/v1/workouts` creates one member-owned `in_progress` workout
- `PUT /api/v1/workouts/{id}` replaces ordered exercise data while the workout
  is still mutable
- `POST /api/v1/workouts/{id}/finish` explicitly finishes a non-empty workout
- `GET /api/v1/workouts` and `GET /api/v1/workouts/{id}` now serve workout
  history back through the authenticated runtime, with list ordering fixed to
  newest workout created first
- one member can own many finished workouts, but only one `in_progress`
  workout at a time
- workout runtime is still separate from auth, profile state, visits, and
  lobby or matchmaking intent
- Milestone 1.5 now proves the bounded live deployment can bootstrap APOLLO,
  consume the identified arrival subject in-cluster, and persist the visit
  without widening into broader product runtime
- workout runtime is proven locally; deployed truth is still unchanged from
  Milestone 1.5 and does not claim live in-cluster workout surfaces
- recommendations, lobby membership, and matchmaking remain deferred
