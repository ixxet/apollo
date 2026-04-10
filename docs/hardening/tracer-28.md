# Tracer 28 Hardening

Tracer 28 claims:

- APOLLO session principals now carry one explicit global role:
  `member`, `supervisor`, `manager`, or `owner`, plus one deterministic
  capability set derived from that role.
- competition staff reads now require explicit `competition_read` capability
  instead of inferring authority from `competition_sessions.owner_user_id`.
- privileged competition mutations now require both the declared capability and
  a trusted-surface header/token proof; session auth alone is no longer enough.
- successful staff-sensitive competition mutations now write one durable
  `competition_staff_action_attributions` row carrying actor user/session/role,
  capability, trusted-surface key, action, and relevant competition target ids.
- existing competition provenance columns such as `owner_user_id` and
  `recorded_by_user_id` still exist as domain truth, but they are no longer the
  sole authorization key.
- Tracer 28 does not claim role management product flows, facility-scoped
  staffing, ATHENA ingress storage changes, gateway approvals, frontend staff
  widening, or `ashton-proto` contract changes.

## Proof

Run from `/Users/zizo/Personal-Projects/ASHTON/apollo`:

```sh
go test ./...
go test -count=5 ./internal/...
go vet ./...
go build ./cmd/apollo
git diff --check
```

Focused runtime and integrity coverage now proves:

- authenticated principals are deterministic: role and capability sets derive
  from APOLLO auth/session truth instead of per-handler inference
- `GET /api/v1/competition/sessions` and
  `GET /api/v1/competition/sessions/{id}` now honor explicit read capability
  across staff roles, while `GET /api/v1/competition/member-stats` stays
  self-scoped member truth
- `POST /api/v1/competition/...` staff mutations require both capability and
  trusted-surface proof, and missing or invalid trusted-surface headers fail
  cleanly
- supervisors can perform bounded live-manage flows, but cannot create session
  structure; managers and owners retain the full current competition capability
  set
- members cannot escalate into staff competition writes even if they present a
  trusted-surface token
- successful session, queue, roster, team, match, and result mutations now
  record actor attribution rows inside the same APOLLO write line
- existing queue-version and state-transition guards remain authoritative for
  stale assignment, replay, and lifecycle mutation safety

## Negative Proof

Tracer 28 does not claim:

- facility-scoped staffing or multi-role assignment models
- role management CLI, UI, or public admin product
- planner, coaching, nutrition, workouts, visits, presence, gateway, or ATHENA
  write widening
- persistent approval objects or freeform apply engines
- shared auth/role contracts in `ashton-proto`
- deployment closeout truth

## Deferred

- facility-scoped staff assignments and broader org/runtime authority models
- owner-only analytics or broader private member-plan reads
- staff runtime outside the current competition control boundary
- public competition reads and member self-service queue UX
- gateway approvals and broader routed-write governance
- planner/coaching/nutrition persistent approval objects
- ATHENA Postgres-backed ingress storage
