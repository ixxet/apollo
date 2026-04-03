# Tracer 5 Hardening

## Audited Claim

Tracer 5 claims:

- ATHENA departure truth can close the correct APOLLO open visit
- duplicate, no-open, unknown-tag, anonymous, and out-of-order departure
  behavior is deterministic
- visit close stays separate from workouts, profile state, eligibility, and
  claimed tags

## Exact Rerun Commands

```bash
docker run --rm -v /Users/zizo/Personal-Projects/ASHTON/ashton-proto:/workspace -w /workspace bufbuild/buf lint
docker run --rm -v /Users/zizo/Personal-Projects/ASHTON/ashton-proto:/workspace -w /workspace bufbuild/buf generate
cd /Users/zizo/Personal-Projects/ASHTON/ashton-proto && go test ./...
cd /Users/zizo/Personal-Projects/ASHTON/ashton-proto && go test -count=5 ./events ./tests/...

cd /Users/zizo/Personal-Projects/ASHTON/athena && go test ./...
cd /Users/zizo/Personal-Projects/ASHTON/athena && go test -count=5 ./internal/publish ./cmd/athena
cd /Users/zizo/Personal-Projects/ASHTON/athena && go build ./cmd/athena

cd /Users/zizo/Personal-Projects/ASHTON/apollo/db && sqlc generate
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./internal/visits ./internal/consumer
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=5 ./internal/visits ./internal/consumer
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./...
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go build ./cmd/apollo
```

## Expected Destructive Failures Vs Real Regressions

Expected negative or defensive outcomes during hardening:

- departure with no matching open visit is non-mutating
- duplicate departure replay resolves as `duplicate`
- out-of-order departure is non-mutating
- unknown-tag departure is non-mutating
- anonymous departure is explicitly ignored

Real regressions would have been:

- arrival behavior breaking after departure support landed
- departure creating or finishing workouts
- departure mutating visibility, availability, eligibility, or claimed tags
- departure closing the wrong facility visit

## Verified Truth

- shared departure contract is real across `ashton-proto`, `athena`, and
  `apollo`
- APOLLO closes by exact member plus facility match
- `departure_source_event_id` persists close idempotency
- local real NATS + Postgres smoke proved:
  - arrival opened a visit
  - departure closed that visit
  - replay stayed non-mutating
  - workouts remained `0`

## Unverified Truth

- no live in-cluster `ATHENA -> NATS -> APOLLO` proof existed at Tracer 5 close
- no APOLLO cluster deployment claim existed at Tracer 5 close

## Carry-Forward Gaps

- the Tracer 5 migration was proven on fresh bootstrap, but not independently
  rehearsed as an in-place upgrade from a pre-Tracer-5 database with existing
  open visits
- live in-cluster boundary proof was deferred and later closed in Milestone 1.5

## Final Verdict

Tracer 5 is closure-clean with one non-blocking carry-forward gap.
