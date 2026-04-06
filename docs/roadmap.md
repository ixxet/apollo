# apollo Roadmap

## Objective

Keep APOLLO moving through narrow member-facing release lines instead of trying
to jump straight to a broad product.

## Current Line

Current active line: `v0.9.0`

- first-party auth and session-backed profile state are real
- visit ingest and close are real
- derived lobby eligibility is real
- explicit workout runtime is real
- deterministic workout recommendation read is real
- minimal member web shell is real locally
- explicit lobby membership runtime is real locally
- deterministic read-only ARES match preview is real locally
- deployment truth is still narrower than the full product surface

## Planned Release Lines

| Planned tag | Intended purpose | Restrictions | What it should not do yet |
| --- | --- | --- | --- |
| `v0.6.1` | optional Milestone 1.6 companion line if APOLLO repo truth changes materially | keep the change bounded to live departure-close support or deployment-truth alignment | do not widen into broader product deployment |
| `v0.10.0` | recommendation persistence | persist recommendation outputs only after the deterministic read line is stable | do not mix persistence with generated coaching |
| `v0.11.0` | generated planning and coaching runtime | build on stable workout and recommendation foundations | do not let visits, departures, or profile state silently drive coaching logic |

## Boundaries

- keep visits, workouts, recommendations, lobby state, and matchmaking as
  distinct state domains
- keep eligibility and explicit lobby membership as separate state domains
- do not infer workouts from arrivals or departures
- do not infer recommendations from arrivals, departures, or visits
- do not infer lobby membership from eligibility, visits, or physical presence
- do not let match preview reads mutate membership, visits, workouts,
  recommendations, or ARES tables
- do not let visit changes silently affect match preview output
- do not widen deployment truth unless a bounded deployment workstream proves it

## Tracer / Workstream Ownership

- `Tracer 2`: visit ingest
- `Tracer 3`: auth and profile state
- `Tracer 4`: derived lobby eligibility
- `Tracer 5`: visit close from departure truth
- `Tracer 6`: explicit workout runtime
- `Tracer 7`: deterministic recommendation read
- `Tracer 11`: minimal member web shell
- `Tracer 12`: explicit lobby membership runtime
- `Tracer 13`: first deterministic ARES match preview
- later lines: recommendation persistence and generated coaching
