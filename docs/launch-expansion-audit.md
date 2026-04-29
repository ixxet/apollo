# Launch Expansion Audit

Status: consolidated read-only audit artifact, refreshed against shipped truth
Scope: APOLLO-centered, with ATHENA, Hestia, Themis, gateway, and platform compatibility noted where they affect launch safety
Date: 2026-04-29
Supersedes: 2026-04-26 revision
Cross-references: [planning/audits/2026-04-29-competition-system-audit.md](../../ashton-platform/planning/audits/2026-04-29-competition-system-audit.md)

## Executive Verdict

The expansion plan from the 2026-04-26 revision is no longer aspirational. The trust substrate has shipped at repo/runtime level. The correct strategic pattern held under execution.

Phase 1 (trust substrate), the internal-only portions of Phases 2–5, and the public-projection portion of Phase 6 are now shipped. Phase 7 retention concepts are partially shipped through a single consolidated `game_identity` projection rather than separate badge/XP/spectate modules. Phase 8 long-term engines remain correctly deferred.

The system is ready for:

- Internal staff competition ops at scale.
- Manager-internal safety/reliability review.
- Public-safe readiness and leaderboards.
- Public/member-safe game identity (CP, badges, rivalry, squad) derived from canonical result truth.
- OpenSkill internal comparison without active read-path switch.
- Internal tournaments, brackets, seeds, snapshots, and advancement.

The system is not yet ready for:

- Public tournaments with stakes.
- Public safety UI.
- OpenSkill on the active rating read path.
- Cross-sport transfer, calibration, decay, climbing caps as policy wrapper features (legacy + OpenSkill comparison shipped, custom policy wrapper not yet).
- Persistent/sediment teams (current teams remain session-scoped).
- Quests, season XP track, MVP voting, drafts, drills, onboarding flows, spectator feed.
- Messaging/chat, broad social graph, public profiles, public scouting.
- Deploy-level smoke verification of the new surfaces (3B-LC closed at repo/runtime, not deploy).
- Population-scale telemetry counters as exported metrics.

The correct next packet is deploy-level smoke proof of shipped surfaces, telemetry counter export, and a cross-repo compatibility matrix. Public tournaments, retention layer expansion, and OpenSkill read-path switch must wait behind those.

## Change Log Since 2026-04-26

| Area | 2026-04-26 status | 2026-04-29 status |
| --- | --- | --- |
| Docs truth | Stale README claims | Fixed; this audit and the 2026-04-29 competition system audit are aligned |
| Competition CLI parity | Missing | Shipped via shared command DTO path (`apollo competition command run`, tournament read helpers) |
| Capability discovery / dry-run | Missing | Shipped via command/readiness module with structured outcome and dry-run/apply split |
| Application command layer | Not started | Shipped: HTTP and CLI both route through shared command DTOs |
| Rating extraction | Not started | Shipped: `internal/rating/legacy.go` carries the prior Elo-like math unchanged |
| Rating policy versioning | Not started | Shipped: rating events carry engine + version, audit table populated |
| OpenSkill | Not implemented | Shipped as comparison-only (`internal/rating/openskill.go`); active read path remains legacy |
| Match tiers | Not implemented | Implicit via command/readiness policy and result lifecycle states |
| Consensus voting | Not implemented | Modeled through canonical result lifecycle with finalize/dispute/correct/void state transitions |
| Dispute / correction ledger | Not implemented | Shipped via canonical result + lifecycle events with `expected_result_version` guards and supersession-via-correction |
| Rating-aware ARES v2 | Not implemented | Shipped as proposal-only: queue intent + match preview facts; no execution mutation |
| Tournament runtime | No container | Shipped staff-internal: tournaments, brackets, seeds, team snapshots, match bindings, advancement, events |
| Manager-internal safety/reliability | Not implemented | Shipped: reports, blocks, reliability events, audit events behind trusted-surface |
| Public competition readiness | Not implemented | Shipped: redacted public projection contract |
| Public leaderboards | Not implemented | Shipped: redacted, allowlisted, computed from public-safe projections |
| Game identity (CP, badges, rivalry, squad) | Planned as separate retention modules | Shipped consolidated: derived projection from public-safe competition rows |
| Calibration, decay, climbing caps as policy wrapper | Not implemented | Still not implemented; rating wrapper expansion is the next rating packet |
| Cross-sport transfer | Not implemented | Still not implemented |
| Persistent/sediment teams | Not implemented | Still not implemented; teams remain session-scoped |
| ATHENA real ingress | Mock only | Still mock |
| Telemetry counters as exported metrics | Not implemented | Still not implemented; test anchors strong but metrics plane not exported |
| Cross-repo compatibility matrix | Not started | Still not formalized; ownership and touchpoints documented in 2026-04-29 audit |
| Deploy-level smoke proof | Not applicable | Pending; 3B-LC closed at repo/runtime only |

## Evidence Anchors

Current APOLLO and platform truth is grounded in these artifacts.

