# Tracer 11 Hardening

## Audited Claim

Tracer 11 claims:

- a minimal APOLLO member web shell is real
- the shell uses existing authenticated APIs for:
  - profile read
  - workout list
  - workout detail
  - workout update
  - workout finish
  - recommendation read
- the shell does not widen backend scope
- workout and recommendation boundaries remain unchanged
- deployment truth stays unchanged

Audited repo truth:

- `apollo` `main == origin/main == ed845acf3d42cf17ca657fad5e07327a63e668d6`
- `ashton-platform` `main == origin/main == aa2ca193c9cd945d94fdab7786407c1085013286`

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
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./...
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=2 ./...
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=5 ./internal/server -run '^(TestWebUIRoutesRedirectAndRenderAgainstSessionState|TestWebUIAssetsAreServedThroughTheEmbeddedShell|TestMemberWebShellRuntimeSupportsCoreMemberFlowWithoutBoundaryDrift|TestWorkoutRuntimeRoundTripThroughAuthenticatedSession|TestWorkoutRuntimeEnforcesOwnershipAndStateConflicts|TestWorkoutRuntimeListsNewestWorkoutFirst|TestWorkoutRuntimeEndpointsStaySideEffectFreeAcrossVisitsEligibilityAndClaimedTags|TestWorkoutRecommendationRuntimeFollowsDeterministicPrecedence|TestWorkoutRecommendationEndpointStaysSideEffectFreeAcrossVisitsEligibilityAndClaimedTags|TestWorkoutRecommendationEndpointMapsLookupFailuresClearly|TestWorkoutEndpointsRejectMalformedBodiesBeforeCallingTheService|TestAuthAndProfileEndpointsRejectTokenAndSessionEdgeCases)$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && for i in 1 2 3 4 5; do node --test ./internal/server/web/assets/app.test.mjs; done
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go build ./cmd/apollo
```

```bash
docker run -d --rm --name tracer11-apollo-hardening \
  -e POSTGRES_USER=apollo \
  -e POSTGRES_PASSWORD=apollo \
  -e POSTGRES_DB=apollo \
  -p 55442:5432 \
  postgres:16-alpine

cd /Users/zizo/Personal-Projects/ASHTON/apollo
APOLLO_DATABASE_URL='postgres://apollo:apollo@127.0.0.1:55442/apollo?sslmode=disable' \
  go run ./cmd/apollo migrate up

APOLLO_DATABASE_URL='postgres://apollo:apollo@127.0.0.1:55442/apollo?sslmode=disable' \
APOLLO_HTTP_ADDR='127.0.0.1:18096' \
APOLLO_SESSION_COOKIE_SECRET='0123456789abcdef0123456789abcdef' \
APOLLO_SESSION_COOKIE_SECURE='true' \
APOLLO_LOG_VERIFICATION_TOKENS='true' \
  go run ./cmd/apollo serve

curl -sS -i http://127.0.0.1:18096/api/v1/health
curl -sS -i http://127.0.0.1:18096/app
curl -sS -i http://127.0.0.1:18096/api/v1/workouts
curl -sS -i -X POST http://127.0.0.1:18096/api/v1/auth/verification/start \
  -H 'Content-Type: application/json' \
  --data '{"student_id":"student-web-hardening-011","email":"web-hardening-011@example.com"}'
curl -sS -i -c /tmp/tracer11-hardening.cookies \
  'http://127.0.0.1:18096/api/v1/auth/verify?token=<token from server log>'

curl -sS -i -b /tmp/tracer11-hardening.cookies http://127.0.0.1:18096/
curl -sS -i -b /tmp/tracer11-hardening.cookies http://127.0.0.1:18096/app/login
curl -sS -i -b /tmp/tracer11-hardening.cookies http://127.0.0.1:18096/app
curl -sS -i -b /tmp/tracer11-hardening.cookies http://127.0.0.1:18096/api/v1/profile
curl -sS -i -b /tmp/tracer11-hardening.cookies http://127.0.0.1:18096/api/v1/recommendations/workout

curl -sS -b /tmp/tracer11-hardening.cookies \
  -H 'Content-Type: application/json' \
  -X POST http://127.0.0.1:18096/api/v1/workouts \
  --data '{"notes":"hardening workout"}' | jq -r '.id' > /tmp/tracer11-hardening.workout

curl -sS -i -b /tmp/tracer11-hardening.cookies \
  -H 'Content-Type: application/json' \
  -X PUT http://127.0.0.1:18096/api/v1/workouts/$(cat /tmp/tracer11-hardening.workout) \
  --data '{"notes":"missing exercises"}'
curl -sS -i -b /tmp/tracer11-hardening.cookies \
  -H 'Content-Type: application/json' \
  -X PUT http://127.0.0.1:18096/api/v1/workouts/$(cat /tmp/tracer11-hardening.workout) \
  --data '{"notes":"hardening workout updated","exercises":[{"name":"bench press","sets":3,"reps":8,"weight_kg":84.5,"rpe":8.5},{"name":"row","sets":3,"reps":10}]}'
