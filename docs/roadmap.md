# apollo Roadmap

## Objective

Keep APOLLO moving through narrow bounded release lines instead of trying to
jump straight to a broad product.

## Current Line

Current repo/runtime working line on `main`: Phase 3B.20.1 cohesion hardening
over Phase 3B.20 game identity, Phase 3B.19 public competition readiness, Phase
3B.18 internal social safety/reliability foundation, Phase 3B.17 internal
tournament runtime, and Phase 3B.16 competition analytics foundation,
Game Identity Policy Tuning Loop, Rating Policy Simulation / Golden Expansion,
Rating Policy Wrapper, Scale Gate Numeric Ceilings, CLI Demo Spine,
Co-presence / Private Daily Presence Gate,
Tracer 28 `v0.19.x`, Milestone 2.0 hardening
`v0.19.1`, Phase 3 shared substrate B, Phase 3A.1 member shell foundation,
Phase 3A.4 member-safe schedule calendar, Phase 3B.1 ops read foundation, and
Phase 3B.7 customer status/communication, Phase 3B.8 booking edit/replacement,
Phase 3B.9 public availability/request calendar, and Phase 3B.10 bounded staff
schedule-control lines

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
  instant booking, public self-edit/rebook, AI/LLM negotiation, and deploy
  claims remain out
- Phase 3B.8 booking edit and replacement is now real in repo/runtime on
  `main`: manager/owner staff can edit only requested, under-review, or
  needs-changes requests through trusted-surface APOLLO APIs with
  `expected_version`; edits rerun APOLLO availability truth, increment version,
  preserve source/channel/public receipt truth, and create no schedule block;
  approved bookings are not mutated in place, and approved rebook creates a new
  requested replacement linked to the original with required idempotency-key
  semantics while approval remains the only reservation-creating path
- Phase 3B.9 public availability/request calendar is now real in repo/runtime
  on `main`: unauthenticated public clients can read
  `/api/v1/public/booking/options/{optionID}/availability` with strict RFC3339
  windows and a 14-day max; APOLLO returns public-safe requestable windows plus
  generic closed/booked/unavailable blocks only; public submit remains
  request-only, creates no reservation, and staff approval remains the only
  confirmed booking path
- Phase 3B.10 bounded staff schedule-control support is now real in
  repo/runtime on `main`: APOLLO remains schedule authority, schedule reads
  return a manage hint for Themis role-aware controls, manager/owner schedule
  writes stay trusted-surface gated, generic schedule-block cancel uses
  `expected_version`, and booking-linked reservations can only be cancelled
  through the booking request lifecycle
- Phase 3B.12 competition lifecycle/result trust is now real in repo/runtime on
  `main`: APOLLO exposes canonical result identity, result/version status
  truth, recorded/finalized/disputed/corrected/voided lifecycle facts,
  correction supersession, direct and command-backed result transitions, and
  rating guards so only finalized or corrected canonical results feed rating
  paths; privileged live commands still require role capability plus
  trusted-surface proof, idempotency remains reported unsupported until a
  durable substrate exists, and deployed truth is unchanged
- Phase 3B.13 rating foundation is now real in repo/runtime on `main`:
  APOLLO extracts the current legacy rating math behind explicit
  `rating_engine`, `engine_version`, and `policy_version` identifiers, writes
  auditable legacy compute/policy/rebuild rating events, records
  `source_result_id`, `rating_event_id`, and deterministic
  `projection_watermark` data on rating projections, and preserves existing
  member rating read behavior while still consuming finalized/corrected
  canonical results only
- Phase 3B.14 OpenSkill dual-run is now real in repo/runtime on `main`:
  APOLLO computes OpenSkill comparison facts beside legacy rating outputs from
  the same finalized/corrected canonical results, records internal
  legacy/OpenSkill deltas and delta flags, and keeps the active member rating
  read path on the legacy projection
