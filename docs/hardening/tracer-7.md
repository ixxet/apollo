# Tracer 7 Hardening

## Audited Claim

Tracer 7 claims:

- authenticated `GET /api/v1/recommendations/workout` is real
- precedence is deterministic:
  - `resume_in_progress_workout`
  - `start_first_workout`
  - `recovery_day` within `24h`
  - `repeat_last_finished_workout`
- recommendation reads are side-effect free
- recommendation logic is derived from explicit workout data only

## Exact Rerun Commands

```bash
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./...
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=2 ./...
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=5 ./internal/recommendations
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=10 ./internal/server -run '^(TestWorkoutRecommendationRuntimeFollowsDeterministicPrecedence|TestWorkoutRecommendationEndpointRequiresAuthentication|TestWorkoutRecommendationEndpointReturnsTheStableRecommendationShape|TestWorkoutRecommendationEndpointMapsLookupFailuresClearly|TestAuthAndProfileEndpointsRejectTokenAndSessionEdgeCases)$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=5 ./internal/server -run '^(TestWorkoutRuntimeRoundTripThroughAuthenticatedSession|TestWorkoutRuntimeEndpointsStaySideEffectFreeAcrossVisitsEligibilityAndClaimedTags)$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test -count=5 ./internal/consumer -run '^TestIdentifiedPresenceLifecycleDoesNotFinishExistingInProgressWorkout$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go build ./cmd/apollo
```

## Expected Destructive Failures Vs Real Regressions

Expected negative or boundary results during hardening:

- unauthenticated recommendation reads return `401`
- cold-start, in-progress, fresh-finished, and aged-finished cases each map to
  a documented recommendation type
- sequential smoke is the product proof; earlier parallel operator races do not
  count as product failures

Real regressions would have been:

- recommendation reads creating, updating, or finishing workouts
- visits, preferences, claimed tags, or eligibility mutating on read
- precedence changing under repeated runs

## Verified Truth

- recommendation precedence is explicit and stable
- responses are structured and member-scoped
- recommendation reads are derived from workout data only for this slice
- local smoke proved:
  - cold start -> `start_first_workout`
  - in-progress -> `resume_in_progress_workout`
  - fresh finish -> `recovery_day`
  - aged finish -> `repeat_last_finished_workout`

## Unverified Truth

- no live in-cluster recommendation runtime proof was added
- no generated plans, LLM coaching, or persisted recommendation runtime was
  proven

## Carry-Forward Gaps

- recommendation success-path observability is still thin
- if recommendation scope widens later, corrupted-row handling should get a
  stronger end-to-end check than the current service-level coverage

## Final Verdict

Tracer 7 is closure-clean with accepted non-blocking carry-forward gaps.
