# Tracer 13 Hardening

## Audited Claim

Tracer 13 claims:

- authenticated `GET /api/v1/lobby/match-preview` is real
- preview candidate selection stays explicit and deterministic
- preview reads are side-effect free
- preview reads depend on explicit joined lobby membership, not visits or
  physical presence
- the member shell exposes one narrow read-only match preview panel
- deployment truth remains unchanged

## Exact Rerun Commands

```bash
cd /Users/zizo/Personal-Projects/ASHTON/apollo

go test ./...
go test -count=2 ./...
go test -count=5 ./internal/ares
go test -count=5 ./internal/server -run '^(TestLobbyMatchPreviewEndpointRequiresAuthentication|TestLobbyMatchPreviewEndpointReturnsStructuredPreview|TestLobbyMatchPreviewEndpointMapsErrorsClearly|TestLobbyMatchPreviewRuntimeSupportsDeterministicPreviewRead|TestLobbyMatchPreviewRuntimeExcludesIneligibleJoinedMembers|TestLobbyMatchPreviewRuntimeStaysSideEffectFreeAcrossVisitsWorkoutsRecommendationsAndMembership|TestLobbyMatchPreviewShellPanelRendersReadOnlyPreviewWithoutMutationActions)$'
go test ./cmd/apollo ./internal/server -run '^(TestBuildServerDependenciesWiresLobbyMembershipAndMatchPreviewRuntime|TestLobbyMatchPreviewEndpointRequiresAuthentication|TestLobbyMatchPreviewEndpointReturnsStructuredPreview|TestLobbyMatchPreviewRuntimeSupportsDeterministicPreviewRead|TestLobbyMatchPreviewRuntimeExcludesIneligibleJoinedMembers)$'
go build ./cmd/apollo
```

## Verified Match Preview Runtime Smoke

Use the exact smoke block in
[`docs/runbooks/member-state.md`](../runbooks/member-state.md) under
`Verified Match Preview Runtime Smoke`.

Minimum trustworthy outcomes:

- preview returns deterministic, structured read-only output over explicit
  joined lobby membership
- repeated preview reads are stable while membership and profile inputs are
  unchanged
- preview reads do not mutate `apollo.visits`, `apollo.workouts`, or lobby
  membership rows
- deployment truth remains local-only for this tracer

## Expected Destructive Failures Vs Real Regressions

Expected boundary results during hardening:

- unauthenticated preview reads return `401`
- empty or odd candidate sets still return deterministic structured output
- ineligible joined members are excluded from preview candidacy explicitly

Real regressions would have been:

- preview reads mutating membership, visits, workouts, recommendations, or
  ARES tables
- preview candidacy being inferred from visits or physical presence
- repeated preview reads changing under unchanged inputs
- the shell adding hidden mutation buttons or widening into assignment/invites

## Verified Truth

- match preview runtime is mounted in the production handler
- the command-layer dependency builder wires the preview runtime
- repeated server and service reruns held
- the member shell exposes a read-only preview panel over the same runtime

## Unverified Truth

- no live in-cluster match-preview deployment proof was added
- no assignment, invite, notification, or recorded ARES match runtime was
  proven

## Carry-Forward Gaps

Intentional deferred items remain:

- assignment, invites, notifications, and live match execution
- broader social-product widening
- later deterministic coaching and planner lines

## Final Verdict

Tracer 13 is closure-clean with deployment proof still deferred.