curl -sS -i -b /tmp/tracer11-hardening.cookies \
  http://127.0.0.1:18096/api/v1/workouts/$(cat /tmp/tracer11-hardening.workout)
curl -sS -i -b /tmp/tracer11-hardening.cookies \
  http://127.0.0.1:18096/api/v1/workouts
curl -sS -i -b /tmp/tracer11-hardening.cookies \
  -X POST http://127.0.0.1:18096/api/v1/workouts/$(cat /tmp/tracer11-hardening.workout)/finish
curl -sS -i -b /tmp/tracer11-hardening.cookies \
  -X POST http://127.0.0.1:18096/api/v1/workouts/$(cat /tmp/tracer11-hardening.workout)/finish
curl -sS -i -b /tmp/tracer11-hardening.cookies \
  -H 'Content-Type: application/json' \
  -X PUT http://127.0.0.1:18096/api/v1/workouts/$(cat /tmp/tracer11-hardening.workout) \
  --data '{"notes":"post-finish","exercises":[{"name":"row","sets":3,"reps":10}]}'
curl -sS -i -b /tmp/tracer11-hardening.cookies \
  http://127.0.0.1:18096/api/v1/recommendations/workout
curl -sS -i -c /tmp/tracer11-hardening.cookies -b /tmp/tracer11-hardening.cookies \
  -X POST http://127.0.0.1:18096/api/v1/auth/logout
curl -sS -i -b /tmp/tracer11-hardening.cookies http://127.0.0.1:18096/app
curl -sS -i -H 'Cookie: apollo_session=bogus' http://127.0.0.1:18096/app

docker exec -i tracer11-apollo-hardening psql -U apollo -d apollo -c \
  "SELECT count(*) AS visits FROM apollo.visits v JOIN apollo.users u ON u.id = v.user_id WHERE u.student_id = 'student-web-hardening-011'; SELECT count(*) AS workouts FROM apollo.workouts w JOIN apollo.users u ON u.id = w.user_id WHERE u.student_id = 'student-web-hardening-011'; SELECT student_id, preferences::text FROM apollo.users WHERE student_id = 'student-web-hardening-011'; SELECT count(*) AS claimed_tags FROM apollo.claimed_tags ct JOIN apollo.users u ON u.id = ct.user_id WHERE u.student_id = 'student-web-hardening-011'; SELECT w.id, w.status, w.notes, w.started_at, w.finished_at FROM apollo.workouts w JOIN apollo.users u ON u.id = w.user_id WHERE u.student_id = 'student-web-hardening-011' ORDER BY w.started_at DESC;"
```

```bash
cd /Users/zizo/Personal-Projects/ASHTON/apollo && node --input-type=module <<'EOF'
class Element {
  constructor(value = '') {
    this.value = value
    this.textContent = ''
    this.innerHTML = ''
    this.disabled = false
    this.listeners = {}
    this.classNames = new Set()
    this.classList = {
      add: (...names) => names.forEach((name) => this.classNames.add(name)),
      remove: (...names) => names.forEach((name) => this.classNames.delete(name)),
    }
  }
  addEventListener(type, handler) { this.listeners[type] = handler }
  querySelectorAll() { return [] }
}
const elements = {
  '#profile-summary': new Element(),
  '#profile-status': new Element(),
  '#recommendation-card': new Element(),
  '#recommendation-status': new Element(),
  '#workout-list': new Element(),
  '#workouts-status': new Element(),
  '#workout-detail-title': new Element(),
  '#workout-detail-state': new Element(),
  '#workout-notes': new Element(),
  '#exercise-list': new Element(),
  '#workout-error': new Element(),
  '#save-workout': new Element(),
  '#finish-workout': new Element(),
  '#refresh-shell': new Element(),
  '#logout-shell': new Element(),
  '#create-workout': new Element(),
  '#add-exercise': new Element(),
  '#workout-editor': new Element(),
}
globalThis.document = {
  body: { dataset: { apolloView: 'shell' } },
  querySelector(selector) { return elements[selector] ?? null },
}
globalThis.window = { location: { href: 'http://127.0.0.1/app', assign() {} } }
globalThis.fetch = async (path) => {
  if (path === '/api/v1/profile') {
    return { ok: true, status: 200, json: async () => ({ display_name: 'member', student_id: 's-1', email: 'm@example.com', email_verified: true, visibility_mode: 'ghost', availability_mode: 'unavailable' }) }
  }
  if (path === '/api/v1/workouts') {
    return { ok: true, status: 200, json: async () => ([]) }
  }
  if (path === '/api/v1/recommendations/workout') {
    return { ok: false, status: 500, json: async () => ({ error: 'recommendations unavailable' }) }
  }
  throw new Error('unexpected path ' + path)
}
await import('./internal/server/web/assets/app.mjs')
await new Promise((resolve) => setTimeout(resolve, 0))
console.log(JSON.stringify({
  recommendation_status: elements['#recommendation-status'].textContent,
  recommendation_error_class: elements['#recommendation-status'].classNames.has('error-message'),
  recommendation_card_contains_error: elements['#recommendation-card'].innerHTML.includes('recommendations unavailable'),
  workouts_status: elements['#workouts-status'].textContent,
}))
EOF