| Area | Evidence |
| --- | --- |
| Cross-repo competition truth snapshot | [planning/audits/2026-04-29-competition-system-audit.md](../../ashton-platform/planning/audits/2026-04-29-competition-system-audit.md) |
| Roadmap discipline | [docs/roadmap.md](roadmap.md) |
| Current README state | [README.md](../README.md) |
| Authz / capability matrix | [internal/authz/authz.go](../internal/authz/authz.go) |
| Trusted-surface enforcement | [internal/authz/trusted_surface.go](../internal/authz/trusted_surface.go) |
| HTTP route surface | [internal/server/server.go](../internal/server/server.go) |
| Command/readiness module | [internal/competition/command.go](../internal/competition/command.go) |
| Competition service and DTOs | [internal/competition/service.go](../internal/competition/service.go) |
| Result lifecycle and history | [internal/competition/history.go](../internal/competition/history.go) |
| Result/rating projection schema | [db/migrations/011_competition_history_runtime.up.sql](../db/migrations/011_competition_history_runtime.up.sql) |
| Staff authz/attribution schema | [db/migrations/016_competition_authz_runtime.up.sql](../db/migrations/016_competition_authz_runtime.up.sql) |
| Legacy rating module | [internal/rating/legacy.go](../internal/rating/legacy.go) |
| OpenSkill comparison module | [internal/rating/openskill.go](../internal/rating/openskill.go) |
| ARES v2 competition module | [internal/ares/competition.go](../internal/ares/competition.go) |
| Analytics module | [internal/competition/analytics.go](../internal/competition/analytics.go) |
| Tournament module | [internal/competition/tournament.go](../internal/competition/tournament.go), [internal/competition/tournament_repository.go](../internal/competition/tournament_repository.go) |
| Safety/reliability module | [internal/competition/safety.go](../internal/competition/safety.go), [internal/competition/safety_repository.go](../internal/competition/safety_repository.go) |
| Public projection module | [internal/competition/public.go](../internal/competition/public.go) |
| Game identity module | [internal/competition/game_identity.go](../internal/competition/game_identity.go), [internal/competition/game_identity_repository.go](../internal/competition/game_identity_repository.go) |
| CLI root | [cmd/apollo/main.go](../cmd/apollo/main.go) |

Focused proof:

```sh
go test -count=1 ./internal/competition ./internal/rating ./internal/ares ./internal/authz
go test -count=1 ./internal/server -run 'TestCompetition'
```

Result: passed at the time of this revision.

## Status Taxonomy

| Status | Meaning |
| --- | --- |
| Real | Implemented in repo/runtime with service/API/test evidence. |
| Schema-authored | Table exists, but active runtime does not use it as product truth. |
| Internal-only | Implemented for authenticated/staff/internal use, with no public/member-social surface. |
| Public-projection | Derived/redacted projection over canonical truth, with allowlisted output contract. |
| Comparison-only | Implemented but not on the active read path; produces audit/comparison facts only. |
| Deferred | Acknowledged future work that should not ship in the current line. |
| Not planned yet | Mentioned idea with no committed implementation sequence. |
| Kill unless gated | Valid idea only if named gates pass first. |

## Current System Inventory

### Real Today

APOLLO currently has:

- First-party member auth and session state.
- Member profile state and eligibility derived from explicit profile preferences.
- Explicit lobby membership.
- Sport registry and facility-sport capability mapping.
- Competition sessions, teams, rosters, queue membership, matches, side slots, assignment, start/archive lifecycle.
- Canonical result lifecycle: record, finalize, dispute, correct, void with `expected_result_version` guards.
- Versioned legacy active rating projection with rating event audit trail.
- OpenSkill internal comparison rows with delta flags (not on active read path).
- ARES v2 internal proposal: sport/mode/facility/tier queue intent + match preview facts (proposal-only).
- Analytics events and projections derived from finalized/corrected canonical results.
- Staff/internal tournament runtime: tournaments, brackets, seeds, immutable team snapshots, match bindings, advancement events.
- Manager/internal safety/reliability: reports, blocks, reliability events, audit events behind trusted-surface.
- Public-safe competition readiness contract with explicit projection/contract versions.
- Public-safe leaderboards over allowlisted stat/team scopes with redacted participant labels.
- Public/member-safe game identity projections: CP, badge awards, rivalry states, squad identities, all derived from public-safe competition rows.
- Self-scoped member stats/history/game identity routes.
- Shared command/readiness DTO surface used by HTTP and CLI with dry-run/apply.
- Role/capability authz with trusted-surface proof for staff-sensitive mutations.
- Staff action attribution for competition mutations.
- Schedule substrate, booking request lifecycle, public-safe booking intake/status/availability.
- Presence, visit, tap-link, facility streak, ATHENA-backed ops overview surfaces.
- Minimal member shell with API-backed routes.

### Schema-Authored But Not Current Product Truth

| Area | Current status | Ruling |
| --- | --- | --- |
| `apollo.ares_ratings`, `apollo.ares_matches`, `apollo.ares_match_players` | Authored in initial migration; superseded by competition runtime. | Do not build new behavior on these without a deliberate migration decision. |
| `apollo.recommendations` | Authored table; current recommendation/coaching reads are deterministic read-time surfaces. | Do not treat the table as evidence of a persisted recommendation engine. |

The active competition rating projection is `apollo.competition_member_ratings`, populated by the legacy rating module and audited via `apollo.competition_rating_events`.

### Not Real Yet

APOLLO does not yet have:

