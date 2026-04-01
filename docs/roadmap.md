# apollo Roadmap

## Objective

Create the cleanest possible first member-facing slice without dragging the whole recommendation and matchmaking system in on day one.

## First Implementation Slice

- define the minimum user, presence, and workout contract surface
- store member privacy and availability settings
- record visit history separately from workout history
- log one workout entry
- read back both current presence state and workout history
- add member auth with email verification and session cookies
- keep advanced diet guidance and full ARES automation as documented follow-on work

## Boundaries

- no full PWA in the first slice
- no recommendation engine until workout data exists
- no automatic lobby entry from tap-in events
- no matchmaking lobby until user state and activity data are stable

## Exit Criteria

- one user can store profile, privacy, and availability settings successfully
- one visit can be recorded without creating a workout entry
- one user can log one workout successfully
- the visit and workout records can be retrieved through stable read surfaces
- the repo stays small enough to evolve without fighting premature frontend or ML complexity
