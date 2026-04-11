# Phase 12: Operator Target Control - Context

**Gathered:** 2026-04-11
**Status:** Complete

## Phase Boundary

Keep remote execution operator-steerable through explicit target control and explainable recovery behavior.

## Decisions

- Operator control lives under a dedicated `tmuxicate target` command family.
- Disabled or offline targets must be excluded at route time rather than failing later in hidden ways.
- Re-enabling a target should redispatch pending unread work automatically.