- Calibration, decay, climbing caps, comeback bonus, upset bonus as policy wrapper features.
- Cross-sport transfer with skill family graph.
- Persistent / sediment teams with ATHENA-backed promotion event.
- Honor / trust system separate from skill rating.
- Skill drills and PRs.
- Practice mode.
- Composite multi-sport athlete classification (game identity covers per-sport, not composite).
- MVP voting.
- Captain / draft / snake draft.
- First-visit onboarding, buddy auto-pair, welcomer badge.
- Spectator feed, friend cheer, watchlist, highlight notes.
- Predictions / pickem.
- Records / hall of fame surfaces.
- Quests / weekly/monthly challenges as a runtime engine.
- Season XP track, battle pass, prestige.
- Replay/event log as a separate forensic module (lifecycle events serve adjacent purpose, not full event sourcing).
- Pregame state (warm-up, coin toss, preview cards, optional bans).
- Public tournament surfaces.
- Public safety surfaces.
- Public/member rivalry surfaces beyond redacted game identity facts.
- Public records / hall of fame surfaces.
- Frontend contract matrix as an enumerated artifact (touchpoint inventory exists in the 2026-04-29 audit).
- Cross-repo compatibility matrix as an enumerated artifact (ownership documented; version pinning not formalized).
- Telemetry counters as exported runtime metrics.
- Deploy-level smoke proof of the new surfaces.

## CLI Architecture: Shipped Decision

The 2026-04-26 audit posed CLI parity as four options. The shipped pattern is the recommended one: shared application command layer.

```text
HTTP request -> command DTO -> application command handler -> domain service -> repository
CLI flags    -> command DTO -> application command handler -> domain service -> repository
```

The command layer owns input normalization, idempotency / expected-version checks, actor / trusted-surface construction, dry-run planning, and structured outcome enums. The domain service owns business invariants and state transitions. The repository owns persistence only.

This is the durable shape. Future CLI commands should extend this pattern, not introduce a parallel domain model or a raw HTTP wrapper.

## Ten Gates

No tracer should ship without passing the gates relevant to it.

| # | Gate | Definition | Status |
| --- | --- | --- | --- |
| 1 | Docs Truth Gate | README, roadmap, hardening docs, and launch docs match shipped runtime. | Pass. This audit and the 2026-04-29 competition audit are mutually consistent. |
| 2 | Boundary Gate | One release line per domain. No mixed launch of ratings, tournaments, social, and retention. | Pass. 3B-LC scope is explicit; future packets must stay scoped. |
| 3 | Mutation Safety Gate | Mutations use `expected_version`, idempotency, or both; return structured outcome. | Pass. Result and tournament writes use `expected_*_version`; commands return structured outcomes; dry-run/apply split is enforced. |
| 4 | Rating Gate | Rating policy is versioned, golden-tested, auditable, and rollback-capable. | Pass. `rating_engine` and `engine_version` carried per event; legacy fallback preserved; OpenSkill is comparison-only. |
| 5 | Dispute Gate | Result correction and dispute ledger exist before public stakes. | Pass for shipped surfaces. Result lifecycle covers record / finalize / dispute / correct / void. |
| 6 | Telemetry Gate | Counters, latencies, rejects, and failure modes are measured before launch. | Pass for Milestone 3.0. APOLLO exports runtime trust-spine metrics at `/metrics`; Prometheus scrape and alert examples are defined in GitOps. |
| 7 | Privacy Gate | Reporting, moderation audit, rate limits, leak tests, and role checks exist. | Pass for shipped public surfaces. Public output contract is explicit, allowlisted, redacted; trusted-surface proof stays server-side. |
| 8 | Scale Gate | Row-count and latency ceilings are known; rebuild patterns have limits/runbooks. | Pass for Milestone 3.0 ceiling declaration. Full public-stakes scale validation remains deferred. |
| 9 | Frontend Contract Gate | Hestia/Themis production routes call real APIs only; no silent stubs. | Pass. Themis and Hestia touchpoints inventoried with explicit must-not-touch rules in the 2026-04-29 audit. |
| 10 | Cross-Repo Compatibility Gate | Compatibility matrix exists for APOLLO, ATHENA, Hestia, Themis, gateway, proto/platform. | Pass for Milestone 3.0 once the platform compatibility matrix and evidence ledger are committed. |

Net after Milestone 3.0: the shipped trust-spine gates pass for the current bounded surface. Public-stakes expansion still requires a separate packet for full scale validation, policy-wrapper expansion, and product-surface widening.

## Rating Architecture Ruling

Use OpenSkill underneath with custom APOLLO policy wrapped on top. Keep the current Elo-like behavior as the legacy baseline and migration fallback.

Current shipped state:

```text
legacy Elo-like policy:
  active read path
  characterization baseline
  golden comparison source

OpenSkill kernel:
  comparison-only events and rows
  delta flags for parity tracking
  not on the active read path

APOLLO policy wrapper:
  not yet implemented
```

The custom policy wrapper (tiers, multipliers, calibration, transfer, decay, climbing caps, tournament snapshots) is the next rating packet. OpenSkill cannot move to active read path until that wrapper exists.

### Rating Options Re-Stated

| Option | Status |
| --- | --- |
| Keep current Elo-like only | Active reality. Acceptable for current internal-only and public-projection-only stakes. |
| Hard swap to OpenSkill | Rejected. Too risky against populated data with no policy wrapper. |
| Extract legacy, add policy interface, dual-run OpenSkill | Shipped. This is the durable pattern. |
| Permanent blended score from Elo + OpenSkill | Rejected. Hard to explain and audit. |
| Custom-reimplement OpenSkill concepts | Rejected. Math risk not worth it. |

