# apollo

APOLLO is the member-facing application in ASHTON. It will eventually own
profile state, privacy and availability controls, workout logging,
recommendations, and the ARES matchmaking subsystem.

> Current real slice: first-party member auth and session-backed profile state,
> deterministic visit-history ingest and close behavior, and derived lobby
> eligibility from persisted `visibility_mode` / `availability_mode`, plus the
> first explicit member-owned workout runtime and first deterministic workout
> recommendation read, plus the first real member web shell over those
> existing APIs, and the first deterministic, explainable, read-only ARES match
> preview over explicit lobby membership only, plus the first APOLLO-owned
> competition substrate with deterministic sport registry reads,
> facility-sport capability reads, and static sport rules/config for badminton
> and basketball, plus the first APOLLO-owned competition execution runtime
> with authenticated internal HTTP queue state, deterministic assignment into
> session/team/roster/match containers, and explicit session lifecycle
> transitions. APOLLO now proves account ownership, signed session handling,
> the first full visit lifecycle slice, the first real intent-behavior slice,
> explicit workout-history create/update/finish behavior, one narrow member
> shell, a deterministic coaching recommendation runtime, a deterministic match
> preview, one bounded competition substrate, and one bounded competition
> execution substrate without widening into results, ratings, standings, or
> public competition reads. Tracer 23 adds APOLLO-local exercise and equipment
> catalog truth, owner-scoped templates/loadouts, week-rooted planner truth,
> and typed non-medical `coaching_profile` inputs through authenticated
> internal HTTP while keeping workouts, visits, membership, competition
> history, and recommendation precedence separate. Tracer 24 is the tagged
> deterministic coaching line on `v0.15.0`, with `v0.15.1` kept as the narrow
> post-closeout hardening patch on that same line. The current Tracer 28
> repo/runtime closeout line on `main` now also adds explicit principal roles,
> deterministic competition capabilities, trusted-surface-gated privileged
> competition mutations, and durable actor attribution over the existing
> APOLLO competition control boundary while keeping departure-close semantics,
> planner, nutrition, recommendations, member self-service surfaces, and
> deployment truth separate. Phase 3B.1 adds one read-only ops overview
> surface that composes APOLLO schedule truth with ATHENA occupancy and bounded
> analytics truth for supervisor, manager, and owner roles only. Phase 3B.4
> added internal staff-entered booking request truth with request-state
> persistence, booking_read / booking_manage capabilities, trusted-surface
> gated mutations, conflict-aware availability, and approval that creates a
> linked APOLLO schedule reservation block. Phase 3B.5 now adds approved
> booking cancellation that atomically cancels the linked internal reservation
> and retains booking-to-schedule linkage without widening into public booking
> or payments.

This repo is now executable, but still intentionally narrow. The right way to
document it is to separate what is already real from what is only authored in
schema form or preserved as a future plan. Tracer 11 keeps the frontend claim
as narrow as the backend proof: members can bootstrap a session, load profile
summary, read and mutate workouts, finish a workout, and read one deterministic
recommendation through a minimal embedded web shell without changing the server
ownership model or widening deployment truth. Phase 2 keeps that posture:
backend and API/CLI truth may keep growing, but meaningful frontend widening
stays deferred through all of Phase 2.

## Start Here