- Phase 3B.15 ARES v2 is now real in repo/runtime on `main`: APOLLO stores
  explicit competition queue intent facts, records internal match-preview
  proposal facts/events, computes match quality and predicted win probability
  server-side from trusted APOLLO projections, emits explicit explanation
  codes, and keeps ARES out of match lifecycle/result/rating ownership
- Phase 3B.16 competition analytics foundation is now real in repo/runtime on
  `main`: APOLLO stores internal derived stat events and analytics projections
  with explicit `stat_type`, `stat_value`, `source_match_id`,
  `source_result_id`, `sample_size`, `confidence`, `computed_at`,
  `projection_version`, and deterministic projection watermarks over
  finalized/corrected canonical results plus legacy rating facts only
- Phase 3B.17 internal tournament runtime is now real in repo/runtime on
  `main`: APOLLO stores staff/internal tournament containers, single-elimination
  bracket and seed facts, immutable locked team snapshots, match bindings,
  explicit advance reasons, audited round advancement facts, and tournament
  event facts over trusted APOLLO team/match/result truth only
- Phase 3B.18 internal social safety/reliability foundation is now real in
  repo/runtime on `main`: APOLLO stores competition-scoped report facts, block
  facts, reliability events, and safety audit events, exposes capability-gated
  manager-only readiness/review reads, and keeps these private facts out of
  public/member projections and canonical competition result/rating/analytics/
  ARES/tournament truth
- Phase 3B.19 public competition readiness is now real in repo/runtime on
  `main`: APOLLO exposes public-safe readiness and leaderboard projection
  contracts over finalized/corrected canonical result truth plus legacy active
  rating fields, while private/internal safety, manager, command, OpenSkill
  comparison, ARES proposal, tournament ops, source-result, and projection
  watermark facts stay non-public
- Phase 3B.20 game identity is now real in repo/runtime on `main`: APOLLO
  exposes public/member-safe CP, badge award, rivalry state, and squad identity
  projections over public-safe competition projection rows only, with explicit
  policy versions and no UI-owned game formulas
- Rating Policy Wrapper is now real in repo/runtime on `main`: APOLLO active
  legacy-engine rating projection uses `apollo_rating_policy_wrapper_v1`,
  records calibration/provisional/ranked status, applies the fifth-match ranked
  transition, applies inactivity sigma inflation without mu movement, bounds
  positive mu movement, and exposes member-safe additive policy metadata while
  public surfaces remain redacted and OpenSkill remains comparison-only
- Rating Policy Simulation / Golden Expansion is now real in repo/runtime on
  `main`: APOLLO has deterministic local fixtures and CLI JSON output for
  active wrapper scenarios, legacy baseline deltas, OpenSkill sidecar deltas,
  accepted/rejected classification, cutover blockers, and policy risks while
  OpenSkill remains comparison-only
- Frontend Route/API Contract Matrix is now real as docs truth only: the
  platform matrix enumerates current Hestia/Themis routes, APOLLO contract
  consumption, same-origin proxy boundaries, auth/role/state behavior,
  production-backed versus mock/stub status, test coverage, and blocked
  adjacent scope while runtime and deployed truth remain unchanged
- Game Identity Policy Tuning Loop is now real in repo/runtime on `main`:
  APOLLO has deterministic fixtures and `apollo competition game-identity
  tuning --format json` for current CP weights, badge thresholds, rivalry
  activation, squad aggregation, accepted/rejected findings, policy risks, and
  blockers. Optional DB-backed local projection-row analysis is implemented,
  but this worker environment had no `APOLLO_DATABASE_URL`, so no real local DB
  population evidence is claimed. Active game identity policy versions and
  public/member output behavior remain unchanged.
- the current Milestone 2.0 hardening follow-up on `main`, now closed on
  `v0.19.1`, adds graceful
  shutdown plus HTTP/NATS/request bounds, keeps the shared parser as the only
  identified-lifecycle contract path, batches workout exercise list reads, and
  caps per-workout exercise writes without widening the product surface
