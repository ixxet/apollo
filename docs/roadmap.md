# apollo Roadmap

## Objective

Keep APOLLO moving through narrow member-facing release lines instead of trying
to jump straight to a broad product.

## Current Line

Current repo/runtime closeout line on `main`: Tracer 26 `v0.17.0` line

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
  assignment, and explicit session lifecycle transitions are shipped
- authenticated internal HTTP competition result capture, sport-and-mode
  ratings, session-scoped standings, and self-scoped member stats are
  closure-clean in repo/runtime on `main` for the `v0.13.0` line
- authenticated internal HTTP planner catalog reads, template/loadout writes,
  week-rooted planner writes, and typed non-medical `coaching_profile` inputs
  are real in repo/runtime on `main` for the tagged `v0.14.0` line
- Tracer 24 deterministic coaching substrate is tagged on the `v0.15.0` line,
  and `v0.15.1` is the narrow post-closeout hardening patch on that same line
- Tracer 25 conservative nutrition substrate remains closure-clean in
  repo/runtime on `main`: typed non-clinical `nutrition_profile` inputs,
  owner-scoped meal template and meal-log truth, and deterministic read-only
  calorie/macro range recommendations over explicit inputs plus recent meal
  history
- Tracer 26 explanation/helper runtime is now closure-clean in repo/runtime on
  `main`: authenticated internal helper reads, bounded `why` flows, and
  read-only variation previews over the existing deterministic coaching and
  nutrition cores
- deployment truth is still narrower than the full product surface

## Release Lines

Tracer 24 remains tagged on `v0.15.0`, with `v0.15.1` reserved for the narrow
hardening patch on that same line. The current repo/runtime closeout line on
`main` is Tracer 26 helper truth on the `v0.17.0` line.

| Release line | Intended purpose | Restrictions | What it should not do yet |
| --- | --- | --- | --- |
| historical `v0.6.1` note | Milestone 1.6 companion patch only if repo-local closure ever needs a backfill | treat this as historical closure context, not the active next line | do not present this as the active planned release line |
| `v0.14.0` | planner, exercise library, templates / loadouts, and richer profile inputs | keep the line backend/CLI-first and bounded | do not widen into meaningful frontend work, workout instantiation, or recommendation logic |
| `v0.15.0` | deterministic coaching substrate over planner, profile, and workout history | keep it deterministic, bounded, and side-effect free over planner truth | do not let visits, departures, profile state, or helper text become an opaque decision core |
| `v0.16.0` | conservative nutrition substrate with meal logging and calorie / macro ranges | keep it non-clinical and conservative | do not turn the product into a diet app or diagnostic engine |
| `v0.17.0` | explanation, summarization, bounded AI helper flows, and thin agent-facing helper surfaces | keep them subordinate to stable deterministic logic | do not let explanation become the core engine |
| `v0.18.0` | member presence, tap-link, and streak substrate over explicit visit truth | keep presence explicit and auditable | do not invent fake streak counters or silent visit inference |
| `v0.19.0` | role/authz, actor attribution, trusted-surface primitives, and staff runtime boundary substrate | keep authority explicit and reviewable | do not widen into polished ops product or speculative contracts |

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
- do not infer coaching or nutrition truth from presence or streak state alone
- do not infer lobby membership from eligibility, visits, or physical presence
- do not infer competition queue or assignment state from lobby membership
  alone
- keep planner truth separate from coaching or nutrition proposals until apply
- future AI/helper surfaces may propose structured diffs, but they must not
  bypass domain validation, actor attribution, or capability checks
- do not let match preview reads mutate membership, visits, workouts,
  recommendations, ARES tables, or competition execution state
- do not let visit changes silently affect match preview output
- do not let competition history runtime widen into rivalry/badge logic or
  public competition reads
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
- `Tracer 24`: deterministic coaching substrate
- `Tracer 25`: conservative nutrition substrate
- `Tracer 26`: explanation and thin agent-facing helper surfaces
- `Tracer 27`: member presence / tap-link / streak substrate
- `Tracer 28`: role/authz and staff runtime boundary substrate
