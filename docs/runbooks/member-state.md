# APOLLO Member State Runbook

## Purpose

Use this runbook when implementing member auth, profile state, visits, workouts, and later lobby eligibility.

## Rules

- auth is separate from identity linkage
- verification tokens are separate from sessions
- visits are separate from workouts
- presence is separate from matchmaking intent
- identified-arrival consumers must use the shared `ashton-proto` runtime
  contract instead of private wire structs
- `users.preferences` holds flexible member-intent state early
- verification tokens must be stored hashed, expired, and single-use
- session cookies stay signed, `HTTPOnly`, `Secure`, and `SameSite=Strict`
- session state stays server-side; APOLLO is not JWT-first in this tracer
- open-lobby eligibility derives from persisted member state, not visits
- availability is evaluated before visibility so ineligible reasons stay stable
- `ghost + available_now` remains ineligible for the open lobby
- duplicate arrival delivery must be idempotent
- duplicate departure delivery must be idempotent
- unknown and inactive tags must not create visits
- anonymous visit events are ignored
- departures close visits by exact member + facility match only
- departures with no matching open visit must be deterministic no-ops
- departures older than the open visit arrival must not backdate or corrupt
  visit history
- visit closing must not mutate `users.preferences`, claimed tags, workouts, or
  derived eligibility state
- workouts are explicit authenticated member records, not inferred from visits
- only one `in_progress` workout is allowed per member
- finished workouts are immutable in the current runtime
- workout writes must not mutate visits, claimed tags, or `users.preferences`
- workout writes must not change lobby eligibility indirectly
- workout recommendation reads are authenticated and owner-scoped
- workout recommendation precedence is explicit:
  `resume_in_progress_workout`, `start_first_workout`, `recovery_day` for
  `finished_at` values inside `24h`, then `repeat_last_finished_workout`
- workout recommendation reads must not create, update, or finish workouts
- workout recommendation reads must not mutate visits, claimed tags,
  `users.preferences`, or derived eligibility state

## Required Checks

- invalid or missing auth state is rejected clearly
- invalid, expired, and reused verification tokens are rejected clearly
- tampered, expired, and revoked session cookies are rejected clearly
- ghost mode does not imply lobby entry
- unavailable members never join the lobby
- `with_team` members stay out of the open lobby
- invalid persisted eligibility enums return deterministic ineligible reasons
- profile writes only mutate `visibility_mode` and `availability_mode`
- eligibility reads only observe profile state and must not mutate visits,
  workouts, or claimed tags
- profile writes must not mutate visits, workouts, or claimed tags
- workout create, update, finish, detail, and list all require a valid session
- workout reads and writes are owner-scoped
- workout history is ordered by newest workout creation first using
  `started_at DESC, id DESC`
- workout finish is rejected when the workout has no exercise rows
- one member cannot create a second `in_progress` workout
- recommendation reads ignore visit rows and profile state as recommendation
  inputs in the current tracer
- visit creation never creates or starts a workout implicitly
- visit closing never creates a workout implicitly
- visit closing never finishes an `in_progress` workout
- visit closing never mutates claimed tags or profile state
- duplicate delivery does not create a second visit
- malformed presence events are rejected clearly
- producer-consumer compatibility is proven with shared helper or fixture bytes,
  not hand-written JSON strings copied into APOLLO tests

## Local Smoke Notes

- `APOLLO_LOG_VERIFICATION_TOKENS=true` is the intended local dev path for
  observing verification tokens without adding a public inspection API
- the current verified local smoke path on this machine uses a curl cookie jar
  against `127.0.0.1` successfully even with `Secure` cookies enabled

### Verified Workout Runtime Smoke