- deployment truth is still narrower than the full product surface

## Launch Expansion Source Of Truth

The current roadmap describes shipped repo/runtime lines and the bounded
Phase 3B schedule/booking fork. For APOLLO's next competition/rating/tournament
expansion, [`launch-expansion-audit.md`](launch-expansion-audit.md) is the
active operating doc.

That audit consolidates the older public competition, rivalry, badge, and
tournament ideas into one gated plan. Current scan state:

- closed: docs truth, CLI parity, capability/readiness checks, dry-run/apply
  command foundation, legacy rating metadata, OpenSkill dual-run comparison,
  ARES v2 proposal facts, internal analytics, internal tournaments,
  manager/internal safety, public-safe readiness/leaderboards, and
  public/member-safe game identity projections
- closed by Milestone 3.0: bounded APOLLO/ATHENA deploy smoke, APOLLO metrics
  export, Prometheus scrape proof, and cross-repo compatibility matrix
- closed locally in repo/runtime: Scale Gate numeric ceilings for rating
  recompute, public readiness, public leaderboard projections, game identity
  projections, and CLI/API smoke. This is not full production load validation,
  and deployed truth is unchanged.
- closed locally in repo/runtime: CLI Demo Spine. APOLLO CLI now exposes
  service-backed public readiness, public leaderboards, public/member game
  identity, member stats/history, safety readiness/review, session/tournament
  reads, command dry-run/apply, result lifecycle, and ARES preview generation
  without frontend dependency or CLI-owned formulas. This does not change
  deployed truth.
- closed locally in repo/runtime: Rating Policy Wrapper. APOLLO active
  legacy-engine rating projection now records `apollo_rating_policy_wrapper_v1`,
  calibration status, fifth-match ranked transition, inactivity sigma
  inflation, and positive movement cap metadata. OpenSkill remains
  comparison-only, public/member reads do not expose OpenSkill comparison
  facts, and deployed truth is unchanged.
- closed locally in repo/runtime: Rating Policy Simulation / Golden Expansion.
  APOLLO now has deterministic fixture simulation, `apollo competition rating
  simulation --format json`, accepted/rejected scenario classification, legacy
  baseline deltas, OpenSkill sidecar deltas, cutover blockers, and policy risk
  output. This does not change deployed truth.
- closed as docs truth only: Frontend Route/API Contract Matrix. Hestia/Themis
  route/API consumption, proxy boundaries, `/api/v1/public/*` denial, auth/role
  states, empty/error/denied states, production/mock status, current tests, and
  APOLLO source-truth ownership are documented in the platform matrix. This
  does not change runtime or deployed truth.
- closed locally in repo/runtime: Game Identity Policy Tuning Loop. APOLLO now
  has deterministic fixture evaluation and `apollo competition game-identity
  tuning --format json` for CP weights, badge thresholds, rivalry activation,
  squad aggregation, accepted/rejected findings, policy risks, blockers, and
  optional DB-backed local projection-row analysis. Active policy versions and
  public/member output behavior are unchanged; deployed truth is unchanged.
- closed locally in ATHENA repo/runtime: ATHENA Real Ingress Bridge. `athena
  edge ingress-bridge` now classifies ATHENA source/accepted presence,
  identity presence, scope, timestamps, projection replay reasons, and
  source-pass session facts for future APOLLO co-presence, private daily
  presence, and reliability gates. APOLLO runtime and deployed truth are
  unchanged.
- closed locally in APOLLO repo/runtime: Co-presence / Private Daily Presence
  Gate. `apollo presence athena-gate --bridge-report <path> --format json|text`
  classifies ATHENA bridge JSON into APOLLO-owned internal eligibility signals
  for future co-presence and private daily presence/check-in readiness. ATHENA
  remains physical truth owner, APOLLO owns product eligibility
  classification, and no visits, tap-links, streaks, XP, teams, reliability
  scoring, public/member routes, public APIs, frontend UI, or deployed truth
  changed.
