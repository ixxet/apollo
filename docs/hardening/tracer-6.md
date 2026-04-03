# Tracer 6 Hardening

This artifact is backfilled from the original hardening audit and the
closure-hardening reruns that resolved the ordering blocker.

## Audited Claim

Tracer 6 claims:

- authenticated workout create, list, detail, update, and finish are real
- ownership and state transitions are deterministic
- workout history ordering is newest creation first using
  `started_at DESC, id DESC`
- workout runtime stays separate from visits, profile state, eligibility, and
  claimed tags

## Exact Rerun Commands

```bash
cd /Users/zizo/Personal-Projects/ASHTON/apollo/db && sqlc generate
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./internal/workouts -run '^(TestRepositoryListWorkoutsOrdersByNewestStartedAtDespiteFinishedTimestampSkew|TestRepositoryListWorkoutsUsesStableTieBreakerWhenStartedAtMatches|TestRepositoryFinishWorkoutTransitionsOnceAndLeavesFinishedRowsReadOnly)$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=5 ./internal/workouts -run '^(TestRepositoryCreateWorkoutRejectsSecondInProgressWorkoutForSameUser|TestRepositoryFinishWorkoutTransitionsOnceAndLeavesFinishedRowsReadOnly|TestRepositoryListWorkoutsOrdersByNewestStartedAtDespiteFinishedTimestampSkew|TestRepositoryListWorkoutsUsesStableTieBreakerWhenStartedAtMatches|TestUpdateWorkoutValidatesExercisePayloadWithTableDrivenCoverage|TestFinishWorkoutRejectsMissingAndFinishedStatesAndUsesClock)$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=10 ./internal/server -run '^TestWorkoutRuntimeListsNewestWorkoutFirst$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./internal/server -run '^(TestWorkoutEndpointsEmitLifecycleLogsOnSuccessPaths|TestWorkoutEndpointsReturnStableWorkoutShapes|TestWorkoutEndpointsMapErrorsClearly|TestWorkoutEndpointsRejectMalformedBodiesBeforeCallingTheService)$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./...
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=2 ./...
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=5 ./internal/server -run '^(TestWorkoutRuntimeListsNewestWorkoutFirst|TestWorkoutEndpointsEmitLifecycleLogsOnSuccessPaths|TestWorkoutRuntimeRoundTripThroughAuthenticatedSession|TestWorkoutRuntimeEndpointsStaySideEffectFreeAcrossVisitsEligibilityAndClaimedTags)$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=5 ./internal/consumer -run '^TestIdentifiedPresenceLifecycleDoesNotFinishExistingInProgressWorkout$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go build ./cmd/apollo
```

## Expected Destructive Failures Vs Real Regressions

Expected negative results during hardening:

- unauthenticated create, read, update, and finish are rejected
- a second `in_progress` workout returns `409`
- finishing a nonexistent workout returns `404`
- finishing twice returns `409`
- mutating after finish returns `409`
- malformed or nonsensical payloads return `400`

Real regressions would have been:

- mixed app-clock and DB-clock ordering still deciding list order
- workout operations mutating visits, eligibility, preferences, or claimed tags
- visit lifecycle finishing or mutating workouts
- the real `apollo serve` path omitting workout runtime wiring

## Verified Truth

- one `in_progress` workout per member is enforced
- finished workouts are immutable
- history ordering is DB-owned and deterministic:
  `started_at DESC, id DESC`
- success-path lifecycle logs exist for create, update, and finish
- local auth/session smoke proved create, update, finish, readback, and no
  visit/profile/tag side effects

## Unverified Truth

- no live in-cluster workout runtime proof was added
- no broader workout modeling beyond the first free-text exercise slice was
  proven

## Carry-Forward Gaps

- live in-cluster workout runtime remains deferred
- richer workout modeling and exercise taxonomy remain deferred

## Final Verdict

Tracer 6 closure blockers were resolved and the tracer closed cleanly on the
local-runtime claim.
