---
phase: 07-partial-replanning-flow
plan: 02
subsystem: workflow
tags: [go, cobra, coordinator, blockers, run-show]
requires:
  - phase: 07-partial-replanning-flow
    provides: bounded partial replan artifacts, validation, and source-task keyed store helpers
  - phase: 04-blocker-escalation
    provides: escalated blocker workflow, receipt suspension pattern, and task-local blocker rendering
provides:
  - explicit `blocker resolve --action partial_replan` execution over escalated blocker cases
  - task-local run-show rendering for source and replacement partial-replan lineage
  - summary collapse that keeps replacement work attached to the original logical source row
affects: [08-remote-execution-targets, 09-run-timeline-views, blocker-resolution, run-show, run-summaries]
tech-stack:
  added: []
  patterns: [blocker resolve options contract, routed replacement-task creation through existing safeguards, task-local replan lineage rendering]
key-files:
  created: []
  modified:
    - cmd/tmuxicate/main.go
    - cmd/tmuxicate/main_test.go
    - internal/session/blocker_resolve.go
    - internal/session/blocker_resolve_test.go
    - internal/session/run_rebuild.go
    - internal/session/run_rebuild_test.go
    - internal/session/run_summary.go
    - internal/session/run_summary_test.go
key-decisions:
  - "Partial replans are only valid from escalated blocker cases and still route replacement work through `RouteChildTask` so duplicate guards and allowed-owner enforcement remain authoritative."
  - "The superseded task receipt is retired with the existing reroute suspension pattern before replacement creation so old work cannot continue silently in parallel."
  - "Run inspection stays additive: source tasks render `Partial Replan` details, replacement tasks render `Replan Source`, and the summary keeps the source task as the logical row key."
patterns-established:
  - "Operator-triggered recovery actions use a narrow options struct so action-specific validation happens before store reads or task creation."
  - "Run rebuild validates blocker-resolution and replan artifacts together and returns `coordinator artifact mismatch` instead of rendering partial lineage."
requirements-completed: [REPLAN-01, REPLAN-02]
duration: 4min
completed: 2026-04-11
---

# Phase 7 Plan 02: Partial Replan Execution Summary

**Escalated blockers can now create one bounded replacement task through `blocker resolve`, and existing run surfaces rebuild that lineage from disk without duplicating logical summary rows**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-11T10:42:11Z
- **Completed:** 2026-04-11T10:45:27Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- Added `partial_replan` as an explicit blocker-resolution action with CLI flag validation for the replacement task contract.
- Implemented replacement-task creation through the existing routed child-task path while suspending the superseded receipt and persisting immutable replan lineage.
- Extended `run show` and run summaries to render source-side and replacement-side lineage from durable artifacts while failing loudly on broken links.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add red tests for partial-replan CLI validation, execution, and lineage rendering** - `a76e1ab` (`test`)
2. **Task 2: Implement the explicit `partial_replan` blocker-resolution path and CLI contract** - `551d08f` (`feat`)
3. **Task 3: Load partial-replan lineage into `run show` and collapse replacement work into the source summary row** - `551d08f` (`feat`)

**Plan metadata:** pending

## Files Created/Modified
- `cmd/tmuxicate/main.go` - Added `partial_replan` CLI help and replacement-task flags, then wired command input into `BlockerResolveOpts`.
- `cmd/tmuxicate/main_test.go` - Added CLI coverage for partial-replan help output and required replacement-task inputs.
- `internal/session/blocker_resolve.go` - Added action-specific blocker resolve validation, bounded replacement creation, and immutable replan artifact persistence.
- `internal/session/blocker_resolve_test.go` - Added coverage for successful replan execution plus duplicate-artifact and non-escalated rejection paths.
- `internal/session/run_rebuild.go` - Added partial-replan artifact loading, blocker/replan link validation, and task-local source/replacement rendering.
- `internal/session/run_rebuild_test.go` - Added render coverage for both lineage sides and fail-loud broken-link rejection.
- `internal/session/run_summary.go` - Collapsed replacement tasks into the source logical row while keeping replacement state as the effective current work item.
- `internal/session/run_summary_test.go` - Added regression coverage that replacement tasks do not render as a second logical summary row.

## Decisions Made

- Kept `partial_replan` inside `blocker resolve` instead of adding a new command family, preserving the existing operator escalation workflow.
- Reused receipt suspension and routed-task creation to preserve blocker ceilings, duplicate checks, and deterministic routing behavior under recovery.
- Treated partial-replan artifacts as another task-local derived block in `run show`, matching the established review and blocker rendering model.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- One red test fixture initially hit duplicate-route rejection before it exercised duplicate-replan rejection; the fixture was narrowed to a direct child task so the intended failure stayed isolated.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 7 is complete: coordinator recovery can now replace blocked work with one bounded replacement path while preserving durable lineage and explicit operator control.
- Phase 8 can build remote execution-target placement on top of source-task-local replan lineage without redefining blocker recovery or run-summary semantics.

## Self-Check: PASSED

- Verified `.planning/phases/07-partial-replanning-flow/07-02-SUMMARY.md` exists.
- Verified commits `a76e1ab` and `551d08f` exist in git history.

---
*Phase: 07-partial-replanning-flow*
*Completed: 2026-04-11*
