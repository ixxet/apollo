# Tracer 27 Hardening

Tracer 27 claims:

- APOLLO now exposes authenticated `GET /api/v1/presence` as facility-scoped
  member product truth over explicit linked visit rows only.
- each visit that becomes member-visible presence truth now has one durable
  `visit_tap_links` row, created only from the existing approved
  claimed-tag-plus-identified-arrival path.
- facility-scoped streak truth is now durable in
  `member_presence_streaks` and `member_presence_streak_events`, with one
  streak identity per `user_id + facility_key` and one event per credited UTC
  visit day.
- arrival truth may create or ensure tap-link plus facility streak credit, but
  departure truth still only closes the visit row; presence runtime does not
  distort visit ingest or close semantics.
- Tracer 27 does not claim QR/link-management UI, role/authz widening,
  public/social presence, badge logic, helper-owned mutation, or deployment
  closeout truth.

## Proof

Run from `/Users/zizo/Personal-Projects/ASHTON/apollo`:

```sh
go test ./...
go test -count=5 ./internal/...
go vet ./...
go build ./cmd/apollo
git diff --check
```

Focused runtime and integrity coverage now proves:

- authenticated `GET /api/v1/presence` returns facility-scoped `present` /
  `not_present` summaries instead of inventing one global current facility
- member presence reads expose explicit tap-link metadata on each returned
  visit and latest facility streak-event truth on each streak read
- facility streak status stays deterministic: current linked-day runs stay
  `active`, stale facility runs stay `inactive`, and facilities with no streak
  state stay `not_started`
- duplicate arrival replay does not create a second tap-link row, a second
  streak row, or a second streak event for the same facility/day
- departure replay stays deterministic and does not mutate tap-link or streak
  state beyond the original arrival-side credit
- repeated presence reads stay side-effect free across visits, tap-links,
  streak state/events, workouts, recommendations, lobby membership, and
  claimed tags
- already-linked visit replay stays clean even if the original claimed tag is
  later deactivated, because the visit-to-tap-link association is preserved
  explicitly once created

## Negative Proof

Tracer 27 does not claim:

- hidden presence inference beyond explicit linked visit rows
- one global member streak merged across facilities
- fake badge counters, longest-streak gamification, or public leaderboards
- member-managed QR/tag recovery or retroactive anonymous-link repair
- role/authz or staff-runtime widening
- deployment closeout truth
- `ashton-proto` widening

## Deferred

- member-managed tap-link lifecycle and QR/e-tap confirmation flows
- richer facility-local timezone semantics beyond the current explicit UTC
  visit-day credit rule
- public/social presence, badges, and broader gamified identity
- role/authz and trusted-surface substrate
- deployment closeout truth
