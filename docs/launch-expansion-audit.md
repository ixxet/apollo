# Launch Expansion Audit

Status: consolidated read-only audit artifact
Scope: APOLLO-centered, with ATHENA, Hestia, Themis, gateway, and platform compatibility noted where they affect launch safety
Original audit date: 2026-04-26
Current through: 2026-05-03 (post-Frontend Route/API Contract Matrix docs closeout)
Cross-references:

- [Competition system audit](../../ashton-platform/planning/audits/2026-04-29-competition-system-audit.md) — cross-repo competition truth snapshot
- [Milestone 3.0 evidence pack](../../ashton-platform/planning/milestones/milestone-3.0-evidence/README.md) — deploy smoke proof, telemetry export, cross-repo verification
- [Compatibility matrix](../../ashton-platform/planning/compatibility-matrix.md) — current deployed and HEAD versions per repo

## Executive Verdict

Do not launch the full expansion as one line. The system can support the vision, but only if the trust substrate lands before public excitement mechanics.

The current APOLLO codebase is strongest at bounded, authenticated, backend-first workflows: member auth, profile state, presence-derived member context, lobby intent, competition sessions, queueing, assignment, match result capture, staff attribution, schedule/booking controls, public-safe booking intake, public-safe competition readiness/leaderboards, public/member-safe game identity projections, internal tournaments, and manager/internal safety and reliability facts. It is not yet ready for public ratings, public tournaments with stakes, public/member safety UI, messaging/chat, broad public social graph behavior, public profiles/scouting, or OpenSkill public stakes.

## Current Truth Snapshot

Fast scan for agents before reading the full audit:

| Layer | Current status | Do not misread as |
| --- | --- | --- |
| Competition trust spine | Closed through result lifecycle, correction/void/dispute states, legacy rating metadata, Rating Policy Wrapper metadata, OpenSkill comparison-only facts, ARES v2 proposal facts, analytics, internal tournaments, safety/reliability, public readiness/leaderboards, and game identity projections. | Permission to ship public tournaments, OpenSkill active read path, or broad public social mechanics. |
| Public surfaces | Public-safe readiness, leaderboard, and game identity contracts are real projection surfaces. They are redacted, allowlisted, and derived from canonical APOLLO truth. | Public profiles, public tournament stakes, public safety details, public scouting, chat, or broad social graph. |
| Milestone 3.0 | Bounded APOLLO/ATHENA deploy smoke, APOLLO `/metrics`, Prometheus scrape, telemetry export, and cross-repo compatibility matrix are closed. | Full population-scale validation, tuned alert thresholds, full Hestia/Themis/gateway deployment proof, or destructive live mutation probes. |
| Remaining gate | Scale Gate numeric ceilings are declared and locally/runtime-proved for APOLLO rating recompute, public readiness, public leaderboard, game identity, and CLI/API smoke paths. | Full production load validation, tuned alert thresholds under real traffic, or permission to ship public tournaments/OpenSkill read-path switch. |
| CLI/API demo spine | Closed locally in repo/runtime: APOLLO CLI now exposes service-backed public readiness, public leaderboard, public game identity, member stats/history/game identity, safety readiness/review, session/tournament reads, command dry-runs/apply, result lifecycle, and ARES preview generation without frontend dependency. | Deployment readiness, public tournament readiness, OpenSkill active read-path readiness, frontend completion, or CLI-owned formulas. |
| Rating policy wrapper | Closed locally in repo/runtime: APOLLO active rating projection uses the legacy engine under `apollo_rating_policy_wrapper_v1`, with calibration status, fifth-match ranked transition, inactivity sigma inflation, bounded upward movement, and member-safe additive metadata. | OpenSkill cutover, public OpenSkill values, production historical backtesting, public tournaments, or deployed truth. |
| Rating policy simulation | Closed locally in repo/runtime: deterministic fixtures and CLI output now cover active policy behavior, legacy baseline deltas, OpenSkill sidecar deltas, accepted/rejected scenarios, blockers, and risk classification. | OpenSkill cutover, public rating launch readiness, production data backtesting, public tournaments, or deployed truth. |
| Frontend route/API contract matrix | Closed as docs truth: Hestia and Themis route/API consumption, proxy denials, auth/role states, empty/error/denied states, production/mock status, and test coverage are enumerated in the platform matrix. | New frontend behavior, generated contract enforcement, deployed Hestia/Themis proof, public tournaments, OpenSkill read-path switch, or frontend-owned formulas. |
| Historical evidence | Closeout addenda and hardening docs preserve what was true at the time of each tracer. | Current deferred truth unless a current section says it still applies. |

The strategic pattern that held through 3B.20 is:

1. Fix docs truth.
2. Add agent-operable CLI and capability/dry-run surfaces.
3. Extract and version rating behavior.
4. Add match tiers, consensus, disputes, and rating audit.
5. Swap the math kernel to OpenSkill under a custom policy wrapper.
6. Upgrade ARES and tournaments after the trust substrate exists.
7. Add social safety before public surfaces.
8. Add retention mechanics after public trust is durable.

Items 1-7 are closed for the bounded shipped scope. Item 8 remains gated by
Scale Gate ceilings, abuse controls, and explicit retention policy.

Phase 3B.20 has now closed the first trusted game identity layer after 3B.19
public competition readiness and 3B.18 internal social safety/reliability. The
next launch-expansion packet should stay on bounded cohesion hardening or a
separately gated next line, not dashboards, public profiles, public tournaments,
messaging/chat, broad social graph behavior, or an OpenSkill hard swap.

## Evidence Anchors

Current APOLLO facts are grounded in these files:

| Area | Evidence |
| --- | --- |
| Roadmap discipline | [docs/roadmap.md](roadmap.md) |
| Current README state | [README.md](../README.md) |
| Competition service and DTOs | [internal/competition/service.go](../internal/competition/service.go) |
| Result capture and standings/stat derivation | [internal/competition/history.go](../internal/competition/history.go) |
| Current custom rating recompute | [internal/competition/history_repository.go](../internal/competition/history_repository.go) |
| Competition container schema | [db/migrations/009_competition_container_runtime.up.sql](../db/migrations/009_competition_container_runtime.up.sql) |
| Competition execution schema | [db/migrations/010_competition_execution_runtime.up.sql](../db/migrations/010_competition_execution_runtime.up.sql) |
| Result/rating schema | [db/migrations/011_competition_history_runtime.up.sql](../db/migrations/011_competition_history_runtime.up.sql) |
| Competition analytics schema | [db/migrations/028_competition_analytics_foundation.up.sql](../db/migrations/028_competition_analytics_foundation.up.sql) |
| Competition analytics rebuild | [internal/competition/analytics.go](../internal/competition/analytics.go) |
| Staff authz and attribution schema | [db/migrations/016_competition_authz_runtime.up.sql](../db/migrations/016_competition_authz_runtime.up.sql) |
| ARES preview (historical lobby preview) | [internal/ares/service.go](../internal/ares/service.go) |
| ARES v2 competition module (3B.15) | [internal/ares/competition.go](../internal/ares/competition.go) |
| CLI root commands | [cmd/apollo/main.go](../cmd/apollo/main.go) |
| Command/readiness module (3B.11) | [internal/competition/command.go](../internal/competition/command.go) |
| Legacy rating module (3B.13) | [internal/rating/legacy.go](../internal/rating/legacy.go) |
| Rating policy wrapper | [internal/rating/policy.go](../internal/rating/policy.go), [db/migrations/033_competition_rating_policy_wrapper.up.sql](../db/migrations/033_competition_rating_policy_wrapper.up.sql) |
| OpenSkill comparison module (3B.14, comparison-only) | [internal/rating/openskill.go](../internal/rating/openskill.go) |
| Tournament module (3B.17, internal-only) | [internal/competition/tournament.go](../internal/competition/tournament.go), [internal/competition/tournament_repository.go](../internal/competition/tournament_repository.go) |
| Safety / reliability module (3B.18, manager-internal) | [internal/competition/safety.go](../internal/competition/safety.go), [internal/competition/safety_repository.go](../internal/competition/safety_repository.go) |
| Public projection module (3B.19) | [internal/competition/public.go](../internal/competition/public.go) |
| Game identity module (3B.20, public-projection) | [internal/competition/game_identity.go](../internal/competition/game_identity.go), [internal/competition/game_identity_repository.go](../internal/competition/game_identity_repository.go) |
| Role/capability map | [internal/authz/authz.go](../internal/authz/authz.go) |
| Trusted-surface verification | [internal/authz/trusted_surface.go](../internal/authz/trusted_surface.go) |
| HTTP route surface | [internal/server/server.go](../internal/server/server.go) |
| Competition hardening proof | [docs/hardening/tracer-20.md](hardening/tracer-20.md), [docs/hardening/tracer-21.md](hardening/tracer-21.md), [docs/hardening/tracer-22.md](hardening/tracer-22.md), [docs/hardening/tracer-28.md](hardening/tracer-28.md) |

Focused consolidation verification:

```sh
go test -count=1 ./internal/competition ./internal/ares ./internal/authz
```

Result: passed.

## Status Taxonomy

Use these labels consistently when extending this document:

| Status | Meaning |
| --- | --- |
| Real | Implemented in repo/runtime with service/API/test evidence. |
| Schema-authored | Table exists, but active runtime does not use it as product truth. |
| Internal-only | Implemented or planned for authenticated/staff/internal use, with no public/member-social surface. |
| Public-projection | Derived/redacted projection over canonical truth, with allowlisted output contract; never authoritative source truth. Added post-3B.19 to capture public readiness/leaderboards and post-3B.20 game identity reality. |
| Comparison-only | Implemented but not on the active read path; produces audit/comparison facts only. Added post-3B.14 to capture OpenSkill dual-run discipline. |
| Deferred | Acknowledged future work that should not ship in the current line. |
| Not planned yet | Mentioned idea with no committed implementation sequence. |
| Kill unless gated | Valid idea only if named gates pass first. |

Versioning reference: ASHTON uses bounded pre-`1.0.0` tag discipline now.
`semver-lite` is historical shorthand only. Project-wide SemVer governance
remains deferred.

## Agent Onboarding

New agents should read in this order before changing launch-expansion work:

1. This file.
2. [ashton-platform README](../../ashton-platform/README.md), especially source-of-truth and versioning sections.
3. [ashton-platform Implementation Board](../../ashton-platform/planning/IMPLEMENTATION-BOARD.md).
4. [ashton-platform Tracer Matrix](../../ashton-platform/planning/sprints/TRACER-MATRIX.md).
5. [ashton-platform APOLLO repo brief](../../ashton-platform/planning/repo-briefs/apollo.md).
6. APOLLO [README](../README.md) and [roadmap](roadmap.md).
7. The specific code and hardening artifact for the tracer being touched.

Do not continue from memory after compaction or resume. Re-read this file, `git diff`, and the active files first.

## Source-Of-Truth Relationship

This file governs APOLLO launch-expansion sequencing after the current
Phase 3B schedule/booking line. It does not rewrite historical tracer evidence.

Conflict rule:

- For shipped runtime truth, prefer APOLLO `README.md`, `docs/roadmap.md`,
  migrations, and platform repo briefs.
- For future APOLLO competition/rating/tournament/social expansion sequencing,
  prefer this file.
- For cross-repo release coordination, prefer
  `ashton-platform/planning/IMPLEMENTATION-BOARD.md`, which should link back to
  this file for launch-expansion specifics.
- Historical hardening notes remain evidence snapshots and should not be edited
  just because future sequencing changed.

Label caveat: T18a-T57 labels in this audit are launch-expansion packet labels.
They do not replace historical platform Tracer 18-T28 closure records.

## Artifact Maintenance

This document is an operating artifact, not a one-time audit. Every tracer closure that touches launch expansion must update it.

Required closure delta:

- Update status labels in the inventory or feature matrix.
- Add or update evidence anchors.
- Mark any gate progress or newly failed gate.
- Add a decision-log row if the tracer changes a ruling.
- Add a capacity, telemetry, privacy, or rollback note if the tracer affects those surfaces.

This file is a sequencing plan, not a feature freeze. New ideas should be added to the Feature Viability Matrix with likelihood, failure risk, durability, and gates before implementation work starts.

## Current System Inventory

### Real Today

APOLLO currently has:

- First-party member auth and session state.
- Member profile state and eligibility derived from explicit profile preferences.
- Explicit lobby membership.
- Deterministic ARES match preview over joined lobby members.
- Sport registry and facility-sport capability mapping.
- Competition sessions, teams, rosters, queue membership, matches, side slots, assignment, start/archive lifecycle, completion, result capture, standings, and self-scoped member history/stats.
- Custom per-sport/per-mode rating projection from completed competition results.
- Role/capability authz with trusted-surface proof for privileged staff mutations.
- Staff action attribution for competition mutations.
- Shared APOLLO competition command and command outcome DTOs for existing
  competition behavior.
- Competition command readiness/capability checks, dry-run plan output, and
  service-backed CLI parity through `apollo competition command`.
- Canonical match/result lifecycle trust: `match_status`, `result_status`,
  `canonical_result_id`, `dispute_status`, `correction_id`,
  `supersedes_result_id`, `finalized_at`, `corrected_at`, and lifecycle events
  for started, recorded, finalized, disputed, corrected, and voided facts.
- Rating paths consume only finalized or corrected canonical results; recorded,
  disputed, voided, and non-canonical results are excluded.
- Versioned legacy rating foundation: `legacy_elo_like` engine/policy
  identifiers, golden characterization cases, auditable rating compute/policy/
  rebuild events, source result IDs, rating event IDs, and deterministic
  projection watermarks over finalized/corrected canonical result truth.
- Rating Policy Wrapper: APOLLO active rating rows/events now carry
  `apollo_rating_policy_wrapper_v1` over the legacy engine, with
  `calibration_status`, fifth-match ranked transition, inactivity
  sigma-inflation metadata, and positive movement cap metadata. This is
  repo/local runtime truth only; deployed truth is unchanged.
- Rating Policy Simulation / Golden Expansion: APOLLO has deterministic local
  fixtures and CLI proof for active policy behavior, legacy baseline deltas,
  OpenSkill comparison deltas, accepted/rejected scenario classification,
  cutover blockers, and policy risks. This is repo/local runtime truth only;
  deployed truth is unchanged.
- OpenSkill dual-run comparison: internal comparison rows/events record
  legacy/OpenSkill values, deltas, accepted budgets, scenarios, and explicit
  delta flags from the same finalized/corrected canonical result order while
  leaving the legacy-engine policy-wrapped projection as the active read path.
- ARES v2 proposal/match-preview foundation: explicit queue intent facts,
  internal match-preview projections/events, deterministic match quality,
  predicted win probability, and explanation codes over trusted APOLLO queue,
  rating, result, session, and team projections.
- Competition analytics foundation: internal stat events and analytics
  projections for matches played, wins/losses/draws, streaks, legacy rating
  movement, opponent strength, team-vs-solo delta when existing
  participant-count facts support it, and facility/sport/mode splits.
  Projection version, sample size, confidence, computed time, source match,
  source result, and deterministic watermarks are explicit; analytics consume
  only finalized/corrected canonical result truth plus legacy active rating
  facts and do not mutate result, rating, ARES, lifecycle, booking, UI, or
  public truth.
- Schedule substrate, booking request lifecycle, public-safe booking intake/status/availability.
- Presence, visit, tap-link, facility streak, and ATHENA-backed ops overview surfaces.
- Minimal member shell with existing API-backed routes.
- Telemetry counter export (Milestone 3.0): APOLLO `/metrics` carries competition/rating/ARES/booking/safety/reliability/game-identity counters with Prometheus ServiceMonitor + PrometheusRule applied and scrape verified.
- Cross-repo compatibility matrix (Milestone 3.0): published at `ashton-platform/planning/compatibility-matrix.md`; APOLLO and ATHENA confirmed deployed with image SHAs recorded; Hestia/Themis/gateway recorded as repo-only.
- Deploy smoke proof for APOLLO and ATHENA (Milestone 3.0): live health, public readiness redaction, metric export, presence count, and Prometheus scrape verified; APOLLO drift resolved by rolling `sha-bf3119b` → `sha-6b27618`.

### Schema-Authored But Not Current Product Truth

These are easy to misread as real runtime because the schema exists:

| Area | Current status | Ruling |
| --- | --- | --- |
| `apollo.ares_ratings`, `apollo.ares_matches`, `apollo.ares_match_players` | Authored in the initial migration, but not the active competition rating/result runtime. | Do not build new rating behavior on these without a deliberate migration decision. |
| `apollo.recommendations` | Authored table; current recommendation/coaching reads are primarily deterministic read-time surfaces. | Do not treat the table as evidence of a persisted recommendation engine. |

