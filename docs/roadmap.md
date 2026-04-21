# apollo Roadmap

## Objective

Keep APOLLO moving through narrow bounded release lines instead of trying to
jump straight to a broad product.

## Current Line

Current repo/runtime working line on `main`: Phase 3B.8 booking edit and replacement
over the already-closed Tracer 28 `v0.19.x`, Milestone 2.0 hardening
`v0.19.1`, Phase 3 shared substrate B, Phase 3A.1 member shell foundation,
Phase 3A.4 member-safe schedule calendar, Phase 3B.1 ops read foundation, and
Phase 3B.7 customer status/communication lines

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
- Tracer 27 member presence runtime is now closure-clean in repo/runtime on
  `main`: authenticated facility-scoped presence reads over explicit tap-linked
  visit truth, one durable tap-link row per visit, and one durable
  facility-scoped streak state plus streak-event line per member/facility over
  linked visit days only
- Phase 3A.3 member truth completion is now real in repo/runtime on `main`:
  authenticated self-scoped claimed-tag reads/writes now give members one
  honest tap-claim path, member-safe facility hours/meta now compose over the
  existing facility/sport/schedule substrate without leaking staff internals,
  self-scoped competition history now extends beyond aggregate stats, and the
  current presence summary remains the existing visit/tap history surface
- Tracer 28 role/authz runtime is now closure-clean in repo/runtime on `main`:
  authenticated session principals carry one explicit APOLLO-global role plus
  deterministic competition, schedule, and ops-read capability sets derived
  from that role, competition staff reads require explicit capability truth,
  privileged competition mutations require trusted-surface proof, successful
  staff-sensitive competition mutations write durable actor attribution, and
  member self-service surfaces remain separate and unchanged
- Phase 3 shared substrate B scheduling runtime is now real in repo/runtime on
  `main`: APOLLO-owned schedule resources, resource edges, typed blocks,
  RFC3339-windowed calendar reads, block-timezone weekly recurrence, active +
  bookable inventory-claim gating, date exceptions, staff-gated reads, and
  staff-gated block writes are in place while deployed truth remains separate
- Phase 3A.1 member shell foundation is now real in repo/runtime on `main`:
  `/app/home`, `/app/workouts`, `/app/meals`, `/app/tournaments`, and
  `/app/settings` now share one embedded member shell frame over already-real
  member-safe APIs, section failures stay explicit and recoverable, and
  booking plus staff schedule surfaces stay out
- Phase 3A.4 member-safe schedule calendar is now real in repo/runtime on
  `main`: authenticated members can read
  `/api/v1/presence/facilities/{facilityKey}/calendar?from=<RFC3339>&until=<RFC3339>`
  with explicit RFC3339 boundaries, a 14-day max window, and the same
  facility-scope public operating-hours and closure filtering posture already
  used by member facility composition without widening into booking, labels,
  or staff schedule leakage
- Phase 3B.1 ops read foundation is now real in repo/runtime on `main`:
  authenticated supervisor, manager, and owner users can read
  `/api/v1/ops/facilities/{facilityKey}/overview?from=<RFC3339>&until=<RFC3339>`
  with bounded RFC3339 windows, APOLLO schedule-summary truth, ATHENA current
  occupancy, and sanitized aggregate ATHENA analytics without adding writes,
  booking, public entrypoints, HERMES widening, gateway work, or deploy claims
- Phase 3B.4 request-first booking runtime is now real in repo/runtime on
  `main`: supervisors can read internal booking requests, managers and owners
  can create and transition them through trusted-surface-gated staff APIs, each
  transition requires `expected_version`, APOLLO returns conflict-aware
  availability from schedule/resource graph truth, and approval creates a
  linked one-off internal schedule `reservation` / `hard_reserve` block without
  public booking, customer self-service, payments, quotes, Hestia booking UI,
  owner policy writes, admin-role widening, gateway work, HERMES widening, or
  deploy claims
- Phase 3B.5 approved booking lifecycle is now real in repo/runtime on `main`:
  managers and owners can cancel approved internal booking requests through the
  existing trusted-surface-gated `/cancel` transition, APOLLO atomically
  cancels the linked internal reservation block, retains the booking
  `schedule_block_id` for audit, and refuses stale booking versions or linked
  schedule drift without opening public booking, payments, quotes, Hestia
  booking UI, owner policy writes, admin-role widening, gateway work, HERMES
  widening, or deploy claims
- Phase 3B.6 public request entrypoint is now real in repo/runtime on `main`:
  unauthenticated public clients can read public-safe booking options and submit
  bounded idempotency-keyed requests into APOLLO booking truth, public rows carry
  source/channel truth, public submit creates no schedule block, and staff
  approval remains the only reservation-creating path without opening payment,
  quote, customer status portal, instant booking, Hestia/member booking UI,
  owner policy writes, admin-role widening, gateway work, HERMES widening, or
  deploy claims
- Phase 3B.7 customer status and communication is now real in repo/runtime on
  `main`: public submit returns an opaque public receipt code; unauthenticated
  status lookup by receipt code returns only customer-safe status, optional
  staff-authored public message, requested window, and update time; manager and
  owner staff can save the public message through trusted-surface APOLLO APIs;
  supervisors remain read-only; internal notes, request UUIDs, schedule block
  IDs, conflicts, staff IDs, trusted-surface fields, quote/payment fields,
  public availability calendars, instant booking, edit/rebook, AI/LLM
  negotiation, and deploy claims remain out
- Phase 3B.8 booking edit and replacement is now real in repo/runtime on
  `main`: manager/owner staff can edit only requested, under-review, or
  needs-changes requests through trusted-surface APOLLO APIs with
  `expected_version`; edits rerun APOLLO availability truth, increment version,
  preserve source/channel/public receipt truth, and create no schedule block;
  approved bookings are not mutated in place, and approved rebook creates a new
  requested replacement linked to the original with required idempotency-key
  semantics while approval remains the only reservation-creating path
