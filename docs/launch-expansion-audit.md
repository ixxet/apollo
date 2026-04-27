# Launch Expansion Audit

Status: consolidated read-only audit artifact  
Scope: APOLLO-centered, with ATHENA, Hestia, Themis, gateway, and platform compatibility noted where they affect launch safety  
Date: 2026-04-26

## Executive Verdict

Do not launch the full expansion as one line. The system can support the vision, but only if the trust substrate lands before public excitement mechanics.

The current APOLLO codebase is strongest at bounded, authenticated, backend-first workflows: member auth, profile state, presence-derived member context, lobby intent, competition sessions, queueing, assignment, match result capture, staff attribution, schedule/booking controls, and public-safe booking intake. It is not yet ready for public ratings, public leaderboards, rivalry, badges, tournaments with public stakes, social reporting, or broad public competition surfaces.

The correct strategic pattern is:

1. Fix docs truth.
2. Add agent-operable CLI and capability/dry-run surfaces.
3. Extract and version rating behavior.
4. Add match tiers, consensus, disputes, and rating audit.
5. Swap the math kernel to OpenSkill under a custom policy wrapper.
6. Upgrade ARES and tournaments after the trust substrate exists.
7. Add social safety before public surfaces.
8. Add retention mechanics after public trust is durable.

The immediate next tracer should be docs truth plus CLI parity planning, not badges, tournaments, or OpenSkill hard swap.

## Evidence Anchors

Current APOLLO facts are grounded in these files:

| Area | Evidence |
| --- | --- |
| Roadmap discipline | [docs/roadmap.md](roadmap.md) |
| Current README state and inconsistency | [README.md](../README.md) |
| Competition service and DTOs | [internal/competition/service.go](../internal/competition/service.go) |
| Result capture and standings/stat derivation | [internal/competition/history.go](../internal/competition/history.go) |
| Current custom rating recompute | [internal/competition/history_repository.go](../internal/competition/history_repository.go) |
| Competition container schema | [db/migrations/009_competition_container_runtime.up.sql](../db/migrations/009_competition_container_runtime.up.sql) |
| Competition execution schema | [db/migrations/010_competition_execution_runtime.up.sql](../db/migrations/010_competition_execution_runtime.up.sql) |
| Result/rating schema | [db/migrations/011_competition_history_runtime.up.sql](../db/migrations/011_competition_history_runtime.up.sql) |
| Staff authz and attribution schema | [db/migrations/016_competition_authz_runtime.up.sql](../db/migrations/016_competition_authz_runtime.up.sql) |
| ARES preview | [internal/ares/service.go](../internal/ares/service.go) |
| CLI root commands | [cmd/apollo/main.go](../cmd/apollo/main.go) |
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
| Deferred | Acknowledged future work that should not ship in the current line. |
| Not planned yet | Mentioned idea with no committed implementation sequence. |
| Kill unless gated | Valid idea only if named gates pass first. |

