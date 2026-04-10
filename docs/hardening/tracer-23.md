# Tracer 23 Hardening

Tracer 23 claims:

- APOLLO owns exercise-library truth through relational exercise and equipment
  catalog tables plus bounded seed data.
- APOLLO owns owner-scoped template/loadout truth through authenticated
  internal HTTP.
- APOLLO owns week-rooted planner truth through authenticated internal HTTP.
- APOLLO owns typed non-medical `users.preferences.coaching_profile` inputs for
  `goal_key`, `days_per_week`, `session_minutes`, and
  `preferred_equipment_keys`.
- planner/template/profile writes stay separate from visits, workouts,
  recommendations, lobby membership, and competition history.
- workout runtime and deterministic recommendation precedence remain unchanged.

## Proof

Run from `/Users/zizo/Personal-Projects/ASHTON/apollo`:

```sh
go test ./...
go test -count=5 ./internal/...
go vet ./...
go build ./cmd/apollo
```

Focused runtime and integrity coverage now proves:

- authenticated `GET /api/v1/planner/exercises` and
  `GET /api/v1/planner/equipment` return seeded APOLLO-owned catalog truth
- authenticated `GET/POST/PUT /api/v1/planner/templates` is owner-scoped and
  rejects duplicate names per owner
- authenticated `GET/PUT /api/v1/planner/weeks/{week_start}` round-trips one
  ISO-week planner root with ordered sessions and ordered item rows
- template-backed planner sessions copy template item composition into planner
  session items without creating workouts
- authenticated `PATCH /api/v1/profile` round-trips typed non-medical
  `coaching_profile` inputs and preserves unrelated preference keys
- invalid exercise keys, invalid equipment keys, disallowed equipment bindings,
  invalid week roots, invalid day indexes, malformed session shapes, ownership
  failures, and missing auth all reject cleanly
- repeated recommendation reads stay deterministic and unchanged after
  planner/template/profile writes
- planner/template/profile writes do not mutate visits, workouts,
  recommendations, claimed tags, or lobby membership rows

## Negative Proof

Tracer 23 does not claim:

- workout draft creation from planner writes
- workout completion inference from planner state
- planner-aware recommendation logic
- calorie, macro, or meal logging
- public/social planner reads
- meaningful frontend widening
- deployment closeout truth

## Deferred

- deterministic coaching over plan + history + profile
- conservative nutrition guidance
- planner-to-workout instantiation
- facility-specific equipment inventory or availability
- broader shell or public planner UI