The active competition rating projection is `apollo.competition_member_ratings`, created by the competition-history runtime. Phase 3B.13 adds explicit legacy rating metadata to that projection and writes rating audit events in `apollo.competition_rating_events`. Phase 3B.14 adds internal OpenSkill comparison facts in `apollo.competition_rating_comparisons` and OpenSkill comparison events without switching that active projection. Rating Policy Wrapper now adds `apollo_rating_policy_wrapper_v1` as the active policy on that legacy-engine projection, with `calibration_status`, `last_inactivity_decay_at`, `inactivity_decay_count`, and `climbing_cap_applied` on current rows plus policy metadata on rating events. Phase 3B.16 adds internal derived analytics in `apollo.competition_analytics_events` and `apollo.competition_analytics_projections` without creating a public/member analytics surface. Phase 3B.17 adds internal tournament facts, and Phase 3B.18 adds manager/internal report, block, reliability, and safety audit facts without creating public/member safety surfaces.

### Not Real Yet

APOLLO does not yet have:

- Durable command idempotency storage or replay.
- Universal command/dry-run coverage outside the supported Phase 3B.11
  competition command surface.
- OpenSkill read-path switch.
- Match tier classification.
- Player consensus result voting.
- Full proposal/approval workflow for disputes or corrections.
- Comeback bonus, upset bonus, match-tier multipliers, and production
  historical rating backtesting.
- Cross-sport transfer.
- Carry/tank/stability coefficients and broader scouting/profile analytics.
- Persistent/sediment teams.
- Public tournament, season, league, bracket, or seeding surfaces.
- Replay/event logs.
- Public/member social reports, public block lists, honor/trust, no-show scores,
  or messaging/chat surfaces.
- Broader public competition reads beyond the shipped 3B.19 public-safe
  readiness/leaderboard projections.
- Public records, XP, quests, guilds, bounty, prestige, live events, public
  profiles/scouting, or public tournament presentation.
- Broader social rivalry, badge, CP, or squad behavior beyond the shipped 3B.20
  public/member-safe game identity projection layer.
- Generated OpenAPI/client/route contract enforcement for the Hestia/Themis
  route/API matrix.
- Hestia, Themis, and ashton-mcp-gateway live cluster deployment proof (Milestone 3.0 inspected and found none).
- Production-destructive mutation probes against live APOLLO data (Milestone 3.0 explicitly did not run these; mutation contracts remain repo-test-proved).
- Full production load validation beyond the locally/runtime-proved Scale Gate numeric ceilings.
- Live in-flight production SIGTERM proof for APOLLO/ATHENA/gateway graceful shutdown (Milestone 3.0 covered rollout-restart and repo race runs only).

### Important Documentation Inconsistency

The earlier live inconsistency was that README-level docs still implied the
current slice avoided results, ratings, and standings after those surfaces were
already real. That specific problem is no longer the dominant README truth.

Current docs drift to prevent:

- Do not list deploy smoke proof, telemetry export, or the compatibility matrix
  as next work without noting Milestone 3.0 already closed the bounded version.
- Do not describe public readiness, public leaderboards, CP, badges, rivalry, or
  squads as fully absent. The shipped 3B.19/3B.20 versions are projection-only
  and redacted; broader public/social/tournament versions remain deferred.
- Do not treat historical hardening carry-forward gaps as current truth without
  checking this audit and the roadmap.

Ruling: fix docs truth before expanding the surface.

## CLI Parity: Thin Wrapper, Domain Model, Or Command Layer?

The phrase "thin wrapper over existing HTTP/service behavior" needs precision.

The goal is not a weak CLI. The goal is no second source of business truth.

### The Weak Option: Raw HTTP Wrapper Only

CLI calls APOLLO HTTP endpoints exactly like a remote client.

Pros:

- Reuses deployed HTTP behavior.
- Good for remote ops.
- Proves API completeness.

Cons:

- Needs a running server.
- Needs session cookie and trusted-surface header handling.
- Weaker local developer experience.
- Easy to make brittle around auth/bootstrap.
- Harder to use in migrations, local repair, and agent workflows.

Durability: medium. Useful, but not sufficient as the only CLI strategy.

### The Risky Option: Independent CLI Domain Model

CLI implements its own competition/session/rating logic directly.

Pros:

- Can feel strong because it is "domain native."
- Can run without HTTP.

Cons:

- Duplicates invariants.
- Can drift from HTTP behavior.
- Creates two write paths with different validation.
- Increases audit burden.
- Bad fit for a system that already emphasizes bounded surfaces and attribution.

Durability: low. This is disruptive and weaker long term because duplicated truth is weaker than shared truth.

### The Better Near-Term Option: Thin Service Wrapper

CLI opens the same DB-backed services used by the server and calls existing service methods. This is how current `sport` and `schedule` CLI commands already lean.

Pros:

- Reuses domain service validations.
- Does not require a running HTTP server.
- Strong for local ops, tests, and agents.
- Avoids inventing a second domain model.

Cons:

- Needs careful actor/trusted-surface inputs.
- Needs DB access.
- Must not bypass authz/attribution semantics.

Durability: high for local/admin CLI.

### The Best Long-Term Option: Application Command Layer

HTTP and CLI both map into shared command handlers.

Shape:

```text
HTTP request -> command DTO -> application command handler -> domain service -> repository
CLI flags    -> command DTO -> application command handler -> domain service -> repository
```

The command layer owns:

- Input normalization.
- Idempotency and expected-version checks.
- Actor/trusted-surface construction.
- Dry-run planning.
- Structured outcome enums.
- Shared response shape.

The domain service owns:

- Business invariants.
- State transitions.
- Rating/result/session rules.

The repository owns:

- Persistence only.

Ruling: Phase 3B.11 built the shared command layer first for competition
commands, then mapped HTTP and the service-backed CLI into that shared
command/outcome path. Do not build an independent CLI domain model. Durable
command idempotency remains deferred until a storage substrate is deliberately
chosen.

## CLI Demo Spine

Status: closed locally in repo/runtime on 2026-05-03. Deployed truth is
unchanged.

The APOLLO CLI demo path now exposes existing APOLLO service/API truth without a
frontend dependency:

| Demo area | CLI path | Source truth |
| --- | --- | --- |
| Public readiness | `apollo competition public readiness` | `Service.PublicCompetitionReadiness` |
| Public leaderboard | `apollo competition public leaderboard` | `Service.ListPublicCompetitionLeaderboard` |
| Public game identity | `apollo competition public game-identity` | `Service.PublicGameIdentity` |
| Member rating/stat projection | `apollo competition member stats --user-id <uuid>` | `Service.ListMemberStats` over legacy active rating projection |
| Member history | `apollo competition member history --user-id <uuid>` | `Service.ListMemberHistory` |
| Member game identity | `apollo competition member game-identity --user-id <uuid>` | `Service.MemberGameIdentity` |
| Command readiness and dry-run/apply | `apollo competition command readiness` / `apollo competition command run` | shared competition command/outcome handler |
| Session state inspection | `apollo competition session list/show` | `Service.ListSessions` / `Service.GetSession` |
| Tournament state inspection | `apollo competition tournament list/show` | `Service.ListTournaments` / `Service.GetTournament` |
| Safety readiness/review | `apollo competition safety readiness/review` | manager/internal `Service.CompetitionSafetyReadiness` and `Service.GetCompetitionSafetyReview` |
| ARES preview generation | `apollo competition command run --name generate_match_preview` | shared command handler -> `Service.GenerateMatchPreview` |

No CLI path owns formulas for ratings, analytics, public projections, ARES, or
game identity. Rating recompute remains the existing result-finalize/correction
side effect over APOLLO repository truth; there is no independent CLI recompute
domain model. Internal analytics are read through existing public-safe
leaderboard and game-identity projections unless a future packet creates a
separate authorized internal read contract.

Deterministic local smoke:

```sh
cd /Users/zizo/Personal-Projects/ASHTON/apollo
go test -count=1 ./cmd/apollo -run TestCompetitionCLIDemoProjectionSafetyAndPreviewReads
```

Manual CLI/API demo assumptions:

- `APOLLO_DATABASE_URL` points at a migrated local APOLLO database.
- Commands use real local DB rows; placeholder IDs below must come from that
  database or from a dedicated test fixture.
- Safety reads require manager/owner actor attribution plus trusted-surface
  proof.

Representative local sequence:

```sh
apollo competition command readiness --actor-role manager --format json
apollo competition command run --name record_match_result --session-id <competition-session-id> --match-id <match-id> --expected-version 0 --actor-user-id <manager-user-id> --actor-session-id <actor-session-id> --actor-role manager --trusted-surface-key staff-console --input-json '<result-json>' --format json
apollo competition command run --name finalize_match_result --session-id <competition-session-id> --match-id <match-id> --expected-version 1 --actor-user-id <manager-user-id> --actor-session-id <actor-session-id> --actor-role manager --trusted-surface-key staff-console --format json
apollo competition session show --session-id <competition-session-id> --format json
apollo competition public readiness --format json
apollo competition public leaderboard --sport-key badminton --mode-key head_to_head:s2-p1 --stat-type wins --team-scope all --limit 10 --format json
apollo competition public game-identity --sport-key badminton --mode-key head_to_head:s2-p1 --facility-key ashtonbee --team-scope all --format json
apollo competition member stats --user-id <member-user-id> --format json
apollo competition member history --user-id <member-user-id> --format json
apollo competition member game-identity --user-id <member-user-id> --sport-key badminton --mode-key head_to_head:s2-p1 --facility-key ashtonbee --team-scope all --format json
apollo competition command run --name generate_match_preview --session-id <queue-session-id> --expected-version <queue-version> --actor-user-id <manager-user-id> --actor-session-id <actor-session-id> --actor-role manager --trusted-surface-key staff-console --format json
apollo competition safety readiness --actor-user-id <manager-user-id> --actor-session-id <actor-session-id> --actor-role manager --trusted-surface-key staff-console --format json
apollo competition safety review --actor-user-id <manager-user-id> --actor-session-id <actor-session-id> --actor-role manager --trusted-surface-key staff-console --limit 5 --format json
```

Public API reads for server-based demos remain:

```sh
curl -s "$APOLLO_BASE_URL/api/v1/public/competition/readiness"
curl -s "$APOLLO_BASE_URL/api/v1/public/competition/leaderboards?sport_key=badminton&mode_key=head_to_head:s2-p1&stat_type=wins&team_scope=all&limit=10"
curl -s "$APOLLO_BASE_URL/api/v1/public/competition/game-identity?sport_key=badminton&mode_key=head_to_head:s2-p1&facility_key=ashtonbee&team_scope=all&limit=10"
```

## Ten Gates

No tracer should ship without passing the gates relevant to it.

| # | Gate | Definition | Current Status (2026-04-29) | Blocks |
| --- | --- | --- | --- | --- |
| 1 | Docs Truth Gate | README, roadmap, hardening docs, and launch docs match shipped runtime. | Pass. This audit and the 2026-04-29 competition system audit are mutually consistent through 3B.20.1. | All agent-led expansion |
| 2 | Boundary Gate | One release line per domain. No mixed launch of ratings, tournaments, social, and retention. | Pass. Each 3B.X packet shipped one bounded scope; addenda record what was deferred. | Big-bang work |
| 3 | Mutation Safety Gate | Mutations use `expected_version`, idempotency, or both; return structured outcome. | Pass. Result and tournament writes use `expected_*_version`; commands return structured outcomes; dry-run/apply split is enforced. Durable command idempotency storage remains deferred. | Staff/public writes |
| 4 | Rating Gate | Rating policy is versioned, golden-tested, auditable, and rollback-capable. | Pass. `rating_engine` and `engine_version` carried per event; legacy fallback preserved; OpenSkill is comparison-only. | Public ratings, ARES v2 |
| 5 | Dispute Gate | Result correction and dispute ledger exist before public stakes. | Pass for shipped surfaces. Result lifecycle covers record / finalize / dispute / correct / void with version guards and supersession-via-correction. | Public rankings, badges, records, tournaments |
| 6 | Telemetry Gate | Counters, latencies, rejects, and failure modes are measured before launch. | Pass (Milestone 3.0). APOLLO `/metrics` exports the required competition/rating/ARES/booking/safety/reliability/game-identity counters; Prometheus ServiceMonitor and PrometheusRule applied; `up{job="apollo"} == 1` confirmed. Pending: runtime alert thresholds tuned against real load. | Scale/public launch |
| 7 | Privacy Gate | Reporting, moderation audit, rate limits, leak tests, and role checks exist. | Pass for shipped public surfaces (3B.19 readiness/leaderboards, 3B.20 game identity). Public output contract is explicit, allowlisted, redacted; trusted-surface proof stays server-side. Live readiness leak corpus passed in Milestone 3.0. | Social/public surfaces |
| 8 | Scale Gate | Row-count and latency ceilings are known; rebuild patterns have limits/runbooks. | Pass for repo/local runtime numeric ceilings (2026-05-03). APOLLO now has explicit row-count, latency, recompute-duration, projection, and CLI/API smoke ceilings with focused proof. This is not full production load validation, and Milestone 3.0 deployed truth is unchanged. | Public leaderboards |
| 9 | Frontend Contract Gate | Hestia/Themis production routes call real APIs only; no silent stubs. | Pass as docs truth (2026-05-03). The standalone platform matrix enumerates Hestia/Themis routes, APOLLO contract usage, same-origin proxy boundaries, `/api/v1/public/*` denial, auth/role/state behavior, mock-vs-production status, and current tests. This is not generated contract enforcement or deployed Hestia/Themis proof. | Frontend release |
| 10 | Cross-Repo Compatibility Gate | Compatibility matrix exists for APOLLO, ATHENA, Hestia, Themis, gateway, proto/platform. | Pass (Milestone 3.0). Compatibility matrix published at `ashton-platform/planning/compatibility-matrix.md` with deployed and HEAD versions per repo. Caveat: Hestia, Themis, and ashton-mcp-gateway are recorded as repo-only (no cluster deployment in inspected environment). | Coordinated launch |

Net status: ten gates have repo/deploy evidence for their current scope. Gate 8 is closed only for explicit local/runtime numeric ceilings; it is not a claim of full production load validation.

## Rating Architecture Ruling

Use OpenSkill underneath with custom APOLLO policy wrapped on top. Keep the current Elo-like behavior only as a legacy baseline and migration fallback.

Phase 3B.14 now implements OpenSkill as an internal dual-run comparison layer.
The active APOLLO rating behavior remains a versioned legacy-engine APOLLO
projection: custom logistic expectation, fixed K factor, synthetic sigma
shrink, synchronous full-sport recompute after finalized/corrected canonical
result changes, golden characterization tests, auditable legacy
compute/policy/rebuild events, and the explicit
`apollo_rating_policy_wrapper_v1` product-policy layer for calibration,
inactivity sigma inflation, and bounded positive movement.

This should not become "average Elo and OpenSkill forever." The durable hybrid is:

```text
legacy Elo-like policy:
  characterization baseline, fallback, golden comparison

OpenSkill kernel:
  main math engine for mu/sigma, teams, uncertainty, match quality

APOLLO policy wrapper:
  tiers, multipliers, calibration, transfer, decay, climbing caps, tournament snapshots
```

### Rating Options

| Option | Success likelihood | Failure likelihood | Durability | Ruling |
| --- | ---: | ---: | --- | --- |
| Keep current Elo-like only | Medium short term | High long term | Low | Fine for internal small launch only |
| Hard swap to OpenSkill | Medium | Medium-high | Medium | Too risky with populated data |
| Extract legacy, add policy interface, dual-run OpenSkill | High | Low-medium | High | Recommended |
| Permanent blended score from Elo + OpenSkill | Low-medium | High | Low | Avoid; hard to explain and audit |
| Custom-reimplement OpenSkill concepts | Low | High | Low | Avoid; math risk is not worth it |

### Why OpenSkill Is Needed

Current custom rating:

- Tracks `mu`.
- Shrinks `sigma` mechanically.
- Aggregates team average manually.
- Recomputes by deleting and rebuilding per sport.
- Does not natively handle match quality, asymmetric teams, robust uncertainty, or Bayesian team math.

OpenSkill gives:

- Native mu/sigma semantics.
- Team and asymmetric team math.
- Match quality and win probability.
- Better calibration substrate.
- Better long-term scale pattern.

APOLLO policy gives:

- Friendly no-rating mode.
- Casual reduced multiplier.
- Competitive standard multiplier.
- Tournament visibility/snapshot rules.
- Cross-sport seeding.
- Climbing caps.
- Decay and comeback rules.
- Facility/sport/mode-specific policy.

### Current Rating Module

Phase 3B.13 starts the module as:

```text
internal/rating/
  legacy.go
  legacy_test.go
```

OpenSkill dual-run in 3B.14 should attach beside this baseline, not replace it
in place. The public/member read path should stay on the legacy projection until
the dual-run comparison proves the switch.

### Recommended Future Rating Module

```text
internal/rating/
  policy/
    interface.go
    elo_legacy.go
    openskill.go
    tier_wrapper.go
  calibration/
  transfer/
  decay/
  analytics/
  audit/
  golden/
```

### Required Rating Tables

Minimum durable additions after 3B.13:

- `apollo.competition_rating_events` (real in 3B.13)
- `apollo.competition_member_ratings.rating_engine` (real in 3B.13)
- `apollo.competition_member_ratings.engine_version` (real in 3B.13)
- `apollo.competition_member_ratings.policy_version` (real in 3B.13)
- `apollo.competition_member_ratings.source_result_id` (real in 3B.13)
- `apollo.competition_member_ratings.rating_event_id` (real in 3B.13)
- `apollo.competition_member_ratings.projection_watermark` (real in 3B.13)

