# Growing Pains

Use this document to record real product mistakes, schema changes, recommendation
failures, matchmaking edge cases, and the fixes that made `apollo` more realistic.

## How To Read This Document

This file has two layers:

- The summary tables below capture the recurring engineering pressures,
  architectural pivots, and process changes that shaped APOLLO as the platform
  widened from tracer-scale proofs into cross-repo operational product work.
- The dated sections below remain the detailed incident ledger and should stay
  intact as historical evidence.

Use the tables for the high-signal narrative. Use the dated chronology for the
specific symptom, cause, fix, and rule that closed a given mistake.

## Milestone Pivots

| Pivot | Pressure | Adaptation | Lasting rule |
| --- | --- | --- | --- |
| Tracers to bounded packets | Isolated proof work stopped scaling once booking, status, schedule, and staff flows crossed repos | Shift from tracer closure to bounded `3A.x` / `3B.x` packets with hard stops, verification, closeout truth, and post-packet hardening | Once multiple repos or surfaces move together, treat the work as a bounded packet, not a loose tracer |
| Feature momentum to product-spine discipline | "One more feature" kept widening APOLLO, Hestia, Themis, and docs together | Organize work around the request/status/edit/rebook/availability/schedule spine instead of isolated feature wins | Sequence product around durable operational flows, not around attractive isolated capabilities |
| Public booking idea to request-first truth | Intake, status, and availability pressure naturally drifted toward self-booking semantics | Keep public booking request-first; staff approval remains the only path that creates confirmed reservation truth | Public affordances may inform a request, but must not imply confirmed booking unless the backend actually confirms it |
| A/B split to domain-driven stream ownership | Customer-facing booking work no longer fit a simple frontend/backend split | Treat booking/status/availability as part of the B-stream booking/ops spine and park A as the earlier member-shell line | Stream ownership should follow domain truth, not UI/backend stereotypes |
| Competition ambition to internal ops first | APOLLO had real competition runtime, but no internal ops shell and no trust substrate for public stakes | Reframe near-term tournament work as Themis competition ops over APOLLO runtime, with public tournament/social surfaces gated later | Real backend competition truth does not imply public tournament readiness |
| Launch expansion to trust-gated expansion | Ratings, badges, rivalry, leaderboards, and public tournament ideas outran correction/privacy/scale controls | Introduce the launch-expansion audit as the governing sequence for post-current competition/rating/tournament/social work | Public excitement mechanics come after trust substrate, not before |

## Recurring Failure Modes And Adjustments

| Failure mode | Where it showed up | Adjustment | Engineering lesson |
| --- | --- | --- | --- |
| Partial idempotency | Public intake closed first; staff create and Themis forwarding lagged behind | Standardize key lifecycle: generate at render, carry in form, preserve on failure, forward on submit, replay-test end-to-end | Idempotency is a cross-surface contract, not a backend-only feature |
| Duplicate-submit UI | Decision, create, edit, rebook, and schedule flows | Add pending/disabled state as a standard hardening expectation | UI submission state is part of correctness, not polish |
| Repo truth drift from docs truth | Audit findings, release ladders, repo status claims, and route ownership | Tighten README/roadmap/brief/board alignment and treat docs as operating truth | Stale docs create real architecture mistakes |
| "Done locally" drift | Work described as done before commit/push/docs closeout | Require pushed SHAs, final git status, repo truth, deployed truth, and deferred truth in closeout | Implementation is not done until the operational truth is closed |
| Dirty workspace ambiguity | Planning and worker handoffs | Add Gate 0 repo-state checks before serious work | Packet work starts from clean known truth or it starts from noise |
| Cross-repo cohesion gaps | Booking/status/schedule packets landing across APOLLO, Themis, Hestia, and platform docs | Add `.1` hardening passes that patch only real cohesion gaps | Multi-repo features often need a verification-first cleanup pass |
| Planning by memory | Deferred work, packet boundaries, and UI expectations | Add `DEFERMENTS.md` and `FLOWS.md` as active coordination artifacts | Memory does not scale; the control plane needs explicit registers |
| Scope drift through naming ambiguity | Tracer vs packet, A vs B, launch vs runtime, milestone vs sub-phase | Standardize packet format and stream ownership language | Naming is part of execution discipline, not cosmetic metadata |

