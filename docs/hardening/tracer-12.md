# Tracer 12 Hardening

## Audited Claim

Tracer 12 claims:

- authenticated `GET /api/v1/lobby/membership` is real
- authenticated `POST /api/v1/lobby/membership/join` is real
- authenticated `POST /api/v1/lobby/membership/leave` is real
- lobby membership is explicit durable member intent
- eligibility remains separate from membership
- repeated join and repeated leave are deterministic
- the member shell exposes one narrow membership panel over the same runtime
- membership transitions do not mutate visits, workouts, recommendations, or
  claimed tags
- deployment truth remains unchanged

Audited repo truth:

- `apollo` `main == origin/main == 192a259b47ddbf861f9c308bbeffe7cd239c649b`
- `ashton-platform` `main == origin/main == ba7c9e5ee5852c90f5f439bf4584a4ebf32d7af0`

Precondition verified during hardening:

- Workstream A remained fixed; the Tracer 11 shell rejected-`fetch()` failure
  path still mapped to explicit error UI without leaking unhandled rejections

## Exact Rerun Commands

```bash
git -C /Users/zizo/Personal-Projects/ASHTON/apollo status --short
git -C /Users/zizo/Personal-Projects/ASHTON/apollo branch --show-current
git -C /Users/zizo/Personal-Projects/ASHTON/apollo rev-parse HEAD
git -C /Users/zizo/Personal-Projects/ASHTON/apollo rev-parse @{u}
git -C /Users/zizo/Personal-Projects/ASHTON/apollo merge-base @ @{u}

git -C /Users/zizo/Personal-Projects/ASHTON/ashton-platform status --short
git -C /Users/zizo/Personal-Projects/ASHTON/ashton-platform branch --show-current
git -C /Users/zizo/Personal-Projects/ASHTON/ashton-platform rev-parse HEAD
git -C /Users/zizo/Personal-Projects/ASHTON/ashton-platform rev-parse @{u}
git -C /Users/zizo/Personal-Projects/ASHTON/ashton-platform merge-base @ @{u}
```

```bash
cd /Users/zizo/Personal-Projects/ASHTON/apollo && node --test ./internal/server/web/assets/app.test.mjs
cd /Users/zizo/Personal-Projects/ASHTON/apollo && for i in 1 2 3 4 5; do echo "RUN:$i"; node --test ./internal/server/web/assets/app.test.mjs || exit 1; done
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=5 ./internal/membership
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=5 ./internal/server -run '^(TestLobbyMembershipEndpointsRequireAuthentication|TestLobbyMembershipEndpointsReturnStableMembershipShapes|TestLobbyMembershipEndpointsMapErrorsClearly|TestWebUIRoutesRedirectAndRenderAgainstSessionState|TestWebUIAssetsAreServedThroughTheEmbeddedShell)$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=5 ./internal/server -run '^(TestLobbyMembershipRuntimeSupportsExplicitJoinLeaveRoundTrip|TestLobbyMembershipRuntimeRejectsIneligibleAndRepeatedTransitionsDeterministically|TestLobbyMembershipRuntimeDoesNotMutateVisitsWorkoutsRecommendationsOrClaimedTags|TestMemberWebShellRuntimeSupportsCoreMemberFlowWithoutBoundaryDrift)$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./cmd/apollo ./internal/server -run '^(TestBuildServerDependenciesWiresLobbyMembershipRuntime|TestLobbyMembershipEndpointsRequireAuthentication|TestLobbyMembershipEndpointsReturnStableMembershipShapes|TestLobbyMembershipEndpointsMapErrorsClearly|TestLobbyMembershipRuntimeSupportsExplicitJoinLeaveRoundTrip|TestLobbyMembershipRuntimeRejectsIneligibleAndRepeatedTransitionsDeterministically|TestLobbyMembershipRuntimeDoesNotMutateVisitsWorkoutsRecommendationsOrClaimedTags|TestMemberWebShellRuntimeSupportsCoreMemberFlowWithoutBoundaryDrift)$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./...
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=2 ./...
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go build ./cmd/apollo
```