### Required Rating Tables

Shipped:

- `apollo.competition_rating_events` (audit trail with engine + version per row)
- `apollo.competition_member_ratings` (active legacy projection)
- `apollo.competition_rating_comparisons` (OpenSkill comparison rows + delta flags)

Pending until the policy wrapper packet:

- `apollo.competition_member_ratings.calibration_status`
- `apollo.competition_member_ratings.peak_mu`
- `apollo.competition_member_ratings.seeded_from_sport`
- `apollo.competition_member_ratings.seeded_with_ratio`
- `apollo.competition_member_ratings.seed_confidence`
- `apollo.rating_policy_versions` (if a separate policy version registry beyond per-event metadata is needed)

Pending until the scale gate:

- `apollo.rating_snapshots`
- `apollo.rating_rebuild_runs`
- `apollo.rating_anomaly_flags`

### Rating Contract

Stable contract surfaces:

| Field / concept | Requirement | Status |
| --- | --- | --- |
| `rating_engine` | Explicit engine identifier (`legacy_elo_like`, `openskill`, future). | Shipped. |
| `engine_version` | Versioned semantic contract. | Shipped. |
| Sport/mode partition | Preserve partitioning by `sport_key` and mode key. | Shipped. |
| Display policy | Keep `current_rating_mu` / `current_rating_sigma` additive/backward-compatible. | Shipped. |
| Migration source | Rebuild from immutable match/result/rating-event truth. | Shipped. |
| Recompute watermark | Store the result/event offset the projection has processed. | Pending; required before scale gate. |
| Legacy compatibility | Legacy output remains available for one release after any future read-path cutover. | Pending until OpenSkill swap considered. |
| Privacy posture | Ratings remain self-scoped or projection-only until public-read privacy and scale gates pass. | Pass. |

Security hard stop: trusted-surface tokens never appear in browser-delivered Hestia/Themis bundles, local storage, or client-side environment variables. Recompute and admin paths stay CLI/internal or trusted-surface gated.

### Golden Test Cases

Required cases (frozen input/output committed to the repo):

| Case | Status | Purpose |
| --- | --- | --- |
| Unranked 1v1, A wins | Covered by legacy + OpenSkill comparison tests | Baseline |
| Pro beats new player | Covered | Conservative expected result |
| New player beats pro | Covered | Upset and (future) cap behavior |
| Casual 1v1 | Pending policy wrapper | Tier multiplier |
| Friendly match | Pending policy wrapper | No rating write |
| 5v5 even teams | Covered | Team-size weighting |
| 3v5 asymmetric match | Pending OpenSkill active path | OpenSkill team math |
| Draw | Covered | Outcome handling |
| First calibration match | Pending policy wrapper | Provisional behavior |
| Fifth calibration match | Pending policy wrapper | Provisional to ranked transition |
| Cross-sport seed | Pending transfer module | Transfer behavior |
| Inactive return after 90 days | Pending decay module | Sigma inflation / comeback |
| Tournament snapshot | Covered (immutable team snapshot at lock) | Frozen behavior |

### Dual-Run Migration

Shipped state of the migration plan:

1. Extract existing Elo-like logic unchanged into `internal/rating/legacy.go`. ✅
2. Add golden characterization tests for current behavior. ✅
3. Add rating event audit table. ✅
4. Add OpenSkill policy implementation. ✅ (comparison-only)
5. Dual-run legacy and OpenSkill for internal matches. ✅ (comparison-only)
6. Compare deltas by scenario and telemetry. 🟡 (comparison rows exist; runtime metrics plane not yet exported)
7. Switch read path to OpenSkill once acceptable. ⏸️ (gated on policy wrapper)
8. Keep legacy fallback for one release. ⏸️
9. Remove legacy writes later; keep golden fixtures forever. ⏸️

Rollback plan remains: rating events immutable, policy version on every update, previous read path preserved for one release after any cutover, derived current ratings rebuildable from events.

## Feature Viability Matrix

Likelihood columns mean "likelihood of succeeding without damaging system agility/security/robustness if sequenced as recommended" and "likelihood of failure or non-happening if rushed from current state."

