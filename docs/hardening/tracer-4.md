# Tracer 4 Hardening

## Audited Claim

Tracer 4 claims:

- authenticated `GET /api/v1/lobby/eligibility` is real
- eligibility is derived from explicit persisted `visibility_mode` and
  `availability_mode`
- visit history does not create lobby intent
- reads are side-effect free

## Exact Rerun Commands

```bash
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./internal/profile
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./internal/eligibility ./internal/profile
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./internal/server -run '^Test(HealthEndpoint|LobbyEligibilityEndpointRequiresAuthentication|LobbyEligibilityEndpointReturnsTheStableEligibilityShape|LobbyEligibilityEndpointMapsLookupFailuresClearly)$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./internal/server -run '^(TestLobbyEligibilityRoundTripFollowsExplicitProfileStateInsteadOfVisitHistory|TestLobbyEligibilityEndpointDoesNotRequireVisitHistoryWhenExplicitStateMakesMemberEligible)$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./internal/server -run '^TestLobbyEligibilityEndpointRejectsMissingTamperedExpiredAndRevokedSessions$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./internal/server -run '^(TestLobbyEligibilityEndpointIsSideEffectFreeAcrossVisitsWorkoutsAndClaimedTags|TestLobbyEligibilityEndpointSurfacesInvalidPersistedStatesDeterministically)$'
cd /Users/zizo/Personal-Projects/ASHTON/apollo && go test ./...
```

## Expected Destructive Failures Vs Real Regressions

Expected negative results during hardening:

- unauthenticated eligibility reads return `401`
- missing, tampered, expired, and revoked sessions return `401`
- invalid persisted `visibility_mode` and `availability_mode` values map to
  deterministic ineligible reasons
- real visit history still leaves an unavailable member ineligible

Real regressions would have been:

- a visit row creating eligibility on its own
- ghost mode becoming eligible
- eligibility reads mutating visits, workouts, or claimed tags

## Verified Truth

- the eligibility surface is real and authenticated
- precedence is explicit:
  - `discoverable + available_now` -> `eligible`
  - `unavailable` -> `availability_unavailable`
  - `with_team` -> `availability_with_team`
  - `ghost + available_now` -> `visibility_ghost`
- invalid persisted state stays visible as deterministic ineligible output
- eligibility reads are side-effect free

## Unverified Truth

- no live in-cluster APOLLO eligibility deployment proof was part of Tracer 4
- no lobby membership, invite, or match formation runtime was proven

## Carry-Forward Gaps

- lobby membership persistence remains deferred
- invitations and social workflow remain deferred
- matchmaking and ranking remain deferred
- deployment widening remained deferred at Tracer 4 close

## Final Verdict

Tracer 4 is closure-clean.