```bash
docker run -d --rm --name tracer12-apollo-hardening \
  -e POSTGRES_USER=apollo \
  -e POSTGRES_PASSWORD=apollo \
  -e POSTGRES_DB=apollo \
  -p 55443:5432 \
  postgres:16-alpine

cd /Users/zizo/Personal-Projects/ASHTON/apollo
APOLLO_DATABASE_URL='postgres://apollo:apollo@127.0.0.1:55443/apollo?sslmode=disable' \
  go run ./cmd/apollo migrate up

APOLLO_DATABASE_URL='postgres://apollo:apollo@127.0.0.1:55443/apollo?sslmode=disable' \
APOLLO_HTTP_ADDR='127.0.0.1:18097' \
APOLLO_SESSION_COOKIE_SECRET='0123456789abcdef0123456789abcdef' \
APOLLO_SESSION_COOKIE_SECURE='true' \
APOLLO_LOG_VERIFICATION_TOKENS='true' \
  go run ./cmd/apollo serve

curl -sS -i http://127.0.0.1:18097/api/v1/health
curl -sS -i http://127.0.0.1:18097/app
curl -sS -i http://127.0.0.1:18097/api/v1/lobby/membership
curl -sS -i -X POST http://127.0.0.1:18097/api/v1/lobby/membership/join
curl -sS -i -X POST http://127.0.0.1:18097/api/v1/lobby/membership/leave

curl -sS -i -X POST http://127.0.0.1:18097/api/v1/auth/verification/start \
  -H 'Content-Type: application/json' \
  --data '{"student_id":"student-membership-hardening-012","email":"membership-hardening-012@example.com"}'

curl -sS -i -c /tmp/tracer12-hardening.cookies \
  'http://127.0.0.1:18097/api/v1/auth/verify?token=<token from server log>'

curl -sS -i -b /tmp/tracer12-hardening.cookies http://127.0.0.1:18097/api/v1/lobby/membership
curl -sS -i -b /tmp/tracer12-hardening.cookies -X POST http://127.0.0.1:18097/api/v1/lobby/membership/join
curl -sS -i -b /tmp/tracer12-hardening.cookies http://127.0.0.1:18097/api/v1/recommendations/workout
curl -sS -i -b /tmp/tracer12-hardening.cookies http://127.0.0.1:18097/api/v1/workouts
curl -sS -i -b /tmp/tracer12-hardening.cookies http://127.0.0.1:18097/app

curl -sS -i -b /tmp/tracer12-hardening.cookies \
  -H 'Content-Type: application/json' \
  -X PATCH http://127.0.0.1:18097/api/v1/profile \
  --data '{"visibility_mode":"discoverable","availability_mode":"available_now"}'

curl -sS -i -b /tmp/tracer12-hardening.cookies http://127.0.0.1:18097/api/v1/lobby/eligibility
curl -sS -i -b /tmp/tracer12-hardening.cookies -X POST http://127.0.0.1:18097/api/v1/lobby/membership/join
curl -sS -i -b /tmp/tracer12-hardening.cookies -X POST http://127.0.0.1:18097/api/v1/lobby/membership/join
curl -sS -i -b /tmp/tracer12-hardening.cookies http://127.0.0.1:18097/api/v1/lobby/membership
curl -sS -i -b /tmp/tracer12-hardening.cookies -X POST http://127.0.0.1:18097/api/v1/lobby/membership/leave
curl -sS -i -b /tmp/tracer12-hardening.cookies -X POST http://127.0.0.1:18097/api/v1/lobby/membership/leave
curl -sS -i -b /tmp/tracer12-hardening.cookies http://127.0.0.1:18097/api/v1/lobby/membership

docker exec -i tracer12-apollo-hardening psql -U apollo -d apollo -c \
  "SELECT student_id, preferences::text FROM apollo.users WHERE student_id = 'student-membership-hardening-012'; \
   SELECT count(*) AS visits FROM apollo.visits v JOIN apollo.users u ON u.id = v.user_id WHERE u.student_id = 'student-membership-hardening-012'; \
   SELECT count(*) AS workouts FROM apollo.workouts w JOIN apollo.users u ON u.id = w.user_id WHERE u.student_id = 'student-membership-hardening-012'; \
   SELECT count(*) AS claimed_tags FROM apollo.claimed_tags ct JOIN apollo.users u ON u.id = ct.user_id WHERE u.student_id = 'student-membership-hardening-012'; \
   SELECT status, joined_at, left_at FROM apollo.lobby_memberships lm JOIN apollo.users u ON u.id = lm.user_id WHERE u.student_id = 'student-membership-hardening-012';"
```

## Expected Destructive Failures Vs Real Regressions

Expected negative or boundary results during hardening:

- unauthenticated membership read, join, and leave return `401`
- ineligible join returns `409` with a stable eligibility-derived reason
- repeated join returns `409`
- repeated leave returns `409`
- explicit profile changes can make a member eligible, but do not auto-create
  membership
- recommendation and workout reads remain side-effect free through the whole
  membership lifecycle

Real regressions would have been:

- eligibility auto-creating membership
- visits, workouts, recommendations, or physical presence implicitly changing
  membership state
- membership transitions mutating visits, workouts, claimed tags, or
  recommendations
- the shell inventing joined state without a successful server response
- `cmd/apollo` omitting the membership runtime again

## Verified Truth

- the membership endpoints are real and mounted in the production handler
- the member shell includes one narrow membership panel with explicit `Join` and
  `Leave` actions
- Workstream A stayed fixed while Tracer 12 landed
- repeated browser-side reruns held, including:
  - bootstrap rejection mapping
  - refresh rejection mapping
  - membership render
  - membership join/leave success and failure mapping
- repeated service-level membership tests held
- repeated server/runtime/integrity tests held
- the `cmd/apollo` wiring regression test held
- local smoke proved:
  - initial membership reads as `not_joined`
  - ineligible join is rejected clearly
  - explicit profile change can make the user eligible without auto-joining them
  - join persists `joined_at`
  - leave preserves `joined_at` and writes `left_at`
  - repeated join and repeated leave stay deterministic
  - recommendations remain read-only
  - workouts remain unchanged
  - the database ends with one durable `apollo.lobby_memberships` row
  - `visits`, `workouts`, and `claimed_tags` remain `0` for the smoke user

## Unverified Truth

- no live in-cluster membership deployment proof was added
- no invites, parties, match formation, or ARES preview was proven
- no broader APOLLO product deployment claim was added

## Carry-Forward Gaps

No in-scope carry-forward gaps remained at hardening close.

Intentional deferred items are:

- deployed membership proof
- invites, parties, and match formation
- the first deterministic ARES preview in Tracer 13

## Final Verdict

Tracer 12 is closure-clean.
