# apollo Roadmap

## Objective

Keep APOLLO moving through narrow member-facing release lines instead of trying
to jump straight to a broad product.

## Current Line

Current active repo/runtime line: Tracer 21 release line `v0.12.0`

- first-party auth and session-backed profile state are real
- visit ingest and close are real
- derived lobby eligibility is real
- explicit workout runtime is real
- deterministic workout recommendation read is real
- minimal member web shell is real locally
- explicit lobby membership runtime is real locally
- deterministic read-only ARES match preview is real locally
- APOLLO-owned sport registry, facility-sport capability mapping, and static
  sport rules/config are shipped through CLI reads
- authenticated internal HTTP competition session, team, roster, and match
  container primitives are shipped
- authenticated internal HTTP competition queue state, deterministic
  assignment, and explicit session lifecycle transitions are real locally
- deployment truth is still narrower than the full product surface

## Planned Release Lines

| Planned tag | Intended purpose | Restrictions | What it should not do yet |
| --- | --- | --- | --- |
| historical `v0.6.1` note | Milestone 1.6 companion patch only if repo-local closure ever needs a backfill | treat this as historical closure context, not the active next line | do not present this as the active planned release line |
| `v0.13.0` | result capture, ratings, rudimentary standings, and member profile stats | make competition truth real before any public/social surface | do not widen into a broad public social layer |
| `v0.14.0` | planner, exercise library, templates / loadouts, and richer profile inputs | keep the line backend/CLI-first and bounded | do not widen into meaningful frontend work |
| `v0.15.0` | conservative deterministic fitness coaching plus calorie / macro ranges and low-friction meal logging | build on stable workout and planner foundations | do not let visits, departures, or profile state silently drive opaque coaching logic |
| `v0.16.0` | explanation, summarization, and thin agent-facing helper surfaces | keep them subordinate to stable deterministic logic | do not let explanation become the core engine |

## Versioning Discipline

APOLLO now follows formal pre-`1.0.0` semantic versioning.

- `PATCH` releases cover hardening, docs sync, deployment closeout,
  observability, and bounded non-widening fixes
- `MINOR` releases cover new bounded member capabilities or intentional
  contract changes
- pre-`1.0.0` breaking changes still require a `MINOR`, never a `PATCH`

## Boundaries

- keep visits, workouts, recommendations, lobby state, and matchmaking as
  distinct state domains
- keep eligibility and explicit lobby membership as separate state domains
- do not infer workouts from arrivals or departures
- do not infer recommendations from arrivals, departures, or visits
- do not infer lobby membership from eligibility, visits, or physical presence
- do not infer competition queue or assignment state from lobby membership
  alone
- do not let match preview reads mutate membership, visits, workouts,
  recommendations, ARES tables, or competition execution state
- do not let visit changes silently affect match preview output
- do not let competition execution runtime widen into results, ratings,
  standings, rivalry/badge logic, or public competition reads
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
- `Tracer 19`: sport registry, facility-sport capability mapping, and basic sport rules/config
- `Tracer 20`: team, roster, session, and match container primitives
- `Tracer 21`: matchmaking / queue / assignment lifecycle
- `Tracer 22`: result capture, ratings, rudimentary standings, and member stats
- `Tracer 23`: planner/profile widening as backend/CLI-first truth
- `Tracer 24`: deterministic coaching and conservative nutrition guidance
- `Tracer 25`: explanation and thin agent-facing helper surfaces
