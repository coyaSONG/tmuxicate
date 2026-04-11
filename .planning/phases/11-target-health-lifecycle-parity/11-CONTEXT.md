# Phase 11: Target Health & Lifecycle Parity - Context

**Gathered:** 2026-04-11
**Status:** Complete

## Phase Boundary

Make non-local targets observable and ensure remote lifecycle events remain compatible with existing summaries and timelines.

## Decisions

- Target health remains target-scoped rather than pane-scoped.
- Remote lifecycle parity reuses the existing `task accept/wait/block/done` event contract.
- Operator inspection should expose target health directly in `status` and `target` surfaces.