Still deferred until policy/cutover packets:

- `apollo.rating_policy_versions`
- `apollo.competition_member_ratings.peak_mu`
- `apollo.competition_member_ratings.seeded_from_sport`
- `apollo.competition_member_ratings.seeded_with_ratio`
- `apollo.competition_member_ratings.seed_confidence`

Real after Rating Policy Wrapper:

- `apollo.competition_member_ratings.calibration_status`
- `apollo.competition_member_ratings.last_inactivity_decay_at`
- `apollo.competition_member_ratings.inactivity_decay_count`
- `apollo.competition_member_ratings.climbing_cap_applied`

Optional after scale gate:

- `apollo.rating_snapshots`
- `apollo.rating_rebuild_runs`
- `apollo.rating_anomaly_flags`

### Rating Contract

Phase 3B.13 now implements the legacy half of this contract. Any OpenSkill
migration must attach to it without hiding engine/policy versions:

| Field / concept | Requirement |
| --- | --- |
| `rating_engine` | Explicit engine identifier: `legacy_elo_like`, `openskill`, or future engine name. |
| `engine_version` | Versioned semantic contract for the math and policy wrapper. |
| Sport/mode partition | Preserve current partitioning by `sport_key` and mode key unless a migration says otherwise. |
| Display policy | Keep `current_rating_mu` and `current_rating_sigma` additive/backward-compatible for existing member API consumers until a versioned replacement exists. |
| Migration source | Rebuild from immutable match/result/rating-event truth, not from ad hoc current rows. |
| Recompute watermark | Store what result/event offset the projection has processed. |
| Legacy compatibility | Legacy output remains available for one release after OpenSkill read-path cutover. |
| Privacy posture | Ratings remain self-scoped/internal until public-read privacy, dispute, and scale gates pass. |

Security hard stop: trusted-surface tokens must never be exposed to Hestia/Themis browser paths. Recompute/admin paths stay CLI/internal or trusted-surface gated.

### Golden Test Cases

Golden cases are frozen input/output scenarios committed to the repo. They make rating changes reviewable.

Required cases:

| Case | Purpose |
| --- | --- |
| Unranked 1v1, A wins | Baseline |
| Pro beats new player | Conservative expected result |
| New player beats pro | Upset and cap behavior |
| Casual 1v1 | Tier multiplier |
| Friendly match | No rating write |
| 5v5 even teams | Team-size weighting |
| 3v5 asymmetric match | OpenSkill team math |
| Draw | Outcome handling |
| First calibration match | Provisional behavior |
| Fifth calibration match | Provisional to ranked transition |
| Cross-sport seed | Transfer behavior |
| Inactive return after 90 days | Sigma inflation/comeback |
| Tournament snapshot | Frozen/team snapshot behavior |

### Dual-Run Migration

1. Done in 3B.13: extract existing Elo-like logic unchanged into `internal/rating`.
2. Done in 3B.13: add golden characterization tests for current behavior.
3. Done in 3B.13: add legacy rating event audit table and projection metadata.
4. Done in 3B.14: add OpenSkill comparison implementation beside legacy.
5. Done in 3B.14: dual-run Elo legacy and OpenSkill for internal matches.
6. Done in Rating Policy Wrapper: active legacy-engine projection records
   explicit policy version, calibration status, fifth-match ranked transition,
   inactivity sigma inflation, and positive movement caps.
7. Done in Rating Policy Simulation / Golden Expansion: expand deterministic
   rating policy simulation/golden coverage and compare OpenSkill sidecar
   deltas by scenario.
8. Compare OpenSkill deltas against real production data and telemetry.
9. Switch read path to OpenSkill only after wrapper, simulation, rollback, and
   scale evidence are accepted.
10. Keep legacy fallback for one release.
11. Remove legacy writes later; keep golden fixtures forever.

Rollback plan:

- Keep rating events immutable.
- Keep policy version on every rating update.
- Keep previous rating read path for one release.
- Rebuild derived current ratings from events if needed.

### Per-Tracer Rollback Requirements

The highest-risk rating tracers require explicit rollback procedures:

| Tracer | Rollback procedure |
| --- | --- |
| T19a rating extraction | Keep behavior-identical golden characterization tests. If extraction breaks, revert service wiring to the current `internal/competition/history_repository.go` path and keep schema unchanged. |
| T19b policy interface | Keep legacy policy as default. If policy routing fails, rebuild from immutable finalized/corrected result truth under the previous legacy policy version and keep OpenSkill comparison off active reads. |
| T19c rating audit/events | Add events as append-only sidecar first. If event writes fail, fail closed for rating migration work while existing result capture remains unchanged, or make audit sidecar explicitly non-blocking only before public stakes. |
| T23 OpenSkill dual-run/cutover | Keep legacy read path for one release. If OpenSkill deltas exceed the accepted budget, disable OpenSkill read path and rebuild current ratings from legacy policy/events. |

## Feature Viability Matrix

Likelihood columns mean "likelihood of succeeding without damaging system agility/security/robustness if sequenced as recommended" and "likelihood of failure or non-happening if rushed from the current state."

Post-3B.20.1 reading rule: when a row says a public or social mechanic is
"not ready," read that as "not ready beyond the shipped projection-only,
redacted 3B.19/3B.20 contracts" unless the row explicitly says the first
projection is already shipped.

| Feature | Current fit | Success likelihood if sequenced | Failure likelihood if rushed | Durability | Best approach |
| --- | --- | ---: | ---: | --- | --- |
| Docs truth cleanup | Direct docs fix | Very high | Medium | High | Do first; no product widening |
| Competition CLI parity | Shipped for supported existing competition commands in 3B.11; CLI Demo Spine now adds service-backed public/member/safety projection reads without new product behavior | High | Medium | High | Shared command/outcome/service paths; no independent CLI domain model |
| Capability discovery | Shipped as competition command readiness in 3B.11; broader capability discovery remains later | High | Medium | High | Keep role/capability read model explicit |
| Universal dry-run | Shipped only for the supported 3B.11 competition command surface | Medium-high | High | High | Add command planning layer per surface; do not fake write success |
| Internal Themis competition ops shell | First APOLLO-backed shell shipped in 3B.11 | High if tightly scoped | High if it becomes public tournaments/social | High | Staff-only ops shell; no public surface, no browser trusted token |
| Approval/proposal workflow primitive | Fits booking, schedule, disputes, result corrections, social actions | Medium-high | High if generalized too early | Very high | Reusable propose/approve/reject/expire state machine with attribution |
| Match lifecycle state machine | Direct fit for consensus and disputes | High | Very high if skipped | Very high | Canonical states before finalization or rating writes |
| Notification substrate | Fits approval prompts, score votes, dispute alerts | Medium-high | Medium-high if it becomes messaging | High | One-way notifications/outbox only; no chat, DMs, or guild messaging |
| Rating extraction | Good fit | High | Medium | High | Move current math without behavior change |
| Rating policy versioning | Wrapper closed locally in repo/runtime; policy table still deferred | High | Medium | High | Explicit active policy version, policy metadata, focused golden tests |
| OpenSkill hybrid | Good fit after extraction | High | High | High | Dual-run, event audit, fallback |
| Match tiers | Good fit on sessions | High | Medium | High | Add internal-only fields first |
| Rating multipliers | Needs rating policy | High | High | High | Policy wrapper, not raw DB fields only |
| Consensus voting | Needs result workflow change | Medium-high | High | High | Pending result state before completion |
| Disputes/corrections | Needs new ledger | Medium-high | Very high | High | Add before public stakes |
| Rating-aware ARES | First proposal foundation closed in 3B.15 | Medium-high | High | High | Further widening still needs explicit packets; keep proposal-only |
| Climbing cap | Wrapper closed locally in repo/runtime | High | Medium | High | Positive movement cap with focused golden tests |
| Calibration | Wrapper closed locally in repo/runtime | High | Medium | High | First four matches provisional; fifth match ranked |
| Inactivity decay | Wrapper closed locally in repo/runtime for rebuild-time sigma inflation; scheduler still deferred | Medium-high | Medium | High | Sigma inflation only; no mu change; no deploy job in this packet |
| Comeback bonus | Needs UX/policy | Medium | Medium | Medium | Keep small and explainable |
| Cross-sport transfer | Needs rating module and skill graph | Medium-high | Medium | Medium-high | Seeding only at first; internal display |
| Carry/tank/stability | Read-only over completed matches | Medium-high | Medium | Medium | Internal analytics first |
| Upset alerts/bonus | Needs win probability | Medium | Medium-high | Medium | No friendly bonus; cap public effect |
| Sediment teams | Needs persistent team table and ATHENA proof | Medium | Medium-high | High | Promotion event from session/ad-hoc team |
| Persistent teams | Current teams are session-scoped | Medium-high | Medium-high | High | Separate `persistent_teams`; do not mutate session teams into identity |
| Tournament schema | No current container | Medium-high | Medium | High | Schema/container internal first |
| Tournament runtime | Needs disputes and snapshots | Medium | High | High | Staff internal before public |
| Bracket formats | Needs tournament runtime | Medium | High | Medium-high | Single elim, double elim, Swiss, round robin |
| Seeding/byes/manual overrides | Needs rating and TO tooling | Medium | High | High | Keep manual override attributed |
| TO tools | Needs Themis contract | Medium | High | High | Frontend Contract Gate required |
| Pregame | Fits lifecycle extension | Medium | Medium | Medium | Add readiness/warmup state before strategy-heavy bans |
| Replay/event logs | Fits attribution gap | Medium-high | Medium | Very high | Append-only event ledger, not full event sourcing yet |
| Social reporting | No substrate | Medium | High | High | Manager-only triage first |
| Silent block list | No substrate | Medium-high | High | High | Manager-only, audited, not user-facing |
| Honor/trust | No substrate | Medium | High | Medium-high | After reports/disputes; separate from skill |
| Reliability/no-show | Fits schedule/competition | Medium-high | Medium | High | Start with attendance and no-show events |
| Recurring schedule policy | Later schedule substrate | Medium later | High now | High | Isolate timezone, DST, exceptions, end rules, and conflict policy |
| Court splitting/resource substrate | Fits scheduling, lobbies, bookings, tournaments | Medium-high | High if modeled ad hoc | Very high | Resource graph with subdivisions, capacity, and conflict edges |
| Public leaderboards | First public-safe projection shipped in 3B.19; broader public rankings remain gated | Medium later | Very high now | Medium-high | Preserve redaction; declare Scale Gate ceilings before widening |
| Records/hall of fame | Needs event/record engine | Medium later | High now | High | Derived from audited events only |
| Daily check-in XP | Fits presence/streaks | High if private | Medium | Medium | Private, explicit, 1 credit/day, ATHENA-linked |
| Season XP | Needs event ledger | Medium | High | Medium | After XP event table and anti-double-counting |
| Quests/challenges | Needs XP/badge engine | Medium | High | Medium | Toggleable per facility |
| Badges/trophies | First badge award projection shipped in 3B.20; broader trophy/criteria registry remains gated | Medium | Very high if early | High | Keep 3B.20 projection read-only; no broad public trophies until scale/privacy policy is explicit |
| Power rating/CP | First CP projection shipped in 3B.20 from public-safe competition rows; OpenSkill-backed public rating remains deferred | Medium | High | Medium | Keep CP display separate from rating truth; no OpenSkill read-path switch before production backtesting and rollback/cutover proof |
| Spectator feed | Needs public-safe competition read | Medium later | High now | Medium | Start authenticated/private |
| Friend cheer/watchlist | Needs social/privacy | Medium later | High now | Medium | No PII leaks; rate limit |
| Predictions/pickem | Needs tournament/public read | Medium later | Medium-high now | Medium | Bragging-only, no money |
| Skill drills | Medium fit with sports/planner | Medium-high | Medium | High | Staff/witness verification levels |
| Practice mode | Fits schedule/drills | Medium-high | Medium | High | No rating impact; explicit opt-in |
| Composite athlete score | Needs rating across sports | Medium | High | Medium | Internal first, explain confidence |
| MVP voting | Needs voting and anti-abuse | Medium | High | Medium | After consensus/honor patterns exist |
| Draft/captains | Fits queue/assignment | Medium-high | Medium | Medium-high | Separate draft state; do not overload queue |
| Rivalry/nemesis | First redacted rivalry state projection shipped in 3B.20; broader social rivalry remains gated | Medium later | Very high now | Medium | Keep rivalry derived/read-only; no messaging or public profile coupling |
| Guilds/squads | First redacted squad identity projection shipped in 3B.20; persistent guild/social substrate remains deferred | Medium later | High now | Medium | Keep squad identity projection-only; no messaging |
| Bounty | Needs mature rankings | Low now, medium later | Very high now | Low-medium | Schema variables only |
| Live events | Needs event substrate | Medium later | High now | Medium | Variable substrate only |
| Prestige | Needs mature XP curve | Low now, medium later | High now | Low-medium | Far future; preserve skill rating |
| Onboarding/buddy | Fits auth/profile/presence | Medium-high | Medium | High | First-visit flow after frontend gate |
| Frontend contract | Cross-repo gap | High | High if ignored | High | Hestia/Themis route/API matrix |
| Cross-repo compatibility | Platform gap | High | High if ignored | Very high | Version matrix and compatibility tests |

## Consolidated Roadmap

### Phase 1: Trust Substrate

Sequential. Do not parallelize across these unless write scopes are genuinely disjoint and gates are explicit.

| Tracer | Module | Adds | Required gates |
| --- | --- | --- | --- |
| T18a | docs/platform | Docs truth cleanup | 1 |
| T18b | `cmd/apollo/competition` | CLI parity for existing competition/lobby/match/rating reads | 1, 3 |
| T18c | `internal/api/capabilities` | Capability discovery and mutation outcome schema | 3, 6 |
| T18d | application command layer | Shared command DTOs for HTTP/CLI/dry-run | 3, 6 |
| T19a | `internal/rating` | Extract current Elo-like policy unchanged | 4 |
| T19b | `internal/rating/policy` | Versioned policy interface and golden tests | 4 |
| T19c | `internal/rating/audit` | Rating event audit table and rebuild runbook | 4, 8 |
| T20 | competition schema/service | Match tier, rating multiplier, consensus mode fields | 2, 6 |
| T21 | `internal/competition/consensus` | Player/staff score consensus voting | 3, 6 |
| T22 | `internal/competition/dispute` | Dispute ledger and correction flow | 5 |
| T23 | `internal/rating/openskill` | OpenSkill dual-run and read-path migration | 4, 8 |

Unlock after Phase 1:

- Public rating can be considered only after Dispute, Privacy, Telemetry, and Scale gates.
- ARES v2 can consume ratings safely.
- Tournament work can move from schema-only to internal runtime.

### Phase 2: Parallel-Safe Additions

These are not public-stakes features. They can begin alongside Phase 1 when write scopes are isolated and relevant dependency gates are explicit. Rating-dependent rows still wait for OpenSkill dual-run or an explicit legacy-rating-only ruling.

| Tracer | Module | Adds | Constraints |
| --- | --- | --- | --- |
| T18e | `themis` internal competition ops shell | Staff-only competition read/control surface over existing APIs | Gate 9; no public tournaments, no social, no client-side trusted token |
| T18f | `internal/workflow` | Approval/proposal primitive for propose, approve, reject, expire, withdraw | Gates 3, 6; not a generic workflow engine |
| T18g | `internal/competition/lifecycle` | Match lifecycle state machine for pending, vote, consensus, finalized, disputed, corrected | Gates 3, 5, 6; rating fires only from canonical finalized state |
| T18h | `internal/notifications` | One-way notification substrate for approvals, votes, disputes | Gates 6, 7; no messaging, DMs, chat, or guild communications |
| T24p | `internal/rating/transfer` | Cross-sport skill family graph and cold-start seeding | Internal only; no public rank claims |
| T25p | `internal/rating/analytics` | Carry, tank, stability coefficients | Internal display until Privacy Gate |
| T26p | `internal/competition/sediment` | Ad-hoc to persistent team promotion | Requires real ATHENA presence proof |
| T27p | `internal/xp/checkin` | Daily check-in XP | Private, facility-scoped, one credit/day |
| T28p | `internal/tournament` schema | Tournament containers, brackets, snapshots | Schema/internal only |
| T28q | frontend contract docs | Hestia/Themis route/API matrix | Gate 9 |
| T28r | platform compatibility docs | Cross-repo compatibility matrix | Gate 10 |

### Phase 3: Skill-Aware Matchmaking

| Tracer | Module | Adds | Required gates |
| --- | --- | --- | --- |
| T29 | `internal/ares` | Closed by 3B.15 as proposal-only sport/mode/facility queue intent and rating-aware preview foundation | 4, 5, 6 |
| T30 | `internal/rating/calibration` | Provisional/hidden mu, boosted sigma shrink, comeback framing | 4 |
| T31 | `internal/rating/decay` | Nightly sigma inflation for inactive players | 4, 6 |
| T32 | `internal/competition/upset` | Win probability, upset flag, small underdog bonus | 4, 5 |

