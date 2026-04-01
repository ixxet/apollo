# Growing Pains

Use this document to record real product mistakes, schema changes, recommendation
failures, matchmaking edge cases, and the fixes that made `apollo` more realistic.

## 2026-04-01

- Member auth, identity linkage, and matchmaking intent were initially too easy
  to conflate. The fix was to document them as three separate concerns before
  implementation started.
