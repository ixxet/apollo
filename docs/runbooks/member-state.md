# APOLLO Member State Runbook

## Purpose

Use this runbook when implementing member auth, profile state, visits, workouts, and later lobby eligibility.

## Rules

- auth is separate from identity linkage
- visits are separate from workouts
- presence is separate from matchmaking intent
- `users.preferences` holds flexible member-intent state early
- duplicate arrival delivery must be idempotent
- unknown and inactive tags must not create visits
- anonymous visit events are ignored
- visit closing stays deferred until a real departure slice exists

## Required Checks

- invalid or missing auth state is rejected clearly
- ghost mode does not imply lobby entry
- unavailable members never join the lobby
- visit creation never creates a workout implicitly
- duplicate delivery does not create a second visit
- malformed presence events are rejected clearly