### Phase 4: Tournament Runtime

Tournament runtime should consume substrate primitives instead of inventing them inline. Approval/proposal, match lifecycle, notifications, resource splitting, and schedule policy should exist as reusable packets before tournament runtime grows beyond internal staff containers.

| Tracer | Module | Adds | Required gates |
| --- | --- | --- | --- |
| T32a | `internal/schedule/policy` | Recurring schedule policy: timezone, DST, exceptions, end rules, conflict behavior | 3, 6, 9 |
| T32b | `internal/schedule/resources` | Court/resource splitting: full courts, half courts, lanes, conflict edges, capacity | 3, 6 |
| T33 | `internal/tournament/format` | Single elim, double elim, Swiss, round robin | 5 |
| T34 | `internal/tournament/seeding` | Rating seeding, manual override, byes | 5, 6 |
| T35 | `internal/tournament/to_tools` | Themis bracket builder, walkover, officials | 5, 9 |
| T36 | `internal/competition/replay` | Structured match event log | 6 |
| T37 | `internal/competition/pregame` | Warmup, coin toss, preview cards, optional bans | 5 |

### Phase 5: Social And Safety

| Tracer | Module | Adds | Required gates |
| --- | --- | --- | --- |
| T38 | `internal/social` | Silent block list, reports, triage, action attribution | 5, 7 |
| T39 | `internal/honor` | Honor votes, trust tiers, trust history | 5, 7 |
| T40 | `internal/reliability` | Attendance, no-show, substitution, reliability score | 6 |

### Phase 6: Public Surface

| Tracer | Module | Adds | Required gates |
| --- | --- | --- | --- |
| T41 | Hestia/Themis/public reads | Public bracket viewing and tournament pages | 5, 7, 8, 9 |
| T42 | `internal/competition/nemesis` | Mortal Rivals, Iron Sharpens Iron | 5, 7 |
| T43 | `internal/predictions` | Pickem and prediction leaderboards | 7 |
| T44 | `internal/records` | Personal/facility records, milestones, hall of fame | 7, 8 |

### Phase 7: Retention Layer

No public badge should ship before dispute and privacy gates.

| Tracer | Module | Adds | Required gates |
| --- | --- | --- | --- |
| T45 | `internal/badges` | Trophy registry, criteria engine, rarity | 5, 6 |
| T46 | `internal/quests` | Daily/weekly/monthly challenges, podium, toggle | 6, 7 |
| T47 | `internal/xp/season` | Season XP, battle pass, prestige scaffold | 6 |
| T48 | `internal/spectate` | Match feed, cheer, watchlist, highlights | 7 |
| T49 | `internal/skill_drills` | Drill catalog, submissions, PRs, calibration prior | 4, 6 |
| T50 | `internal/composite` | Multi-sport athlete score and classifications | 4, 6 |
| T51 | `internal/mvp` | MVP voting, tiny bonus, MVP history | 5, 7 |
| T52 | `internal/draft` | Auto-captains, suggested picks, snake draft | 5 |
| T53 | `internal/onboarding` | First visit, buddy pair, welcomer, first win | 6, 7, 9 |

### Phase 8: Long-Term Engines

Schema-only now, deploy later.

| Tracer | Module | Adds | Status |
| --- | --- | --- | --- |
| T54 | `internal/guilds` | Squad schema, duties, guild leaderboard variables | Schema only; no messaging |
| T55 | `internal/competition/bounty` | Kill count, bounty value variables | Schema only |
| T56 | `internal/live_events` | Agent-triggered event variable substrate | Schema only |
| T57 | `internal/prestige` | Voluntary XP reset; skill preserved | Far future |

## Feature Family Details

### Substrate Decomposition

The plan should prefer narrow primitives that compose into product features. This keeps Phase 3 from bleeding into Phase 1 and prevents tournament/social/retention work from inventing duplicate state machines.

Accepted substrate packets:

- Internal Themis competition ops shell: staff-only internal operations over existing competition APIs.
- Approval/proposal workflow: reusable propose, approve, reject, expire, withdraw, and attribute primitive.
- Match lifecycle state machine: canonical match states consumed by consensus voting, disputes, corrections, replay, and ratings.
- Notification substrate: one-way prompts and alerts only; no user messaging system.
- Schedule policy: recurring scheduling rules isolated from one-off booking and tournament logic.
- Resource splitting: court and lane subdivision modeled as a reusable resource graph.

Ruling: these packets are not public surfaces. They are substrate work. They may run in parallel with Phase 1 only when their write scope is isolated and their gates are explicit.

### Internal Themis Competition Ops Shell

The audit now separates internal staff competition operations from public
tournaments. Internal Themis competition ops can ship earlier because it is a
staff control surface over existing APOLLO behavior, not a public bracket/social
product. Phase 3B.11 shipped the first internal Themis shell over APOLLO
readiness/command/session contracts only.

Initial allowed scope:

- Competition session list/detail.
- Queue membership view.
- Existing staff actions already represented by APOLLO APIs.
- Match preview, assignment, start/archive, and result lifecycle controls only
  where backend contracts are real. Since Phase 3B.12, result command apply is
  available only through APOLLO-backed versioned lifecycle/result contracts.
- Error, empty, loading, and denied states for every route.

Hard stops:

- No public tournament pages.
- No public leaderboards, badges, CP, squads, rivalry, or social surfaces.
- No new competition domain model in Themis.
- No browser-delivered trusted-surface token.

Security caveat: APOLLO trusted-surface proof is server/staff infrastructure proof, not a browser secret. If Themis is browser-only for a mutation path, that path must either remain read-only or go through a server-side mediator/BFF that owns trusted-surface proof and staff attribution.

### Approval And Proposal Workflow

The reusable primitive is:

```text
proposed -> approved
         -> rejected
         -> expired
         -> withdrawn
```

Best approach:

- Store proposal type, target entity, proposed payload, proposer, approver, rejection reason, expiry, expected version, idempotency key, and attribution.
- Keep the primitive boring and narrow; do not build a generic business-process engine.
- Consumers supply validation and apply logic.
- Approval does not mutate the target until the consumer command applies with the expected version.

Likely consumers:

- Booking changes.
- Schedule changes.
- Competition result corrections.
- Dispute resolutions.
- Social moderation actions.
- Tournament seeding/manual override approval.

Durability: very high if it remains a small state machine plus attribution ledger.

### Match Lifecycle State Machine

Consensus voting and disputes need one canonical lifecycle. Otherwise result finalization, rating updates, corrections, and replay logs will drift.

Recommended states:

```text
pending -> vote_open -> consensus_reached -> finalized
                              |                  |
                              v                  v
                          escalated          disputed
                              |                  |
                              v                  v
                          staff_resolved -> corrected
```

Rules:

- Ratings fire only from canonical finalized/corrected result truth.
- A disputed result remains visible as disputed, not silently overwritten.
- Corrections are additive events that supersede prior canonical projections.
- State transitions require expected version and attribution.
- Lifecycle state is not a substitute for the dispute ledger; it is the state spine the ledger hangs from.

### Notification Substrate

Notifications are required for approvals, score votes, disputes, staff actions, and tournament operations. This should not become messaging.

Allowed scope:

- In-app/staff inbox notifications.
- One-way prompts and alerts.
- Idempotent notification creation.
- Read/unread and acknowledged state.
- Optional future delivery adapters for email or push.

Hard stops:

- No chat.
- No DMs.
- No guild/squad messaging.
- No arbitrary user-to-user content channel.

Durability: high if implemented as an outbox-backed notification primitive with rate limits and role-aware visibility.

### Schedule Policy And Resource Splitting

Recurring schedules and court splitting are substrate packets, not tournament features.

Recurring schedule policy should handle:

- Timezones.
- DST.
- Exceptions and holidays.
- End conditions.
- Skipped occurrences.
- Conflict rules.
- Audit of generated instances.

Resource splitting should model:

- Facility, room, court, sub-court, lane, and station nodes.
- Conflict edges, such as full court conflicts with both half courts.
- Capacity and supported sports/modes per resource.
- Recomposition rules, such as two half courts blocking one full court.
- Consumers for booking, lobbies, tournaments, practice, and capacity planning.

Ruling: one-off scheduling can stay simple. Recurrence and splitting should be isolated until the resource graph and conflict policy are explicit.

### Match Tiers

Best approach:

- Add tier fields to `competition_sessions`.
- Keep all behavior internal first.
- Drive rating multiplier, consensus mode, officiator requirement, replay requirement, and dispute level from policy.

Initial fields:

```sql
match_tier TEXT
rating_multiplier NUMERIC
requires_officiator BOOLEAN
consensus_mode TEXT
max_mu_delta_per_match NUMERIC
```

Durability: high if backed by policy versioning. Low if scattered as if/else logic through handlers.

### Consensus Voting

Current result capture is a staff write that immediately completes the match. Consensus requires a pending state.

Best approach:

- Add `competition_match_score_votes`.
- Add `competition_match_consensus_state`.
- Do not complete match/session until consensus or staff resolution.
- Staff override writes attribution and dispute event.

Failure mode if rushed:

- Duplicate result paths.
- Ratings fire before consensus.
- Disputes become impossible because result was finalized too early.

### Disputes And Corrections

Public ratings require a correction path. Immutability is good, but "immutable wrong result" is not launch-safe.

Best approach:

- Keep original result immutable.
- Add correction events that supersede result for derived reads.
- Rebuild ratings from canonical event stream.
- Attribute every action.

Minimum tables:

- `competition_disputes`
- `competition_dispute_events`
- `competition_result_corrections`
- `competition_canonical_result_view` or derived query pattern

Durability: very high. This is foundational.

### ARES V2

The historical lobby preview still pairs eligible joined lobby members by stable
user ID. Phase 3B.15 adds the separate competition ARES v2 proposal foundation
over explicit queue intent and rating facts.

Best approach:

- Keep explicit matchmaking intent facts for sport, mode, facility, and tier.
- Read active ratings from the versioned legacy projection while OpenSkill
  remains internal comparison only.
- Produce previews with APOLLO-computed quality, win probability, and
  explanations.
- Do not mutate competition execution state, result truth, or rating truth on
  preview.

Durability: high if ARES remains proposal engine, not result/rating owner.

### Cross-Sport Transfer

This is a seeding feature, not a result-truth feature.

Best approach:

- Add skill-family graph.
- Seed new sport rating from best related sport.
- Mark calibration status.
- Keep public display conservative: "provisional from related sport."

Likelihood: medium-high after rating module exists.

### Carry Coefficient

Read-only analytics over completed matches.

Best approach:

- Compute internally first.
- Use expected win probability from rating policy.
- Avoid public shaming labels.
- Expose positive recognition before negative "tank" labels.

Durability: medium-high if not coupled to social features too early.

### Sediment Teams

Current teams are session-scoped. Do not retrofit them into permanent identity.

Best approach:

- Keep `competition_session_teams` as match/session truth.
- Add `persistent_teams`.
- Add `persistent_team_memberships`.
- Add promotion event from ad-hoc session team to persistent team.
- Require ATHENA co-presence proof for sediment claims.

ATHENA dependency:

- Mock data is insufficient for the real claim.
- Cross-repo compatibility and ingress truth must be proven.

### Tournaments

There are two different tournament concepts:

1. Internal tournament containers and staff-run brackets.
2. Public tournaments with standings, spectators, records, and badges.

Internal schema can land earlier. Public tournaments require disputes, privacy, scale, and frontend gates.

Best approach:

- Schema-first for `tournaments`, `tournament_entries`, `tournament_brackets`, `tournament_team_snapshots`.
- Use existing competition matches as match execution truth.
- Add immutable snapshots at lock time.
- Add public read surfaces later.

### Social Safety

Silent block and reporting are not optional once public social surfaces exist.

Best approach:

- Manager-only silent block list.
- Reports with triage state.
- Social action attribution.
- Reporter feedback that does not reveal private action.
- Rate limits and abuse detection before public launch.

Durability: high if kept separate from skill rating.

### XP, Badges, Quests, Records

These are not foundation. They amplify whatever truth exists underneath.

Best approach:

- First add event ledger.
- Then private daily check-in XP.
- Then criteria registry.
- Then badges/quests.
- Public display only after dispute/privacy gates.

Critical ruling:

- No public badges from match outcomes until dispute flow exists.
- No public records until scale and leak tests exist.

### Power Rating

Skill must dominate.

Recommended formula family:

```text
CP = round(mu * 100 * (1 + min(0.20, badge_bonus + xp_bonus)))

badge_bonus capped at 0.10
xp_bonus capped at 0.10
```

Ruling:

- CP is a display score, not rating truth.
- Current 3B.13 rating truth remains legacy APOLLO mu/sigma plus policy metadata;
  any OpenSkill rating truth waits for a later approved dual-run/cutover.
- XP/badges can never let a low-skill grinder outrank a high-skill player in pure skill comparison.

### Frontend Contract

Hestia has an `/app/tournaments` route, but Phase 3B.17 tournament runtime is
staff/internal APOLLO truth only. It is not a public/member tournament contract.

Best approach:

- Add a frontend contract matrix.
- For each Hestia/Themis route, list API endpoints, states, auth role, empty state, error state, and production/stub status.
- Production builds must not silently call stubbed endpoints.
- Payload changes must be additive unless the frontend release is coordinated.
- Current rating fields such as `current_rating_mu` and `current_rating_sigma` should be preserved or versioned; do not break Hestia by swapping the backend rating model.
- Trusted-surface tokens are staff/server-side proof only. They must not appear in browser-delivered frontend bundles, local storage, or client-side environment variables.

Durability: high. This prevents product surfaces from outrunning backend truth.

### Cross-Repo Compatibility

The launch depends on independent repositories:

- APOLLO
- ATHENA
- Hestia
- Themis
- ashton-proto
- gateway/platform

Best approach:

- Maintain compatibility matrix in platform docs.
- Record minimum compatible versions.
- Add smoke tests for cross-repo assumptions.
- Any shared event contract must come from `ashton-proto`, not local copies.

Durability: very high.

Minimum matrix:

| Repo | Required for current audit path | Compatibility trigger |
| --- | --- | --- |
| APOLLO | Always | Any backend schema/API/rating/competition change. |
| ATHENA | Presence, sediment teams, check-in XP, ops occupancy | Any physical-presence contract, event, or staleness semantics change. |
| Hestia | Member shell, public/member tournament views, member profile stats | Any member-facing payload or route contract change. |
| Themis | Staff schedule controls, future TO tools, staff competition UI | Any staff API, trusted-surface, tournament, or moderation workflow change. |
| ashton-proto | Shared wire contracts only | Any cross-repo event/API contract that should not be local JSON. |
| gateway/platform | Routed deployment, compatibility docs, release matrix | Any public route, deploy truth, or multi-service version requirement. |

No `ashton-proto` widening is needed for rating internals unless rating/result events become shared cross-repo wire contracts.

### Branding And Naming

ASHTON is the internal platform codename. Public brand naming is not decided in APOLLO. Public naming, distribution branding, and brand hardening should be decided in platform/frontend planning docs before Hestia or public tournament surfaces expose permanent names.

## Test Data Strategy

Test data must compose across repos without inventing fake truth:

| Layer | Fixture strategy | Notes |
| --- | --- | --- |
| ATHENA physical truth | Use TouchNet fixture HTML/CSV replay and edge tap fixtures for source events. | Mock/CSV prove shape; live sediment/check-in claims require real ATHENA ingress before public trust. |
| APOLLO member truth | Use full active migration stack and explicit user/session/profile/lobby fixtures. | Do not use stale partial migration bootstraps for generated query surfaces. |
| Competition truth | Build sessions/teams/matches through service/API helpers, then record results. | Avoid direct DB shortcuts for behavior tests except narrow migration tests. |
| Rating truth | Use golden cases plus rating event fixtures. | Golden outputs are reviewable behavior contracts, not casual examples. |
| Frontend truth | Hestia/Themis mocks must match APOLLO payloads and empty/error states. | Production routes cannot silently stub data. |
| Cross-repo truth | Shared event payloads come from `ashton-proto` helpers/fixtures where contracts are shared. | No local recopied JSON shape for shared lifecycle events. |

Golden cases, ATHENA fixtures, and APOLLO competition fixtures should be linked in each tracer's hardening artifact so reviewers can tell which truth layer is under test.

## Leak Test Specification

Privacy Gate requires concrete leak tests.

Minimum leak tests:

| Surface | Probe | Current Status (2026-04-29) |
| --- | --- | --- |
| Member history | Cannot see other member match history through IDs or filters | Self-scoped reads enforced |
| Competition staff read | Member role gets 403 | Capability matrix enforced |
| Public booking status | Receipt lookup never exposes request UUID, staff notes, schedule block ID, staff ID, conflicts, trusted surface fields | Maintained |
| Public competition readiness | Anonymous/member read never exposes private presence, block/report state, hidden ratings, dispute notes, staff notes, raw user IDs, internal request/match/canonical-result IDs, projection watermarks, sample sizes, OpenSkill mu/sigma/comparison, ARES internals, command internals, or trusted-surface fields | Public output contract enforces; 3B.19.1 hardening removed `sample_size` and `confidence` |
| Public game identity | CP/badge/rivalry/squad facts redacted to public-safe scope; no raw IDs, no hidden ratings, no policy internals beyond version strings | Enforced; 3B.20.1 hardening scoped rivalry to projection context |
| Social reports | Reporter cannot learn exact action against reported user | Manager-internal only; no member-facing surface yet |
| Silent blocks | Blocked user cannot infer block existence from explicit response | Manager-internal only |
| Leaderboards | Hidden/private/ghost users are excluded or anonymized by policy | Allowlisted projection enforces |
| Tournament public | Roster, age/student identity, and presence are not leaked beyond approved display fields | Public tournaments not yet exposed; rule reaffirmed for future packet |

Action: every public surface added in future packets must extend this table with its own probe set and current status before launch.

## Telemetry Requirements

Milestone 3.0 closed the bounded telemetry export requirement for the shipped
APOLLO trust spine: APOLLO exposes `/metrics`, the required
competition/rating/ARES/booking/safety/reliability/game-identity counters are
present, and Prometheus scrape proof was recorded. This is enough for Gate 6 to
be marked pass for the current shipped scope.

Still open: tuned alert thresholds, production-load validation beyond the
locally/runtime-proved numeric ceilings, and runtime alert validation under
real load. Do not claim full production Scale Gate closure from metric
existence alone.

Minimum counters before public competition:

- `competition_result_write_attempt_total`
- `competition_result_write_reject_total{reason}`
- `competition_consensus_vote_total{tier,outcome}`
- `competition_dispute_open_total{tier,reason}`
- `competition_dispute_resolution_duration_seconds`
- `rating_update_duration_seconds{policy_version}`
- `rating_rebuild_rows_scanned_total`
- `rating_policy_delta_total{legacy,openskill}`
- `ares_preview_total{sport,mode,tier}`
- `ares_queue_assignment_failure_total{reason}`
- `trusted_surface_failure_total{reason}`
- `booking_public_submit_total`
- `booking_idempotency_conflict_total`
- `booking_approval_conflict_total`
- `public_leak_test_failure_total`
- `tournament_advancement_total{format}` (post-3B.17)
- `tournament_seed_lock_total` (post-3B.17)
- `safety_report_open_total{category}` (post-3B.18)
- `safety_block_active_total` (post-3B.18)
- `reliability_event_total{kind}` (post-3B.18)
- `public_competition_readiness_request_total` (post-3B.19)
- `public_leaderboard_request_total` (post-3B.19)
- `public_game_identity_request_total` (post-3B.20)
- `game_identity_projection_duration_seconds` (post-3B.20)

Minimum alert examples:

- Rating update p95 exceeds defined ceiling.
- Dispute queue older than SLA.
- Trusted-surface failures spike.
- Public receipt not-found rate spikes.
- Result duplicate rejects spike.

## Observability Plan

Dashboards were not required for the internal-only cleanup phases, but future
widening beyond the shipped public-safe projections needs visible operational
health. The current exported metrics are the baseline; dashboards and alert
threshold tuning remain the next observability hardening layer.

Minimum dashboards:

- Competition operations: result attempts, rejects, consensus state, dispute backlog, staff attribution writes.
- Rating health: update duration, rebuild duration, rows scanned, policy version, legacy/OpenSkill deltas.
- ARES queue health: preview volume, queue intent distribution, assignment failures, match quality distribution.
- Public safety: public read errors, leak-test failures, report volume, block/report moderation backlog.
- Booking/schedule baseline: public submits, idempotency conflicts, approval conflicts, linked-reservation drift.

Initial SLO targets should be conservative and explicit:

- Internal competition result write p95 under 500 ms while row volume remains below the Phase 1 scale ceiling.
- Rating rebuild for one sport under a documented local budget before any public leaderboard.
- Dispute L1 response within 1 hour, staff review within 24 hours, tournament panel within 72 hours.
- Public status/read endpoints p95 under 300 ms after public route exposure.

These are planning targets, not current guarantees. A tracer must either
implement metrics, extend existing metrics, or state why metrics are out of
scope.

## Capacity Constraints

The Scale Gate now has explicit APOLLO-local numeric ceilings before any
broader public rankings work. These are repo/local runtime ceilings, not full
production load validation:

| Resource | Current posture | Required ceiling before widening |
| --- | --- | --- |
| Rating recompute | Synchronous full-sport delete/rebuild after finalized/corrected canonical result changes. | Max 10,000 finalized/corrected canonical results per sport/mode; max 20,000 result-side rows; max 2,500 rated participants; local p95 <= 3s; hard max <= 5s. |
| Public readiness read | APOLLO public-safe read over canonical result and analytics projection counts. | Local p95 <= 150ms; hard max <= 300ms. |
| Public leaderboard read/projection | APOLLO public-safe projection over allowlisted analytics rows with response cap. | Max 5,000 candidate rows per filtered request; response cap <= 100 rows; local p95 <= 300ms; hard max <= 750ms. |
| Game identity projection | APOLLO public/member-safe projection over public-safe competition rows. | Max 10,000 public-safe projection rows; max 2,500 participant-context rows; max 25,000 rivalry/squad candidate comparisons; local p95 <= 500ms; hard max <= 1s. |
| CLI/API smoke | APOLLO CLI/API paths route through existing command/service/HTTP handlers. | Full bounded smoke sequence <= 30s. |
| Postgres | Single service DB assumptions in local/runtime tests. | Database size, connection pool, backup, and restore targets documented. |
| NATS publish | ATHENA publish retry/dedupe exists, but APOLLO downstream idempotency still matters. | Restart/replay behavior and duplicate event budget documented. |
| Homelab/deploy nodes | Milestone 3.0 proved bounded APOLLO/ATHENA smoke and Prometheus scrape; Hestia/Themis/gateway live deployment proof was not present in the inspected environment. | Node count, CPU/RAM headroom, tunnel/proxy limits, and per-service deployment status documented before broader public claims. |
| Frontend routes | Hestia/Themis consume APOLLO APIs. | Payload stability and fallback/error behavior documented before public release. |

### Scale Gate Numeric Ceiling Proof

Verified locally on 2026-05-03:

| Path | Measured rows | Measured duration | Ceiling result |
| --- | --- | --- | --- |
| Rating recompute kernel | 10,000 canonical results, 20,000 result-side rows, 2,500 rated participants | p95 23.644417ms; hard max 23.644417ms | Pass against p95 <= 3s and hard max <= 5s |
| Public readiness API read | Existing APOLLO runtime fixture | p95 1.020583ms; hard max 1.020583ms | Pass against p95 <= 150ms and hard max <= 300ms |
| Public leaderboard API read/projection | 5,000 candidate-row service proof; response cap 100 | API p95 1.151333ms; hard max 1.151333ms | Pass against p95 <= 300ms and hard max <= 750ms |
| Game identity API projection | 10,000 projection-row ceiling declared; 2,500 participant-context service proof; 1,225 candidate pair comparisons after response cap | API p95 1.721708ms; hard max 1.721708ms | Pass against p95 <= 500ms and hard max <= 1s |
| CLI smoke | Command readiness plus result record/finalize/dispute/correct/void through APOLLO CLI command path | 301.188125ms | Pass against <= 30s |
| API smoke | Public readiness, public leaderboard, public game identity, member game identity, command readiness through APOLLO HTTP handlers | 19.024666ms | Pass against <= 30s |

Reproducible commands:

```sh
go test -count=1 -v ./internal/competition -run 'TestCompetitionScaleCeiling'
go test -count=1 -v ./internal/server -run 'TestCompetitionScaleCeilingAPIReadSmoke'
go test -count=1 -v ./cmd/apollo -run 'TestCompetitionCommandCLIResultLifecycleSmoke'
go test -count=1 ./internal/competition ./internal/rating ./internal/server
```

Runtime notes:

- APOLLO now rejects rating recompute attempts above the declared local row
  ceilings before rebuilding the active legacy projection.
- Public leaderboard and game identity repository reads are bounded by APOLLO
  candidate-row caps, and service response caps remain unchanged.
- Existing telemetry still exports rating rebuild row/duration and game
  identity duration signals; no deploy/GitOps/Prometheus change was required.
- Hestia and Themis contracts are unchanged; no frontend verification was
  required because no frontend contract changed.
- Deployed truth remains the Milestone 3.0 APOLLO/ATHENA truth. These numbers
  do not claim full production load validation.
- OpenSkill remains comparison-only and off the active read path.
- Public tournaments remain blocked.

## Data Quality Rules

- Presence must not create workout, lobby, match, social, or rating state implicitly.
- Lobby eligibility and lobby membership remain separate.
- Lobby membership and competition queue remain separate.
- ARES preview remains non-mutating.
- Match result truth must be tied to real match side slots.
- Public display must be derived from canonical, dispute-aware result state.
- Rating updates must be reproducible from event/audit rows.
- Every public/staff mutation must have replay protection.
- Every correction must preserve original facts and add superseding facts.
- Rating backfills must be idempotent.
- Concurrent result writes must remain safe and produce one canonical result path.
- Schema migrations must have integration tests for current and migrated state where ratings/results are touched.

## Persistence Invariants

Minimum persistence rules:

- Raw/accepted ATHENA physical observations are append-only and privacy-safe; APOLLO must not rewrite ATHENA source truth.
- APOLLO competition original results remain immutable; corrections are additive superseding facts.
- Rating current rows are derived projections and must be rebuildable from canonical result/rating event truth.
- Staff action attributions survive as audit facts and are not replaced by derived summaries.
- Public booking receipt/status truth must keep opaque receipt codes and never require exposing internal IDs.
- Idempotency keys and expected versions are part of replay safety and must survive process restarts.
- Analytics projections are derived and rebuildable from canonical result,
  legacy rating, and analytics event facts; they must never become source
  result, rating, ARES, or public truth.
- Backups must preserve result, correction, rating event, analytics event, staff attribution, booking receipt, and ATHENA observation history before public surfaces depend on them.

Postgres-loss recovery target is not defined yet. Before public competition launch, define which projections can be rebuilt, which append-only facts must be restored from backup, and which external sources can be replayed.

## Decision Log

Use this table to link future rulings to PRs, commits, or conversation artifacts.

| Date | Decision | Source |
| --- | --- | --- |
| 2026-04-26 | Launch expansion must be gated; trust substrate before public excitement layer. | This audit consolidation. |
| 2026-04-26 | Hybrid OpenSkill means OpenSkill kernel plus APOLLO policy plus legacy Elo characterization/fallback, not two permanent competing ratings. | This audit consolidation. |
| 2026-04-26 | CLI should start as a service-backed wrapper and evolve toward a shared application command layer; no independent CLI domain model. | This audit consolidation. |
| 2026-04-27 | Substrate decomposition accepted: internal Themis ops shell, approval/proposal workflow, match lifecycle, notifications, schedule policy, and resource splitting are reusable primitives, not public surfaces. | This audit consolidation. |
| 2026-04-27 | Phase 3B.11 shipped only command/readiness/CLI/Themis ops foundation; result trust, OpenSkill, analytics, tournament runtime, public competition surfaces, and game identity were still deferred at that closeout. | 3B.11 closeout. |
| 2026-04-27 | Phase 3B.12 shipped lifecycle/result trust only: canonical result identity, recorded/finalized/disputed/corrected/voided facts, correction supersession, and finalized/corrected-only rating consumption. Rating extraction, OpenSkill, analytics, tournament runtime, public surfaces, and game identity were still deferred at that closeout. | 3B.12 closeout. |
| 2026-04-27 | Phase 3B.12.1 cohesion hardening found no runtime, Themis, Hestia, or docs truth drift; no patch worker changes were required. | 3B.12.1 hardening closeout. |
| 2026-04-27 | Phase 3B.13 shipped legacy rating foundation only: current rating math is explicit, versioned, golden-tested, auditable, bound to finalized/corrected canonical results, and stored with deterministic projection watermarks. OpenSkill remains deferred to 3B.14. | 3B.13 closeout. |
| 2026-04-28 | Phase 3B.14 shipped OpenSkill dual-run comparison only: internal OpenSkill comparison rows/events, legacy/OpenSkill deltas, accepted budgets, scenarios, and delta flags are real while the legacy rating projection remains the active read path. OpenSkill cutover, ARES v2, analytics, tournaments, public surfaces, CP/badges/rivalry/squads, and SemVer governance were still deferred at that closeout. | 3B.14 closeout. |
| 2026-04-28 | Phase 3B.14.1 cohesion hardening fixed OpenSkill delta flag/storage boundary coherence, added focused boundary coverage, and corrected stale 3B.14/SemVer docs truth. OpenSkill remains internal dual-run only, the legacy read path remains active, canonical-result-only rating guards still hold, comparison facts remain deterministic/auditable, and no Hestia 3B.14 comparison leak was proven. | 3B.14.1 hardening closeout. |
| 2026-04-28 | Phase 3B.15 shipped ARES v2 proposal/match-preview foundation only: queue intent facts, internal preview projections/events, match quality, predicted win probability, and explanation codes are real over trusted APOLLO projections while ARES remains a proposal engine and not a result, rating, booking, or public competition owner. OpenSkill read-path switch, analytics, tournament runtime, public competition surfaces, CP/badges/rivalry/squads, and SemVer governance were still deferred at that closeout. | 3B.15 closeout. |
| 2026-04-28 | Phase 3B.16 shipped competition analytics foundation only: internal stat events and analytics projections are deterministic, versioned, and derived from finalized/corrected canonical results plus legacy active rating facts. Dashboard-first work, public profiles/stats/scouting, carry coefficient, tournament runtime, public competition surfaces, CP/badges/rivalry/squads, OpenSkill read-path switch, and SemVer governance were still deferred at that closeout. | 3B.16 closeout. |
| 2026-04-28 | Phase 3B.17 shipped internal tournament runtime only: staff-only tournament, bracket, seed, immutable team snapshot, match binding, round advancement, advance-reason, and tournament event facts are real; advancement consumes finalized/corrected canonical result truth only and does not own result, rating, analytics, ARES, public, or game identity truth. Public tournaments remain deferred beyond 3B.19. | 3B.17 closeout. |
| 2026-04-28 | Phase 3B.18 shipped internal social safety/reliability foundation only: competition-scoped report facts, block facts, reliability events, safety audit events, capability-gated manager readiness/review reads, and aligned safety/reliability commands are real; safety facts remain private manager/internal truth and do not mutate result, rating, analytics, ARES, tournament, public, member, or game identity truth. Public competition readiness closed later in 3B.19. | 3B.18 closeout. |
| 2026-04-29 | Phase 3B.19 shipped public-safe competition readiness and leaderboard projections only: redacted contracts over finalized/corrected canonical result truth plus legacy active rating fields. Public tournaments, public safety details, messaging/chat, OpenSkill read-path switch, and broader social graph remain deferred. | 3B.19 closeout and 3B.19.1 hardening closeout. |
| 2026-04-29 | Phase 3B.20 shipped public/member-safe game identity projections only: CP, badge awards, rivalry state, and squad identity are APOLLO-derived from public-safe competition projection rows. Broader social rivalry, persistent guilds, messaging/chat, public tournaments, and OpenSkill cutover remain deferred. | 3B.20 closeout and 3B.20.1 hardening closeout. |
| 2026-04-29 | Milestone 3.0 closed bounded deploy smoke, telemetry export, Prometheus scrape proof, and cross-repo compatibility matrix truth for APOLLO/ATHENA scope while recording Hestia, Themis, and gateway as repo-only in the inspected environment. Scale Gate remains partial. | Milestone 3.0 evidence pack and compatibility matrix. |
| 2026-05-03 | Scale Gate numeric ceilings are declared and locally/runtime-proved for APOLLO rating recompute, public readiness, public leaderboard, game identity, and CLI/API smoke paths. This does not change deployed truth and is not full production load validation. | `go test -count=1 -v ./internal/competition -run 'TestCompetitionScaleCeiling'`; `go test -count=1 -v ./internal/server -run 'TestCompetitionScaleCeilingAPIReadSmoke'`; `go test -count=1 -v ./cmd/apollo -run 'TestCompetitionCommandCLIResultLifecycleSmoke'`; focused package test run. |
| 2026-05-03 | CLI Demo Spine is closed locally in repo/runtime: APOLLO CLI exposes the competition spine through existing service/command truth for public/member projections, command dry-run/apply, result lifecycle, ARES preview generation, analytics-backed projection reads, sessions, tournaments, and manager/internal safety reads without frontend dependency or CLI-owned formulas. Deployed truth is unchanged. | `go test -count=1 ./cmd/apollo`; `go test -count=1 ./internal/competition ./internal/rating ./internal/server`; `go vet ./...`; `go build ./cmd/apollo`; `go test -count=1 ./...`; `go test -race ./internal/...`. |
| 2026-05-03 | Rating Policy Wrapper is closed locally in APOLLO repo/runtime: active legacy-engine rating projection uses `apollo_rating_policy_wrapper_v1` with calibration status, fifth-match ranked transition, inactivity sigma inflation, and upward movement cap metadata. OpenSkill remains comparison-only against the legacy baseline; deployed truth is unchanged. | `go test -count=1 ./internal/rating`; `go test -count=1 ./internal/competition ./internal/rating ./internal/server ./cmd/apollo`; `go test -count=1 ./db/migrations`; `go vet ./...`; `go build ./cmd/apollo`; `go test -count=1 ./...`; `go test -race ./internal/...`. |
| 2026-05-03 | Rating Policy Simulation / Golden Expansion is closed locally in APOLLO repo/runtime: deterministic fixtures and `apollo competition rating simulation --format json` prove active wrapper scenarios, legacy baseline deltas, OpenSkill sidecar deltas, accepted/rejected classification, cutover blockers, and policy risks. OpenSkill remains comparison-only; deployed truth is unchanged. | `git diff --check`; `go test -count=1 ./internal/rating`; `go test -count=1 ./internal/competition ./internal/rating ./internal/server ./cmd/apollo`; `go vet ./...`; `go build ./cmd/apollo`; `go test -count=1 ./...`; `go test -race ./internal/...`. |