| Feature | Current state | Success likelihood if sequenced | Failure likelihood if rushed | Durability | Best approach |
| --- | --- | ---: | ---: | --- | --- |
| Docs truth cleanup | Shipped (this revision) | Very high | Medium | High | Maintain via every closeout. |
| Competition CLI parity | Shipped | High | Medium | High | Extend via shared command layer. |
| Capability discovery / dry-run | Shipped | High | Medium | High | Extend command/readiness contract. |
| Application command layer | Shipped | High | Medium | High | New commands extend the pattern. |
| Rating extraction | Shipped | High | Medium | High | Stable. |
| Rating policy versioning | Shipped | High | Medium | High | Stable. |
| OpenSkill comparison | Shipped | High | High if pushed to active read | High | Keep comparison-only. |
| OpenSkill active read path | Not shipped | High after policy wrapper | High if rushed | High | Wait for policy wrapper. |
| Match tiers | Implicit via command policy | High | Medium | High | Add explicit fields when wrapper lands. |
| Rating multipliers | Not shipped | High | High | High | Wrapper, not raw DB only. |
| Calibration | Not shipped | High | Medium | High | First N matches provisional; needs wrapper. |
| Inactivity decay | Not shipped | Medium-high | Medium | High | Sigma inflation only; audit every tick. |
| Climbing cap | Not shipped | High | Medium | High | Wrapper with golden tests. |
| Comeback bonus | Not shipped | Medium | Medium | Medium | Keep small and explainable. |
| Cross-sport transfer | Not shipped | Medium-high | Medium | Medium-high | Seeding only; internal display first. |
| Carry / tank / stability analytics | Internal analytics shipped | Medium-high | Medium | Medium | Internal only until privacy gate widens. |
| Upset alerts / bonus | Not shipped | Medium | Medium-high | Medium | No friendly bonus; cap public effect. |
| Consensus voting | Not shipped explicitly | Medium-high | High if rushed | High | Modeled implicitly via lifecycle today; explicit voting pending. |
| Disputes / corrections | Shipped via lifecycle | High | Very high if rushed | High | Stable. |
| Rating-aware ARES v2 (proposal) | Shipped | Medium-high | High if pushed beyond proposal | High | Keep ARES proposal-only. |
| Sediment teams | Not shipped | Medium | Medium-high | High | Needs ATHENA real ingress and persistent team table. |
| Persistent teams | Not shipped | Medium-high | Medium-high | High | Separate from session teams. |
| Tournament containers | Shipped staff-internal | Medium-high | Medium | High | Extend to public only after gates. |
| Tournament runtime | Shipped staff-internal | Medium-high | Medium | High | Same. |
| Bracket formats (single / double / Swiss / round robin) | Single elim shipped via current bracket model | Medium | High if rushed | Medium-high | Add formats incrementally; document seeding policy. |
| Seeding / byes / manual overrides | Shipped (manual override attributed) | Medium | High | High | Stable. |
| TO tools (Themis) | Themis competition shell shipped | Medium | High | High | Frontend Contract Gate maintained. |
| Pregame | Not shipped | Medium | Medium | Medium | Add readiness/warmup before strategy bans. |
| Replay / event logs | Lifecycle events serve adjacent | Medium-high | Medium | Very high | Append-only event ledger when needed. |
| Manager-internal safety reports | Shipped | Medium | High if exposed publicly | High | Manager-only triage; stay internal. |
| Manager-internal silent block list | Shipped | Medium-high | High if exposed publicly | High | Stable internal. |
| Honor / trust | Not shipped | Medium | High | Medium-high | After reports/disputes mature. |
| Reliability / no-show events | Manager-internal shipped | Medium-high | Medium | High | Member-facing reliability deferred. |
| Public competition readiness | Shipped | High | Medium | High | Stable. |
| Public leaderboards | Shipped (redacted, allowlisted) | High | High if redaction breaks | High | Hold output contract. |
| Public game identity (CP, badge, rivalry, squad) | Shipped | High | High if policies drift | High | Tune policies once population data accumulates. |
| Public tournaments | Not shipped | Medium later | Very high now | Medium-high | After disputes, privacy, scale, telemetry. |
| Records / hall of fame | Not shipped | Medium later | High now | High | Derived from audited events only. |
| Daily check-in XP | Not shipped | High if private | Medium | Medium | Private, ATHENA-linked, one credit/day. |
| Season XP / battle pass | Not shipped | Medium | High | Medium | After event ledger and anti-double-counting. |
| Quests / challenges | Not shipped | Medium | High | Medium | Toggleable per facility. |
| Badges / trophies (independent) | Consolidated under game identity | High | High if independent system added now | High | Keep derived from canonical truth. |
| Power rating / CP | Shipped via game identity | High | High if dilutable | Medium | Skill-dominant formula; wrapper can refine. |
| Spectator feed | Not shipped | Medium later | High now | Medium | Authenticated/private first. |
| Friend cheer / watchlist | Not shipped | Medium later | High now | Medium | No PII leaks; rate limit. |
| Predictions / pickem | Not shipped | Medium later | Medium-high now | Medium | Bragging-only. |
| Skill drills | Not shipped | Medium-high | Medium | High | Staff/witness verification levels. |
| Practice mode | Not shipped | Medium-high | Medium | High | No rating impact; explicit opt-in. |
| Composite athlete score | Not shipped | Medium | High | Medium | Internal first; explain confidence. |
| MVP voting | Not shipped | Medium | High | Medium | After consensus/honor patterns exist. |
| Draft / captains | Not shipped | Medium-high | Medium | Medium-high | Separate draft state from queue. |
| Rivalry / nemesis | Shipped redacted via game identity | Medium later for richer surface | Very high if exposed beyond redaction | Medium | Hold current redacted form; richer surface deferred. |
| Squads / guilds | Shipped redacted via game identity (squad identity grouping) | Medium later for richer surface | High now if expanded | Medium | Schema-only widening; no messaging ever. |
| Bounty | Not shipped | Low now, medium later | Very high now | Low-medium | Schema variables only. |
| Live events | Not shipped | Medium later | High now | Medium | Variable substrate only. |
| Prestige | Not shipped | Low now, medium later | High now | Low-medium | Far future; preserve skill rating. |
| Onboarding / buddy | Not shipped | Medium-high | Medium | High | First-visit flow after frontend gate maintained. |
| Frontend contract | Touchpoint inventory shipped | High | High if ignored | High | Maintain inventory per release. |
| Cross-repo compatibility | Ownership documented; version matrix pending | High | High if ignored | Very high | Formalize matrix as next platform packet. |
| Telemetry counters | Test anchors only | High | Very high at scale | High | Export metrics before public-stakes expansion. |
| Deploy-level smoke | Pending | High | Very high | High | Required next packet. |