## Truth-Model Lessons

| Lesson | Trigger | Architectural change | Guardrail |
| --- | --- | --- | --- |
| Public-safe truth needs its own surface | Receipt/status/message/availability work | Add explicit public-safe booking reads instead of filtering internal objects ad hoc | Never expose internal booking objects directly to public surfaces |
| Request truth and reservation truth are different layers | Booking lifecycle work | Keep request rows, approval-created reservation blocks, and public receipt/status truth separate | A request is not a reservation |
| Approved-change behavior needs explicit policy | Edit/rebook and schedule-control work | Use replacement flows and bounded schedule actions instead of casually mutating approved reservation truth | Do not widen into in-place approved mutation without a dedicated policy line |
| Authority model must drive workflow shape | APOLLO authz showed supervisor live-manage vs manager/owner structure-manage splits | Design supervisor proposal rails instead of silently widening operational authority | Role boundaries should shape the workflow, not be bypassed by UI convenience |
| Substrate problems can masquerade as UI asks | Court splitting, recurrence, and operating-hours editing | Reclassify these as schedule/resource modeling work instead of shell-level polish | Do not fake substrate gaps in UI |
| Competition runtime does not equal a public competition product | APOLLO already owns sessions, teams, matches, results, standings, and ratings | Separate internal competition ops from public tournament/social expansion | Backend capability alone is not launch permission |
| Repo truth does not equal deployed truth | ATHENA and later cross-repo planning | Report runtime, repo, and deploy truth separately in closeout and planning docs | Never collapse code state and live state into one claim |

## Process Changes That Stuck

| Change | Why it happened | What changed |
| --- | --- | --- |
| Planning chat / worker chat split | This chat crossed from planning into implementation and contaminated packet flow | Planning chats now own strategy, packet decomposition, and runtime contract reasoning; worker chats own UI execution, implementation, verification, closeout, commits, and pushes |
| Standard four-prompt worker packet | Ad hoc worker setup caused duplication, missed gates, and muddy closeouts | Every substantial packet now ships with Gate 0, backend prompt, surface prompt, docs/closeout prompt, verification matrix, and commit ladder |
| `.1` cohesion hardening | Functionally correct packets still landed with state, copy, or wiring gaps | Short verification-first hardening passes became normal after messy cross-repo packets |
| Docs as control plane | README/roadmap drift repeatedly produced stale assumptions | README, roadmap, deferments, flows, repo briefs, and implementation board now act as operating artifacts, not passive notes |
| Watch-later demotion for speculative scaling | Some thresholds were reasonable but not measured | Scalability notes moved out of "triggers" and into watch-later guidance unless backed by measured pressure |
| Hidden activation for AI/commercial ideas | AI and payment ideas risked muddying runtime priorities | Treat AI/commercial lines as planning-first or hidden activation unless they directly strengthen the current product spine |

## Open Frictions

| Area | Friction | Why it matters | Next move | Guardrail |
| --- | --- | --- | --- | --- |
| Competition ops | APOLLO owns real competition runtime, but Themis still lacks the internal competition shell that would make the system demo legible | Backend truth exists, but operator truth is still too implicit | Build internal competition ops over existing APOLLO contracts before any public tournament widening | Do not invent a second competition model in the frontend |
| Supervisor authority | The authz model already implies proposal-style workflows, but that workflow is not yet first-class | Without explicit proposal rails, supervisor capability either stays underused or widens informally | Add bounded supervisor proposal workflow over real contracts | Do not widen supervisor mutation authority by implication |
| Scheduling substrate | Recurrence, operating-hours editing, and court/resource splitting remain real substrate gaps | These will keep reappearing as "small UI asks" until the substrate is explicit | Treat them as dedicated schedule/resource packets | Do not paper over substrate gaps with shell-only affordances |
| Demo readiness | Runtime breadth has advanced faster than demo packaging and hosted proof | The system can feel less complete than it is | Treat demo packaging as its own track | Do not confuse runtime progress with demo readiness |

## 2026-04-01

- Member auth, identity linkage, and matchmaking intent were initially too easy
  to conflate. The fix was to document them as three separate concerns before
  implementation started.