## Hard Non-Touches

Reaffirmed from the 2026-04-29 competition system audit. Future packets must not implicitly create or modify:

- messaging / chat
- broad public social graph
- public profiles / scouting
- public / member safety UI
- public safety report / block / reliability details
- public tournaments
- OpenSkill active read path
- booking / commercial / proposal workflows beyond shipped scope
- public self-booking, public self-edit/rebook, payments, quotes, invoices, deposits, checkout
- recurring schedule policy or broad operating-hours editing
- project-wide SemVer governance
- unscoped deployment truth or Prometheus manifest changes outside an explicit
  deploy/observability packet
- Hestia staff controls
- Themis public surface
- UI-owned formulas for ratings, analytics, matchmaking, tournaments, safety, or game identity

These are not the same as Kill Criteria. Hard Non-Touches name surfaces that must not be widened in any packet. Kill Criteria name conditions that disqualify a tracer from shipping at all.

## Kill Criteria

Kill or defer a tracer if any of these are true:

- It requires public result trust before disputes exist.
- It exposes social/leaderboard/rivalry surfaces before reporting and moderation exist.
- It depends on ATHENA physical truth but only mock data is available.
- It needs Hestia/Themis API routes that are stubbed or undefined.
- It requires cross-repo contract changes without compatibility matrix updates.
- It requires public scale but rating recompute duration/row ceiling is unknown.
- It adds a second domain model instead of sharing service/application command behavior.
- It makes retention rewards from untrusted or correctable data.
- It exposes trusted-surface credentials through browser-delivered frontend code.
- It turns notifications into chat, DMs, guild messaging, or arbitrary user-to-user content.
- It implements recurring schedules or court/resource splitting ad hoc inside tournament runtime.

## Recommended Next Packet Stack

After 3B.20.1 cohesion hardening, Milestone 3.0, Scale Gate numeric ceilings,
CLI Demo Spine, Rating Policy Wrapper, and Rating Policy Simulation / Golden
Expansion, the following packets are closed for their bounded scope:

| Closed packet | Current status |
| --- | --- |
| Deploy-level smoke proof | Closed by Milestone 3.0 for APOLLO/ATHENA trust-spine proof; Hestia/Themis/gateway remain repo-only in the inspected environment. |
| Telemetry counter export | Closed by Milestone 3.0 baseline `/metrics` export and Prometheus scrape proof; alert thresholds still need tuning. |
| Cross-repo compatibility matrix | Closed by Milestone 3.0 compatibility matrix; keep it updated when any repo contract or deploy truth changes. |
| Scale Gate Numeric Ceilings | Closed locally in repo/runtime; not full production load validation. |
| CLI Demo Spine | Closed locally in repo/runtime; APOLLO CLI/API demo path is agent-operable without frontend dependency. |
| Rating Policy Wrapper | Closed locally in repo/runtime; active legacy-engine projection is policy-wrapped with calibration/decay/cap metadata; OpenSkill remains comparison-only; not deployed truth. |
| Rating Policy Simulation / Golden Expansion | Closed locally in repo/runtime; deterministic fixtures, CLI JSON output, accepted/rejected scenarios, cutover blockers, and policy risks are documented; OpenSkill remains comparison-only; not deployed truth. |

Current next launch-expansion packets in priority order:

| Priority | Packet | Why now |
| --- | --- | --- |
| 1 | Game identity policy hardening | First-version CP/badge/rivalry/squad policies need usage-driven tuning. Define the tuning loop before population data accumulates. |
| 2 | ATHENA real ingress (cross-repo) | Sediment teams, daily check-in XP, reliability presence verification all depend on stronger physical-truth ingress than current bounded proof. |
| 3 | Live destructive-probe and SIGTERM proof plan | Milestone 3.0 intentionally avoided production-destructive mutation probes and in-flight production SIGTERM proof. Plan these before higher-stakes live operation. |

What to avoid as a next packet:

- Public tournaments (Scale Gate and public tournament privacy/product policy
  still need work first).
- OpenSkill read-path switch (policy wrapper and deterministic simulation are
  closed locally, but production backtesting, rollback, and cutover evidence
  remain deferred).
- Honor / MVP / quests / drafts / onboarding (retention layer needs scale policy
  and abuse controls, not only repo/deploy proof).
