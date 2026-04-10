# Milestone 2.0 Hardening

Milestone 2.0 does not widen APOLLO's product surface. It hardens the existing
runtime and keeps docs aligned with the real boundary.

## Scope

- graceful HTTP shutdown for `apollo serve`
- bounded HTTP server timeouts
- bounded JSON request-body decoding
- bounded NATS handler contexts and connection drain on shutdown
- shared-parser-only identified lifecycle contract path
- batched workout exercise list reads instead of per-workout fan-out
- conservative per-workout exercise-count cap

## Proof

Run from `/Users/zizo/Personal-Projects/ASHTON/apollo`:

```sh
go test ./...
go test -count=5 ./internal/...
go test -race ./internal/...
go vet ./...
go build ./cmd/apollo
git diff --check
```

Focused destructive coverage now includes:

- serve-command shutdown exits cleanly when the command context is canceled
- oversized workout request bodies fail before the workout service runs
- malformed and unknown-field workout payloads fail cleanly
- identified arrival/departure payloads stay on the shared `ashton-proto`
  contract path, including empty-hash contract rejection
- duplicate arrivals, duplicate departures, and out-of-order visit replay stay
  deterministic
- workout list reads batch exercise loading and reject oversized exercise sets

## Negative Proof

Milestone 2.0 does not claim:

- a new APOLLO feature line
- session/auth model widening
- planner, nutrition, coaching, visits, or presence surface expansion
- gateway approvals, ATHENA storage changes, or deployment closeout truth

## Truth Split

- local/runtime truth: APOLLO runtime boundaries and workout safety are harder
  and better bounded on the `v0.19.1` patch line
- deployed truth: unchanged
- deferred truth: broader staff product, persistent approvals, ATHENA ingress
  storage, and frontend widening
