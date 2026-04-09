# Tracer 19 Hardening

## Audited Claim

Tracer 19 claims:

- APOLLO owns sport definitions for badminton and basketball
- APOLLO owns facility-sport capability mappings for those sports
- APOLLO owns static sport rules/config strong enough for later validation
- the first truthful surface is CLI-only and deterministic
- ARES, team/session runtime, matchmaking, results, ratings, standings, and
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

## Verified Sport Substrate Smoke

Run the CLI against a migrated APOLLO database:

```bash
apollo sport list --format text
apollo sport show --sport-key badminton --format json
apollo sport capability list --facility-key ashtonbee --format text
```

Minimum trustworthy outcomes:

- `sport list` returns badminton then basketball in stable key order
- `sport show` returns one sport plus its facility capability rows
- `sport capability list` returns only APOLLO-owned mapping truth and does not
  imply hours, closures, scheduling, or live availability
- repeated reads stay stable while the database contents are unchanged

## Expected Destructive Failures Vs Real Regressions

Expected boundary results during hardening:

- duplicate `sport_key` inserts fail
- invalid participant ranges fail
- capability rows for unknown sports or unknown facilities fail
- zone mappings outside the allowed facility scope fail

Real regressions would have been:

- reusing ARES tables or preview code for sport registry truth
- widening the first surface into HTTP, shell, or public product reads
- introducing team/session containers, matchmaking, results, ratings, or
  standings
- letting facility hours, closures, or occupancy masquerade as capability or
  availability truth

## Verified Truth

- APOLLO now has dedicated sport substrate tables and sqlc bindings
- CLI reads are deterministic for sport list/show and facility capability list
- negative integrity checks reject invalid sport/config/mapping writes
- existing ARES preview and member-facing runtime wiring remain separate

## Unverified Truth

- no live deployment proof was added for sport substrate reads
- no internal HTTP sport surface was added
- no team/session, matchmaking, results, ratings, or standings runtime was
  proven

## Carry-Forward Gaps

Intentional deferred items remain:

- team, roster, session, and match containers
- matchmaking queue, assignment, and lifecycle
- results, ratings, standings, and member stats
- public or frontend competition surfaces

## Final Verdict

Tracer 19 sport substrate is shipped, bounded, and backend-first with
deployment proof still deferred.