- closed as platform docs/runbook truth only: Live Destructive Probe Plan.
  Future APOLLO/ATHENA live mutation and SIGTERM proof now has fixture gates,
  abort criteria, evidence ledger, rollback expectations, and command
  skeletons. No live destructive probe, rollout restart, pod kill, DB write,
  APOLLO runtime change, deploy/GitOps change, or deployed-truth change
  happened.
- still deferred: OpenSkill active read path, public tournaments, public/member
  safety UI details, messaging/chat, broad public social graph, public profiles
  and scouting, production historical rating backtesting, recurring schedule
  policy, court/resource splitting, booking/commercial proposal workflows, and
  project-wide SemVer governance

Historical tracer entries in this roadmap remain evidence of what shipped.
They should not be used as permission to skip the launch-expansion gates.

## Competition CLI Demo Spine

Repo/local runtime truth: agents can prove the competition spine without a
frontend by running the focused APOLLO CLI smoke:

```sh
cd /Users/zizo/Personal-Projects/ASHTON/apollo
go test -count=1 ./cmd/apollo -run TestCompetitionCLIDemoProjectionSafetyAndPreviewReads
```

Manual CLI demos require `APOLLO_DATABASE_URL` to point at a migrated local
APOLLO database with real session/member/actor rows. The supported read map is:

- `apollo competition public readiness`
- `apollo competition public leaderboard`
- `apollo competition public game-identity`
- `apollo competition member stats --user-id <member-user-id>`
- `apollo competition member history --user-id <member-user-id>`
- `apollo competition member game-identity --user-id <member-user-id>`
- `apollo competition command readiness`
- `apollo competition command run`
- `apollo competition session list/show`
- `apollo competition tournament list/show`
- `apollo competition safety readiness/review`

The CLI path routes through APOLLO services and command handlers. It does not
own rating, analytics, ARES, safety, public projection, or game-identity
formulas.

## Rating Policy Wrapper

Repo/local runtime truth: APOLLO active rating rebuilds now route through
`rating.RebuildActivePolicy`, which keeps the legacy engine active while
recording `apollo_rating_policy_wrapper_v1` as the active policy version.

The wrapper covers focused policy behavior only:

- first four rated matches are provisional
- the fifth rated match transitions to ranked
- inactivity after the configured threshold inflates sigma only, capped at the
  initial sigma
- positive mu movement is bounded by the explicit climbing cap
- member stats expose additive policy metadata through APOLLO-owned service and
  CLI/API paths

OpenSkill remains internal comparison-only and is not used by active public or
member rating reads. This is not deployed truth and not production historical
backtesting proof.

## Rating Policy Simulation / Golden Expansion

Repo/local runtime truth: APOLLO now has deterministic fixture proof through
`rating.BuildActivePolicySimulationReport` and
`apollo competition rating simulation --format json`.

The simulation proof covers:

- active wrapper output from `rating.RebuildActivePolicy`
- unchanged legacy baseline output from `rating.RebuildLegacy`
- OpenSkill sidecar deltas from `rating.RebuildOpenSkillComparison`
- accepted scenarios for unranked 1v1, stronger/new expected result, upset,
  fifth-match transition, inactivity, climbing cap, draw, 5v5, OpenSkill
  sidecar rows, and public/member read safety
- rejected 3v5 asymmetric active-policy/cutover evidence, retained only as
  comparison stress
- cutover blockers and policy risks

OpenSkill active read path, rollback/cutover mechanics, production historical
backtesting, public ratings, and public tournaments remain deferred.

## Game Identity Policy Tuning Loop

Repo/local runtime truth: APOLLO now has deterministic fixture proof through
`competition.BuildGameIdentityPolicyTuningReport` and
`apollo competition game-identity tuning --format json`.

The tuning proof covers:

- active CP weights: matches played 10, wins 30, draws 15, losses 5
- active badge thresholds: first match at 1 match, first win at 1 win, regular
  competitor at 5 matches
