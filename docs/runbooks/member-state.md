# APOLLO Member State Runbook

## Purpose

Use this runbook when implementing member auth, profile state, visits, workouts, and later lobby eligibility.

## Rules

- auth is separate from identity linkage
- visits are separate from workouts
- presence is separate from matchmaking intent
- `users.preferences` holds flexible member-intent state early

## Required Checks

- invalid or missing auth state is rejected clearly
- ghost mode does not imply lobby entry
- unavailable members never join the lobby
- visit creation never creates a workout implicitly