## Consolidated Roadmap

### Phase 1: Trust Substrate

Status: shipped.

| Tracer | Module | Adds | Status |
| --- | --- | --- | --- |
| T18a | docs/platform | Docs truth cleanup | ✅ Shipped (3B-LC + this revision) |
| T18b | `cmd/apollo/competition` | CLI parity | ✅ Shipped |
| T18c | `internal/api/capabilities` (via command/readiness) | Capability discovery + outcome schema | ✅ Shipped |
| T18d | application command layer | Shared command DTOs for HTTP/CLI/dry-run | ✅ Shipped |
| T19a | `internal/rating/legacy` | Extract current Elo-like unchanged | ✅ Shipped |
| T19b | `internal/rating/policy` | Versioned policy interface + golden tests | ✅ Shipped |
| T19c | `internal/rating/audit` | Rating event audit table | ✅ Shipped |
| T20 | competition schema/service | Match tier policy | 🟡 Implicit via command/readiness; explicit fields pending wrapper |
| T21 | `internal/competition/consensus` | Score consensus voting | 🟡 Modeled via lifecycle states; explicit player vote pending |
| T22 | `internal/competition/dispute` | Dispute ledger + correction flow | ✅ Shipped via canonical result lifecycle |
| T23 | `internal/rating/openskill` | OpenSkill dual-run | ✅ Shipped as comparison-only |

### Phase 2: Parallel-Safe Additions

Status: partial.

| Tracer | Module | Adds | Status |
| --- | --- | --- | --- |
| T24p | `internal/rating/transfer` | Cross-sport seeding | ⏸️ Not shipped |
| T25p | `internal/rating/analytics` (consolidated under competition analytics) | Carry / tank / stability | 🟡 Internal analytics shipped; coefficients not surfaced |
| T26p | `internal/competition/sediment` | Ad-hoc → persistent team promotion | ⏸️ Not shipped; needs ATHENA real ingress |
| T27p | `internal/xp/checkin` | Daily check-in XP | ⏸️ Not shipped |
| T28p | `internal/competition/tournament` schema + runtime | Tournament containers and runtime | ✅ Shipped staff-internal (further than schema-only) |
| T28q | frontend contract docs | Hestia/Themis route/API matrix | 🟡 Touchpoint inventory in 2026-04-29 audit; enumerated matrix pending |
| T28r | platform compatibility docs | Cross-repo compatibility matrix | 🟡 Repo ownership table present; version pinning pending |

### Phase 3: Skill-Aware Matchmaking

Status: partial.

| Tracer | Module | Adds | Status |
| --- | --- | --- | --- |
| T29 | `internal/ares/competition` | Sport/mode/facility queue intent + rating-aware previews | ✅ Shipped as proposal-only |
| T30 | `internal/rating/calibration` | Provisional, hidden mu, boosted sigma shrink | ⏸️ Not shipped |
| T31 | `internal/rating/decay` | Sigma inflation for inactive players | ⏸️ Not shipped |
| T32 | `internal/competition/upset` | Win probability flag + small underdog bonus | ⏸️ Not shipped |

### Phase 4: Tournament Runtime

Status: shipped staff-internal; public deferred.

| Tracer | Module | Adds | Status |
| --- | --- | --- | --- |
| T33 | `internal/tournament/format` | Single elim, double elim, Swiss, round robin | 🟡 Single-elim model present; additional formats pending |
| T34 | `internal/tournament/seeding` | Rating seeding + manual override + byes | ✅ Shipped (manual override attributed) |
| T35 | `internal/tournament/to_tools` | Themis bracket builder | ✅ Themis competition shell shipped |
| T36 | `internal/competition/replay` | Structured match event log | 🟡 Lifecycle events serve adjacent purpose; full replay log not shipped |
| T37 | `internal/competition/pregame` | Warm-up, coin toss, preview cards, optional bans | ⏸️ Not shipped |

### Phase 5: Social And Safety

Status: manager-internal shipped; public deferred.

| Tracer | Module | Adds | Status |
| --- | --- | --- | --- |
| T38 | `internal/competition/safety` | Silent block list + reports + triage + attribution | ✅ Shipped manager-internal |
| T39 | `internal/honor` | Honor votes + trust tiers | ⏸️ Not shipped |
| T40 | `internal/competition/safety` (reliability) | Attendance + no-show + reliability score | 🟡 Manager-internal events shipped; member-facing surface deferred |

### Phase 6: Public Surface

Status: partial. Public projections shipped; public tournaments and richer public surfaces deferred.

| Tracer | Module | Adds | Status |
| --- | --- | --- | --- |
| T41a | `internal/competition/public` | Public readiness + leaderboards | ✅ Shipped |
| T41b | Hestia/Themis public reads | Public bracket viewing + tournament pages | ⏸️ Not shipped |
| T42 | `internal/competition/nemesis` (consolidated under game identity) | Mortal Rivals, Iron Sharpens Iron | 🟡 Redacted rivalry facts shipped via game identity; richer surface deferred |
| T43 | `internal/predictions` | Pickem + prediction leaderboards | ⏸️ Not shipped |
| T44 | `internal/records` | Personal/facility records, milestones, hall of fame | ⏸️ Not shipped |