Versioning reference: ASHTON uses formal pre-`1.0.0` semantic versioning now. `semver-lite` is historical shorthand only. See [ashton-platform Versioning Policy](../../ashton-platform/README.md#versioning-policy).

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
- Schedule substrate, booking request lifecycle, public-safe booking intake/status/availability.
- Presence, visit, tap-link, facility streak, and ATHENA-backed ops overview surfaces.
- Minimal member shell with existing API-backed routes.

### Schema-Authored But Not Current Product Truth

These are easy to misread as real runtime because the schema exists:

| Area | Current status | Ruling |
| --- | --- | --- |
| `apollo.ares_ratings`, `apollo.ares_matches`, `apollo.ares_match_players` | Authored in the initial migration, but not the active competition rating/result runtime. | Do not build new rating behavior on these without a deliberate migration decision. |
| `apollo.recommendations` | Authored table; current recommendation/coaching reads are primarily deterministic read-time surfaces. | Do not treat the table as evidence of a persisted recommendation engine. |

The active competition rating projection is `apollo.competition_member_ratings`, created by the competition-history runtime.

### Not Real Yet

APOLLO does not yet have:

- Durable command idempotency storage or replay.
- Universal command/dry-run coverage outside the supported Phase 3B.11
  competition command surface.
- Command-surface result finalization beyond dry-run planning.
- OpenSkill.
- Rating policy versioning, rating events, or rating audit log.
- Match tier classification.
- Player consensus result voting.
- Dispute/correction ledger.
- Rating-aware ARES.
- Sport/mode/facility queue intent.
- Calibration, decay, comeback bonus, upset bonus, climbing caps.
- Cross-sport transfer.
- Carry/tank/stability analytics.
- Persistent/sediment teams.
- Tournament, season, league, bracket, or seeding containers.
- Replay/event logs.
- Social reports, silent block list, honor/trust, no-show reliability.
- Public competition reads, leaderboards, records, rivalry, badges, XP, quests, guilds, bounty, prestige, or live events.
- Frontend contract matrix for Hestia/Themis route/API completeness.
- Cross-repo compatibility matrix.

### Important Documentation Inconsistency

The README still contains an older claim that the current slice avoids results, ratings, and standings, while later sections and code show those are now real. This is not cosmetic. Agents will use docs as operating truth. Stale docs can produce wrong implementation plans.

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

## Ten Gates

No tracer should ship without passing the gates relevant to it.

| # | Gate | Definition | Blocks |
| --- | --- | --- | --- |
| 1 | Docs Truth Gate | README, roadmap, hardening docs, and launch docs match shipped runtime. | All agent-led expansion |
| 2 | Boundary Gate | One release line per domain. No mixed launch of ratings, tournaments, social, and retention. | Big-bang work |
| 3 | Mutation Safety Gate | Mutations use `expected_version`, idempotency, or both; return structured outcome. | Staff/public writes |
| 4 | Rating Gate | Rating policy is versioned, golden-tested, auditable, and rollback-capable. | Public ratings, ARES v2 |
| 5 | Dispute Gate | Result correction and dispute ledger exist before public stakes. | Public rankings, badges, records, tournaments |
| 6 | Telemetry Gate | Counters, latencies, rejects, and failure modes are measured before launch. | Scale/public launch |
| 7 | Privacy Gate | Reporting, moderation audit, rate limits, leak tests, and role checks exist. | Social/public surfaces |
| 8 | Scale Gate | Row-count and latency ceilings are known; rebuild patterns have limits/runbooks. | Public leaderboards |
| 9 | Frontend Contract Gate | Hestia/Themis production routes call real APIs only; no silent stubs. | Frontend release |
| 10 | Cross-Repo Compatibility Gate | Compatibility matrix exists for APOLLO, ATHENA, Hestia, Themis, gateway, proto/platform. | Coordinated launch |

## Rating Architecture Ruling

Use OpenSkill underneath with custom APOLLO policy wrapped on top. Keep the current Elo-like behavior only as a legacy baseline and migration fallback.

OpenSkill is not implemented today. Current APOLLO rating behavior is a legacy APOLLO projection: custom logistic expectation, fixed K factor, synthetic sigma shrink, and synchronous full-sport recompute after result capture.

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

### Recommended Rating Module

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

Minimum durable additions:

- `apollo.rating_policy_versions`
- `apollo.competition_rating_events`
- `apollo.competition_member_ratings.policy_version`
- `apollo.competition_member_ratings.calibration_status`
- `apollo.competition_member_ratings.peak_mu`
- `apollo.competition_member_ratings.seeded_from_sport`
- `apollo.competition_member_ratings.seeded_with_ratio`
- `apollo.competition_member_ratings.seed_confidence`

Optional after scale gate:

- `apollo.rating_snapshots`
- `apollo.rating_rebuild_runs`
- `apollo.rating_anomaly_flags`

### Rating Contract

Any OpenSkill migration must define this contract before code changes:

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

1. Extract existing Elo-like logic unchanged into `internal/rating/policy/elo_legacy.go`.
2. Add golden characterization tests for current behavior.
3. Add rating event audit table.
4. Add OpenSkill policy implementation.
5. Dual-run Elo legacy and OpenSkill for internal matches.
6. Compare deltas by scenario and telemetry.
7. Switch read path to OpenSkill once acceptable.
8. Keep legacy fallback for one release.
9. Remove legacy writes later; keep golden fixtures forever.

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
| T19b policy interface | Keep legacy policy as default. If policy routing fails, disable non-default policy selection and keep legacy output. |
| T19c rating audit/events | Add events as append-only sidecar first. If event writes fail, fail closed for rating migration work while existing result capture remains unchanged, or make audit sidecar explicitly non-blocking only before public stakes. |
| T23 OpenSkill dual-run/cutover | Keep legacy read path for one release. If OpenSkill deltas exceed the accepted budget, disable OpenSkill read path and rebuild current ratings from legacy policy/events. |

## Feature Viability Matrix

Likelihood columns mean "likelihood of succeeding without damaging system agility/security/robustness if sequenced as recommended" and "likelihood of failure or non-happening if rushed from the current state."

| Feature | Current fit | Success likelihood if sequenced | Failure likelihood if rushed | Durability | Best approach |
| --- | --- | ---: | ---: | --- | --- |
| Docs truth cleanup | Direct docs fix | Very high | Medium | High | Do first; no product widening |
| Competition CLI parity | Shipped for supported existing competition commands in 3B.11 | High | Medium | High | Shared command/outcome path; no independent CLI domain model |
| Capability discovery | Shipped as competition command readiness in 3B.11; broader capability discovery remains later | High | Medium | High | Keep role/capability read model explicit |
| Universal dry-run | Shipped only for the supported 3B.11 competition command surface | Medium-high | High | High | Add command planning layer per surface; do not fake write success |
| Internal Themis competition ops shell | First APOLLO-backed shell shipped in 3B.11 | High if tightly scoped | High if it becomes public tournaments/social | High | Staff-only ops shell; no public surface, no browser trusted token |
| Approval/proposal workflow primitive | Fits booking, schedule, disputes, result corrections, social actions | Medium-high | High if generalized too early | Very high | Reusable propose/approve/reject/expire state machine with attribution |
| Match lifecycle state machine | Direct fit for consensus and disputes | High | Very high if skipped | Very high | Canonical states before finalization or rating writes |
| Notification substrate | Fits approval prompts, score votes, dispute alerts | Medium-high | Medium-high if it becomes messaging | High | One-way notifications/outbox only; no chat, DMs, or guild messaging |
| Rating extraction | Good fit | High | Medium | High | Move current math without behavior change |
| Rating policy versioning | Good fit | High | Medium | High | Interface, policy table, golden tests |
| OpenSkill hybrid | Good fit after extraction | High | High | High | Dual-run, event audit, fallback |
| Match tiers | Good fit on sessions | High | Medium | High | Add internal-only fields first |
| Rating multipliers | Needs rating policy | High | High | High | Policy wrapper, not raw DB fields only |
| Consensus voting | Needs result workflow change | Medium-high | High | High | Pending result state before completion |
| Disputes/corrections | Needs new ledger | Medium-high | Very high | High | Add before public stakes |
| Rating-aware ARES | Needs queue intent and rating reads | Medium-high | High | High | Sport/mode/facility queue intent first |
| Climbing cap | Needs rating policy | High | Medium | High | Policy wrapper with golden tests |
| Calibration | Needs policy and status fields | High | Medium | High | First N matches provisional |
| Inactivity decay | Needs job/audit | Medium-high | Medium | High | Sigma inflation only; audit every tick |
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
| Public leaderboards | Not ready | Medium later | Very high now | Medium-high | After disputes, privacy, scale |
| Records/hall of fame | Needs event/record engine | Medium later | High now | High | Derived from audited events only |
| Daily check-in XP | Fits presence/streaks | High if private | Medium | Medium | Private, explicit, 1 credit/day, ATHENA-linked |
| Season XP | Needs event ledger | Medium | High | Medium | After XP event table and anti-double-counting |
| Quests/challenges | Needs XP/badge engine | Medium | High | Medium | Toggleable per facility |
| Badges/trophies | Needs criteria registry | Medium | Very high if early | High | No public badges until dispute gate |
| Power rating/CP | Needs OpenSkill and XP/badge caps | Medium | High | Medium | Skill-dominant formula with capped bonuses |
| Spectator feed | Needs public-safe competition read | Medium later | High now | Medium | Start authenticated/private |
| Friend cheer/watchlist | Needs social/privacy | Medium later | High now | Medium | No PII leaks; rate limit |
| Predictions/pickem | Needs tournament/public read | Medium later | Medium-high now | Medium | Bragging-only, no money |
| Skill drills | Medium fit with sports/planner | Medium-high | Medium | High | Staff/witness verification levels |
| Practice mode | Fits schedule/drills | Medium-high | Medium | High | No rating impact; explicit opt-in |
| Composite athlete score | Needs rating across sports | Medium | High | Medium | Internal first, explain confidence |
| MVP voting | Needs voting and anti-abuse | Medium | High | Medium | After consensus/honor patterns exist |
| Draft/captains | Fits queue/assignment | Medium-high | Medium | Medium-high | Separate draft state; do not overload queue |
| Rivalry/nemesis | Needs public/social/privacy | Medium later | Very high now | Medium | Private/internal first |
| Guilds/squads | No substrate | Medium later | High now | Medium | Schema-only first, no messaging |
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

These are not public-stakes features. They can begin alongside Phase 1 when write scopes are isolated and relevant dependency gates are explicit. Rating-dependent rows still wait for rating extraction.

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
| T29 | `internal/ares` | Sport/mode/facility queue intent and rating-aware previews | 4, 5, 6 |
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
- Match preview, assignment, start/archive, and result capture only where
  backend contracts are real. In Phase 3B.11, result command apply remains
  dry-run-only until result trust work reopens it.
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

ARES today pairs eligible joined lobby members by stable user ID. That is intentionally narrow.

Best approach:

- Add explicit matchmaking intent:
  - sport
  - mode
  - facility
  - tier
  - queue type
  - mentor/practice/ranked flags
- Read ratings from `internal/rating`.
- Produce previews with quality/explanations.
- Do not mutate competition execution state on preview.

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
- Rating truth remains OpenSkill mu/sigma plus policy metadata.
- XP/badges can never let a low-skill grinder outrank a high-skill player in pure skill comparison.

### Frontend Contract

Hestia has an `/app/tournaments` route, but backend tournament runtime is not real yet.

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

| Surface | Probe |
| --- | --- |
| Member history | Cannot see other member match history through IDs or filters |
| Competition staff read | Member role gets 403 |
| Public booking status | Receipt lookup never exposes request UUID, staff notes, schedule block ID, staff ID, conflicts, trusted surface fields |
| Public competition future | Anonymous/member read never exposes private presence, block/report state, hidden ratings, dispute notes, staff notes |
| Social reports | Reporter cannot learn exact action against reported user |
| Silent blocks | Blocked user cannot infer block existence from explicit response |
| Leaderboards | Hidden/private/ghost users are excluded or anonymized by policy |
| Tournament public | Roster, age/student identity, and presence are not leaked beyond approved display fields |

## Telemetry Requirements

Current observability is mostly logs and hardening proof, not a full metrics plane. Do not claim telemetry readiness until metrics are exported and alert thresholds are defined.

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

Minimum alert examples:

- Rating update p95 exceeds defined ceiling.
- Dispute queue older than SLA.
- Trusted-surface failures spike.
- Public receipt not-found rate spikes.
- Result duplicate rejects spike.

## Observability Plan

Dashboards are not required for internal-only Phase 1 cleanup, but they are required before public competition surfaces.

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

These are planning targets, not current guarantees. A tracer must either implement metrics or state why metrics are out of scope.

## Capacity Constraints

The Scale Gate needs explicit ceilings before public rankings:

| Resource | Current posture | Required ceiling before widening |
| --- | --- | --- |
| Rating recompute | Synchronous full-sport delete/rebuild after result capture. | Max rows scanned and max duration per sport/mode before public leaderboard. |
| Postgres | Single service DB assumptions in local/runtime tests. | Database size, connection pool, backup, and restore targets documented. |
| NATS publish | ATHENA publish retry/dedupe exists, but APOLLO downstream idempotency still matters. | Restart/replay behavior and duplicate event budget documented. |
| Homelab/deploy nodes | Deployment truth is separate from repo/runtime truth. | Node count, CPU/RAM headroom, and tunnel/proxy limits documented in deployment repo before public claims. |
| Frontend routes | Hestia/Themis consume APOLLO APIs. | Payload stability and fallback/error behavior documented before public release. |

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
- Backups must preserve result, correction, rating event, staff attribution, booking receipt, and ATHENA observation history before public surfaces depend on them.

Postgres-loss recovery target is not defined yet. Before public competition launch, define which projections can be rebuilt, which append-only facts must be restored from backup, and which external sources can be replayed.

## Decision Log

Use this table to link future rulings to PRs, commits, or conversation artifacts.

| Date | Decision | Source |
| --- | --- | --- |
| 2026-04-26 | Launch expansion must be gated; trust substrate before public excitement layer. | This audit consolidation. |
| 2026-04-26 | Hybrid OpenSkill means OpenSkill kernel plus APOLLO policy plus legacy Elo characterization/fallback, not two permanent competing ratings. | This audit consolidation. |
| 2026-04-26 | CLI should start as a service-backed wrapper and evolve toward a shared application command layer; no independent CLI domain model. | This audit consolidation. |
| 2026-04-27 | Substrate decomposition accepted: internal Themis ops shell, approval/proposal workflow, match lifecycle, notifications, schedule policy, and resource splitting are reusable primitives, not public surfaces. | This audit consolidation. |
| 2026-04-27 | Phase 3B.11 shipped only command/readiness/CLI/Themis ops foundation; result trust, OpenSkill, analytics, tournament runtime, public competition surfaces, and game identity remain deferred. | 3B.11 closeout. |

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

## Immediate Action List

1. Closed by 3B.11: APOLLO command/outcome/readiness DTOs, service-backed
   competition CLI parity, and first internal Themis ops shell over APOLLO
   contracts.
2. Next: 3B.12 competition lifecycle and result trust.
3. Then: rating extraction/policy/audit before OpenSkill.
4. Later: analytics, tournament runtime, public competition surfaces, and game
   identity only after their gates are met.

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
