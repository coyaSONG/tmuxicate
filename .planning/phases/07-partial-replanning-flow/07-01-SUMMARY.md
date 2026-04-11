---
phase: 07-partial-replanning-flow
plan: 01
subsystem: workflow
tags: [go, coordinator, blockers, yaml, mailbox]
requires:
  - phase: 04-blocker-escalation
    provides: blocker-case contracts, resolution actions, and source-task keyed coordinator artifacts
  - phase: 06-adaptive-routing-signals
    provides: current coordinator artifact patterns and run-show additive evidence conventions
provides:
  - validated `PartialReplan` protocol artifacts with bounded single-replacement lineage
  - canonical `coordinator/runs/<run-id>/replans/<source-task-id>.yaml` storage helpers
  - coordinator-store CRUD and replacement-task lookup for later blocker and run-show flows
affects: [07-02, blocker-resolution, run-show, run-summaries]
tech-stack:
  added: []
  patterns: [source-task keyed partial replan artifacts, fail-loud path-to-yaml validation, single-replacement bounded recovery]
key-files:
  created: []
  modified:
    - internal/protocol/coordinator.go
    - internal/protocol/validation.go
    - internal/protocol/protocol_test.go
    - internal/mailbox/paths.go
    - internal/mailbox/coordinator_store.go
    - internal/mailbox/coordinator_store_test.go
key-decisions:
  - "Partial replans are represented by one immutable artifact keyed to the blocked source task rather than runtime-only blocker state."
  - "The contract permits only one superseded task and one replacement task, and validation rejects recursive or duplicate replacement lineage."
patterns-established:
  - "Coordinator workflow artifacts continue to validate path authority by checking `run_id` and `source_task_id` against the source-keyed filename."
  - "Later session code can recover replacement lineage from store helpers instead of scanning ad hoc YAML directly."
requirements-completed: [REPLAN-01, REPLAN-02]
duration: 2min
completed: 2026-04-11
---

# Phase 7 Plan 01: Partial Replan Contract Summary

**Bounded partial replan artifacts keyed by source task with explicit superseded and replacement lineage for blocker recovery**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-11T10:37:59Z
- **Completed:** 2026-04-11T10:39:41Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Added a canonical `PartialReplan` protocol contract and `partial_replan` blocker-resolution action with strict bounded-lineage validation.
- Added canonical replan storage paths under `coordinator/runs/<run-id>/replans/` plus source-task keyed create/read behavior.
- Added replacement-task lookup coverage so later session rebuild code can map replacement work back to the blocked source task from disk alone.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add red tests for partial-replan validation, bounded replacement semantics, and coordinator-store lookup** - `e7ed292` (`test`)
2. **Task 2: Implement the durable partial-replan contract, canonical storage paths, and lookup helpers** - `cb0bcbc` (`feat`)

**Plan metadata:** pending

## Files Created/Modified
- `internal/protocol/coordinator.go` - Added `BlockerResolutionActionPartialReplan`, `PartialReplanStatus`, and the canonical `PartialReplan` artifact.
- `internal/protocol/validation.go` - Added fail-loud bounded-lineage validation for partial replans and the new resolution action enum.
- `internal/protocol/protocol_test.go` - Added direct validation coverage for required lineage fields and non-recursive replacement rules.
- `internal/mailbox/paths.go` - Added canonical replan directory and source-task keyed artifact paths.
- `internal/mailbox/coordinator_store.go` - Added create/read/find helpers for partial replans plus path-vs-YAML validation.
- `internal/mailbox/coordinator_store_test.go` - Added CRUD and replacement-task lookup coverage for durable replan artifacts.

## Decisions Made

- Kept partial replans immutable in this phase: create/read/find only, with no update API or second-generation lineage mutation.
- Required `blocker_source_task_id` to equal `source_task_id` so the replan stays local to the blocked logical work item.
- Reused the source-task keyed coordinator-store pattern from review handoffs and blocker cases rather than introducing a run-wide replanning index.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- `gofumpt` was not installed in the workspace, so formatting verification used `gofmt` before running the Go test suite.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `07-02` can now create one bounded replacement task and persist durable lineage through coordinator-store helpers instead of inventing runtime-only state.
- Run rebuild and summary code can attach source-task and replacement-task views from the new artifact contract while failing loudly on mismatches.

## Self-Check: PASSED

- Verified `.planning/phases/07-partial-replanning-flow/07-01-SUMMARY.md` exists.
- Verified commits `e7ed292` and `cb0bcbc` exist in git history.

---
*Phase: 07-partial-replanning-flow*
*Completed: 2026-04-11*