```bash
docker run -d --rm --name tracer6-apollo-smoke \
  -e POSTGRES_USER=apollo \
  -e POSTGRES_PASSWORD=apollo \
  -e POSTGRES_DB=apollo \
  -p 55436:5432 \
  postgres:16-alpine

cd /Users/zizo/Personal-Projects/ASHTON/apollo
APOLLO_DATABASE_URL='postgres://apollo:apollo@127.0.0.1:55436/apollo?sslmode=disable' \
  go run ./cmd/apollo migrate up

APOLLO_DATABASE_URL='postgres://apollo:apollo@127.0.0.1:55436/apollo?sslmode=disable' \
APOLLO_HTTP_ADDR='127.0.0.1:18085' \
APOLLO_SESSION_COOKIE_SECRET='0123456789abcdef0123456789abcdef' \
APOLLO_SESSION_COOKIE_SECURE='true' \
APOLLO_LOG_VERIFICATION_TOKENS='true' \
  go run ./cmd/apollo serve

curl -sS -i http://127.0.0.1:18085/api/v1/health
curl -sS -i -X POST http://127.0.0.1:18085/api/v1/auth/verification/start \
  -H 'Content-Type: application/json' \
  --data '{"student_id":"student-smoke-002","email":"smoke-002@example.com"}'

curl -sS -c /tmp/apollo-smoke.cookies \
  'http://127.0.0.1:18085/api/v1/auth/verify?token=<token from server log>'

curl -sS -b /tmp/apollo-smoke.cookies \
  -H 'Content-Type: application/json' \
  -X POST http://127.0.0.1:18085/api/v1/workouts \
  --data '{"notes":"smoke workout"}'

curl -sS -b /tmp/apollo-smoke.cookies \
  -H 'Content-Type: application/json' \
  -X PUT http://127.0.0.1:18085/api/v1/workouts/<workout-id> \
  --data '{"notes":"smoke workout updated","exercises":[{"name":"bench press","sets":3,"reps":8,"weight_kg":84.5,"rpe":8.5},{"name":"row","sets":3,"reps":10}]}'

curl -sS -b /tmp/apollo-smoke.cookies \
  http://127.0.0.1:18085/api/v1/workouts/<workout-id>

curl -sS -b /tmp/apollo-smoke.cookies \
  -X POST http://127.0.0.1:18085/api/v1/workouts/<workout-id>/finish

curl -sS -b /tmp/apollo-smoke.cookies \
  http://127.0.0.1:18085/api/v1/workouts

docker exec -i tracer6-apollo-smoke psql -U apollo -d apollo -c \
  "SELECT count(*) AS visits FROM apollo.visits; SELECT count(*) AS workouts FROM apollo.workouts; SELECT preferences::text FROM apollo.users WHERE student_id = 'student-smoke-002';"
```

Expected smoke outcomes:
- health returns `200 OK`
- verification start returns `202 Accepted`
- verify returns `{"status":"verified"}`
- workout create returns `status="in_progress"`
- workout update returns ordered exercise rows
- workout finish returns `status="finished"` with `finished_at`
- workout list returns workouts newest created first
- the server logs `workout created`, `workout updated`, and `workout finished`
  with `user_id` and `workout_id`
- `apollo.visits` remains `0` for the smoke user
- `users.preferences` stays at the default ghost/unavailable state

### Tracer 6 Hardening Commands

Use these exact commands when rerunning the workout-runtime closure checks:

```bash
cd /Users/zizo/Personal-Projects/ASHTON/apollo

go test ./...
go test -count=2 ./...
go test -count=5 ./internal/workouts -run '^(TestRepositoryCreateWorkoutRejectsSecondInProgressWorkoutForSameUser|TestRepositoryFinishWorkoutTransitionsOnceAndLeavesFinishedRowsReadOnly|TestRepositoryListWorkoutsOrdersByNewestStartedAtDespiteFinishedTimestampSkew|TestRepositoryListWorkoutsUsesStableTieBreakerWhenStartedAtMatches|TestUpdateWorkoutValidatesExercisePayloadWithTableDrivenCoverage|TestFinishWorkoutRejectsMissingAndFinishedStatesAndUsesClock)$'
go test -count=10 ./internal/server -run '^TestWorkoutRuntimeListsNewestWorkoutFirst$'
go test -count=5 ./internal/server -run '^(TestWorkoutRuntimeListsNewestWorkoutFirst|TestWorkoutEndpointsEmitLifecycleLogsOnSuccessPaths|TestWorkoutRuntimeRoundTripThroughAuthenticatedSession|TestWorkoutRuntimeEndpointsStaySideEffectFreeAcrossVisitsEligibilityAndClaimedTags)$'
go test -count=5 ./internal/consumer -run '^TestIdentifiedPresenceLifecycleDoesNotFinishExistingInProgressWorkout$'
go build ./cmd/apollo
```

These reruns are the minimum trustworthy set for Tracer 6 because they prove:
- workout ordering stays stable under repeated runs and clock-skew regressions
- workout lifecycle logs are emitted on success paths
- auth/session-backed workout runtime still works end to end
- workout runtime stays separate from visit lifecycle and eligibility state

### Verified Recommendation Runtime Smoke

