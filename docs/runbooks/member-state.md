# APOLLO Member State Runbook

## Purpose

Use this runbook when implementing member auth, profile state, visits, workouts, and later lobby eligibility.

## Rules

- auth is separate from identity linkage
- verification tokens are separate from sessions
- visits are separate from workouts
- presence is separate from matchmaking intent
- identified-arrival consumers must use the shared `ashton-proto` runtime
  contract instead of private wire structs
- `users.preferences` holds flexible member-intent state early
- verification tokens must be stored hashed, expired, and single-use
- session cookies stay signed, `HTTPOnly`, `Secure`, and `SameSite=Strict`
- session state stays server-side; APOLLO is not JWT-first in this tracer
- open-lobby eligibility derives from persisted member state, not visits
- availability is evaluated before visibility so ineligible reasons stay stable
- `ghost + available_now` remains ineligible for the open lobby
- duplicate arrival delivery must be idempotent
- duplicate departure delivery must be idempotent
- unknown and inactive tags must not create visits
- anonymous visit events are ignored
- departures close visits by exact member + facility match only
- departures with no matching open visit must be deterministic no-ops
- departures older than the open visit arrival must not backdate or corrupt
  visit history
- visit closing must not mutate `users.preferences`, claimed tags, workouts, or
  derived eligibility state

## Required Checks

- invalid or missing auth state is rejected clearly
- invalid, expired, and reused verification tokens are rejected clearly
- tampered, expired, and revoked session cookies are rejected clearly
- ghost mode does not imply lobby entry
- unavailable members never join the lobby
- `with_team` members stay out of the open lobby
- invalid persisted eligibility enums return deterministic ineligible reasons
- profile writes only mutate `visibility_mode` and `availability_mode`
- eligibility reads only observe profile state and must not mutate visits,
  workouts, or claimed tags
- profile writes must not mutate visits, workouts, or claimed tags
- visit creation never creates a workout implicitly
- visit closing never creates a workout implicitly
- visit closing never mutates claimed tags or profile state
- duplicate delivery does not create a second visit
- malformed presence events are rejected clearly
- producer-consumer compatibility is proven with shared helper or fixture bytes,
  not hand-written JSON strings copied into APOLLO tests

## Local Smoke Notes

- `APOLLO_LOG_VERIFICATION_TOKENS=true` is the intended local dev path for
  observing verification tokens without adding a public inspection API
- plain HTTP cookie jars will not replay a `Secure` session cookie back to
  `localhost`; local smoke should either send the cookie explicitly or terminate
  TLS in front of APOLLO
