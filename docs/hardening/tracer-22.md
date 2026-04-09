# Tracer 22 Hardening

## Audited Claim

Tracer 22 claims:

- APOLLO now owns immutable competition match result truth
- APOLLO now owns a real `completed` state distinct from `archived`
- APOLLO now owns sport-and-mode-scoped member rating projections derived from
  trusted recorded competition results
- APOLLO now owns session-scoped standings derived from trusted result truth
- APOLLO now owns self-scoped member competition stats derived from trusted
  result truth plus current rating projections
- all Tracer 22 write and read surfaces remain authenticated/internal only
- ARES remains preview-only and is no longer an ambiguous second home for
  competition history
- public competition reads, rivalry/badge logic, and deployment truth remain
  unchanged

## Exact Rerun Commands

```bash
cd /Users/zizo/Personal-Projects/ASHTON/apollo

go test ./...
go test -count=5 ./internal/...
go vet ./...
go build ./cmd/apollo
git diff --check
```

## Verified Execution Proof

Minimum trustworthy HTTP outcomes:

- `POST /api/v1/competition/sessions/{id}/matches/{matchID}/result` records one
  immutable result against one real in-progress match under owner-scoped
  authenticated internal HTTP
- result recording transitions one match from `in_progress` to `completed`
  without overloading `archived`
- the parent session transitions to `completed` only after every active match in
  the session has official result truth
- repeated `GET /api/v1/competition/sessions/{id}` reads remain deterministic
  after result capture and include result plus session-scoped standings truth
- repeated `GET /api/v1/competition/member-stats` reads remain deterministic and
  stay self-scoped
- ratings stay partitioned by `sport_key` plus derived
  `competition_mode:s<sides_per_match>-p<participants_per_side>` mode keys

## Expected Destructive Failures Vs Real Regressions

Expected boundary results during hardening:

- duplicate result application fails
- result writes before `in_progress` fail
- result writes after `completed` or `archived` fail
- wrong-owner result writes fail
- invalid side counts, bad side indexes, bad team bindings, and garbage outcome
  values fail
- per-sport and per-mode rating separation stays intact

Real regressions would have been:

- treating `archived` as official result/completion truth
- letting `apollo.ares_*` become a second competition-history home
- letting standings or member stats drift away from trusted recorded results
- mutating visits, workouts, recommendations, lobby membership, profile
  preference state, or ARES preview data as a side effect of result capture
- widening into public leaderboards, public homepage standings, rivalry logic,
  badges, or any broad social surface

## Verified Truth

- APOLLO now has dedicated immutable result storage tied directly to real
  competition matches and match side slots
- result capture is wired through the existing competition runtime instead of
  inventing a second execution/history model
- `completed` is now a real match/session state distinct from `archived`
- ratings are recomputed deterministically from trusted recorded results and are
  separated by sport and derived mode key
- standings are session-scoped only and remain derived from trusted result truth
- member stats are self-scoped derived reads and do not widen the profile domain
- existing ARES preview and Tracer 21 execution integrity checks still hold
- `ashton-proto` remained untouched because no shared competition-history
  contract was required

## Unverified Truth

- no live deployment proof was added for competition-history runtime
- no public competition read surface was added
- no rivalry, badge, achievement, or rematch-prompt logic was added

## Carry-Forward Gaps

Intentional deferred items remain:

- public leaderboards and public homepage standings
- season, ladder, league, or tournament containers beyond session scope
- box scores, set-by-set scoring, and result correction workflows
- rivalry, streak, badge, and rematch logic
- deployment widening for competition-history runtime

## Final Verdict

Tracer 22 competition-history runtime is bounded, deterministic, backend-first,
and closure-clean in repo/runtime on `main` for the `v0.13.0` line; deployed
truth, public competition reads, and social-layer widening remain deferred.
