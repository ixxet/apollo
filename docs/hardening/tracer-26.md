# Tracer 26 Hardening

Tracer 26 claims:

- APOLLO now exposes authenticated internal helper reads for coaching and
  nutrition without widening beyond deterministic domain logic.
- coaching helper reads stay grounded in the existing deterministic
  `CoachingRecommendation`, `CoachingExplanation`, and `PlanChangeProposal`
  contract, with bounded `why` topics and read-only `easier` / `harder`
  variation previews.
- nutrition helper reads stay grounded in the existing deterministic
  `Recommendation` and `RecommendationExplanation` contract, with bounded
  `why` topics plus read-only `cheaper` / `simpler` guidance proposals that
  keep calorie and macro targets unchanged.
- helper reads, `why`, and variation previews stay authenticated/internal,
  read-only, and separate from planner writes, nutrition writes, presence,
  role/authz, and deployment truth.
- Tracer 26 does not claim helper persistence, model-backed calls, planner
  apply rails, nutrition apply rails, or `ashton-proto` widening.

## Proof

Run from `/Users/zizo/Personal-Projects/ASHTON/apollo`:

```sh
go test ./...
go test -count=5 ./internal/...
go vet ./...
go build ./cmd/apollo
git diff --check
```

Focused runtime and integrity coverage now proves:

- authenticated `GET /api/v1/helpers/coaching` is deterministic across rerun
  and stays read-only over existing planner truth
- authenticated `GET /api/v1/helpers/coaching/why` explains bounded
  deterministic topics without widening into chat-owned state
- authenticated `GET /api/v1/helpers/coaching/variation` produces read-only
  adjacent recommendation previews and does not mutate planner rows
- authenticated `GET /api/v1/helpers/nutrition` is deterministic across rerun
  and stays read-only over explicit meal/profile truth
- authenticated `GET /api/v1/helpers/nutrition/why` explains bounded
  deterministic topics without widening into generic nutrition chat
- authenticated `GET /api/v1/helpers/nutrition/variation` returns read-only
  cheaper/simpler guidance proposals while keeping calorie and macro targets
  unchanged
- malformed helper topics and unsupported variation requests fail cleanly with
  conservative `400` responses
- helper reads do not mutate recommendations storage, planner rows, meal logs,
  meal templates, workouts, visits, membership, competition state, ARES rows,
  claimed tags, or coaching feedback rows outside the existing explicit
  feedback-write routes

## Negative Proof

Tracer 26 does not claim:

- helper-owned planner or nutrition mutation
- apply or persistence rails for helper previews
- model-backed helper calls or chat-first product runtime
- presence/tap-link/streak logic
- role/authz or actor-attribution widening
- public/social helper surfaces or meaningful frontend widening
- deployment closeout truth
- `ashton-proto` widening

## Deferred

- helper apply / approval rails
- helper persistence and audit substrate
- presence/tap-link/streak substrate
- role/authz and actor-attribution substrate
- deployment closeout truth