- The first visit schema relied only on `source_event_id` dedupe, which still
  allowed separate arrival events to open multiple concurrent visits for the
  same member and facility. The fix was to add a partial unique index for open
  visits on `(user_id, facility_key)` while keeping workout history separate.

## 2026-04-02

- Symptom: deriving lobby behavior from the normalized profile read path would
  have silently treated corrupted persisted enum values as safe defaults.
  Cause: profile reads intentionally coerce sparse or invalid preferences to
  predictable fallback values, but eligibility needs to distinguish missing
  state from bad stored state.
  Fix: centralize preference-mode parsing, keep sparse fields deterministic, and
  surface invalid persisted `visibility_mode` or `availability_mode` as
  explicit ineligible reasons.
  Rule: when member state drives behavior instead of just display, invalid
  stored intent must be surfaced deterministically instead of being silently
  normalized away.

- Symptom: APOLLO still owned a private event contract even though
  `ashton-proto` already defined the identified-arrival schema and payload.
  Cause: the first Tracer 2 pass validated behavior, but the consumer tests and
  runtime parser still depended on hand-written JSON shapes.
  Fix: switch the consumer onto the shared `ashton-proto/events` helper and use
  shared fixture and helper-generated bytes in unit and integration tests.
  Rule: if APOLLO consumes a cross-repo event, the active wire contract must be
  imported or enforced from `ashton-proto`, not recopied locally.

- Symptom: APOLLO temporarily kept a second, looser identified-arrival parse
  path so empty-hash payloads could be ignored before the shared schema ran.
  Cause: the original tracer rule treated anonymous misroutes as a local
  no-op, but that widened APOLLO away from the active `ashton-proto` contract.
  Fix: Milestone 2.0 removed the pre-parse branch and made the shared parser
  the only active identified-lifecycle contract path; empty-hash payloads now
  fail as schema violations before any visit mutation.
  Rule: cross-repo lifecycle subjects get one active parse path; if the shared
  contract rejects the payload, APOLLO should not invent a parallel looser
  parser for the same subject.

- Symptom: adding `users.email_verified_at` to the generated auth/profile query
  surface broke older APOLLO integration tests even though the new runtime code
  was correct.
  Cause: multiple tests were still booting only the Tracer 2 migration set, so
  generated queries referenced columns that did not exist in those temporary
  databases.
  Fix: centralize APOLLO test schema bootstrapping through one helper that
  applies the full active migration stack.
  Rule: if generated queries depend on new columns, every integration test must
  boot the full current schema, not a stale subset of migrations.

- Symptom: Docker-backed Postgres tests hung on this machine before the first
  auth integration assertion even ran.
  Cause: the shared dockertest helper assumed `host.docker.internal` existed,
  but that hostname did not resolve in this local environment.
  Fix: make the helper detect whether `host.docker.internal` resolves and fall
  back to `127.0.0.1`, then add a regression test around the resolver path.
  Rule: local integration helpers must detect platform-specific Docker host
  differences instead of assuming Docker Desktop DNS behavior.

- Symptom: the local auth smoke could receive the correct `Secure` session
  cookie, but a plain HTTP cookie jar would not send it back to `/api/v1/profile`.
  Cause: curl and browsers correctly refuse to replay `Secure` cookies over
  non-TLS HTTP.
  Fix: keep the production cookie policy intact and document local smoke to send
  the cookie explicitly or terminate TLS in front of APOLLO.
  Rule: do not weaken real auth security semantics just to make localhost smoke
  more convenient.

- Symptom: the first Tracer 5 end-to-end departure test consumed the shared
  `athena.identified_presence.departed` fixture unchanged but still resolved to
  `unknown_tag` instead of closing the visit.
  Cause: the seeded member owned the arrival fixture hash
  `tag_tracer2_001`, while the shared departure fixture intentionally used a
  different identified hash, `tag_tracer5_001`.
  Fix: seed the second claimed tag explicitly in the integration setup and keep
  the shared departure payload bytes unchanged.
  Rule: when an integration test claims to use shared contract bytes unchanged,
  the seed data must be aligned to that contract instead of mutating the
  payload under test.

