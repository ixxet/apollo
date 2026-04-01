# apollo

APOLLO is the member-facing multi-mode application in ASHTON. It will handle member profiles, privacy and availability controls, visit history, workout logging, recommendations, and the ARES matchmaking subsystem.

This repo remains docs-first until the platform foundation is stable. The detailed brief lives in [ashton-platform/planning/repo-briefs/apollo.md](https://github.com/ixxet/ashton-platform/blob/main/planning/repo-briefs/apollo.md).

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

## Current State

Docs-first stub only. No Go API or Svelte frontend scaffold has been created yet.