- Cross-sport transfer (nice-to-have; doesn't unblock anything pressing).
- New retention modules independent of game identity (consolidation under game identity is working; don't fragment it).
- Messaging / chat / broad public social graph (Hard Non-Touch).

These priorities and avoidances are forward-looking guidance. The Immediate Action List below records what is already closed and what remains.

## Immediate Action List

1. Closed by 3B.12 and re-verified by 3B.12.1: APOLLO canonical lifecycle/result trust, correction
   supersession, and finalized/corrected-only rating consumption boundary;
   Themis renders APOLLO-backed result states without owning result truth.
2. Closed by 3B.13: APOLLO legacy rating extraction, policy/audit metadata,
   golden cases, rating events, source result binding, and deterministic
   projection watermarks.
3. Closed by 3B.14: OpenSkill dual-run comparison beside the legacy baseline,
   with read-path cutover still deferred until comparison evidence is accepted.
4. Closed by 3B.15: ARES v2 proposal/match-preview foundation over trusted
   APOLLO queue intent and rating facts.
5. Closed by 3B.16: internal derived competition analytics over trusted
   result/rating facts only.
6. Closed by 3B.17: internal staff tournament runtime over trusted APOLLO
   team/match/result truth only.
7. Closed by 3B.18: internal manager-first report, block, reliability, and
   safety audit facts over APOLLO competition truth only.
8. Closed by 3B.19 and hardened by 3B.19.1: public-safe competition readiness
   and leaderboard projections over allowlisted, redacted APOLLO truth.
9. Closed by 3B.20 and hardened by 3B.20.1: public/member-safe game identity
   projections for CP, badge awards, rivalry state, and squad identity.
10. Closed by Milestone 3.0: bounded APOLLO/ATHENA deploy smoke, APOLLO metrics
   export, Prometheus scrape proof, and compatibility matrix truth.
11. Closed by Scale Gate: repo/local numeric ceilings for rating recompute,
   public readiness, public leaderboard, game identity, and CLI/API smoke.
12. Closed by CLI Demo Spine: service-backed local CLI/API demo route over
   existing APOLLO competition truth without frontend dependency.
13. Closed by Rating Policy Wrapper: APOLLO active legacy-engine projection
   records `apollo_rating_policy_wrapper_v1`, calibration/provisional/ranked
   status, inactivity sigma-inflation metadata, and climbing-cap metadata while
   OpenSkill remains internal comparison-only.
14. Closed by Rating Policy Simulation / Golden Expansion: APOLLO has
    deterministic local simulation proof, CLI JSON output, accepted/rejected
    scenario classification, legacy baseline deltas, OpenSkill sidecar deltas,
    cutover blockers, and policy risks while OpenSkill remains comparison-only.
15. Closed by Frontend Route/API Contract Matrix: Hestia/Themis route/API
    contract truth is enumerated as docs truth only, including proxy boundaries,
    public proxy denials, auth/role/state behavior, stub/mock status, and test
    coverage. Runtime and deployed truth are unchanged.
16. Next: Game Identity Policy Tuning Loop before broader identity or public
    surface widening.

## Proof Commands

Minimum APOLLO proof set before gate closeout:

```sh
go test ./...
go test -count=5 ./internal/...
go test -race ./internal/...
go vet ./...
go build ./cmd/apollo
git diff --check
```

Focused proofs for rating/competition changes:

```sh
go test -count=1 ./internal/competition ./internal/ares ./internal/authz
go test -count=1 ./internal/server -run 'Competition|MatchPreview'
```

Run Hestia/Themis test suites only when their API contracts, mocks, routes, or payload expectations change. Do not add frontend test requirements to backend-only tracers unless Gate 9 is in scope.

## Final Ruling

The expansion ideas are mostly valid. The danger is not vision; it is sequencing.

The durable architecture is:

- Competition owns result truth.
- Rating owns rating policy and math.
- ARES owns matchmaking proposals.
- Tournament owns tournament containers and snapshots.
- Social owns moderation and safety.
- Achievements owns XP, badges, quests, records, and display criteria.
- Frontends consume real contracts only.
- Cross-repo compatibility is explicit.

Hybrid OpenSkill plus legacy Elo is the right migration path if "hybrid" means OpenSkill as the durable kernel, APOLLO policy as the product layer, and legacy Elo as characterization/fallback during migration. It is not durable if it means two competing scoring systems forever.

## 3B.11 Closeout Addendum

Date: 2026-04-27

Phase 3B.11 `Competition Command + Ops Foundation` is closed in repo/runtime
truth for APOLLO and Themis, with platform docs synced. It shipped only the
agentic and staff-operable foundation:

- APOLLO shared competition command DTO.
- APOLLO shared command outcome DTO.
- APOLLO competition command readiness/capability truth.
- APOLLO dry-run plan shape over supported existing competition commands.
- APOLLO service-backed CLI parity through `apollo competition command`.
- APOLLO internal HTTP command readiness and command execution routes.
- Themis internal `/ops/competition` shell consuming APOLLO readiness, session,
  command, and outcome contracts.
- Themis disabled, denied, rejected, planned, accepted, and unavailable states
  over APOLLO-backed payloads only.
- Platform, APOLLO, and Themis docs synchronized to this shipped scope.

Phase 3B.11.1 `Cohesion Hardening` is also closed. It patched two cohesion
findings:

- Themis now preserves structured APOLLO command outcomes on non-2xx command
  responses, so denied and rejected outcomes render from APOLLO truth instead
  of a generic client error.
- This audit no longer points to pre-3B.11 docs/CLI planning as the immediate
  next tracer.

Verification notes:

- APOLLO focused 3B.11 competition tests passed.
- APOLLO `go build ./cmd/apollo` passed.
- APOLLO `go test -count=1 ./...` passed during 3B.11.1.
- The named APOLLO presence test
  `TestPresenceRuntimeExposesFacilityScopedSummaryAndStaysDeterministic`
  passed during 3B.11.1; the earlier `ashtonbee streak current_count = 1, want
  2` failure was not reproduced and is not treated as a 3B.11 regression.
- Themis `npm run check`, `npm test`, `npm run build`, and focused
  `competition|ops shell` Playwright tests passed.
- Hestia stayed untouched.
- Deployed truth stayed unchanged.

Still deferred:

- Result lifecycle/trust: Phase 3B.12.
- Rating extraction: Phase 3B.13.
- OpenSkill dual-run is closed in Phase 3B.14; read-path switch remains deferred.
- ARES v2 proposal foundation is closed in Phase 3B.15; further ARES widening
  requires a separate packet.
- Competition analytics foundation is closed in Phase 3B.16; carry
  coefficient and broader public/profile/scouting analytics remain deferred.
- Tournament runtime: Phase 3B.17.
- Social safety: Phase 3B.18.
- Public competition surfaces: Phase 3B.19.
- CP, badges, rivalry, and squads: Phase 3B.20.
- Proposal workflow, recurring schedule, court splitting, booking/commercial
  work, browser trusted-surface token, and public/Hestia competition expansion
  remain out of scope until separately reopened.

## 3B.12 Closeout Addendum

Date: 2026-04-27

Phase 3B.12 `Competition Lifecycle + Result Trust` is closed in repo/runtime
truth for APOLLO and Themis, with platform docs synced. It shipped only the
trusted match/result spine:

- APOLLO canonical `match_status`, `result_status`, `canonical_result_id`, and
  `result_version` truth on competition match projections.
- APOLLO result facts for recorded, finalized, disputed, corrected, and voided
  states.
- APOLLO lifecycle events for `competition.match.started`,
  `competition.result.recorded`, `competition.result.finalized`,
  `competition.result.disputed`, `competition.result.corrected`, and
  `competition.result.voided`.
- APOLLO correction linkage through `correction_id` and
  `supersedes_result_id`, plus `finalized_at` and `corrected_at`.
- APOLLO command and direct HTTP integration for result transitions, with
  result-version guards and dry-run support through the command path.
- APOLLO rating consumption guards: only finalized or corrected canonical
  results feed current rating/stat/history paths.
- Themis internal `/ops/competition` rendering of APOLLO-backed match/result
  state, dry-runs, denied/rejected outcomes, disabled states, and correction/
  supersession fields without local result truth or fake data.

Verification notes:

- APOLLO `git diff --check`, `go build ./cmd/apollo`, focused
  lifecycle/result/rating-boundary tests, `go test -count=1 ./cmd/apollo`, and
  `go test -count=1 ./...` passed.
- Themis `git diff --check`, `npm run check`, `npm test`, `npm run build`, and
  focused `competition|result` Playwright tests passed.
- Hestia stayed untouched.
- Deployed truth stayed unchanged.

Still deferred:

- OpenSkill dual-run is closed in Phase 3B.14; read-path switch remains deferred.
- ARES v2 proposal foundation is closed in Phase 3B.15; further ARES widening
  requires a separate packet.
- Competition analytics foundation is closed in Phase 3B.16; carry
  coefficient and broader public/profile/scouting analytics remain deferred.
- Tournament runtime: Phase 3B.17.
- Social safety: Phase 3B.18.
- Public competition surfaces: Phase 3B.19.
- CP, badges, rivalry, and squads: Phase 3B.20.
- Proposal workflow, recurring schedule, court splitting, booking/commercial
  work, browser trusted-surface token, and public/Hestia competition expansion
  remain out of scope until separately reopened.

Later closed by 3B.20. Current post-3B.20 launch-expansion work should start
with 3B.20.1 cohesion hardening unless another narrow production issue is found.

## 3B.12.1 Cohesion Hardening Addendum

Date: 2026-04-27

Phase 3B.12.1 `Cohesion Hardening` is closed with no runtime, UI, or truth-drift
findings. The pass re-verified the 3B.12 lifecycle/result trust spine and did
not require a patch worker.

Confirmed rulings:

- APOLLO rating/stat/history paths consume only finalized or corrected
  canonical results via `canonical_result_id`.
- Recorded, disputed, voided, superseded, and non-canonical results remain
  excluded from rating consumption.
- APOLLO lifecycle/result commands, direct result endpoints, result-version
  guards, correction linkage, and lifecycle events remain coherent.
- Themis `/ops/competition` consumes APOLLO lifecycle/result contracts only and
  does not own or infer canonical result truth.
- Platform, APOLLO, and Themis docs preserve the 3B.12 shipped/deferred line.
- Hestia stayed untouched.
- Deployed truth stayed unchanged.

Verification notes:

- APOLLO `git diff --check`, focused lifecycle/result/rating/CLI tests,
  `go build ./cmd/apollo`, and `go test -count=1 ./...` passed.
- Themis `git diff --check`, `npm run check`, `npm test`, `npm run build`, and
  focused `/ops/competition` Playwright tests passed.
- Platform `git diff --check` passed; no docs checker is defined.
- Hestia `git status -sb` stayed clean.

Still deferred:

- OpenSkill dual-run is closed in Phase 3B.14; read-path switch remains deferred.
- ARES v2 proposal foundation is closed in Phase 3B.15; further ARES widening
  requires a separate packet.
- Competition analytics foundation is closed in Phase 3B.16; carry
  coefficient and broader public/profile/scouting analytics remain deferred.
- Tournament runtime: Phase 3B.17.
- Social safety: Phase 3B.18.
- Public competition surfaces: Phase 3B.19.
- CP, badges, rivalry, and squads: Phase 3B.20.
- Proposal workflow, recurring schedule, court splitting, booking/commercial
  work, browser trusted-surface token, and public/Hestia competition expansion
  remain out of scope until separately reopened.

Later closed by 3B.20. Current post-3B.20 launch-expansion work should start
with 3B.20.1 cohesion hardening unless another narrow production issue is found.

## 3B.13 Rating Foundation Addendum

Date: 2026-04-27

Phase 3B.13 `Rating Foundation` is closed in APOLLO repo/runtime truth, with no
Themis or Hestia runtime changes. It shipped only the legacy rating foundation:

- APOLLO current rating math is extracted into `internal/rating` as the
  versioned `legacy_elo_like` baseline.
- APOLLO records explicit `rating_engine`, `engine_version`, and
  `policy_version` values on active member rating projections.
- APOLLO records auditable `competition.rating.legacy_computed`,
  `competition.rating.policy_selected`, and
  `competition.rating.projection_rebuilt` events.
- APOLLO rating projections store `source_result_id`, `rating_event_id`, and
  deterministic `projection_watermark` values.
- Golden legacy rating cases preserve current `mu`, `sigma`, `delta_mu`, and
  `delta_sigma` behavior.
- Rating projections continue to consume only finalized or corrected canonical
  results; recorded, disputed, voided, superseded, and non-canonical results
  stay out of the active rating projection.

OpenSkill attaches in 3B.14 beside this legacy baseline as a dual-run
comparison. It must not replace the read path, remove legacy events, or hide
engine/policy versions until a later packet explicitly proves the cutover.

Still deferred:

- OpenSkill dual-run is closed in Phase 3B.14; read-path switch remains deferred.
- ARES v2 proposal foundation is closed in Phase 3B.15; further ARES widening
  requires a separate packet.
- Competition analytics foundation is closed in Phase 3B.16; carry
  coefficient and broader public/profile/scouting analytics remain deferred.
- Tournament runtime: Phase 3B.17.
- Social safety: Phase 3B.18.
- Public competition surfaces: Phase 3B.19.
- CP, badges, rivalry, and squads: Phase 3B.20.
- Project-wide SemVer governance, proposal workflow, recurring schedule, court
  splitting, booking/commercial work, browser trusted-surface token, and
  public/Hestia competition expansion remain out of scope until separately
  reopened.

## 3B.14 OpenSkill Dual-Run Addendum

Date: 2026-04-28

Phase 3B.14 `OpenSkill Dual-Run` is closed in APOLLO repo/runtime truth, with
no Themis or Hestia runtime changes. It shipped only internal comparison truth:

- APOLLO computes OpenSkill values beside legacy rating outputs from the same
  finalized/corrected canonical result order.
- APOLLO records `legacy_mu`, `legacy_sigma`, `openskill_mu`,
  `openskill_sigma`, `delta_from_legacy`, `accepted_delta_budget`, and
  `comparison_scenario` on internal comparison audit facts/events.
- APOLLO records `competition.rating.openskill_computed` events and records
  `competition.rating.delta_flagged` when comparison deltas exceed the accepted
  budget.
- APOLLO keeps `apollo.competition_member_ratings` as the legacy active rating
  projection and does not expose OpenSkill values through public/member rating
  reads.
- Rating projections and comparison facts continue to consume only finalized or
  corrected canonical results; recorded, disputed, voided, superseded, and
  non-canonical results stay out of the active rating path.

Still deferred:

- OpenSkill read-path switch remains deferred.
- ARES v2 proposal foundation is closed in Phase 3B.15; further ARES widening
  requires a separate packet.
- Competition analytics foundation is closed in Phase 3B.16; carry
  coefficient and broader public/profile/scouting analytics remain deferred.
- Tournament runtime: Phase 3B.17.
- Social safety: Phase 3B.18.
- Public competition surfaces: Phase 3B.19.
- CP, badges, rivalry, and squads: Phase 3B.20.
- Project-wide SemVer governance, proposal workflow, recurring schedule, court
  splitting, booking/commercial work, browser trusted-surface token, and
  public/Hestia competition expansion remain out of scope until separately
  reopened.

## 3B.14.1 Cohesion Hardening Addendum

Date: 2026-04-28

Phase 3B.14.1 `Cohesion Hardening` is closed in APOLLO repo/runtime truth and
platform docs truth. It did not add product scope, migrations, APIs, public
surfaces, Themis runtime changes, or Hestia runtime changes.

Confirmed and fixed:

- APOLLO OpenSkill `delta_flagged` now uses the same 4-decimal persisted
  numeric boundary as `delta_from_legacy` and `accepted_delta_budget`, matching
  the Postgres audit constraint and avoiding near-budget rebuild rejection.
- APOLLO added focused comparison-boundary coverage for the persisted
  delta-budget scale.
- APOLLO docs no longer contain stale pre-3B.14 statements that OpenSkill is
  not implemented or that OpenSkill dual-run is the next step.
- APOLLO and platform docs now distinguish current bounded pre-`1.0.0` tag
  discipline from deferred project-wide SemVer governance.

Confirmed unchanged:

- OpenSkill remains internal dual-run comparison only.
- `apollo.competition_member_ratings` remains the legacy active read path.
- Ratings and comparison facts still consume only finalized or corrected
  canonical results.
- OpenSkill comparison facts remain deterministic and auditable.
- Hestia's existing proxy-boundary risk does not expose 3B.14 OpenSkill
  comparison truth because APOLLO exposes no OpenSkill comparison API route.
- Deployed truth remains unchanged.

Still deferred:

- OpenSkill read-path switch remains deferred.
- ARES v2 proposal foundation is closed in Phase 3B.15; further ARES widening
  requires a separate packet.
- Competition analytics foundation is closed in Phase 3B.16; carry
  coefficient and broader public/profile/scouting analytics remain deferred.
- Tournament runtime: Phase 3B.17.
- Social safety: Phase 3B.18.
- Public competition surfaces: Phase 3B.19.
- CP, badges, rivalry, and squads: Phase 3B.20.
- Project-wide SemVer governance, proposal workflow, recurring schedule, court
  splitting, booking/commercial work, browser trusted-surface token, and
  public/Hestia competition expansion remain out of scope until separately
  reopened.

## 3B.15 ARES v2 Addendum

Date: 2026-04-28

Phase 3B.15 `ARES v2` is closed in APOLLO repo/runtime truth, with no Themis
or Hestia runtime changes and deployed truth unchanged. It shipped only the
internal proposal/match-preview foundation:

- APOLLO stores explicit competition queue intent facts with
  `queue_intent_id`, `facility_key`, `sport_key`, `mode_key`, and `tier`.
- APOLLO records `competition.queue_intent.updated` facts for queue intent
  create/update/withdrawal changes.
- APOLLO generates deterministic internal match-preview proposals from trusted
  APOLLO queue, rating, result, session, and team projections.
- APOLLO computes `match_quality`, `predicted_win_probability`, and explicit
  `explanation_code` / `explanation_codes` server-side.
- APOLLO records `competition.match_preview.generated` facts and preview member
  projections for internal inspection.
- The command foundation can update queue intent tier facts and generate match
  previews through APOLLO command contracts.

Confirmed unchanged:

- ARES remains a proposal engine only.
- ARES does not own match lifecycle, canonical result truth, result
  finalization/correction/voiding, or active rating truth.
- `apollo.competition_member_ratings` remains the legacy active rating read
  path.
- OpenSkill remains internal dual-run comparison only.
- Themis and Hestia do not compute match quality, predicted win probability,
  explanation codes, or rating truth.
- Hestia has no public/member matchmaking expansion from this packet.

Still deferred:

- OpenSkill read-path switch remains deferred.
- Competition analytics foundation is closed in Phase 3B.16; carry
  coefficient and broader public/profile/scouting analytics remain deferred.
- Tournament runtime: Phase 3B.17.
- Social safety: Phase 3B.18.
- Public competition surfaces: Phase 3B.19.
- CP, badges, rivalry, and squads: Phase 3B.20.
- Project-wide SemVer governance, proposal workflow, recurring schedule, court
  splitting, booking/commercial work, browser trusted-surface token, and
  public/Hestia competition expansion remain out of scope until separately
  reopened.

## 3B.16 Competition Analytics Addendum

Date: 2026-04-28

Phase 3B.16 `Competition Analytics Foundation` is closed in APOLLO
repo/runtime truth, with no Themis or Hestia runtime changes and deployed truth
unchanged. It shipped only internal derived analytics truth:

- APOLLO stores internal competition stat events in
  `apollo.competition_analytics_events`.
- APOLLO stores current internal analytics projections in
  `apollo.competition_analytics_projections`.
- Analytics facts expose `stat_type`, `stat_value`, `source_match_id`,
  `source_result_id`, `sample_size`, `confidence`, `computed_at`,
  `projection_version`, and deterministic projection watermarks where
  applicable.
- APOLLO derives matches played, wins/losses/draws, current streaks, legacy
  rating movement, opponent strength, facility/sport/mode splits, and
  team-vs-solo delta only from existing participant-count facts.
- Analytics rebuilds consume finalized/corrected canonical result truth plus
  legacy active rating facts only; recorded, disputed, voided, superseded, and
  non-canonical results are excluded.
- Analytics rebuilds do not mutate result, rating, ARES, lifecycle, booking,
  UI, member, or public truth.

Confirmed unchanged:

- `apollo.competition_member_ratings` remains the legacy active rating read
  path.
- OpenSkill remains internal dual-run comparison only.
- ARES remains internal proposal/match-preview truth only.
- Themis and Hestia do not compute analytics, confidence, sample size, rating
  movement, opponent strength, or team/solo deltas.
- Hestia has no public/member analytics, profile, scouting, leaderboard, CP,
  badge, squad, rivalry, tournament, or public rank expansion from this
  packet.

Still deferred:

- Dashboard-first analytics work remains deferred.
- Public profiles/stats/scouting remain deferred until public readiness gates.
- Carry coefficient remains deferred.
- Tournament runtime remains deferred to Phase 3B.17.
- Social safety remains deferred to Phase 3B.18.
- Public competition surfaces remain deferred to Phase 3B.19.
- CP, badges, rivalry, and squads were later closed in Phase 3B.20.
- OpenSkill read-path switch remains deferred.
- Project-wide SemVer governance, proposal workflow, recurring schedule, court
  splitting, booking/commercial work, browser trusted-surface token, and
  public/Hestia competition expansion remain out of scope until separately
  reopened.

## 3B.17 Internal Tournament Runtime Addendum

Date: 2026-04-28

Phase 3B.17 `Internal Tournament Runtime` is closed in APOLLO repo/runtime
truth, with no Themis, Hestia, Prometheus, or deployed-truth changes. It shipped
only staff/internal tournament facts:

- APOLLO stores internal tournament containers in
  `apollo.competition_tournaments`; visibility is constrained to `internal`.
- APOLLO stores single-elimination bracket and seed facts in
  `apollo.competition_tournament_brackets` and
  `apollo.competition_tournament_seeds`.
- APOLLO stores immutable locked team snapshots and snapshot members in
  `apollo.competition_tournament_team_snapshots` and
  `apollo.competition_tournament_team_snapshot_members`.
- APOLLO stores match bindings in
  `apollo.competition_tournament_match_bindings`; bindings reference APOLLO
  competition matches instead of replacing match truth.
- APOLLO stores explicit round advancement facts in
  `apollo.competition_tournament_advancements` with
  `advance_reason = canonical_result_win`.
- APOLLO records `competition.tournament.created`,
  `competition.tournament.seeded`, `competition.tournament.team_locked`,
  `competition.tournament.match_bound`, and
  `competition.tournament.round_advanced` event facts.
- APOLLO exposes staff-gated tournament HTTP contracts and CLI reads through
  existing competition capability/trusted-surface boundaries.

Confirmed boundaries:

- Tournament advancement consumes finalized/corrected canonical result truth
  only; recorded, disputed, voided, superseded, and non-canonical result states
  are rejected.
- Tournaments do not mutate or replace canonical result, lifecycle, rating,
  analytics, ARES, booking, public, member, UI, or game identity truth.
- Team snapshots, seeds, match bindings, advancement facts, and tournament
  events are append-only/immutable once recorded.
- The first supported format is intentionally narrow:
  `single_elimination`.
- Themis may consume APOLLO tournament contracts later, but no Themis runtime
  shell was required for this packet.
- Hestia has no public/member tournament, bracket, leaderboard, scouting,
  analytics, CP, badge, squad, rivalry, or public rank expansion from this
  packet.

Still deferred:

- Public competition readiness closed in Phase 3B.19; public tournaments remain deferred.
- CP, badges, rivalry, and squads were later closed in Phase 3B.20.
- Messaging/chat, public/member safety UI, public profiles, scouting, and
  leaderboards remain deferred.
- OpenSkill read-path switch remains deferred.
- Dashboard-first analytics, public profiles/stats/scouting, and carry
  coefficient remain deferred.
- Project-wide SemVer governance, proposal workflow, recurring schedule, court
  splitting, booking/commercial work, browser trusted-surface token, and
  public/Hestia competition expansion remain out of scope until separately
  reopened.

## 3B.18 Social Safety + Reliability Addendum

Date: 2026-04-28

Phase 3B.18 `Social Safety + Reliability` is closed in APOLLO repo/runtime
truth, with Themis consuming manager-only APOLLO safety/reliability read
contracts and with no Hestia, Prometheus, or deployed-truth changes. It shipped
only internal social safety and reliability facts:

- APOLLO stores competition-scoped report facts in
  `apollo.competition_safety_reports`.
- APOLLO stores competition-scoped active block facts in
  `apollo.competition_safety_blocks`.
- APOLLO stores operational reliability events in
  `apollo.competition_reliability_events`.
- APOLLO stores auditable safety/reliability event facts in
  `apollo.competition_safety_events`.
- APOLLO exposes manager-only safety/reliability readiness and review reads
  through capability-gated, trusted-surface-gated HTTP contracts.
- APOLLO accepts aligned safety/reliability commands through the existing
  command/outcome foundation without changing public competition contracts.
- Themis renders manager/internal safety and reliability visibility from APOLLO
  contracts only, with unavailable/denied states when APOLLO does not provide
  the manager contract.

Confirmed boundaries:

- Reports and blocks are private safety facts, not public truth.
- Reliability events are internal operational facts.
- Safety/reliability facts are immutable and auditable once recorded.
- Manager-only access requires `competition_safety_review` and trusted-surface
  proof.
- Private reporter, subject, actor, and report details are excluded from public
  and member-safe projections.
- Safety/reliability facts do not mutate canonical result, lifecycle, rating,
  analytics, ARES, tournament, booking, public, member, UI, or game identity
  truth.
- Hestia has no public/member safety, report, block, social, public profile,
  leaderboard, tournament, CP, badge, squad, rivalry, messaging, or chat
  expansion from this packet.

Still deferred:

- Public competition readiness closed in Phase 3B.19.
- Public tournaments remain deferred.
- Public/social/member safety surfaces, messaging/chat, public profiles,
  scouting, and leaderboards remain deferred.
- CP, badges, rivalry, and squads were later closed in Phase 3B.20.
- OpenSkill read-path switch remains deferred.
- ARES behavior changes remain deferred.
- Dashboard-first analytics and carry coefficient remain deferred.
- Project-wide SemVer governance, proposal workflow, recurring schedule, court
  splitting, booking/commercial work, browser trusted-surface token, and
  public/Hestia competition expansion remain out of scope until separately
  reopened.

3B.18.1 Cohesion Hardening followed this packet to verify and patch
safety/reliability cohesion before 3B.19 public competition readiness.

## 3B.18.1 Cohesion Hardening Addendum

Date: 2026-04-29

Phase 3B.18.1 `Cohesion Hardening` is closed in APOLLO repo/runtime truth,
with APOLLO, Themis, and Hestia cohesion fixes pushed and with Prometheus,
platform docs, and deployed truth unchanged. It patched only real 3B.18
safety/reliability boundary bugs:

- APOLLO filters safety/reliability command metadata out of the general
  competition readiness contract unless the actor has
  `competition_safety_review`.
- APOLLO validates competition-scoped safety/reliability actors before
  recording report, block, or reliability facts.
- APOLLO rejects out-of-scope reporter, subject, blocker, blocked, and
  reliability subject users without persisting private safety facts.
- APOLLO safety/reliability storage now prevents deletion as well as mutation
  of recorded report, block, reliability, and audit facts.
- Themis preserves APOLLO safety/reliability denial reasons in manager-only
  ops states instead of replacing them with generic competition-copy failures.
- Themis renders unavailable or denied safety/reliability fallback states
  without fake pending report, block, or reliability facts.
- Hestia's same-origin APOLLO proxy now allowlists member-safe routes and
  blocks private competition safety/reliability and session routes.
- Hestia strips trusted-surface headers before forwarding same-origin APOLLO
  proxy requests.

Confirmed boundaries:

- Reports and blocks remain private safety facts, not public truth.
- Reliability events remain internal operational facts.
- Safety/reliability audit facts remain immutable and auditable.
- Manager-only access remains gated by APOLLO capability and trusted-surface
  checks.
- Private report, block, reliability, actor, reporter, and subject details do
  not leak to public or member-safe projections.
- Safety/reliability facts do not mutate canonical result, lifecycle, rating,
  analytics, ARES, tournament, booking, public, member, UI, or game identity
  truth.
- Themis consumes APOLLO safety/reliability contracts only and does not infer
  hidden safety policy client-side.
- Hestia exposes no public/member safety, report, block, social, public
  profile, leaderboard, tournament, CP, badge, squad, rivalry, messaging, or
  chat surface from this hardening pass.
- Prometheus/GitOps and live deployment state were inspected but not changed.

Verification completed:

- APOLLO passed `git diff --check`, `sqlc generate -f db/sqlc.yaml`, focused
  safety/reliability, command readiness, audit, migration, authz/privacy, and
  non-mutation tests, `go test -count=1 ./internal/competition
  ./internal/server ./db/migrations ./cmd/apollo`, `go vet ./...`,
  `go build ./cmd/apollo`, and `go test -count=1 ./...`.
- Themis passed `git diff --check`, `npm run check`, `npm test`,
  `npm run build`, and focused desktop/mobile Playwright safety/reliability
  denial and review tests.
- Hestia passed `git diff --check`, `npm run check`, `npm test`,
  `npm run build`, and focused desktop/mobile Playwright proxy-boundary tests.
- Platform docs were checked for 3B.18 shipped truth and 3B.19/3B.20 deferred
  truth; no platform docs patch was required.
- Prometheus/GitOps status was clean and no deployment commit was made.

Still deferred:

- Public competition readiness closed in Phase 3B.19.
- Public tournaments remain deferred.
- Public/social/member safety surfaces, messaging/chat, public profiles,
  scouting, and leaderboards remain deferred.
- CP, badges, rivalry, and squads were later closed in Phase 3B.20.
- OpenSkill read-path switch remains deferred.
- ARES behavior changes remain deferred.
- Dashboard-first analytics and carry coefficient remain deferred.
- Project-wide SemVer governance, proposal workflow, recurring schedule, court
  splitting, booking/commercial work, browser trusted-surface token, and
  public/Hestia competition expansion remain out of scope until separately
  reopened.

## 3B.19 Public Competition Readiness Addendum

Date: 2026-04-29

Phase 3B.19 `Public Competition Readiness` is closed in APOLLO repo/runtime
truth, with Hestia consuming the APOLLO public-safe contract through a
server-mediated public page and with deployed truth unchanged. It shipped only
the public-safe competition readiness layer:

- APOLLO exposes `GET /api/v1/public/competition/readiness`.
- APOLLO exposes `GET /api/v1/public/competition/leaderboards`.
- Public competition contracts consume APOLLO projections only.
- Public leaderboard rows are derived from finalized/corrected canonical result
  truth plus legacy active rating fields.
- Public leaderboard participant labels are redacted and do not expose user IDs
  or public profile identity.
- Public leaderboard contracts do not expose internal analytics metadata such
  as sample size, confidence, source result IDs, or projection watermarks.
- Hestia renders `/competition` from the APOLLO public-safe contracts only.
- Hestia keeps browser-visible `/api/v1/public/*` proxy access denied and
  continues stripping trusted-surface request headers.
- Leak tests cover private safety/reliability, OpenSkill comparison,
  source-result, projection watermark, sample size, confidence, command, ARES
  proposal, and internal competition truth.

Confirmed boundaries:

- Private report, block, reliability, actor, reporter, subject, manager,
  command, trusted-surface, and ops facts remain non-public.
- OpenSkill comparison facts remain internal dual-run truth; the active public
  rating source remains the legacy active projection.
- ARES proposal facts, match quality, predicted win probability, and
  explanation codes remain non-public.
- Internal tournament runtime and public tournament presentation remain
  deferred.
- Public projections do not create game identity truth.
- Themis remains an internal privileged ops shell and is not public source
  truth.

Still deferred:

- Public tournaments remain deferred.
- CP, badges, rivalry, and squads were later closed in Phase 3B.20.
- Messaging/chat and public social graph remain out of scope.
- OpenSkill read-path switch remains deferred.
- Public/member safety UI remains deferred.
- Booking/commercial/proposal workflows remain parked.
- Project-wide SemVer governance remains deferred.
- Deployment remains unchanged until a separate deploy packet explicitly
  verifies and reports it.

That path was later closed by 3B.20 and hardened by 3B.20.1.

## 3B.19.1 Cohesion Hardening Addendum

Date: 2026-04-29

Phase 3B.19.1 `Cohesion Hardening` verified the just-shipped public
competition readiness line across APOLLO, Hestia, Themis, Prometheus, and
platform docs. One real 3B.19 cohesion issue was patched: public leaderboard
DTOs no longer expose analytics `sample_size` or `confidence` metadata. Those
values may remain internal query inputs or ordering aids, but they are not part
of the public contract.

Additional hardening:

- APOLLO now has a focused public-readiness test proving recorded-only,
  non-final canonical result truth does not count toward public readiness and
  does not appear in public leaderboards.
- APOLLO leak tests now reject public exposure of sample size, confidence,
  rating engine/policy internals, rating values, source-result identity,
  projection watermarks, OpenSkill comparison facts, safety/reliability facts,
  ARES proposal facts, command/readiness internals, and manager/ops truth.
- Hestia public competition parsing continues to use an allowlist and now drops
  sample size and confidence if APOLLO or a test double sends them.
- Hestia has focused desktop/mobile coverage for unavailable public competition
  reads rendering as unavailable instead of fake records.
- Themis remains untouched and internal-only.
- Prometheus remains untouched.
- Deployed truth remains unchanged.

Confirmed boundaries:

- Public contracts expose public-safe competition projections only.
- Public projection inputs are finalized/corrected canonical results plus
  legacy active rating fields only.
- OpenSkill comparison facts remain internal and are not exposed publicly.
- Safety/report/block/reliability facts remain private manager/internal truth.
- Manager/internal command, readiness, ARES proposal, tournament ops, and
  operational truth remain non-public.
- Hestia `/competition` consumes APOLLO public-safe contracts server-side.
- Hestia browser proxy denies `/api/v1/public/*` passthrough, blocks internal
  competition routes, and strips trusted-surface headers.

Still deferred:

- Public tournaments remain deferred.
- CP, badges, rivalry, and squads were later closed in Phase 3B.20.
- Messaging/chat and public social graph remain out of scope.
- OpenSkill read-path switch remains deferred.
- Public/member safety UI remains deferred.
- Booking/commercial/proposal workflows remain parked.
- Project-wide SemVer governance remains deferred.

## 3B.20 Game Identity Layer Addendum

Date: 2026-04-29

Phase 3B.20 `Game Identity Layer` is closed in APOLLO/Hestia repo/runtime
truth, with deployed truth unchanged. It shipped only the first trusted game
identity projection layer:

- APOLLO exposes `GET /api/v1/public/competition/game-identity`.
- APOLLO exposes `GET /api/v1/competition/game-identity`.
- APOLLO derives CP, badge award facts, rivalry state facts, and squad
  identity facts from public-safe competition projection rows only.
- CP, badge, rivalry, and squad policies are explicit and versioned:
  `apollo_cp_v1`, `apollo_badge_awards_v1`, `apollo_rivalry_state_v1`, and
  `apollo_squad_identity_v1`.
- Public output uses redacted participant labels. Member output is self-scoped.
- Hestia renders APOLLO-provided game identity contracts only on `/competition`
  and the member competition surface.
- Hestia keeps browser-visible `/api/v1/public/*` proxy access denied and
  continues stripping trusted-surface request headers.
- Leak and boundary tests cover user IDs, source result IDs, canonical result
  IDs, OpenSkill comparison facts, safety/reliability facts, command/readiness
  internals, ARES proposal facts, tournament ops truth, analytics sample size,
  confidence, and projection watermarks.

Confirmed boundaries:

- Game identity is derived projection truth; no storage migration was required.
- Game identity does not mutate canonical result, rating, analytics, ARES,
  tournament, safety, public-readiness, booking, member, or UI truth.
- Public/member projections consume trusted APOLLO competition projections only.
- Hestia does not compute CP, badge, rivalry, or squad truth client-side.
- Themis remains internal and is not public game identity source truth.
- Prometheus remains untouched.

Still deferred:

- Messaging/chat remains out of scope.
- Broad public social graph remains deferred beyond the shipped redacted
  squad/rivalry identity facts.
- OpenSkill read-path switch remains deferred.
- Public OpenSkill comparison exposure remains deferred.
- Public/member safety detail exposure remains deferred.
- Public tournaments remain deferred.
- Booking/commercial/proposal workflows remain parked.
- Project-wide SemVer governance remains deferred.

## 3B.20.1 Cohesion Hardening Addendum

Date: 2026-04-29

Phase 3B.20.1 `Cohesion Hardening` verified the just-shipped game identity
layer across APOLLO, Hestia, Themis, Prometheus, and platform docs. Two real
3B.20 cohesion issues were patched:

- APOLLO rivalry states now compare participants only inside the same
  sport/mode/facility/team-scope projection context.
- APOLLO badge and rivalry labels now stay scoped to the exact projection row,
  so broad requests with multiple rows for the same user cannot overwrite CP
  row labels.

Additional hardening:

- APOLLO has focused coverage for cross-context rivalry prevention and
  duplicate-user row label stability.
- Hestia roadmap truth now matches the shipped APOLLO-provided game identity
  surface instead of treating CP/badges/rivalry/squads as still deferred.
- Platform docs now record 3B.20.1 as a hardening closeout, not the next open
  launch-expansion packet.

Confirmed boundaries:

- APOLLO owns CP, badge award, rivalry state, and squad identity projections.
- Identity policies remain explicit and versioned.
- Public/member-safe contracts exclude private/internal APOLLO truth.
- Game identity remains read-only and does not mutate result, rating,
  analytics, ARES, tournament, safety, public-readiness, booking, member, or UI
  truth.
- Hestia consumes APOLLO projections only and proxy boundaries remain intact.
- Themis and Prometheus remain untouched.
- Deployed truth remains unchanged.

Still deferred:

- Messaging/chat remains out of scope.
- Broad public social graph remains deferred beyond shipped redacted
  squad/rivalry identity facts.
- OpenSkill read-path switch remains deferred.
- Public OpenSkill comparison exposure remains deferred.
- Public/member safety detail exposure remains deferred.
- Public tournaments remain deferred.
- Booking/commercial/proposal workflows remain parked.
- Project-wide SemVer governance remains deferred.

## Rating Policy Wrapper Addendum

Date: 2026-05-03

Rating Policy Wrapper is closed in APOLLO repo/local runtime truth only.
Deployed truth remains unchanged.

Implemented truth:

- `rating.RebuildLegacy` remains the unchanged legacy characterization
  baseline.
- APOLLO active rating rebuild now uses `rating.RebuildActivePolicy` over the
  legacy engine, with active `policy_version` set to
  `apollo_rating_policy_wrapper_v1`.
- Active current rating rows persist calibration status, last inactivity decay
  time, inactivity decay count, and whether the climbing cap was applied.
- Rating events persist policy metadata for calibration status, inactivity
  decay application, and climbing cap application.
- The fifth rated match transitions a participant from provisional to ranked.
- Inactivity decay inflates sigma only after the configured threshold and is
  capped at the initial sigma.
- Positive mu movement is bounded by the explicit climbing cap.
- Member stats expose additive policy metadata through APOLLO service/CLI/API
  contracts; public readiness, public leaderboards, and game identity stay
  redacted and do not expose OpenSkill comparison facts.
- OpenSkill remains internal comparison-only, generated beside active truth and
  not used as the active read path.

Verification:

```sh
cd /Users/zizo/Personal-Projects/ASHTON/apollo
git diff --check
go test -count=1 ./internal/rating
go test -count=1 ./internal/competition ./internal/rating ./internal/server ./cmd/apollo
go test -count=1 ./db/migrations
go vet ./...
go build ./cmd/apollo
go test -count=1 ./...
go test -race ./internal/...
```

Confirmed boundaries:

- Hestia, Themis, Prometheus, gateway, ATHENA, and deploy/GitOps were untouched.
- Public tournaments remain blocked.
- Public OpenSkill values remain blocked.
- OpenSkill active read-path switch remains deferred.
- Deterministic simulation/golden expansion was still deferred at this close.
- Production historical backtesting remains deferred.

Rating Policy Simulation / Golden Expansion closed next. Frontend Route/API
Contract Matrix later closed as docs truth only.

## Rating Policy Simulation / Golden Expansion Addendum

Date: 2026-05-03

Rating Policy Simulation / Golden Expansion is closed in APOLLO repo/local
runtime truth only. Deployed truth remains unchanged.

Implemented truth:

- APOLLO rating has deterministic simulation fixtures under
  `rating.BuildActivePolicySimulationReport`.
- `rating.RebuildActivePolicy` remains the active policy path, with
  `rating.RebuildLegacy` preserved as the unchanged legacy baseline.
- OpenSkill comparison remains sidecar-only through
  `rating.RebuildOpenSkillComparison`; no active read path was switched.
- Local agents can run `apollo competition rating simulation --format json`
  for the proof report.
- The report includes active policy output, legacy baseline deltas, OpenSkill
  sidecar deltas, accepted/rejected scenario classification, cutover blockers,
  and policy risks.
- Public/member reads remain allowlisted and do not expose OpenSkill comparison
  facts.

Accepted scenarios:

- unranked 1v1, A wins
- stronger player beats new player
- new player beats stronger player
- repeated wins through fifth-match ranked transition
- inactivity return after threshold
- climbing-cap activation
- draw handling
- 5v5 even teams
- OpenSkill comparison delta rows as internal sidecar proof
- public/member read-safety sentinel

Rejected scenario:

- 3v5 asymmetric match is rejected as active-policy or cutover evidence. It is
  retained only as comparison stress for OpenSkill sidecar deltas.

Cutover blockers:

- OpenSkill active read path remains deferred.
- Rollback/cutover mechanics remain unbuilt.
- Production historical backtesting remains unproven.
- Public tournament readiness remains blocked.

Policy risk table:

| Risk | Classification | Current handling |
| --- | --- | --- |
| Synthetic fixtures may not represent production population dynamics | Moderate | Defer to production backtest and telemetry review. |
| Asymmetric team math remains a legacy-average limitation | High | Keep 3v5 as comparison-only stress until a future OpenSkill cutover packet. |
| OpenSkill delta budgets are not product rating claims | Moderate | Keep deltas internal and sidecar-only. |
| Public claim safety depends on allowlisted routes | Low | Keep public/member contracts redacted and route-matrix work next. |

Verification:

```sh
cd /Users/zizo/Personal-Projects/ASHTON/apollo
git diff --check
go test -count=1 ./internal/rating
go test -count=1 ./internal/competition ./internal/rating ./internal/server ./cmd/apollo
go vet ./...
go build ./cmd/apollo
go test -count=1 ./...
go test -race ./internal/...
```

Confirmed boundaries:

- Hestia, Themis, Prometheus, gateway, ATHENA, and deploy/GitOps were untouched.
- No schema changes were required.
- No public API, frontend route, or public rating claim was added.
- Public tournaments remain blocked.
- Public OpenSkill values remain blocked.
- OpenSkill active read-path switch remains deferred.
- Production historical backtesting remains deferred.

## Frontend Route/API Contract Matrix Addendum

Date: 2026-05-03

Frontend Route/API Contract Matrix is closed as docs truth only. Runtime truth
and deployed truth remain unchanged.

Implemented docs truth:

- Platform now owns the standalone matrix at
  `ashton-platform/planning/FRONTEND-ROUTE-API-CONTRACT-MATRIX.md`.
- The matrix enumerates current Hestia routes, current Themis routes, APOLLO
  endpoints consumed by each route, HTTP methods, server-mediated versus
  same-origin proxy calls, auth/session and role behavior, empty/loading/error/
  denied states, production-backed versus mock/stub status, current test
  coverage, APOLLO source-truth ownership, and blocked adjacent scope.
- Hestia remains the customer-facing shell and consumes APOLLO contracts only.
- Themis remains the privileged internal ops shell and consumes APOLLO
  contracts only.
- Both browser-visible proxies deny `/api/v1/public/*`; APOLLO still owns all
  proxied contract authority.

Documented drift risks:

- The matrix is docs-backed, not generated from OpenAPI, route code, or typed
  client contracts.
- Themis proxy allowlisting is path-prefix based for some internal surfaces.
- Hestia proxy/client allow some member-safe APOLLO contracts that current
  visible routes do not use.
- Existing frontend tests are mock-APOLLO repo tests, not deployed Hestia/
  Themis proof.

Confirmed boundaries:

- No APOLLO schema, API route, migration, handler, or runtime behavior changed.
- No Hestia or Themis runtime code changed.
- Deployed truth is unchanged.
- APOLLO remains source truth for competition, rating, public projection, game
  identity, booking, schedule, auth/session, safety, and reliability.
- Public tournaments remain blocked.
- OpenSkill active read path and public OpenSkill values remain blocked.
- Frontend-owned formulas remain blocked.
- Public/member safety detail surfaces remain blocked.
- Messaging/chat, broad public social graph, public profiles/scouting, public
  self-service booking expansion, booking/commercial/proposal workflows, and
  gateway/deploy work remain deferred.

Verification:

```sh
cd /Users/zizo/Personal-Projects/ASHTON/hestia
git diff --check
npm run check
npm test
npm run build

cd /Users/zizo/Personal-Projects/ASHTON/themis
git diff --check
npm run check
npm test
npm run build

cd /Users/zizo/Personal-Projects/ASHTON/apollo
git diff --check

cd /Users/zizo/Personal-Projects/ASHTON/ashton-platform
git diff --check
```

Next launch-expansion packet: Game Identity Policy Tuning Loop.