- rivalry activation: top two CP rows inside the same sport/mode/facility/
  team-scope context, active when CP gap is <= 50, emerging above that gap
- squad aggregation: participant count and CP total by sport/mode/facility/
  team-scope context
- accepted findings to keep current v1 policies unchanged
- rejected findings against active retuning or broader identity/social surface
  expansion without stronger evidence
- policy risk table and blockers before broader identity, social, or tournament
  work
- optional `--include-db` local projection-row analysis when
  `APOLLO_DATABASE_URL` points at a migrated APOLLO database

This worker environment had no `APOLLO_DATABASE_URL`, so no real local DB
population evidence is claimed. Active policy versions and public/member output
behavior remain unchanged.

## Release Lines

Tracer 24 remains tagged on `v0.15.0`, and `v0.15.1` remains the narrow
hardening patch on that same line. The current repo/runtime working line on
`main` is Phase 3B.20 game identity over Phase 3B.19 public competition
readiness, Phase 3B.18 internal social safety/reliability foundation, Phase
3B.17 internal tournament runtime, Phase 3B.16 competition analytics
foundation, Phase 3B.13 rating foundation, Phase 3B.14 OpenSkill comparison
evidence, Rating Policy Wrapper, Rating Policy Simulation / Golden Expansion,
Game Identity Policy Tuning Loop, Phase 3B.15 ARES v2 proposal foundation, and
the closed
Tracer 28 authz/staff-boundary truth,
Milestone 2.0 hardening follow-up, scheduling substrate, member-safe calendar,
ops-read, approved booking lifecycle, public request/status/availability lines,
and staff-side edit/replacement plus bounded staff schedule-control lines.

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
| later than `Phase 3B.1` | `Phase 3B.10 bounded staff schedule controls` on `main`: APOLLO-owned booking request truth plus bounded internal schedule-control support over typed schedule blocks | keep members denied, supervisors read-only, manager/owner writes trusted-surface gated, public availability sanitized, and booking-linked reservations cancellable only through booking requests | do not widen into instant booking, public self-edit/rebook, broader customer self-service/status portals, quotes/payments, in-place approved booking mutation through schedule controls, recurring schedule rules, broad hours policy editing, owner policy writes, admin role widening, AI/LLM negotiation, HERMES widening, gateway widening, or deploy claims |
| `Phase 3B.11` | competition command foundation on `main`: shared APOLLO competition command/outcome DTOs, readiness/capability checks, dry-run plan shape, and service-backed CLI parity over existing competition behavior | keep APOLLO as competition truth, keep Themis as a consumer, preserve existing authz/trusted-surface boundaries, and report unsupported idempotency/version behavior explicitly | closed by 3B.12 result trust; do not widen into OpenSkill, analytics, tournament runtime, public competition surfaces, Hestia member/public expansion, CP, badges, rivalry, squads, browser trusted-surface tokens, proposal workflow, booking, or deploy claims |
| `Phase 3B.12` | competition lifecycle/result trust on `main`: canonical result identity, result statuses, dispute status, correction supersession, direct and command-backed result transitions, lifecycle events, and finalized/corrected-only rating guards | keep APOLLO as canonical result truth, keep Themis as a consumer, preserve authz/trusted-surface/version boundaries, and keep corrections additive/auditable | do not widen into rating engine extraction, OpenSkill, ARES v2, analytics, tournament runtime, public competition surfaces, Hestia member/public expansion, CP, badges, rivalry, squads, browser trusted-surface tokens, proposal workflow, booking, or deploy claims |
| `Phase 3B.13` | legacy rating foundation on `main`: current APOLLO rating math extracted behind explicit engine/policy versions, golden cases, rating compute/policy/rebuild events, source result binding, rating event IDs, and deterministic projection watermarks | keep current public/member rating reads unchanged, keep APOLLO as rating truth, and derive projections only from finalized/corrected canonical results | do not widen into OpenSkill, ARES v2, analytics, tournament runtime, public competition surfaces, Hestia member/public expansion, CP, badges, rivalry, squads, proposal workflow, booking, or deploy claims |
| `Phase 3B.14` | OpenSkill dual-run comparison on `main`: OpenSkill comparison values, internal audit rows/events, accepted delta budgets, delta flags, and deterministic rebuilds over finalized/corrected canonical result truth | keep the legacy projection as the active read path, keep comparison facts internal, and preserve APOLLO as rating truth | do not widen into OpenSkill read-path switch, ARES v2, analytics, tournament runtime, public competition surfaces, Hestia member/public expansion, CP, badges, rivalry, squads, proposal workflow, booking, or deploy claims |
| `Phase 3B.15` | ARES v2 proposal/match-preview foundation on `main`: explicit queue intent facts, deterministic internal match previews, match quality, predicted win probability, and explanation codes over trusted APOLLO projections | keep ARES proposal-only, keep APOLLO as preview fact owner, keep legacy rating read path active, and keep Themis/Hestia as consumers only if separately changed | competition analytics closes separately in 3B.16; do not widen into OpenSkill read-path switch, dashboard-first analytics, public profiles/stats/scouting, carry coefficient, tournament runtime, public competition surfaces, Hestia member/public expansion, CP, badges, rivalry, squads, proposal workflow, booking, or deploy claims |
| `Phase 3B.16` | competition analytics foundation on `main`: internal stat events and analytics projections over finalized/corrected canonical results plus legacy rating facts | keep analytics internal, derived, deterministic, versioned, and separate from UI/public truth | do not widen into dashboards, public profiles/stats/scouting, carry coefficient, OpenSkill read-path switch, tournament runtime, public competition surfaces, Hestia member/public expansion, CP, badges, rivalry, squads, proposal workflow, booking, or deploy claims |
| `Phase 3B.17` | internal tournament runtime on `main`: staff-only tournament containers, single-elimination bracket/seed/team-snapshot/match-binding/advancement facts, explicit advance reasons, and tournament events over trusted APOLLO competition truth | keep tournaments internal, staff-run, additive, audited, and bound to finalized/corrected canonical result truth only | do not widen into public tournaments, Hestia member/public competition expansion, booking/commercial/proposal workflow, OpenSkill read-path switch, ARES behavior changes, dashboard-first analytics, CP, badges, rivalry, squads, or deploy claims |
| `Phase 3B.18` | internal social safety/reliability foundation on `main`: competition-scoped report facts, block facts, reliability events, safety audit events, manager-only readiness/review reads, and aligned safety/reliability commands | keep safety and reliability manager/internal, capability-gated, auditable, immutable, and separate from canonical competition truth | do not widen into public/member safety UI, messaging/chat, public profiles/scouting/leaderboards/tournaments, Hestia expansion, CP, badges, rivalry, squads, OpenSkill read-path switch, ARES behavior changes, analytics dashboards, booking/commercial/proposal workflow, SemVer governance, or deploy claims |
| `Phase 3B.19` | public competition readiness on `main`: public-safe readiness and leaderboard projection contracts over finalized/corrected canonical result truth plus legacy active rating fields | keep public contracts projection-only, redacted, deterministic, and separate from private/internal truth | do not widen into public tournaments, CP, badges, rivalry, squads, messaging/chat, public social graph, OpenSkill read-path switch, ARES proposal exposure, analytics dashboards, booking/commercial/proposal workflow, SemVer governance, or deploy claims |
| `Phase 3B.20` | game identity layer on `main`: public/member-safe CP, badge award, rivalry state, and squad identity projection contracts over public-safe competition projection rows | keep APOLLO as game identity owner, keep policies explicit/versioned, keep Hestia as consumer only, and keep outputs redacted or self-scoped | do not widen into messaging/chat, broad public social graph, public safety detail exposure, OpenSkill read-path switch, public tournaments, booking/commercial/proposal workflow, project-wide SemVer governance, fake UI data, or deployment claims |
| Rating Policy Wrapper | post-CLI Demo APOLLO rating policy wrapper on `main`: active legacy-engine projection records `apollo_rating_policy_wrapper_v1`, calibration status, fifth-match ranked transition, inactivity sigma inflation, and climbing-cap metadata | keep legacy engine active, keep OpenSkill comparison-only, keep public outputs redacted, and keep deployed truth separate | closed by Rating Policy Simulation; do not claim OpenSkill cutover, production backtesting, public tournaments, frontend completion, or deployed truth |
| Rating Policy Simulation / Golden Expansion | post-wrapper APOLLO rating proof on `main`: deterministic fixtures, CLI JSON output, accepted/rejected scenarios, legacy baseline deltas, OpenSkill sidecar deltas, cutover blockers, and policy risks | keep proof local/CLI, keep OpenSkill sidecar-only, keep public/member reads allowlisted, and keep deployed truth separate | do not claim OpenSkill cutover, production backtesting, public ratings, public tournaments, frontend completion, or deployed truth |
| Frontend Route/API Contract Matrix | post-rating-proof docs truth: Hestia/Themis route/API consumption, proxy denials, auth/role/state behavior, production/mock status, tests, and APOLLO ownership are frozen in the platform matrix | keep this docs-only, keep APOLLO source truth, keep Hestia/Themis as consumers, and require future frontend packets to update the matrix | do not claim runtime changes, generated contract enforcement, deployed frontend proof, public tournaments, OpenSkill active reads, frontend-owned formulas, public/member safety UI, or new product surface |
| Game Identity Policy Tuning Loop | APOLLO game identity proof on `main`: deterministic fixtures, CLI JSON/text output, accepted/rejected findings, policy risks, blockers, and optional DB-backed local projection-row analysis for CP, badges, rivalry, and squads | keep APOLLO as game identity policy owner, keep active v1 policy behavior unchanged unless stronger evidence justifies a version bump, and keep deployed truth separate | do not claim production population backtesting, active retuning, public tournaments, broad social graph, persistent guilds, frontend-owned formulas, or deployed truth |
| ATHENA Real Ingress Bridge | ATHENA local/runtime proof on `main`: CLI JSON/text report over source/accepted presence, identity presence, scope, timestamps, projection replay reasons, and source-pass sessions for future APOLLO trust gates | keep ATHENA as physical truth owner, keep APOLLO product behavior unchanged, keep source truth separate from accepted-presence truth, and keep deployed truth separate | do not claim persistent teams, XP ledger, reliability scoring, accepted-presence session cutover, frontend routes, public API, schema/proto changes, or live DB proof |
| Co-presence / Private Daily Presence Gate | APOLLO local/runtime proof on `main`: `apollo presence athena-gate` reads ATHENA bridge JSON and emits internal co-presence/private daily presence eligibility classification | keep ATHENA as physical truth owner, keep APOLLO as product eligibility owner, keep this read-only and CLI-first, and keep deployed truth separate | do not mutate visits, tap-links, streaks, XP, teams, reliability, public/member routes, public APIs, frontend UI, schema/proto, or deploy truth |
| Live Destructive Probe Plan | platform docs/runbook truth: future APOLLO/ATHENA live mutation and SIGTERM proof is planned with canary/fixture gates, abort criteria, evidence ledger, rollback expectations, and command skeletons | keep this planning-only until a separate operator-approved execution gate runs | do not claim live destructive proof, mutate deployed data, change runtime, touch frontends, switch OpenSkill, or unblock public tournaments |
| launch expansion audit | post-current APOLLO competition/rating/tournament/social expansion | follow [`launch-expansion-audit.md`](launch-expansion-audit.md) gates and packet order | do not jump directly to OpenSkill cutover, public tournaments, public safety UI, public profiles/scouting, broad social graph behavior, or broader public leaderboard/game-identity expansion |