cd /Users/zizo/Personal-Projects/ASHTON/apollo && node --input-type=module <<'EOF'
class Element {
  constructor(value = '') {
    this.value = value
    this.textContent = ''
    this.innerHTML = ''
    this.disabled = false
    this.listeners = {}
    this.classNames = new Set()
    this.classList = {
      add: (...names) => names.forEach((name) => this.classNames.add(name)),
      remove: (...names) => names.forEach((name) => this.classNames.delete(name)),
    }
  }
  addEventListener(type, handler) { this.listeners[type] = handler }
  querySelectorAll() { return [] }
}
const elements = {
  '#profile-summary': new Element(),
  '#profile-status': new Element(),
  '#recommendation-card': new Element(),
  '#recommendation-status': new Element(),
  '#workout-list': new Element(),
  '#workouts-status': new Element(),
  '#workout-detail-title': new Element(),
  '#workout-detail-state': new Element(),
  '#workout-notes': new Element(),
  '#exercise-list': new Element(),
  '#workout-error': new Element(),
  '#save-workout': new Element(),
  '#finish-workout': new Element(),
  '#refresh-shell': new Element(),
  '#logout-shell': new Element(),
  '#create-workout': new Element(),
  '#add-exercise': new Element(),
  '#workout-editor': new Element(),
}
let unhandled = ''
process.on('unhandledRejection', (error) => {
  unhandled = error.message
})
globalThis.document = {
  body: { dataset: { apolloView: 'shell' } },
  querySelector(selector) { return elements[selector] ?? null },
}
globalThis.window = { location: { href: 'http://127.0.0.1/app', assign() {} } }
globalThis.fetch = async () => {
  throw new Error('network down')
}
await import('./internal/server/web/assets/app.mjs')
await new Promise((resolve) => setTimeout(resolve, 25))
console.log(JSON.stringify({
  profile_status: elements['#profile-status'].textContent,
  recommendation_status: elements['#recommendation-status'].textContent,
  workouts_status: elements['#workouts-status'].textContent,
  unhandled,
}))
EOF
```

## Expected Destructive Failures Vs Real Regressions

Expected negative or boundary results during hardening:

- unauthenticated `/app` requests redirect to `/app/login`
- unauthenticated workout reads return `401`
- malformed workout update payloads return `400`
- duplicate finish and post-finish updates return `409`
- revoked or bogus session cookies redirect back to `/app/login`
- a backend `500` on recommendation read surfaces an explicit error in the shell

Real regressions would have been:

- shell routes bypassing session ownership checks
- workout ordering, ownership, or finish semantics drifting from the backend
- recommendation reads mutating workouts, visits, preferences, or claimed tags
- hidden frontend-specific API widening
- recommendation or workout failures being silently treated as success

## Verified Truth

- the embedded shell routes are real and mounted in the production handler:
  - `/`
  - `/app/login`
  - `/app`
- repeated Go and browser-side reruns held
- auth/session boundaries remain intact for both JSON APIs and shell routes
- workout list/detail/update/finish semantics remain backend-authoritative
- recommendation reads remain deterministic and side-effect free
- local smoke proved:
  - auth/session bootstrap is real
  - `/app` is protected
  - workout update validation failures map clearly
  - duplicate finish and post-finish update conflicts map clearly
  - recommendation reads stay read-only
  - visits, preferences, and claimed tags stayed unchanged for the smoke user
- representative shell `500` handling is explicit:
  - recommendation failures surface a visible error state instead of a silent no-op

## Unverified Truth

- no deployed member-shell truth was added
- no browser-automation or full end-to-end UI harness was added
- no broader frontend stack, offline support, or PWA workflow was proven
- no explicit stale-request cancellation proof was added for rapid list/detail/finish races

## Carry-Forward Gaps

- network failures are not mapped cleanly in the shell yet; a rejected `fetch`
  leaves loading copy in place and produces an unhandled rejection
- request cancellation or sequencing guards are still absent, so stale
  list/detail/finish races are not explicitly proven safe yet
- the shell remains locally proven only; deployment truth is unchanged

## Final Verdict

Tracer 11 is closure-clean with carry-forward gaps.

Those gaps are local UI operational debt:

- network failures are not surfaced cleanly in the shell yet
- stale-request sequencing is not explicitly hardened yet
- deployed member-shell truth remains unproven and unclaimed
