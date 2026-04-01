# ADR 001: Member State Model

## Status

Accepted.

## Context

APOLLO is not a single-purpose tap-in or matchmaking app. Members need to use
the platform in multiple modes:

- private coaching only
- on-site but invisible
- on-site and recruitable
- on-site and already with a team

The system must avoid collapsing presence, privacy, availability, and workout
activity into one state.

## Decision

Model member state with separate concerns:

- `presence_state`
- `visibility_mode`
- `availability_mode`
- `coaching_profile`
- `visit_history`
- `workout_history`

Tap-in or arrival affects presence, not matchmaking intent.

## Storage Strategy

Start with flexible state storage in `users.preferences` JSONB for fields that
are still evolving:

- `visibility_mode`
- `availability_mode`
- `coaching_profile`

Keep visits, workouts, exercises, and ARES entities relational.

## Consequences

- APOLLO can evolve without repeated early migrations for every state nuance
- matchmaking stays opt-in
- coaching and social features can coexist without forcing one another