## Current Phase 3B Line

Phase 3B.20 game identity is now real in repo/runtime on `main`, and Phase
3B.20.1 has hardened its cohesion. Milestone 3.0 then proved bounded APOLLO/
ATHENA deploy smoke, APOLLO metrics export, Prometheus scrape proof, and the
cross-repo compatibility matrix; Hestia, Themis, and `ashton-mcp-gateway`
remain recorded as repo-only in the inspected environment. APOLLO exposes
public/member-safe game identity contracts that consume public-safe competition
projection rows only: CP, badge award facts, rivalry state facts, and squad
identity facts. Rating Policy Wrapper and Rating Policy Simulation / Golden
Expansion are also closed locally in repo/runtime: the active legacy-engine
rating projection records `apollo_rating_policy_wrapper_v1`, calibration
status, inactivity sigma inflation, and climbing-cap metadata, and deterministic
simulation/CLI proof records active wrapper scenarios, legacy deltas, OpenSkill
sidecar deltas, accepted/rejected classification, blockers, and risks while
OpenSkill remains comparison-only. Frontend Route/API Contract Matrix is closed
as docs truth only in the platform matrix; it records current Hestia/Themis
route/API consumption and does not change APOLLO runtime or deployed truth.
Game Identity Policy Tuning Loop is closed locally in repo/runtime: APOLLO now
has deterministic CP/badge/rivalry/squad policy fixtures, CLI JSON/text output,
accepted/rejected findings, policy risks, blockers, and optional DB-backed
local projection-row analysis while active policy versions remain unchanged.
Co-presence / Private Daily Presence Gate is closed locally in repo/runtime:
APOLLO now has `apollo presence athena-gate` over ATHENA bridge JSON for
internal co-presence and private daily presence/check-in readiness
classification. ATHENA remains physical truth owner, APOLLO owns product
eligibility classification, and the proof does not mutate visits, tap-links,
streaks, XP, teams, reliability, public/member routes, public APIs, frontend
UI, or deployed truth.
Live Destructive Probe Plan is closed as platform docs/runbook truth only:
future APOLLO/ATHENA live mutation and SIGTERM proof has a plan, but no live
destructive probe, rollout restart, pod kill, DB write, APOLLO runtime change,
deploy/GitOps change, or deployed-truth change happened.
The contracts redact public participant identity, scope member
output to the caller, keep rivalry and labels scoped to their projection
row/context, and exclude private/internal safety, manager, command, OpenSkill
comparison, ARES proposal, tournament ops, source-result, projection watermark,
sample/confidence metadata, and operational truth.

Any later widening should stay separate:

- broader APOLLO authz/admin widening only if a real product boundary needs it
- OpenSkill read-path switch only after production backtesting, rollback, and
  cutover evidence is accepted
- public tournaments
- public/member safety UI
- carry coefficient and broader scouting/profile analytics until separate gates
- public self-edit/rebook, broader customer self-service/status portal, and instant booking
- in-place approved-booking editing
- recurring schedule rules, broad operating-hours editing, and owner policy controls
- messaging/chat or broad public social presentation
- booking/commercial/proposal workflows and project-wide SemVer governance
- public competition expansion must follow `launch-expansion-audit.md`
- full staff shell/product widening, HERMES widening, gateway coupling, and
  Hestia/Themis/gateway deployment work

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

APOLLO now follows bounded pre-`1.0.0` tag discipline for repo-local release
lines. Project-wide SemVer governance remains deferred.

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
- treat 3B.19 public readiness/leaderboards and 3B.20 game identity as separate
  derived projection layers over competition truth, not as permission to push
  public/social behavior into the competition-history runtime itself
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
- post-current launch expansion: see `launch-expansion-audit.md`; its T18a-T57
  packet labels are launch-expansion labels, not replacements for the
  historical Tracer 18-T28 closure records above
