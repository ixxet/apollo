# Tracer 20 Hardening

## Audited Claim

Tracer 20 claims:

- APOLLO now owns session-rooted competition container truth
- authenticated internal HTTP session containers are real
- session-scoped team containers are real
- session-scoped roster membership rows are real
- session-scoped match containers are real
- sport/facility binding now gates container creation through existing Tracer 19
  sport truth and bounded facility refs
- auth sessions, lobby membership, and ARES preview remain separate
- queueing, assignment, results, ratings, standings, social surfaces, and
  deployment truth remain unchanged

## Exact Rerun Commands

```bash
cd /Users/zizo/Personal-Projects/ASHTON/apollo

go test ./...
go test -count=5 ./internal/...
go vet ./...
go build ./cmd/apollo
git diff --check
```

## Verified Competition Container Proof

Minimum trustworthy HTTP outcomes:

- `POST /api/v1/competition/sessions` creates one owner-scoped session bound to
  one sport, facility, optional zone, and `participants_per_side`
- `POST /api/v1/competition/sessions/{id}/teams` creates one session-scoped
  team side container
- `POST /api/v1/competition/sessions/{id}/teams/{teamID}/members` adds one
  roster membership row without touching lobby membership, profile state, or
  ARES preview
- `POST /api/v1/competition/sessions/{id}/matches` creates one session-scoped
  match container with ordered side slots only
- repeated `GET /api/v1/competition/sessions/{id}` reads remain deterministic
  while inputs are unchanged

## Expected Destructive Failures Vs Real Regressions

Expected boundary results during hardening:

- duplicate session creation for the same owner/sport/display name fails
- duplicate team `side_index` in one session fails
- roster conflicts inside one session fail
- team-size mismatch at match creation fails
- invalid sport/facility/zone bindings fail
- wrong-owner reads or writes fail
- archiving a session with draft matches fails
- mutating a matched team roster fails

Real regressions would have been:

- reusing `apollo.sessions` for competition sessions
- reusing `apollo.lobby_memberships` as roster truth
- reusing `internal/ares` or `apollo.ares_*` as real match/session truth
- widening into queueing, assignment, results, ratings, standings, or public
  competition reads
- mutating profile, membership, visits, workouts, recommendations, or ARES
  state from competition container operations

## Verified Truth

- APOLLO now has dedicated competition container tables and sqlc bindings
- owner-scoped internal HTTP competition session/team/roster/match writes are
  deterministic and repeatable
- negative integrity checks reject invalid bindings, roster conflicts, invalid
  transitions, and ownership failures cleanly
- existing lobby membership and ARES preview integrity checks still hold

## Unverified Truth

- no live deployment proof was added for competition container runtime
- no queueing, assignment, lifecycle execution, results, ratings, or standings
  runtime was added
- no public or frontend competition surface was added

## Carry-Forward Gaps

Intentional deferred items remain:

- queueing, assignment, and session lifecycle orchestration
- results, ratings, standings, and member stats
- public team identity, social prompts, rivalry, and badges
- deployment widening for competition runtime

## Final Verdict

Tracer 20 competition containers are bounded, backend-first, and local/runtime
only with deployment truth still deferred.