- the current Milestone 2.0 hardening follow-up on `main`, now closed on
  `v0.19.1`, adds graceful
  shutdown plus HTTP/NATS/request bounds, keeps the shared parser as the only
  identified-lifecycle contract path, batches workout exercise list reads, and
  caps per-workout exercise writes without widening the product surface
- deployment truth is still narrower than the full product surface

## Release Lines

Tracer 24 remains tagged on `v0.15.0`, and `v0.15.1` remains the narrow
hardening patch on that same line. The current repo/runtime working line on
`main` is Phase 3B.8 booking edit and replacement over the closed Tracer 28
authz/staff-boundary truth, Milestone 2.0 hardening follow-up, scheduling
substrate, member-safe calendar, ops-read, approved booking lifecycle, and
public request/status lines.

| Release line | Intended purpose | Restrictions | What it should not do yet |
| --- | --- | --- | --- |
| historical `v0.6.1` note | Milestone 1.6 companion patch only if repo-local closure ever needs a backfill | treat this as historical closure context, not the active next line | do not present this as the active planned release line |
| `v0.14.0` | planner, exercise library, templates / loadouts, and richer profile inputs | keep the line backend/CLI-first and bounded | do not widen into meaningful frontend work, workout instantiation, or recommendation logic |
| `v0.15.0` | deterministic coaching substrate over planner, profile, and workout history | keep it deterministic, bounded, and side-effect free over planner truth | do not let visits, departures, profile state, or helper text become an opaque decision core |
| `v0.16.0` | conservative nutrition substrate with meal logging and calorie / macro ranges | keep it non-clinical and conservative | do not turn the product into a diet app or diagnostic engine |
| `v0.17.0` | explanation, summarization, bounded AI helper flows, and thin agent-facing helper surfaces | keep them subordinate to stable deterministic logic | do not let explanation become the core engine |
| `v0.18.0` | member presence, tap-link, and streak substrate over explicit visit truth | keep presence explicit and auditable | do not invent fake streak counters or silent visit inference |
| `v0.19.0` | role/authz, actor attribution, trusted-surface primitives, and staff runtime boundary substrate | keep authority explicit and reviewable | do not widen into polished ops product or speculative contracts |
| `v0.19.1` | Milestone 2.0 hardening follow-up for runtime boundaries, workout safety, and docs truth | keep the line patch-only and non-widening | do not add new member/staff product capability or deploy claims |
| later than `Phase 3B.1` | `Phase 3B.8 booking edit and replacement` on `main`: APOLLO-owned staff and public request persistence, staff APIs, public-safe options, idempotent public submit, opaque receipt/status lookup, separate public-safe customer messages, source/channel truth, availability decisions, approval-created linked schedule reservations, approved cancellation of those reservations, pending request edit, and approved replacement request lineage | keep members denied, supervisor read-only, manager/owner managed, trusted-surface gated for staff mutations, idempotency-keyed for public submit and staff rebook, versioned, request-first, and APOLLO-authoritative for conflict truth, receipt/status mapping, public messages, linked reservation cancellation, pending edit, and replacement lineage | do not widen into instant booking, public self-edit, public availability calendars, broader customer self-service/status portals, quotes/payments, in-place approved booking mutation, direct staff schedule controls, owner policy writes, admin role widening, AI/LLM negotiation, HERMES widening, gateway widening, or deploy claims |

## Current Phase 3B Line

Phase 3B.8 booking edit and replacement is now real in repo/runtime on `main`,
with deployed truth still separate and unchanged. Phase 3B.7 customer
status/message lookup remains the latest public-facing customer-safe line this
staff-side edit/replacement work builds on.

Any later widening should stay separate:

- broader APOLLO authz/admin widening only if a real product boundary needs it
- public availability calendar, public self-edit, broader customer self-service/status portal, and instant booking
- in-place approved-booking editing
- direct staff schedule controls unless a separate schedule-control line earns them
- public competition, rivalry, or social presentation
- staff shell, HERMES widening, gateway coupling, and deploy work

## Verified Audit Carry-Forward

The `2026-04-13` backend logic audit reran `go test -count=1 ./...` and
re-read the presence, workouts, competition, and runtime bootstrap paths before
narrowing the remaining APOLLO follow-up work.

| Area | Ruling | Next honest line |
| --- | --- | --- |
| streak active-status grace logic | verified correct as shipped; the current code keeps the full next UTC day active | leave runtime behavior unchanged and add an explicit grace-day regression test only if a bounded `v0.19.x` hardening patch reopens the presence line |
| streak reset `currentStartDay` flow | verified correct as shipped; reset already carries `creditDay` through the upsert path | leave runtime behavior unchanged and add a reset regression test if the presence line is reopened |
| competition standings tiebreak | narrowed: `SideIndex` is still the final deterministic tiebreak, but the deletion-and-recreation failure mode was not proven because team removal is draft-only and blocked once matches reference the team | keep runtime unchanged on the current line |
| identified-presence NATS handlers | verified low: handlers still derive their timeout from `context.Background()` instead of `serveCtx` | fold into the next bounded `v0.19.x` runtime hardening patch if APOLLO needs another one |
| query-instantiation cleanup | verified low and broader than workouts alone; multiple repos still construct `store.New(...)` per method, including workouts, membership, and coaching | treat as a narrow mechanical cleanup only when one of those repositories is already open for real behavior work |

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
- keep presence, tap-link, and streak truth facility-scoped in the current
  runtime; do not invent one global current facility when multiple facilities
  have distinct visit truth
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