```bash
docker run -d --rm --name tracer7-apollo-smoke \
  -e POSTGRES_USER=apollo \
  -e POSTGRES_PASSWORD=apollo \
  -e POSTGRES_DB=apollo \
  -p 55439:5432 \
  postgres:16-alpine

cd /Users/zizo/Personal-Projects/ASHTON/apollo
APOLLO_DATABASE_URL='postgres://apollo:apollo@127.0.0.1:55439/apollo?sslmode=disable' \
  go run ./cmd/apollo migrate up

APOLLO_DATABASE_URL='postgres://apollo:apollo@127.0.0.1:55439/apollo?sslmode=disable' \
APOLLO_HTTP_ADDR='127.0.0.1:18088' \
APOLLO_SESSION_COOKIE_SECRET='0123456789abcdef0123456789abcdef' \
APOLLO_SESSION_COOKIE_SECURE='true' \
APOLLO_LOG_VERIFICATION_TOKENS='true' \
  go run ./cmd/apollo serve

curl -sS -i -X POST http://127.0.0.1:18088/api/v1/auth/verification/start \
  -H 'Content-Type: application/json' \
  --data '{"student_id":"student-rec-smoke-001","email":"rec-smoke-001@example.com"}'
curl -sS -i -X POST http://127.0.0.1:18088/api/v1/auth/verification/start \
  -H 'Content-Type: application/json' \
  --data '{"student_id":"student-rec-smoke-002","email":"rec-smoke-002@example.com"}'
curl -sS -i -X POST http://127.0.0.1:18088/api/v1/auth/verification/start \
  -H 'Content-Type: application/json' \
  --data '{"student_id":"student-rec-smoke-003","email":"rec-smoke-003@example.com"}'

curl -sS -i -c /tmp/tracer7-user1.cookies \
  'http://127.0.0.1:18088/api/v1/auth/verify?token=<user1 token from server log>'
curl -sS -i -c /tmp/tracer7-user2.cookies \
  'http://127.0.0.1:18088/api/v1/auth/verify?token=<user2 token from server log>'
curl -sS -i -c /tmp/tracer7-user3.cookies \
  'http://127.0.0.1:18088/api/v1/auth/verify?token=<user3 token from server log>'

curl -sS -i -b /tmp/tracer7-user1.cookies \
  http://127.0.0.1:18088/api/v1/recommendations/workout

curl -sS -i -b /tmp/tracer7-user2.cookies \
  -H 'Content-Type: application/json' \
  -d '{"notes":"resume smoke"}' \
  http://127.0.0.1:18088/api/v1/workouts
curl -sS -i -b /tmp/tracer7-user2.cookies \
  http://127.0.0.1:18088/api/v1/recommendations/workout

curl -sS -i -b /tmp/tracer7-user3.cookies \
  -H 'Content-Type: application/json' \
  -d '{"notes":"finished smoke"}' \
  http://127.0.0.1:18088/api/v1/workouts
curl -sS -i -b /tmp/tracer7-user3.cookies \
  -H 'Content-Type: application/json' \
  -X PUT http://127.0.0.1:18088/api/v1/workouts/<user3-workout-id> \
  --data '{"notes":"finished smoke updated","exercises":[{"name":"bench press","sets":3,"reps":8},{"name":"row","sets":3,"reps":10}]}'
curl -sS -i -b /tmp/tracer7-user3.cookies \
  -X POST http://127.0.0.1:18088/api/v1/workouts/<user3-workout-id>/finish
curl -sS -i -b /tmp/tracer7-user3.cookies \
  http://127.0.0.1:18088/api/v1/recommendations/workout

docker exec -i tracer7-apollo-smoke psql -U apollo -d apollo -c \
  "UPDATE apollo.workouts SET started_at = now() - interval '31 hours 30 minutes', finished_at = now() - interval '30 hours' WHERE id = '<user3-workout-id>';"
curl -sS -i -b /tmp/tracer7-user3.cookies \
  http://127.0.0.1:18088/api/v1/recommendations/workout

docker exec -i tracer7-apollo-smoke psql -U apollo -d apollo -c \
  "SELECT student_id, preferences::text FROM apollo.users WHERE student_id IN ('student-rec-smoke-001','student-rec-smoke-002','student-rec-smoke-003') ORDER BY student_id; SELECT user_id, count(*) AS visits FROM apollo.visits WHERE user_id IN (SELECT id FROM apollo.users WHERE student_id IN ('student-rec-smoke-001','student-rec-smoke-002','student-rec-smoke-003')) GROUP BY user_id ORDER BY user_id; SELECT student_id, count(ct.id) AS claimed_tags FROM apollo.users u LEFT JOIN apollo.claimed_tags ct ON ct.user_id = u.id WHERE student_id IN ('student-rec-smoke-001','student-rec-smoke-002','student-rec-smoke-003') GROUP BY student_id ORDER BY student_id;"
```

Expected recommendation smoke outcomes:
- cold-start member returns `start_first_workout`
- member with an `in_progress` workout returns `resume_in_progress_workout`
- member with a fresh finished workout returns `recovery_day`
- the same member returns `repeat_last_finished_workout` after the workout is
  aged past the `24h` recovery window
- `users.preferences` remain at the default ghost/unavailable state for the
  smoke users
- `apollo.visits` stays empty for the smoke users
- `apollo.claimed_tags` stays empty for the smoke users