### Phase 7: Retention Layer

Status: consolidated under game identity. Per-feature richer surfaces deferred.

| Tracer | Module | Adds | Status |
| --- | --- | --- | --- |
| T45 | `internal/competition/game_identity` (badge awards) | Trophy registry + criteria + rarity | ✅ First-version badge awards shipped (first match, first win, regular competitor) |
| T46 | `internal/quests` | Daily/weekly/monthly challenges | ⏸️ Not shipped |
| T47 | `internal/xp/season` | Season XP, battle pass, prestige scaffold | ⏸️ Not shipped |
| T48 | `internal/spectate` | Match feed, cheer, watchlist, highlights | ⏸️ Not shipped |
| T49 | `internal/skill_drills` | Drill catalog, submissions, PRs | ⏸️ Not shipped |
| T50 | `internal/composite` | Multi-sport composite score | ⏸️ Not shipped |
| T51 | `internal/mvp` | MVP voting | ⏸️ Not shipped |
| T52 | `internal/draft` | Auto-captains, suggested picks, snake draft | ⏸️ Not shipped |
| T53 | `internal/onboarding` | First-visit flow, buddy pair, welcomer | ⏸️ Not shipped |

### Phase 8: Long-Term Engines

Status: correctly deferred. Schema-only-when-needed.

| Tracer | Module | Adds | Status |
| --- | --- | --- | --- |
| T54 | `internal/guilds` | Squad schema + duties + leaderboard variables; no messaging | ⏸️ Not shipped (squad identity grouping exists in game identity but not full guild module) |
| T55 | `internal/competition/bounty` | Kill count + bounty value variables | ⏸️ Not shipped |
| T56 | `internal/live_events` | Agent-triggered event variable substrate | ⏸️ Not shipped |
| T57 | `internal/prestige` | Voluntary XP reset; skill preserved | ⏸️ Not shipped |

## Recommended Next Packet Stack

| Priority | Packet | Why now |
| --- | --- | --- |
| 1 | Deploy-level smoke proof | 3B-LC closed at repo/runtime; live cluster verification of new surfaces is the next credibility jump and the audit's named residual risk. |
| 2 | Telemetry counter export | Required before any public-stakes expansion. Test anchors are not runtime metrics. |
| 3 | Cross-repo compatibility matrix | One artifact in `ashton-platform`. Locks version pinning between APOLLO, ATHENA, Hestia, Themis, ashton-proto. Prevents silent breakage. |
| 4 | Rating policy wrapper expansion | Calibration + decay + climbing caps. Required before OpenSkill can ever switch to active read path. |
| 5 | ATHENA real ingress (cross-repo) | Sediment teams, daily check-in XP, reliability presence verification all blocked on this. CSV replay adapter is the cheapest unblock. |
| 6 | Game identity policy hardening | First-version policies need usage-driven tuning. Define the tuning loop before population data accumulates. |

What to avoid as a next packet:

- ❌ Public tournaments (Dispute Gate locally clean, but Privacy/Scale/Telemetry need work first).
- ❌ OpenSkill read-path switch (policy wrapper has to land first).
- ❌ Honor / MVP / quests / drafts / onboarding (retention layer needs trust substrate at deploy level, not just repo level).
- ❌ Cross-sport transfer (nice-to-have; doesn't unblock anything pressing).
- ❌ New retention modules independent of game identity (consolidation under game identity is working; don't fragment it).
- ❌ Messaging / chat / broad public social graph (correctly listed as Hard Non-Touches).

## Hard Non-Touches

Reaffirmed from the 2026-04-29 audit. Future packets must not implicitly create or modify:

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
- deployment truth or Prometheus manifests
- Hestia staff controls
- Themis public surface
- UI-owned formulas for ratings, analytics, matchmaking, tournaments, safety, or game identity

## Leak Test Specification

Privacy Gate requires concrete leak tests. Public surfaces shipped without leak tests are not gate-safe.

| Surface | Probe | Status |
| --- | --- | --- |
| Member history | Cannot see other member match history through IDs or filters | Self-scoped reads enforced |
| Competition staff read | Member role gets 403 | Capability matrix enforced |
| Public booking status | Receipt lookup never exposes request UUID, staff notes, schedule block ID, staff ID, conflicts, trusted-surface fields | Maintained |
| Public competition readiness | Anonymous/member read never exposes raw user IDs, internal request IDs, match IDs, canonical result IDs, projection watermarks, sample sizes, OpenSkill mu/sigma/comparison, ARES internals, safety facts, command internals, or trusted-surface fields | Public output contract enforces |
| Public leaderboards | Same redaction rules; participant labels redacted | Enforced |
| Public game identity | CP/badge/rivalry/squad facts redacted to public-safe scope; no raw IDs, no hidden ratings, no policy internals beyond version strings | Enforced |
| Social reports | Reporter cannot learn exact action against reported user | Manager-internal only; not yet exposed to members |
| Silent blocks | Blocked user cannot infer block existence from explicit response | Manager-internal only |
| Hidden / private / ghost users | Excluded or anonymized by policy from leaderboards | Allowlisted projection enforces |
| Tournament public | Roster, age/student identity, presence not leaked | Public tournaments not yet exposed |

Action: every public surface added in future packets must extend this table with its own probe set before launch.

## Telemetry Requirements

Current observability now includes APOLLO runtime metrics export for the launch-expansion trust spine. The metrics plane is intentionally limited to counters and duration summaries needed for deploy smoke and operator alerts; it does not add public product behavior.

Minimum counters before any public-stakes packet beyond current public projections:

- `competition_result_write_attempt_total`
- `competition_result_write_reject_total{reason}`
- `competition_consensus_vote_total{tier,outcome}` (when explicit consensus voting ships)
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
- `tournament_advancement_total{format}`
- `tournament_seed_lock_total`
- `safety_report_open_total{category}`
- `safety_block_active_total`
- `reliability_event_total{kind}`
- `game_identity_projection_duration_seconds`

Minimum alert examples:

- Rating update p95 exceeds defined ceiling.
- Dispute queue older than SLA.
- Trusted-surface failures spike.
- Public receipt not-found rate spikes.
- Result duplicate rejects spike.
- Game identity projection duration regresses beyond defined ceiling.

## Milestone 3.0 Scale Ceilings

These are declared ceilings for the current shipped projections, not proof of full public-stakes scale:

- Public competition leaderboards default to 25 rows and clamp at 100 rows per request.
- Game identity projections default to 10 rows and clamp at 50 rows per request.
- APOLLO JSON body parsing remains capped at 1 MiB for normal JSON writes and 16 KiB for public booking intake.
- Rating rebuild and game identity projection duration alert examples use a 2 second average-duration ceiling for Milestone 3.0 deploy smoke.
- Rating rebuild row scans are observable through `rating_rebuild_rows_scanned_total`; row-growth tuning remains deferred until real production volume requires it.
- Public tournaments, public safety surfaces, OpenSkill active-read-path switch, and retention mechanics remain outside these ceilings.

## Data Quality Rules

Reaffirmed and extended:

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
- Game identity policies must be versioned per projection layer (CP, badge, rivalry, squad) and must not bypass canonical result truth.
- OpenSkill comparison rows must not be read by any active product surface.

## Kill Criteria

Kill or defer a tracer if any of these are true:

- It requires public result trust before disputes exist (passed for shipped surfaces; reaffirm per future surface).
- It exposes social/leaderboard/rivalry surfaces beyond current redacted projections before reporting and moderation are member-safe.
- It depends on ATHENA physical truth but only mock data is available.
- It needs Hestia/Themis API routes that are stubbed or undefined.
- It requires cross-repo contract changes without compatibility matrix updates.
- It requires public scale but rating recompute duration / row ceiling is unknown.
- It adds a second domain model instead of sharing service/application command behavior.
- It makes retention rewards from untrusted or correctable data.
- It moves OpenSkill onto active reads without the policy wrapper landing.
- It widens game identity output beyond the explicit public output contract without an updated leak test.

## Immediate Action List

1. Plan the deploy-level smoke proof packet against current shipped surfaces.
2. Plan the telemetry counter export packet using the list above.
3. Plan the cross-repo compatibility matrix packet in `ashton-platform`.
4. Begin design of the rating policy wrapper packet (calibration + decay + climbing caps).
5. Plan an ATHENA real-ingress unblock (CSV replay adapter is the cheapest path).
6. Define a game identity policy tuning loop before population data accumulates.
7. Maintain this audit and the dated competition system audit in lockstep on every closeout.

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
go test -count=1 ./internal/competition ./internal/rating ./internal/ares ./internal/authz
go test -count=1 ./internal/server -run 'TestCompetition'
```

Run Hestia/Themis test suites only when their API contracts, mocks, routes, or payload expectations change. Do not add frontend test requirements to backend-only tracers unless Gate 9 is in scope.

## Final Ruling

The expansion plan from the 2026-04-26 revision held under execution. The trust substrate is shipped. The internal-only and public-projection surfaces that depended on it are shipped. The retention layer concepts that the original plan scattered across modules are consolidated under a single derived game identity projection — leaner and harder to drift than the original plan.

The danger is not vision; it remains sequencing.

The durable architecture is reaffirmed:

- Competition owns result truth.
- Rating owns rating policy and math.
- ARES owns matchmaking proposals.
- Tournament owns tournament containers and snapshots.
- Safety owns moderation, manager-internal only.
- Game identity owns derived public-safe projections (CP, badges, rivalry, squad).
- Frontends consume real contracts only.
- Cross-repo compatibility is explicit (formalization pending).

Hybrid OpenSkill plus legacy Elo remains the migration path. OpenSkill stays the durable kernel, APOLLO policy is the product layer, legacy Elo is characterization and fallback. The next rating packet is the policy wrapper. Until it lands, the read path stays legacy.

Public expansion (tournaments, richer rivalry, records, predictions, public safety, public game identity widening) waits behind deploy proof, telemetry, scale gate, and cross-repo compatibility matrix. Retention layer expansion (quests, season XP, drills, MVP, draft, onboarding) waits behind the policy wrapper and deploy proof.

The next packet is deploy-level smoke proof of shipped surfaces. Everything else can wait.