| Reader | Start With | Why |
| --- | --- | --- |
| Recruiter or interviewer | [`Runtime Surfaces`](#runtime-surfaces), [`Current State Block`](#current-state-block), [`Why APOLLO Matters`](#why-apollo-matters) | These sections show the real backend slice quickly |
| Engineer | [`Architecture`](#architecture), [`Ownership And Boundaries`](#ownership-and-boundaries), [`Known Caveats`](#known-caveats) | These sections explain what APOLLO owns and where the product is still incomplete |
| Maintainer | [`docs/README.md`](docs/README.md), [`docs/glossary.md`](docs/glossary.md), [`docs/runbooks/member-state.md`](docs/runbooks/member-state.md) | These docs explain the current line, vocabulary, and member-state rules |

## Architecture

The standalone Mermaid source for this flow lives at
[`docs/diagrams/apollo-visit-ingest.mmd`](docs/diagrams/apollo-visit-ingest.mmd).

```mermaid
flowchart LR
  member["member or test client"]
  health["HTTP health<br/>/api/v1/health"]
  auth["auth + session boundary"]
  profile["profile + eligibility"]
  workouts["workout runtime"]
  recs["deterministic workout recommendation"]
  db["Postgres<br/>users, sessions, claimed_tags,<br/>visits, workouts, exercises"]
  cli["CLI<br/>apollo visit list"]
  athena["athena<br/>identified arrival and departure publish"]
  nats["NATS<br/>athena.identified_presence.arrived<br/>athena.identified_presence.departed"]
  consumer["visit consumer<br/>shared contract parse"]
  visits["visit service<br/>dedupe, idempotency,<br/>open and close rules"]
  future["future APOLLO domains<br/>lobby membership,<br/>ARES, persisted recommendations"]

  member --> auth --> db
  member --> profile --> db
  member --> workouts --> db
  member --> recs --> db
  health --- auth
  athena --> nats --> consumer --> visits --> db
  db --> cli
  db -. future expansion .-> future
```

## Runtime Surfaces

| Surface | Path / Command | Status | Notes |
| --- | --- | --- | --- |
| HTTP health | `GET /api/v1/health` | Real | Indicates service health and whether the NATS consumer is enabled |
| Serve command | `apollo serve` | Real | Starts the health endpoint and optional NATS consumer |
| Shell root | `GET /` | Real | Redirects to `/app/login` or `/app/home` based on whether the session cookie is valid |
| Member login shell | `GET /app/login` | Real | Public HTML bootstrap for verification start + token verification over the existing auth APIs |
| Member web shell | `GET /app`, `GET /app/{section}` | Real | Protected routed HTML shell over `/app/home`, `/app/workouts`, `/app/meals`, `/app/tournaments`, and `/app/settings`; it stays member-safe, allows one bounded facility-calendar outlook over the presence family, keeps booking and staff schedule surfaces out, and replaces total bootstrap or section failure with explicit recoverable UI |
| Verification start | `POST /api/v1/auth/verification/start` | Real | Starts registration or passwordless sign-in with student ID + email |
| Verification consume | `GET/POST /api/v1/auth/verify` | Real | Consumes a stored token, marks it used, verifies email ownership, and issues a signed session cookie |
| Profile read | `GET /api/v1/profile` | Real | Requires a valid session cookie and returns persisted member profile state |
| Presence read | `GET /api/v1/presence` | Real in repo/runtime | Requires a valid session cookie and returns facility-scoped member presence, explicit tap-link metadata, recent linked visit truth, and facility streak state over explicit visit rows only |
| Presence claim list/create | `GET/POST /api/v1/presence/claims` | Real in repo/runtime | Requires a valid session cookie, stays self-scoped, and exposes one honest member-owned claimed-tag path while rejecting malformed, inactive, foreign, duplicate, and replayed claims cleanly |
| Member facility composition | `GET /api/v1/presence/facilities` | Real in repo/runtime | Requires a valid session cookie and derives member-safe facility hours and capability metadata from APOLLO-owned facility refs, sport capabilities, and public schedule windows without exposing staff-only schedule internals |
| Member facility calendar | `GET /api/v1/presence/facilities/{facilityKey}/calendar?from=<RFC3339>&until=<RFC3339>` | Real in repo/runtime | Requires a valid session cookie, keeps explicit RFC3339 boundaries with a 14-day max window, and projects only facility-scope public operating-hours and closure windows without exposing raw schedule occurrences, conflicts, or staff metadata |
| Profile update | `PATCH /api/v1/profile` | Real | Requires a valid session cookie and updates `visibility_mode`, `availability_mode`, bounded non-medical `coaching_profile` inputs, and bounded non-clinical `nutrition_profile` inputs (`dietary_restrictions`, cuisine preferences, `budget_preference`, `cooking_capability`) |
| Planner exercise catalog | `GET /api/v1/planner/exercises` | Real | Requires a valid session cookie and returns APOLLO-owned exercise definitions with allowed equipment keys |
| Planner equipment catalog | `GET /api/v1/planner/equipment` | Real | Requires a valid session cookie and returns APOLLO-owned equipment definitions with one bounded `is_machine` flag |
| Planner template list/create | `GET/POST /api/v1/planner/templates` | Real | Requires a valid session cookie and returns or creates owner-scoped reusable templates/loadouts |
| Planner template detail/update | `GET/PUT /api/v1/planner/templates/{id}` | Real | Requires a valid session cookie, stays owner-scoped, and enforces duplicate-name and catalog validation rules |
| Planner week read/write | `GET/PUT /api/v1/planner/weeks/{week_start}` | Real | Requires a valid session cookie and reads or replaces one owner-scoped ISO-week planner record without creating workouts |
| Lobby eligibility read | `GET /api/v1/lobby/eligibility` | Real | Requires a valid session cookie and derives open-lobby eligibility from stored profile state only |
| Lobby membership read | `GET /api/v1/lobby/membership` | Real | Requires a valid session cookie and returns explicit durable lobby membership state without inferring from visits or eligibility |
| Lobby match preview read | `GET /api/v1/lobby/match-preview` | Real | Requires a valid session cookie and returns a deterministic, read-only preview over explicit joined lobby membership only; repeated reads stay stable while membership and eligibility inputs are unchanged |
| Lobby membership join | `POST /api/v1/lobby/membership/join` | Real | Requires a valid session cookie and an eligible member profile; repeated join stays deterministic |
| Lobby membership leave | `POST /api/v1/lobby/membership/leave` | Real | Requires a valid session cookie and an already joined member; repeated leave stays deterministic |
| Workout create | `POST /api/v1/workouts` | Real | Requires a valid session cookie and creates one member-owned `in_progress` workout |
| Workout list | `GET /api/v1/workouts` | Real | Requires a valid session cookie and returns workout history ordered by newest creation first (`started_at DESC, id DESC`) |
| Workout detail | `GET /api/v1/workouts/{id}` | Real | Requires a valid session cookie and is owner-scoped |
| Workout update | `PUT /api/v1/workouts/{id}` | Real | Requires a valid session cookie and replaces draft exercise data while the workout is `in_progress` |
| Workout finish | `POST /api/v1/workouts/{id}/finish` | Real | Requires a valid session cookie and finishes a non-empty `in_progress` workout |
| Workout effort feedback | `PUT /api/v1/workouts/{id}/effort-feedback` | Real | Requires a valid session cookie, is owner-scoped, and accepts one bounded effort enum for finished workouts only |
| Workout recovery feedback | `PUT /api/v1/workouts/{id}/recovery-feedback` | Real | Requires a valid session cookie, is owner-scoped, and accepts one bounded recovery enum for finished workouts only |
| Workout recommendation | `GET /api/v1/recommendations/workout` | Real | Requires a valid session cookie and returns one deterministic coaching recommendation from explicit workout history |
| Coaching recommendation | `GET /api/v1/recommendations/coaching?week_start=YYYY-MM-DD` | Real | Requires a valid session cookie and returns deterministic Tracer 24 coaching proposal/explanation output without mutating planner truth |
| Nutrition meal template list/create | `GET/POST /api/v1/nutrition/meal-templates` | Real | Requires a valid session cookie and returns or creates owner-scoped reusable meal templates with bounded nutrition totals only |
| Nutrition meal template update | `PUT /api/v1/nutrition/meal-templates/{id}` | Real | Requires a valid session cookie, stays owner-scoped, and rejects duplicate names or empty nutrition payloads cleanly |
| Nutrition meal log list/create | `GET/POST /api/v1/nutrition/meal-logs` | Real | Requires a valid session cookie and returns or creates owner-scoped meal-log truth with manual or template-backed entries |
| Nutrition meal log update | `PUT /api/v1/nutrition/meal-logs/{id}` | Real | Requires a valid session cookie, stays owner-scoped, and keeps explicit update payloads deterministic |
| Nutrition recommendation | `GET /api/v1/recommendations/nutrition` | Real in repo/runtime | Requires a valid session cookie and returns conservative non-clinical calorie/macro ranges with thin structured limitations and no planner mutation |
| Logout | `POST /api/v1/auth/logout` | Real | Revokes the current server-side session and clears the cookie |
| Visit readback | `apollo visit list --student-id ... --format text|json` | Real | Lists visit history for a member |
| Sport registry read | `apollo sport list --format text|json` | Real | Lists deterministic APOLLO-owned sport definitions for badminton and basketball |
| Sport detail read | `apollo sport show --sport-key <key> --format text|json` | Real | Returns one sport definition plus its facility-sport capability rows |
| Facility-sport capability read | `apollo sport capability list [--sport-key ...] [--facility-key ...] --format text|json` | Real | Lists deterministic APOLLO-owned facility-sport capability mappings without scheduling or live availability claims |
| Competition session list | `GET /api/v1/competition/sessions` | Real in repo/runtime | Authenticated competition staff read requiring explicit `competition_read` capability |
| Competition session detail | `GET /api/v1/competition/sessions/{id}` | Real in repo/runtime | Authenticated competition staff detail read requiring explicit `competition_read` capability |
| Competition member stats | `GET /api/v1/competition/member-stats` | Real | Authenticated self-scoped member stats remain separate from staff competition authz |
| Competition member history | `GET /api/v1/competition/history` | Real in repo/runtime | Authenticated self-scoped match history returns only the caller's own completed competition outcomes and minimal event context, not staff/session control detail |
| Competition session create | `POST /api/v1/competition/sessions` | Real in repo/runtime | Requires `competition_structure_manage` plus trusted-surface proof and writes durable actor attribution on success |
| Competition team / roster / match structure writes | `POST /api/v1/competition/sessions/{id}/teams`, `.../teams/{teamID}/members`, `.../matches`, and archive/remove actions | Real in repo/runtime | Structure mutations require `competition_structure_manage` plus trusted-surface proof; provenance columns remain domain truth, not the sole authz key |
| Competition queue open / join / remove | `POST /api/v1/competition/sessions/{id}/queue/open`, `.../queue/members`, and `.../queue/members/{userID}/remove` | Real in repo/runtime | Live-manage mutations require `competition_live_manage` plus trusted-surface proof over explicit joined lobby membership plus current eligibility |
| Competition assignment / lifecycle / result | `POST /api/v1/competition/sessions/{id}/assignment`, `.../start`, `.../archive`, and `.../matches/{matchID}/result` | Real in repo/runtime | Deterministic live-manage mutations require `competition_live_manage` plus trusted-surface proof; stale queue/state replay still fails safely |
| Ops facility overview | `GET /api/v1/ops/facilities/{facilityKey}/overview?from=<RFC3339>&until=<RFC3339>[&bucket_minutes=N]` | Real in repo/runtime | Requires `ops_read`, is supervisor/manager/owner only, composes APOLLO schedule calendar truth with ATHENA current occupancy and bounded analytics, and returns sanitized aggregate ops truth without booking writes, raw tap hashes, or identity-level presence detail |
| Booking request list/detail | `GET /api/v1/booking/requests`, `GET /api/v1/booking/requests/{id}` | Real in repo/runtime | Requires `booking_read`; supervisor, manager, and owner can read request state, source/channel, and APOLLO-computed availability without payment fields |
| Booking request create/transition | `POST /api/v1/booking/requests`, `.../{id}/review`, `.../needs-changes`, `.../reject`, `.../cancel`, and `.../approve` | Real in repo/runtime | Staff create/transition requires `booking_manage` plus trusted-surface proof; manager and owner can create or transition requests, every transition requires `expected_version`, approval creates a linked internal `reservation` / `hard_reserve` schedule block through APOLLO schedule truth, and approved cancellation cancels that linked block atomically |
| Public booking request intake | `GET /api/v1/public/booking/options`, `POST /api/v1/public/booking/requests` | Real in repo/runtime | Unauthenticated bounded intake; options expose only active/bookable/public-labeled choices, submit requires an idempotency key, creates only `requested` public-source requests, returns a neutral receipt, and never creates schedule blocks |
| Event consumer | `apollo serve` with `APOLLO_NATS_URL` | Real | Consumes `athena.identified_presence.arrived` and `athena.identified_presence.departed` from NATS |
| Recommendation storage | `apollo.recommendations` | Schema authored | Tracer 7 does not persist recommendation reads yet |
| Match preview runtime | `GET /api/v1/lobby/match-preview` | Real | ARES preview logic is active as a read-only runtime over explicit lobby membership only |

## Ownership And Boundaries

| APOLLO Owns | APOLLO Does Not Own |
| --- | --- |
| member profile and preference state | raw facility presence truth |
| derived lobby eligibility and explicit lobby membership intent | invites or match formation |
| visit history as member-facing context | occupancy counting |
| workout history | broad staff product workflows outside the bounded competition and ops-read control boundaries |
| deterministic recommendation and coaching context | the shared wire contract definitions |
| sport registry, facility-sport capability, and static sport rules/config | ATHENA-owned facility hours, closures, and raw live availability until a later APOLLO scheduling substrate composes over those inputs |
| competition session / team / roster / match containers, queue/assignment/lifecycle truth, result capture, ratings, standings, self-scoped member stats, and the bounded competition staff authz substrate | public competition reads, role-management product flows, facility-scoped staffing, persistent approval objects, rivalry/badge logic, and broad social competition surfaces |
| read-only ops composition over APOLLO schedule truth and ATHENA occupancy/analytics truth plus internal request-first booking runtime truth | raw tap identities, identity-level presence search, ATHENA analytics semantics, HERMES staff UX, public booking, customer self-booking, quotes/payments, Hestia booking UI, and deploy orchestration |
| explicit matchmaking intent and deterministic ARES preview | tool routing, invites, notifications, and global approval policy |

APOLLO owns member intent. That is the key boundary. Presence can affect member
context, but tap-in alone must not create workout logs, matchmaking lobby
eligibility, or any social state.

## Current Data Model

| Area | Status | Current Runtime Use |
| --- | --- | --- |
| `apollo.users` | Real | Member records now support visit linkage, email verification state, one explicit APOLLO-global role, and flexible profile preferences |
| `apollo.email_verification_tokens` | Real | Stores hashed verification tokens with expiry and single-use semantics |
| `apollo.sessions` | Real | Stores server-side session state keyed by a signed cookie value |
| `apollo.claimed_tags` | Real | Links ATHENA identity hashes to member accounts |
| `apollo.visits` | Real | Stores visit open/close history with deterministic departure idempotency |
| `apollo.visit_tap_links` | Real in repo/runtime | Stores the explicit per-visit linkage that makes a visit member-visible presence truth |
| `apollo.member_presence_streaks` and `apollo.member_presence_streak_events` | Real in repo/runtime | Store facility-scoped streak state and one append-only event per credited member/facility UTC visit day |
| `apollo.lobby_memberships` | Real | Stores explicit durable join/leave state separate from eligibility, visits, and workouts |
| `apollo.workouts` and `apollo.exercises` | Real | Stores explicit workout draft and finished history with ordered exercise rows |
| `apollo.equipment_definitions`, `apollo.exercise_definitions`, and `apollo.exercise_definition_equipment` | Real | Stores APOLLO-owned exercise-library truth separate from workout history |
| `apollo.workout_templates` and `apollo.workout_template_items` | Real | Stores owner-scoped reusable templates/loadouts with catalog-backed item rows |
| `apollo.planner_weeks`, `apollo.planner_sessions`, and `apollo.planner_session_items` | Real | Stores week-rooted planner truth separate from workouts, visits, and recommendations |
| `apollo.workout_effort_feedback` and `apollo.workout_recovery_feedback` | Real | Stores one owner-scoped feedback row per finished workout for deterministic coaching ladder inputs |
| `apollo.nutrition_meal_templates` and `apollo.nutrition_meal_logs` | Real in repo/runtime | Stores owner-scoped reusable meal templates plus explicit meal-log history separate from planner, workouts, and persisted recommendation storage |
| `apollo.sports` | Real | Stores APOLLO-owned sport definitions and static rule profiles for the current competition substrate line |
| `apollo.facility_catalog_refs`, `apollo.facility_zone_refs`, `apollo.sport_facility_capabilities`, and `apollo.sport_facility_capability_zones` | Real | Stores bounded facility identifier references plus APOLLO-owned facility-sport support mappings without duplicating ATHENA hours or metadata |
| `apollo.schedule_resources`, `apollo.schedule_resource_edges`, `apollo.schedule_blocks`, and `apollo.schedule_block_exceptions` | Real in repo/runtime | Stores APOLLO-owned scheduling substrate truth over zones, bookable resources, resource graphs, typed blocks, RFC3339-windowed calendar reads, block-timezone weekly recurrence, active+bookable inventory-claim gating, and explicit date exceptions |
| `apollo.booking_requests` | Real in repo/runtime | Stores staff-entered and public-submitted booking request truth with contact/purpose/scope/window fields, state/version truth, source/channel truth, staff attribution where applicable, conflict-aware availability at read time, and a nullable linked schedule block retained after approval and approved cancellation |
| `apollo.booking_request_idempotency_keys` | Real in repo/runtime | Stores hashed public idempotency keys, normalized payload hashes, and linked request IDs so duplicate public submits cannot create duplicate requests |
| `apollo.competition_sessions`, `apollo.competition_session_queue_members`, `apollo.competition_session_teams`, `apollo.competition_team_roster_members`, `apollo.competition_matches`, and `apollo.competition_match_side_slots` | Real | Stores APOLLO-local session-rooted queue, assignment, lifecycle, and container truth separate from downstream result, rating, and standing projections |
| `apollo.competition_staff_action_attributions` | Real in repo/runtime | Stores durable actor/session/role/capability/trusted-surface attribution for successful staff-sensitive competition mutations |
| `apollo.ares_*` tables | Schema authored | Historical match and rating writes are deferred; the current preview runtime reads explicit membership and profile state without mutating ARES tables |
| `apollo.recommendations` | Schema authored | Tracer 7 recommendation reads are derived at read time; persisted recommendation records remain deferred |
| `users.preferences` JSONB | Real schema, bounded runtime use | Stores `visibility_mode`, `availability_mode`, typed non-medical `coaching_profile` inputs, and typed non-clinical `nutrition_profile` inputs while durable planner and nutrition runtime truth stays relational |

## Current Phase 3B Line

Phase 3B.6 public request entrypoint is now real in repo/runtime on `main`,
but deployed truth stays separate and unchanged. It builds on the latest
closed Phase 3B.5 approved booking lifecycle and the APOLLO schedule
substrate: APOLLO owns request state, source/channel truth, availability
truth, neutral public intake, approval conflict checks, the linked schedule
reservation block, and approved cancellation of that linked block.

| Topic | Locked statement |
| --- | --- |
| booking ownership | APOLLO owns booking request persistence, public intake persistence/API truth, state/version truth, source/channel truth, staff HTTP APIs, availability decisions, approval-to-schedule-block writes, and approved cancellation of linked reservation blocks |
| auth shape | members have no booking capability; supervisors get `booking_read`; managers and owners get `booking_read` plus `booking_manage` |
| mutation proof | staff create and transition routes require trusted-surface headers; public submit is unauthenticated but bounded and idempotency-keyed; every staff transition after create requires `expected_version` |
| public intake shape | public options return active/bookable/public-labeled choices only; public submit accepts option ID, contact fields, organization, purpose, attendee count, and RFC3339 windows; the response is neutral and omits request UUIDs, conflicts, staff notes, actor/session IDs, trusted-surface data, schedule block IDs, and graph internals |
| approval shape | approval creates a one-off internal `reservation` / `hard_reserve` schedule block on the requested facility/zone/resource scope using APOLLO schedule/resource graph conflict truth |
| cancellation shape | cancelling an approved request reuses `POST /api/v1/booking/requests/{id}/cancel`, locks and validates the linked reservation, cancels it, and retains `schedule_block_id` for audit |
| deferred with this line | public availability calendar, instant booking, status portal, in-place approved editing, customer self-service, payments, quotes, invoices, deposits, Hestia/member booking UI, owner policy writes, admin role widening, gateway, HERMES, and deploy work |

## Technology Stack

| Layer | Technology | Status | Line | Notes |
| --- | --- | --- | --- | --- |
| Service runtime | Go 1.23 | Instituted | `v0.0.x` -> `v0.6.0` | The current executable slice is a Go service |
| HTTP router | chi | Instituted | `v0.1.x` -> `v0.6.0` | Current API surface is intentionally narrow and tracer-driven |
| CLI | Cobra | Instituted | `v0.2.x` -> `v0.6.0` | `serve` and `visit list` are real |
| Database driver | pgx | Instituted | `v0.0.x` -> `v0.6.0` | Used for runtime persistence |
| SQL generation | sqlc | Instituted | `v0.1.x` -> `v0.6.0` | Auth, session, profile, and visit queries are generated from checked-in SQL |
| Eventing | NATS | Instituted | `v0.2.x` -> `v0.6.0` | Consumes ATHENA identified arrival and departure events |
| Shared contract | `ashton-proto` generated packages + runtime helper | Instituted | `v0.2.x` -> `v0.6.0` | APOLLO no longer owns a private copy of the event wire format |
| Auth path | first-party student ID + email verification + signed session cookie | Real | `v0.1.x` | Tokens are stored hashed in Postgres and sessions are server-side rows referenced by a signed cookie |
| Workout runtime | relational workout model | Real | `v0.5.0` | Authenticated create, update, finish, read, and list behavior is active |
| Recommendation runtime | deterministic derived read over workouts | Real | `v0.6.0` | Authenticated `GET /api/v1/recommendations/workout` is active without persisting outputs |
| Minimal member web shell | embedded HTML/CSS/JS over existing APIs | Real | `v0.7.0` | Tracer 11 keeps the UI thin, leaves workout state transitions backend-authoritative, and now maps total bootstrap or refresh network failure into explicit recoverable error UI |
| Lobby membership runtime | explicit member-scoped membership state | Real | `v0.8.0` | Tracer 12 keeps membership server-authoritative, durable, and separate from eligibility and visits |
| ARES match preview runtime | deterministic read-only preview over explicit lobby membership | Real | `v0.9.0` | Tracer 13 keeps candidate selection explicit, excludes ineligible joined members, and stays read-only |
| ARES rating engine | preview-only historical groundwork | Deferred | later than `v0.13.0` | Tracer 22 keeps competition history in the competition runtime and leaves ARES read-only |
| Sport and facility-sport registry | sport catalog, facility-sport capability mapping, and basic sport rules/config for at least two sports | Shipped | `v0.10.0` | CLI-only substrate read over seeded registry tables; deployment truth and public surfaces remain deferred |
| Team / session substrate | session-rooted team, roster, session, and match container primitives | Shipped | `v0.11.0` | Tracer 20 settled the bounded competition container model before execution widening |
| Matchmaking lifecycle | queue, assignment, and session lifecycle truth | Shipped | `v0.12.0` | Tagged Tracer 21 release line adds authenticated internal HTTP execution truth without widening into results, rivalry, badges, or public reads |
| Results, ratings, and member stats | result capture, ratings, session-scoped standings, and member profile stats | Closure-clean on `main` | `v0.13.0` | Competition history is now authenticated internal HTTP/runtime truth while public/social reads and deployed truth remain deferred |
| Planner and exercise library | planner state, exercise library, templates / loadouts, and richer profile inputs | Real on `main` | `v0.14.0` | Tracer 23 keeps the line authenticated/internal, backend-first, and separate from workout history and recommendation logic |
| Deterministic fitness coaching | conservative deterministic coaching recommendation and structured plan-change proposal over planner/profile/workout truth | Tagged | `v0.15.0` | Tracer 24 is the tagged coaching line, and `v0.15.1` is the bounded hardening patch on that same line |
| Conservative nutrition substrate | typed nutrition profile inputs, owner-scoped meal template/log truth, and read-only calorie/macro recommendation ranges | Closure-clean on `main` | `v0.16.0` line | Keep it non-clinical, bounded, and separate from planner mutation or chatbot-first flows |
| Explanation and agent-facing helpers | explanation/summarization helpers plus bounded `why` and read-only variation previews over deterministic core logic | Closure-clean on `main` | `v0.17.0` | Preserve helper subordination to the deterministic domain core |
| Facility-scoped member presence | facility-scoped presence read, explicit tap-link rows, and facility streak state/events over visit truth | Closure-clean on `main` | `v0.18.0` | Keep presence explicit, facility-scoped, and separate from matchmaking, coaching, nutrition, and role/authz widening |
| Role/authz and staff boundary substrate | explicit principal roles, deterministic competition capabilities, trusted-surface-gated privileged staff mutations, and durable actor attribution | Closure-clean on `main` | `v0.19.0` | Keep authority explicit and reviewable without widening into role-management product flows, persistent approvals, or deployment claims |
| Ops read foundation | read-only facility overview over APOLLO schedule truth and ATHENA occupancy/analytics truth | Closure-clean on `main` | `Phase 3B.1` | Keep it internal, aggregate, supervisor/manager/owner-only, and separate from booking, staff shell UI, gateway work, and deployment claims |
| Request-first booking runtime | staff-entered and public-submitted booking requests with conflict-aware approval into APOLLO schedule reservations and approved cancellation of linked reservations | Closure-clean on `main` | `Phase 3B.6` | Keep it request-first and approval-first; public intake stays neutral and non-reserving while payments, quotes, instant booking, status portals, in-place approved editing, Hestia booking UI, owner policy writes, gateway work, HERMES, and deployment claims remain deferred |
| Frontend widening | broader shell, PWA, offline sync, and richer design-system work | Deferred | later than `v0.17.0` | Not part of Phase 2 |

## Current Ingest Path

| Step | Current Behavior |
| --- | --- |
| ATHENA publishes lifecycle events | Subjects are `athena.identified_presence.arrived` and `athena.identified_presence.departed` |
| APOLLO parses the payload | The shared `ashton-proto` helper is the only active contract path; schema-invalid payloads, including empty identity hashes, are rejected before any mutation |
| APOLLO resolves member identity | `claimed_tags` maps the ATHENA identity hash to an active user |
| APOLLO enforces idempotency | Duplicate arrival ids, duplicate departure ids, and already-open visits resolve deterministically |
| APOLLO records the lifecycle | Arrivals open visits, departures close matching open visits for the same member and facility |

This flow is intentionally narrower than the future product shape. It proves the
boundary from physical truth to member history first, then layers explicit
member-owned auth, intent, and workout runtime without letting visits imply
exercise, recommendations, or matchmaking.

## Known Caveats

| Area | Current caveat | Why it matters |
| --- | --- | --- |
| Verification delivery | The default runtime is still dev-first; verification is easy to test locally but not yet a full production-grade delivery path | APOLLO proves ownership and sessions, but not yet a polished end-user delivery experience |
| Claimed tags | `apollo.claimed_tags` is real schema and runtime dependency, but there is still no end-user flow to manage tag linkage | Visit ingest is narrower than the eventual member-account model |
| Product shell | The current line now has one narrow embedded member shell only and Phase 2 keeps it that way | Do not confuse one authenticated shell with a full product frontend, offline support, or broader design-system work |
| Calendar window contract | Staff and member schedule calendar reads are RFC3339-only on purpose | Later user-friendly date pickers or labels must still compile down to explicit RFC3339 window boundaries instead of reintroducing ambiguous date-only semantics |
| Ops overview config | The ops overview route needs ATHENA HTTP config to be useful | APOLLO owns the composition, but current occupancy and analytics are still upstream ATHENA truth; missing or failing ATHENA reads fail clearly instead of fabricating ops data |
| Admin role | APOLLO still has no distinct admin role | Owner currently stands in for the owner/admin CLI posture; any future admin parity must be an explicit authz widening, not an assumption |
| ARES and recommendation persistence | Runtime scope now includes a deterministic match preview read, but historical ARES writes and recommendation persistence are still deferred | Readers should not mistake the preview runtime for assignment, invitations, notifications, or stored coaching |

## Current State Block

### Already real in this repo

- `apollo serve` starts a real Go process with health reporting
- APOLLO can start a member verification flow from student ID + email
- verification tokens are generated, stored hashed, expired, invalidated after use, and can be surfaced in local development through explicit token logging
- successful verification marks the user email as verified and issues a signed `HTTPOnly`, `Secure`, `SameSite=Strict` session cookie
- authenticated profile reads and writes are real for `visibility_mode` and `availability_mode`
- authenticated `GET /api/v1/lobby/eligibility` is real and derives
  `eligible`, `reason`, `visibility_mode`, and `availability_mode` from stored
  member state only
- authenticated `GET /api/v1/lobby/membership` and
  `POST /api/v1/lobby/membership/join|leave` are real and keep lobby
  membership explicit, durable, and separate from eligibility or physical
  presence
- authenticated `GET /api/v1/presence` is real in repo/runtime and returns
  facility-scoped presence truth over explicit tap-linked visits only, with
  one `present` / `not_present` status per facility instead of a fake global
  current facility
- authenticated `GET /api/v1/lobby/match-preview` is real and returns a
  deterministic, explainable, read-only preview over explicit joined lobby
  membership only
- match preview reads exclude joined members who no longer meet current
  open-lobby eligibility and use stable user-id ordering plus stable tie-breaks
  to produce repeatable pairings
- authenticated `POST/GET/PUT /api/v1/workouts` and
  `POST /api/v1/workouts/{id}/finish` are real and keep workout state
  member-owned, explicit, and owner-scoped
- workout history lists newest created workouts first using DB-owned
  `started_at DESC, id DESC` ordering instead of mixed app-clock and DB-clock
  timestamps
- authenticated `GET /api/v1/recommendations/workout` is real and uses explicit
  precedence: `resume_in_progress_workout`, `start_first_workout`,
  `recovery_day` for workouts finished inside `24h`, then
  `repeat_last_finished_workout`
- authenticated `GET /api/v1/planner/exercises` and
  `GET /api/v1/planner/equipment` are real and return APOLLO-owned catalog
  truth for planner/template validation without widening into facility
  inventory or live availability
- authenticated `GET/POST/PUT /api/v1/planner/templates` are real and keep
  reusable loadouts owner-scoped with duplicate-name protection per member
- authenticated `GET/PUT /api/v1/planner/weeks/{week_start}` are real and keep
  week-rooted planner truth separate from workouts, recommendations, visits,
  membership, and competition history
- authenticated `PATCH /api/v1/profile` now supports bounded
  `coaching_profile.goal_key`, `days_per_week`, `session_minutes`,
  `experience_level`, and `preferred_equipment_keys` writes while preserving
  unrelated preference data
- authenticated `PATCH /api/v1/profile` now also supports bounded
  non-clinical `nutrition_profile` writes for `dietary_restrictions`,
  `meal_preference.cuisine_preferences`, `budget_preference`, and
  `cooking_capability`
- authenticated
  `PUT /api/v1/workouts/{id}/effort-feedback|recovery-feedback` writes are
  real, owner-scoped, and restricted to finished workouts with bounded enum
  payloads
- authenticated `GET /api/v1/recommendations/coaching?week_start=...` is real
  and returns deterministic Tracer 24 coaching recommendation output with a
  response-only structured plan-change proposal and structured explanation
  evidence over planner/profile/workout truth
- authenticated `GET/POST/PUT /api/v1/nutrition/meal-templates` is real and
  keeps reusable meal templates owner-scoped with duplicate-name protection per
  member
- authenticated `GET/POST/PUT /api/v1/nutrition/meal-logs` is real and keeps
  meal logging explicit, owner-scoped, deterministic, and reusable through
  template-backed entries without turning planner or workouts into nutrition
  state
- authenticated `GET /api/v1/recommendations/nutrition` is real in
  local/runtime and returns conservative calorie/macro range output with
  strategy flags plus structured non-clinical limitations over explicit
  profile/history inputs only
- `GET /`, `GET /app/login`, protected `GET /app`, and protected
  `GET /app/{home,workouts,meals,tournaments,settings}` are real and provide
  one routed embedded member shell over the already-real auth, profile,
  presence, planner, workout, nutrition, lobby, match-preview, competition
  member-stats, and recommendation APIs
- the member shell stays API-backed and backend-authoritative: it does not
  invent recommendation state, it does not optimistic-write workout or settings
  transitions, it does not bypass session ownership checks, and it does not
  consume staff schedule routes
- the member shell makes section boundaries explicit where member-safe backend
  truth does not yet exist, so schedule and booking stay out instead of being
  faked through staff surfaces
- the member shell now keeps one stable nav/frame posture across home,
  workouts, meals, tournaments, and settings while keeping failure and retry
  state scoped to the shell or section that actually failed
- the member shell continues to show one narrow lobby membership panel with
  explicit `Join` and `Leave` actions over the real membership APIs
- the member shell continues to show one narrow read-only match preview panel
  with grouped matches, reasons, unmatched members, and explicit failure state
  without
  adding action buttons
- total shell bootstrap or refresh network failure now replaces loading copy
  with explicit recoverable error states for profile, membership, match
  preview, workouts, and recommendation reads
- recommendation reads are deterministic, member-scoped, and side-effect free:
  they do not create, update, or finish workouts and they do not mutate visits,
  profile state, claimed tags, or eligibility state
- planner/template/profile writes do not instantiate workout drafts, finish
  workouts, widen recommendation inputs, infer from visits, or mutate lobby
  membership, claimed tags, or competition state
- nutrition profile writes, meal template writes, meal-log writes, and
  nutrition recommendation reads do not mutate planner weeks, coaching
  feedback, workouts, visits, membership, competition state, ARES rows, or
  persisted recommendation storage
- match preview reads are deterministic and side-effect free: they do not
  create matches, assignments, invites, sessions, or any other domain state
- only one `in_progress` workout is allowed per member at a time
- finished workouts are immutable through the current runtime surface
- logout revokes the current server-side session and clears the cookie
- APOLLO can consume `athena.identified_presence.arrived` and
  `athena.identified_presence.departed` from NATS
- the consumer uses the shared `ashton-proto` helper instead of a private event
  struct
- malformed payloads, wrong source values, wrong types, bad enums, and invalid
  timestamps are rejected clearly
- duplicate arrivals, duplicate departures, unknown tags, empty-hash contract
  violations, already-open visits, no-open departures, and out-of-order
  departures all resolve deterministically
- repeated arrival replay for an already linked visit stays deterministic:
  APOLLO does not create a second tap-link row or a second facility streak
  event for the same visit day
- `apollo visit list` reads back recorded visit history for a specific student
- `apollo sport list`, `apollo sport show`, and `apollo sport capability list`
  are real and keep sport identity, facility support, and static rules/config
  deterministic and read-only
- authenticated `GET/POST /api/v1/competition/sessions`, nested team/roster
  writes, match container writes, queue open/join/remove, deterministic
  assignment, start, archive, and result actions are real and keep competition
  session truth separate from auth sessions, lobby membership, and ARES preview
- authenticated session principals now carry one explicit APOLLO-global role,
  deterministic competition, schedule, and ops-read capability sets derived
  from that role, and optional trusted-surface context for privileged
  competition mutations
- competition session writes bind to sport registry and facility capability
  truth, but do not assume hours, closures, scheduling, or live availability
- competition staff reads now require explicit `competition_read` capability
  instead of owner-scoped filtering over `competition_sessions.owner_user_id`
- competition structure writes remain session-rooted and capability-scoped:
  manual container edits stay bounded to `draft` sessions and require
  `competition_structure_manage` plus trusted-surface proof
- competition queue state is session-rooted, APOLLO-owned, and versioned: it
  requires explicit joined lobby membership plus current eligibility, does not
  reuse lobby membership itself as final assignment truth, and privileged
  queue/lifecycle/result mutations require `competition_live_manage` plus
  trusted-surface proof
- competition assignment is deterministic and side-effect bounded: it seeds
  team, roster, and match container truth from the queue without mutating ARES
  preview, visits, workouts, recommendations, or profile state
- competition lifecycle transitions are explicit and bounded to
  `draft -> queue_open -> assigned -> in_progress -> archived`
- successful staff-sensitive competition mutations now write durable actor
  attribution rows carrying actor user/session/role, capability,
  trusted-surface key, action, and relevant competition target ids
- authenticated `GET /api/v1/ops/facilities/{facilityKey}/overview` is real in
  repo/runtime as a read-only supervisor/manager/owner route over explicit
  RFC3339 windows; it composes APOLLO schedule occurrences with ATHENA current
  occupancy plus bounded aggregate analytics and omits raw tap hashes,
  identity-level presence, booking writes, public entrypoints, and staff shell
  UI
- the bounded live cluster deployment now proves APOLLO can boot its schema,
  connect to in-cluster NATS, and persist the live ATHENA identified
  arrival/departure-close boundary into Postgres

### Real but intentionally narrow

- the active member-facing write surface is limited to auth, profile settings,
  explicit lobby membership, explicit workout history, and the thin web shell
  over those same APIs
- the active nutrition surface is authenticated/internal only: typed profile
  inputs, meal templates, meal logs, and read-only range recommendations
- open-lobby eligibility remains a derived read and does not auto-create
  membership
- explicit lobby membership remains intent only; it does not auto-open queues
  or auto-assign members into competition execution truth
- visit recording and visit closing remain separate from auth and profile state
- the live cluster proof is still only the visit-ingest boundary; it does not
  widen APOLLO into a broader product deployment
- deterministic recommendation reads remain real and side-effect free
- competition execution surfaces are authenticated internal HTTP only and stay
  role/capability-scoped, trusted-surface-gated for privileged mutations,
  local/runtime-only, and separate from the thin member shell
- ops overview is authenticated internal HTTP only, role/capability-scoped,
  read-only, ATHENA-dependent for occupancy truth, and local/runtime-only
- ARES preview stays read-only and side-effect free even though real queue,
  assignment, lifecycle, and competition-history runtime now exist elsewhere in
  APOLLO
- nutrition guidance stays non-clinical, read-only, and conservative: no meal
  plan apply path, no diagnosis, no helper-owned decision core, and no public
  or social nutrition surface land in Tracer 25
- recommendation persistence, generated plans, invites, notifications,
  rivalry/badge logic, public competition reads, and deployment widening remain
  deferred beyond Tracer 28

### Authored in schema, not yet active in runtime

- ARES rating and match tables
- persisted recommendation storage

### Deferred on purpose

- The planned release lines below are the authoritative widening path. These
  bullets are only the short boundary reminders.

- tying visit creation or visit closing to workout logging
- auto-starting a workout from arrival or auto-finishing a workout from
  departure
- inferring recommendations from visits, departures, or physical presence
- storing recommendation reads or widening into generated explanation layers
  before the planner and deterministic coaching lines are stable
- turning nutrition into a diet app, diagnostic engine, or meal-plan chatbot
  before the bounded explanation/helper line exists
- letting tap-in imply lobby or matchmaking intent
- adding invites or match formation before explicit lobby membership is stable
- widening the competition-history runtime into rivalry, badges, public
  leaderboards, or public competition reads before later tracers land
- adding the recommendation pipeline before workout data exists
- meaningful frontend widening before the Phase 2 ladder closes cleanly

## Release History

| Release line | Exact tags | Status | What became real | What stayed deferred |
| --- | --- | --- | --- | --- |
| `v0.0.x` | `v0.0.1` | Shipped | bootstrap baseline, first schema and service shape | auth, eligibility, workouts, and recommendations |
| `v0.1.x` | `v0.1.0`, `v0.1.1` | Shipped | auth and profile foundation line | explicit lobby, workouts, and recommendations |
| `v0.2.x` | `v0.2.0`, `v0.2.1` | Shipped | eligibility plus visit-ingest line | visit close, workouts, and recommendations |
| `v0.4.x` | `v0.4.0`, `v0.4.1` | Shipped | visit close plus bounded live deploy deepening | workout runtime, broader product deploy, and recommendations |
| `v0.5.0` | `v0.5.0` | Shipped | explicit workout runtime | recommendation persistence, generated planning, and matchmaking |
| `v0.6.0` | `v0.6.0` | Shipped | deterministic recommendation runtime | web shell, lobby membership, ARES, and generated planning |
| `v0.7.0` | `v0.7.0` | Shipped | minimal member web shell over existing APIs | deployment truth, lobby membership, ARES, and generated planning |
| `v0.8.0` | `v0.8.0` | Shipped | explicit lobby membership runtime | ARES preview, invites, and generated planning |
| `v0.9.0` | `v0.9.0` | Shipped | first deterministic ARES match preview over explicit lobby membership | assignment, invites, notifications, and live match execution |
| `v0.10.0` | `v0.10.0` | Shipped | sport registry, facility-sport capability mapping, and static sport rules/config | team/session containers, matchmaking, results, and standings |
| `v0.11.0` | `v0.11.0` | Shipped | session-rooted team, roster, session, and match container primitives over authenticated internal HTTP | queueing, assignment, results, ratings, standings, public competition reads, and deployment widening |
| `v0.12.0` | `v0.12.0` | Shipped | authenticated internal HTTP queue state, deterministic assignment, and explicit session lifecycle transitions | results, ratings, standings, rivalry/badge logic, public competition reads, and deployment widening |
| `v0.13.0` | `v0.13.0` | Shipped | result capture, ratings, session-scoped standings, and self-scoped member stats | public competition reads, broader ARES history, and deployment widening |
| `v0.14.0` | `v0.14.0` | Shipped | planner, exercise library, templates/loadouts, and richer profile inputs | coaching logic, nutrition, and meaningful frontend widening |
| `v0.15.0` | `v0.15.0` | Shipped | deterministic coaching substrate over planner/profile/workout truth | nutrition, explanation/helper widening, and deployment closeout |
| `v0.15.1` | `v0.15.1` | Shipped | bounded Tracer 24 hardening only | new product widening |
| `v0.16.0` | `v0.16.0` | Shipped | conservative nutrition substrate | diagnosis, obsessive nutrition sprawl, helper-first AI, and deployment widening |
| `v0.17.0` | `v0.17.0` | Shipped | explanation, bounded AI helpers, and thin agent-facing helper reads | helper persistence, model-backed calls, public social widening, and deployment widening |
| `v0.18.0` | `v0.18.0` | Shipped | facility-scoped member presence, explicit tap-link truth, and facility-scoped streak state/events | fake counters, cross-facility merging, role/authz widening, and deployment widening |
| `v0.19.0` | - | Closure-clean on `main` | explicit role/authz, trusted-surface-gated competition staff mutations, and durable actor attribution over the existing competition control boundary | role-management product flows, facility-scoped staffing, persistent approvals, ATHENA ingress storage, `ashton-proto` widening, and deployment widening |
| `v0.19.1` | `v0.19.1` | Shipped | Milestone 2.0 hardening follow-up for shutdown, bounded HTTP/NATS/request handling, shared-parser ingest discipline, workout safety, and docs truth | new product widening |
| `Phase 3B.1` | - | Closure-clean on `main` | read-only ops facility overview over APOLLO schedule truth plus ATHENA occupancy and bounded analytics truth with `ops_read` authz | booking writes, public booking, quotes/payments, staff shell UI, HERMES widening, gateway work, and deployment claims |
| `Phase 3B.4` | - | Closure-clean on `main` | internal staff-entered booking request persistence, `booking_read` / `booking_manage`, request state/version truth, trusted-surface mutations, conflict-aware availability, and approval-created linked schedule reservations | public booking, customer self-service, payments, quotes, invoices/deposits, Hestia booking UI, owner policy writes, admin role widening, HERMES/gateway/deploy work |
| `Phase 3B.5` | - | Closure-clean on `main` | approved internal booking cancellation with atomic linked reservation cancellation and retained schedule linkage | public booking, customer self-service, payments, quotes, invoices/deposits, Hestia booking UI, in-place approved editing, owner policy writes, admin role widening, HERMES/gateway/deploy work |
| `Phase 3B.6` | - | Runtime-local on `main` | APOLLO-owned public request intake API with public-safe options, idempotent neutral request submit, public source/channel truth, and no schedule-block creation until staff approval | public availability calendar, instant booking, status portal, customer self-service, payments, quotes, invoices/deposits, Hestia booking UI, owner policy writes, admin role widening, HERMES/gateway/deploy work |

## Release Lines

Tracer 24 remains the tagged coaching line on `v0.15.0`, and `v0.15.1`
remains the narrow hardening patch on that same line. The current
repo/runtime closeout truth on `main` is Tracer 28 authz/staff-boundary truth
plus the Milestone 2.0 hardening follow-up closed on `v0.19.1`. Later planned
lines begin below.

| Release line | Intended purpose | Restrictions | What it should not do yet |
| --- | --- | --- | --- |
| historical `v0.6.1` note | Milestone 1.6 companion patch if repo-local APOLLO truth ever needed backfilled closeout | treat this as historical closure context, not the active next line | do not present this as the active planned release line |
| `v0.14.0` | planner, exercise library, templates / loadouts, and richer profile inputs | keep the line backend/CLI-first and bounded | do not widen into meaningful frontend work, workout instantiation, or recommendation logic |
| `v0.15.0` | deterministic coaching substrate over planner, profile, and workout history | build on stable workout and planner foundations | do not let visits, departures, or profile state silently drive opaque coaching logic |
| `v0.16.0` | conservative nutrition substrate with meal logging and calorie / macro ranges | keep it non-clinical and conservative | do not turn the product into a diet app or diagnostic engine |
| `v0.17.0` | explanation, summarization, bounded AI helper flows, and thin agent-facing helper surfaces | keep them subordinate to stable deterministic logic | do not let explanation become the core engine |
| `v0.18.0` | member presence, tap-link, and streak substrate over explicit visit truth | keep presence explicit and auditable | do not invent fake streak counters or silent visit inference |
| `v0.19.0` | role/authz, actor attribution, trusted-surface primitives, and staff runtime boundary substrate | keep authority explicit and reviewable | do not widen into polished ops product or speculative contracts |
| `v0.19.1` | Milestone 2.0 hardening follow-up for runtime boundaries, workout safety, and docs truth | keep the line patch-only and non-widening | do not add new member/staff product capability or deploy claims |
| later than `Phase 3B.1` | `Phase 3B.6 public request entrypoint` on `main`: staff-entered and public-submitted booking requests over APOLLO schedule truth with public-safe options, idempotent neutral public submit, conflict-aware staff approval, and approved cancellation of linked reservations | keep the surface request-first, supervisor read-only, manager/owner managed, trusted-surface gated for staff mutations, idempotency-keyed for public submit, versioned, and APOLLO-authoritative for availability, approval conflicts, and linked reservation cancellation | do not widen into instant booking, customer status portals, quotes/payments, in-place approved editing, Hestia/member booking UI, owner policy writes, admin role widening, HERMES, gateway, or deploy claims |

## Versioning Discipline

APOLLO now follows formal pre-`1.0.0` semantic versioning.

- `PATCH` releases cover hardening, docs sync, deployment closeout,
  observability, and bounded non-widening fixes
- `MINOR` releases cover new bounded member capabilities or intentional
  contract changes
- pre-`1.0.0` breaking changes still require a `MINOR`, never a `PATCH`

## Project Structure

| Path | Purpose |
| --- | --- |
| `cmd/apollo/` | CLI entrypoint and serve command |
| `internal/auth/` | verification token lifecycle, server-side sessions, and signed cookie handling |
| `internal/authz/` | explicit role, capability, and trusted-surface helpers for bounded competition, schedule, and ops-read staff authz |
| `internal/athena/` | strict APOLLO-side ATHENA HTTP reader for current occupancy and bounded analytics |
| `internal/eligibility/` | derived open-lobby eligibility from authenticated member state |
| `internal/ares/` | deterministic read-only match preview service and repository over explicit joined lobby membership |
| `internal/membership/` | explicit lobby membership repository and service over durable member intent |
| `internal/consumer/` | NATS consumer and strict event parsing |
| `internal/profile/` | authenticated profile state read and update over `users.preferences` |
| `internal/planner/` | owner-scoped exercise library, templates/loadouts, and week-rooted planner repository/service |
| `internal/coaching/` | deterministic coaching recommendation and feedback repository/service over planner, profile, and workout truth |
| `internal/nutrition/` | owner-scoped meal-template, meal-log, and conservative nutrition recommendation repository/service |
| `internal/visits/` | visit service and repository boundary |
| `internal/workouts/` | workout repository and service for explicit member-owned workout history |
| `internal/recommendations/` | deterministic workout recommendation service and repository |
| `internal/competition/` | session-rooted competition queue, assignment, lifecycle, result, container, and actor-attribution repository/service over sport/facility truth plus explicit role/capability authz |
| `internal/ops/` | read-only facility overview composition over APOLLO schedule truth and ATHENA occupancy/analytics truth |
| `internal/server/web/` | embedded member-shell HTML, CSS, JS, and browser-side tests for the thin APOLLO web shell |
| `internal/store/` | sqlc-generated models and query bindings |
| `internal/sports/` | sport registry and facility-sport capability repository and service |
| `internal/server/` | health, auth, competition, profile, planner, coaching, nutrition, membership, match preview, workout, recommendation, session middleware, and embedded shell wiring |
| `db/migrations/` | current schema for users, auth/session state, visits, lobby membership, workouts, planner, coaching feedback, nutrition runtime, sport substrate, competition execution/history runtime, ARES, and recommendations |
| `db/queries/` | checked-in SQL for auth, profile, planner, coaching feedback, nutrition runtime, competition execution/history, visit, lobby membership, sport substrate, and match preview operations |
| `docs/` | roadmap, ADRs, runbook, growing pains, and diagrams |

## Deployment Boundary

APOLLO owns its runtime, schema, and consumer logic. Infrastructure, GitOps,
and cluster policy still live outside this repo in the Prometheus/Talos layer.
Milestone 1.6 already proved one bounded live APOLLO departure-close path; the
broader auth, eligibility, membership, workout, recommendation, member-shell,
competition execution, and competition-history surfaces remain repo-local/runtime
truth unless a future deployment workstream proves them live. This README is
documenting APOLLO's internal system logic and product boundary, not the
homelab substrate.

## Docs Map

- [Docs index](docs/README.md)
- [Glossary](docs/glossary.md)
- [APOLLO diagram](docs/diagrams/apollo-visit-ingest.mmd)
- [Roadmap](docs/roadmap.md)
- [Growing pains](docs/growing-pains.md)
- [Hardening artifacts](docs/hardening/README.md)
- [Member state runbook](docs/runbooks/member-state.md)
- [ADR 001: member state model](docs/adr/001-member-state-model.md)
- [ADR 002: member auth](docs/adr/002-member-auth.md)
- [ADR index](docs/adr/README.md)

## Why APOLLO Matters

APOLLO is where the platform starts to look like a product instead of only an
operations system. Even in its current narrow form, it already shows contract
discipline, first-party auth taste, deterministic failure handling, relational
schema design, event-driven ingestion, and a strong boundary between presence,
profile state, workout history, recommendation logic, and matchmaking intent.
The current tracer now also proves one bounded competition authority/runtime
line: explicit role/capability-bound competition staff reads, trusted-surface-
gated privileged mutations, durable actor attribution, deterministic
assignment/lifecycle truth, immutable result capture, sport-and-mode-scoped
ratings, session-scoped standings, and self-scoped member stats over
authenticated internal HTTP. The thin member shell remains narrow and
separate; public competition truth, social widening, role-management product
flows, and deployment widening are still deferred.
