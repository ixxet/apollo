# Tracer 21 Hardening

## Audited Claim

Tracer 21 claims:

- APOLLO now owns session-rooted competition queue truth
- APOLLO now owns deterministic assignment from queued eligible members into
  session/team/roster/match container truth
- APOLLO now owns explicit session lifecycle transitions beyond
  `draft`-only containers
- lobby membership remains explicit member intent, not final assignment truth
- ARES preview remains read-only and separate from execution truth
- results, ratings, standings, rivalry/badge logic, public competition reads,
  and deployment truth remain unchanged

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

- `POST /api/v1/competition/sessions/{id}/queue/open` transitions one owner
  session from `draft` to `queue_open`
- `POST /api/v1/competition/sessions/{id}/queue/members` only admits members
  who are both explicitly joined in the lobby and currently eligible
- `POST /api/v1/competition/sessions/{id}/assignment` requires the expected
  queue version and deterministically seeds teams, roster rows, and one
  `assigned` match
- `POST /api/v1/competition/sessions/{id}/start` transitions one assigned
  session into `in_progress`
- repeated `GET /api/v1/competition/sessions/{id}` reads remain deterministic
  after queue, assignment, and lifecycle changes while inputs are unchanged

## Expected Destructive Failures Vs Real Regressions

Expected boundary results during hardening:

- duplicate queue join fails
- queue capacity overflow fails
- stale queue version at assignment fails
- assignment with an ineligible or no-longer-joined member fails
- replay assignment fails once execution truth is already seeded
- invalid lifecycle transitions fail
- wrong-owner reads or writes fail

Real regressions would have been:

- reusing `apollo.lobby_memberships` itself as final queue or assignment truth
- letting ARES preview mutate real execution state
- allowing manual team/roster writes after non-`draft` lifecycle transitions
- widening into results, ratings, standings, rivalry/badge logic, or public
  competition reads
- mutating visits, workouts, recommendations, profile state, or membership
  state as an implicit side effect of competition execution flows

## Verified Truth

- APOLLO now has dedicated queue storage plus session-owned queue versioning
- assignment is deterministic and materializes through existing Tracer 20
  competition container tables instead of inventing a second execution model
- lifecycle transitions are explicit, bounded, and integration-tested through
  authenticated internal HTTP
- existing membership and ARES preview integrity checks still hold
- `ashton-proto` remained untouched because no shared execution contract was
  required

## Unverified Truth

- no live deployment proof was added for competition execution runtime
- no result capture, ratings, standings, or member stats runtime was added
- no public or frontend competition surface was added
- no tag was created by this hardening pass

## Carry-Forward Gaps

Intentional deferred items remain:

- result capture and history
- ratings, standings, and member stats
- rivalry, badges, rematch prompts, and public competition reads
- deployment widening for competition execution runtime

## Final Verdict

Tracer 21 execution runtime is bounded, deterministic, backend-first, and
release-real at tagged line `v0.12.0`; deployed truth and results/history
truth remain deferred.