- Symptom: the first manual `apollo serve` workout smoke returned empty replies
  from the new workout endpoints even though the handler and server integration
  suites were green.
  Cause: the real CLI serve path built `server.Dependencies` separately and
  still omitted the workout service, so the live HTTP route panicked on a nil
  dependency while the test-only server environment stayed healthy.
  Fix: factor runtime dependency assembly into one helper, wire the workout
  service into the real serve path, add a regression test for that assembly,
  and rerun the manual `apollo serve` smoke.
  Rule: when HTTP dependencies are assembled outside the handler tests, smoke
  the real entrypoint and add a regression around the assembly helper instead
  of assuming integration coverage already proves the live path.

- Symptom: repeated hardening runs could list an older finished workout ahead of
  a newer `in_progress` workout even though the product rule was supposed to be
  stable workout history ordering.
  Cause: the list query compared DB-owned `started_at` to app-owned
  `finished_at` through `COALESCE(finished_at, started_at)`, so small clock skew
  between Postgres and the app process could reorder workouts unexpectedly.
  Fix: make the runtime rule explicit as newest workout created first, order the
  list on DB-owned `started_at DESC, id DESC`, and add regression coverage for
  finished-vs-in-progress skew and same-timestamp tie-breakers.
  Rule: if ordering must stay deterministic, do not compare timestamps written
  by different clocks unless the product rule explicitly tolerates that skew.

- Symptom: the repo already had an authored `apollo.recommendations` table,
  which made the first recommendation tracer look like it needed persistence
  before there was any trustworthy recommendation runtime.
  Cause: early schema-first planning preserved future storage, but that storage
  was broader than the first executable proof actually required.
  Fix: keep Tracer 7 as a derived read over explicit workout history, leave the
  authored table unused, and prove deterministic precedence before widening into
  stored or generated recommendations.
  Rule: a future-facing table is not a requirement; first recommendation slices
  should prove deterministic read behavior before they persist outputs.

## 2026-04-04

- Symptom: the first Tracer 11 UI pass was at risk of turning into a frontend
  rewrite instead of a proof that members could use already-real APOLLO APIs.
  Cause: the repo had no existing frontend scaffold, which made it tempting to
  introduce a framework and new endpoint shapes just to make the shell feel
  more complete.
  Fix: keep the shell embedded in the Go server, stay on the existing auth,
  profile, workout, and recommendation APIs, add browser-side helper tests, and
  prove the flow with a disposable local APOLLO runtime instead of widening the
  backend surface.
  Rule: the first member shell must prove UI-to-runtime integration quality on
  top of existing APIs before any broader frontend stack or contract changes are
  justified.

## 2026-04-05

- Symptom: a full browser-side `fetch()` rejection during APOLLO shell bootstrap
  or refresh left the UI stuck on loading copy and leaked an unhandled promise
  rejection.
  Cause: the shell only handled per-request non-2xx API responses; it did not
  own a shared guard for total request rejection across the top-level refresh
  path.
  Fix: route initial shell boot and refresh-triggered reloads through one
  guarded refresh path, replace loading copy with explicit recoverable error
  status for profile, workouts, and recommendation, and add browser-side
  regression coverage for both bootstrap and refresh failure.
  Rule: thin shells still need one shared top-level failure path; handling only
  non-2xx JSON responses is not enough when the transport itself can fail.

- Symptom: Tracer 12 membership tests passed in the custom server harness, but
  the real `apollo serve` smoke panicked on `GET /api/v1/lobby/membership` with
  an empty reply.
  Cause: the test harness wired the new membership service into
  `server.Dependencies`, but `cmd/apollo` still built the live dependency graph
  without `deps.Membership`.
  Fix: wire the membership service through `buildServerDependencies`, then add a
  command-layer regression test that fails if the runtime dependency builder
  ever omits lobby membership again.
  Rule: when APOLLO gains a new runtime dependency, prove both the handler
  harness and the real command wiring; custom integration envs are not enough
  by themselves.

## 2026-04-06

