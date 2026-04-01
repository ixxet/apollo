# Growing Pains

Use this document to record real product mistakes, schema changes, recommendation
failures, matchmaking edge cases, and the fixes that made `apollo` more realistic.

## 2026-04-01

- Member auth, identity linkage, and matchmaking intent were initially too easy
  to conflate. The fix was to document them as three separate concerns before
  implementation started.

- The first visit schema relied only on `source_event_id` dedupe, which still
  allowed separate arrival events to open multiple concurrent visits for the
  same member and facility. The fix was to add a partial unique index for open
  visits on `(user_id, facility_key)` while keeping workout history separate.
