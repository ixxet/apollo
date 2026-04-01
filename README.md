# apollo

APOLLO is the member-facing multi-mode application in ASHTON. It will handle member profiles, privacy and availability controls, visit history, workout logging, recommendations, and the ARES matchmaking subsystem.

This repo now has its first executable tracer slice. The detailed brief lives in [ashton-platform/planning/repo-briefs/apollo.md](https://github.com/ixxet/ashton-platform/blob/main/planning/repo-briefs/apollo.md).

## Role In The Platform

- member-facing application
- depends on `ashton-proto` and `athena`
- owns member intent and coaching context
- contains ARES as an internal subsystem, not a separate repo

## First Execution Goal

The first APOLLO slice should be narrow and real:

- create one member profile with privacy and availability settings
- record one visit-history state from ATHENA-backed presence
- log one workout independently of a tap-in event
- keep advanced diet guidance and richer recommendations as follow-on work

## Authentication

The first APOLLO auth path is student ID + email verification + signed session cookie.

OAuth is explicitly deferred unless a real institutional or external provider becomes available later.

## Current State

Tracer 2 now has a narrow executable slice:

- a Go runtime scaffold exists with `apollo serve` and `/api/v1/health`
- APOLLO can consume `athena.identified_presence.arrived` and record one visit
- claimed tags map ATHENA identity hashes to member records without pulling auth into the tracer
- duplicate arrivals, unknown tags, malformed payloads, and anonymous events are handled deterministically
- workout creation remains separate and is not triggered by visit events
- this repo is ready for a `v0.1.0` tracer-close tag

See:

- `docs/roadmap.md`
- `docs/runbooks/member-state.md`
- `docs/growing-pains.md`