- Symptom: the first Tracer 13 pass added `users.updated_at` so match preview
  inputs could produce a stable `generated_at`, but older visit-repository
  tests started failing with `column u.updated_at does not exist`.
  Cause: most APOLLO integration tests used the shared full-schema helper, but
  `internal/visits/repository_test.go` still booted a hand-picked migration
  subset that stopped before the new user column.
  Fix: keep the deterministic preview watermark on persisted input state, then
  extend the custom visit test schema bootstrapping to include the current
  migration stack so generated queries and temporary databases stay aligned.
  Rule: if a repo keeps any hand-curated integration schema list, it must be
  updated every time the generated SQL surface depends on a new migration.

## 2026-04-09

- Symptom: keeping richer planning inputs in `users.preferences` risked turning
  `coaching_profile` into an untyped JSON dump that could drift away from the
  new planner catalog.
  Cause: the fastest storage path was to keep the nested object flexible, but
  leaving preferred equipment keys unvalidated would have let profile state
  reference non-existent planner catalog rows.
  Fix: keep planner, template, and exercise truth relational, but validate the
  bounded `coaching_profile` equipment-key list against the APOLLO equipment
  catalog before writes while preserving unrelated preference keys.
  Rule: if JSON-backed member intent points at relational planner truth, the
  JSON layer must stay typed and catalog-validated instead of becoming a loose
  side channel.

## 2026-04-10

- Symptom: the first Tracer 28 pass was at risk of pretending
  `competition_sessions.owner_user_id` was still the honest authorization key
  just because earlier competition runtime slices were owner-scoped.
  Cause: the existing session, queue, team, roster, and match substrate already
  used `owner_user_id` throughout repository filters, which made it tempting to
  preserve that as the actual authority model and only rename it in docs.
  Fix: keep `owner_user_id` as provenance only, add explicit APOLLO-local role
  and capability truth on the authenticated principal, and move staff
  competition access checks onto centralized capability plus trusted-surface
  proof.
  Rule: once a tracer claims explicit authz, provenance columns may remain
  useful domain truth, but they must stop being the sole authorization key.

- Symptom: Tracer 28 trusted-surface work was at risk of widening into a fake
  approved-device product with its own registry and lifecycle semantics.
  Cause: staff mutation second-factor requirements naturally suggested labels,
  device concepts, rotation flows, and broader machine governance beyond the
  tracer boundary.
  Fix: keep the primitive APOLLO-local and config-backed only: one request-time
  header/token proof for privileged competition mutations, with actor
  attribution carrying the trusted-surface key used on success.
  Rule: when a tracer only needs bounded second-factor proof, do not invent a
  full device-registry product around it.

- Symptom: the first Tracer 24 coaching shape was at risk of overclaiming exact
  progression from workout history into planner truth.
  Cause: planner sessions are keyed to APOLLO exercise and equipment
  definitions, but workout history still stores freeform exercise names, so the
  two domains do not share a canonical per-exercise identity yet.
  Fix: keep Tracer 24 coaching deterministic and narrow: derive decisions from
  profile, the target planner week, the latest finished workout, and explicit
  effort/recovery feedback, then return response-only plan diffs over existing
  planner items without claiming exact history-to-plan exercise matching.
  Rule: if execution history and planner truth do not share a canonical entity
  identity, do not fake exact deterministic progression; narrow the proposal
  surface until the substrate is actually real.

- Symptom: Tracer 24 was locally valid in runtime, but repo-truth closeout was
  still blocked at the end.
  Cause: `README.md` still carried an older planned-release ladder that no
  longer matched `docs/roadmap.md` or the control-plane implementation board.
  Fix: land a docs-only follow-up that aligns the APOLLO README planned-release
  table with the committed Tracer 24 through Tracer 28 ladder before claiming
  repo-truth closeout.
  Rule: release-ladder tables are part of repo truth; closeout is not finished
  until README, roadmap, and control-plane docs all say the same thing.

- Symptom: repeated Docker-backed integration reruns could still fail at
  `StartPostgres()` with transient `connection refused` errors even though the
  same APOLLO runtime code would pass on rerun.
  Cause: the shared Postgres harness relied on an opaque dockertest retry loop,
  kept an overly aggressive startup cadence, and did not close failed `pgxpool`
  attempts on each retry before trying again.
  Fix: replace the opaque retry path with an explicit readiness loop, close
  failed pool attempts per retry, add a conservative ping timeout and startup
  budget, and add unit coverage for cleanup and deadline behavior in
  `internal/testutil`.
  Rule: repeated Docker-backed integration harnesses need explicit readiness
  budgets, explicit retry cadence, and per-attempt connection cleanup; do not
  hide that behavior inside a generic retry helper.

