# Tracer 25 Hardening

Tracer 25 claims:

- APOLLO owns typed non-clinical `users.preferences.nutrition_profile` inputs
  for dietary restrictions, cuisine preferences, `budget_preference`, and
  `cooking_capability`.
- APOLLO owns owner-scoped meal template truth and meal-log truth through
  authenticated internal HTTP.
- APOLLO owns a conservative, deterministic, read-only nutrition recommendation
  surface with calorie and macro ranges, strategy flags, and thin structured
  limitations over explicit profile inputs plus recent meal history.
- nutrition writes and recommendation reads stay separate from planner,
  workouts, visits, membership, competition, ARES, and persisted recommendation
  storage.
- Tracer 25 does not claim diagnosis, clinical advice, meal-plan chat, public
  social nutrition surfaces, or opaque helper-owned decision cores.

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

- authenticated `PATCH /api/v1/profile` round-trips typed `nutrition_profile`
  writes while preserving unrelated preference data
- authenticated `GET/POST/PUT /api/v1/nutrition/meal-templates` is owner-scoped
  and rejects duplicate names or empty nutrition payloads cleanly
- authenticated `GET/POST/PUT /api/v1/nutrition/meal-logs` is owner-scoped,
  supports deterministic template reuse, and rejects malformed or partial
  explicit rewrites cleanly
- authenticated `GET /api/v1/recommendations/nutrition` is deterministic across
  rerun, stays conservative with sparse history, and returns non-clinical
  limitation text instead of overclaiming certainty
- nutrition runtime coverage proves wrong-owner mutation failures, dietary
  restriction conflicts, invalid template payloads, missing nutrition payloads,
  and missing `logged_at` on explicit updates
- nutrition profile writes, meal template writes, meal-log writes, and nutrition
  recommendation reads do not mutate visits, workouts, planner rows, coaching
  feedback, lobby membership, competition state, ARES rows, claimed tags, or
  persisted recommendation storage

## Negative Proof

Tracer 25 does not claim:

- diagnosis, medical posture, or clinical nutrition advice
- BMR/TDEE/body-fat or lab-driven claim engines
- meal-plan chatbot flows or generic nutrition chat
- planner mutation, workout mutation, or recommendation apply semantics
- role/authz widening, presence/streak widening, or public/social nutrition
  reads
- meaningful frontend widening or deployment closeout truth
- `ashton-proto` widening

## Deferred

- richer explanation and bounded helper surfaces
- presence/tap-link/streak substrate
- role/authz and actor-attribution substrate
- public/social nutrition reads
- deployment closeout truth