- Symptom: `go test -count=10 ./internal/server` was not a stable stress
  command for this hardening pass because it could hit Go's default package
  timeout before the Docker-backed integration harness finished repeated
  schema bootstraps and auth/runtime setup.
  Cause: the command mixed lightweight handler tests with slower full-runtime
  integration coverage under one package-wide timeout ceiling, which created a
  stress artifact that looked like product instability even when the narrower
  helper/runtime proofs stayed green.
  Fix: keep the helper hardening proof anchored to focused runtime/integrity
  reruns plus the full required `go test ./...` and `go test -count=5
  ./internal/...` suite, and record the timeout-ceiling artifact separately
  from real runtime regressions.
  Rule: when a stress command exceeds the package timeout ceiling, record it as
  harness or command-shape noise unless narrower reruns show the same product
  failure.

- Symptom: the first Tracer 26 helper design was at risk of forcing nutrition
  into the same diff/apply shape as coaching even though the real deterministic
  nutrition substrate only owned target ranges, strategy flags, and
  explanation.
  Cause: the planning ladder wanted a shared helper narrative, but mirroring
  coaching's `PlanChangeProposal` would have implied nutrition write/apply
  semantics that do not exist yet.
  Fix: keep Tracer 26 asymmetric on purpose: coaching helper previews wrap the
  existing plan-change proposal, while nutrition helper previews use read-only
  guidance proposals that preserve the current calorie/macro targets and do not
  claim an apply path.
  Rule: helper surfaces must preserve real domain asymmetry; do not invent a
  mirrored proposal/apply contract just to make two domains look uniform.

- Symptom: the first Tracer 25 nutrition pass was at risk of turning
  `users.preferences` into a second untyped runtime store just because Tracer 23
  had already widened bounded profile inputs there.
  Cause: `nutrition_profile` belongs next to other declared member intent, which
  made it tempting to keep meal logs and reusable templates in the same JSONB
  path for speed.
  Fix: keep only typed non-clinical nutrition inputs in
  `users.preferences.nutrition_profile`, but move durable meal template and meal
  log truth into dedicated nutrition tables with owner-scoped queries and
  runtime/integrity coverage.
  Rule: JSON-backed member intent may stay typed and bounded, but any nutrition
  state that acts like history or reusable runtime truth should graduate into
  explicit tables instead of hiding in preferences.

- Symptom: Tracer 25 nutrition was at risk of inheriting the richer Tracer 24
  coaching proposal pattern and overclaiming planner mutation or helper-owned
  decisioning before those lines were actually in scope.
  Cause: coaching already had structured proposal and explanation output, so
  reusing that shape wholesale would have felt consistent even though nutrition
  was supposed to stay narrower.
  Fix: keep Tracer 25 nutrition recommendation output read-only and structured:
  conservative ranges, strategy flags, thin reason/evidence/limitations, and no
  apply path, no chatbot-first flow, and no opaque model-owned core.
  Rule: when richer helper or apply semantics belong to a later tracer, earlier
  deterministic nutrition lines must stay thin instead of borrowing those
  semantics early.

- Symptom: the first Tracer 27 pass was at risk of inventing one global
  `ambiguous` member-presence state even though APOLLO visit truth is only
  unique per `(user_id, facility_key)`.
  Cause: it was tempting to summarize multiple open facilities under one
  top-level member status, but that would have turned a real facility-scoped
  edge case into a vague product abstraction.
  Fix: keep Tracer 27 facility-scoped on purpose: `GET /api/v1/presence`
  returns one presence/streak block per facility, tap-link rows stay per visit,
  and streak identity stays per member/facility instead of becoming one global
  attendance counter.
  Rule: when physical truth is facility-scoped, the first product-facing
  presence model must stay facility-scoped too; do not hide multi-facility
  reality behind one synthesized member singleton.
