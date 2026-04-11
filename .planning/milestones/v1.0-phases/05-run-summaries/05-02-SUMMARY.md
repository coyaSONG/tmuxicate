---
phase: 05-run-summaries
plan: 02
subsystem: ui
tags: [go, cobra, cli, run-summary, coordinator]
requires:
  - phase: 05-01
    provides: shared RunGraph-derived summary builders and formatter helpers
provides:
  - operator-visible summary section at the top of `tmuxicate run show`
  - root-only completion summary print from `tmuxicate task done`
  - Cobra and session regression coverage for summary placement and completion output
affects: [run-show, task-done, coordinator-reporting]
tech-stack:
  added: []
  patterns:
    - reuse `BuildRunSummary` and `FormatRunSummary` for every operator-visible summary surface
    - gate automatic completion printing on canonical root-message metadata instead of new summary state
key-files:
  created: []
  modified:
    - cmd/tmuxicate/main.go
    - cmd/tmuxicate/main_test.go
    - internal/session/run_rebuild.go
    - internal/session/run_rebuild_test.go
key-decisions:
  - "Run show keeps `Run:` as the first line and inserts the shared summary block immediately after the root metadata header."
  - "Task completion prints a summary only when the completed message carries both `run_id` and a matching `root_message_id`."
patterns-established:
  - "Operator-facing summary output is always rebuilt from `LoadRunGraph` rather than cached or persisted separately."
  - "CLI command tests use real temp-dir session state and captured command output instead of mocking session internals."
requirements-completed: [SUM-01, SUM-02]
duration: 3min
completed: 2026-04-06
---

# Phase 05 Plan 02: Run Summary Surfaces Summary

**Shared run summaries now appear at the top of `run show` and print once on root coordinator completion without replacing task-local detail**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-06T15:17:47Z
- **Completed:** 2026-04-06T15:20:30Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Added red coverage for summary placement in `run show` and root-only summary printing from `task done`.
- Wired `FormatRunGraph` to prepend the shared Phase 5 summary block while keeping the full task-local review and blocker detail below it.
- Extended `task done` to rebuild and print the same summary only when the completed message is the coordinator run root.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add red operator-visible summary integration tests** - `57eeee7` (test)
2. **Task 2: Wire the shared summary formatter into `run show` and root-task completion output** - `83d4c12` (feat)

## Files Created/Modified
- `cmd/tmuxicate/main.go` - Root-only completion summary hook for `task done` using canonical message metadata and the shared summary formatter.
- `cmd/tmuxicate/main_test.go` - Cobra coverage for `run show` summary placement and root-versus-child completion output.
- `internal/session/run_rebuild.go` - Summary insertion into `FormatRunGraph` above the existing task detail blocks.
- `internal/session/run_rebuild_test.go` - Regression coverage that pins summary ordering ahead of task-local review and blocker sections.

## Decisions Made

- Kept `newRunShowCmd` on the existing `LoadRunGraph`/`FormatRunGraph` path so the summary stays additive rather than creating a second CLI surface.
- Resolved the one-time completion print at the root message edge only, which satisfies the Phase 5 scope without a new durable summary marker.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Cobra output capture needed one shared writer in command tests because `run show` uses `cmd.OutOrStdout()` while `task done` previously printed directly to process stdout.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 5 operator-visible summary behavior is now complete on top of the shared RunGraph read model from 05-01.
- Shared planning files were intentionally left untouched for the orchestrator to update after execution.

## Self-Check: PASSED

- Verified `.planning/phases/05-run-summaries/05-02-SUMMARY.md` exists.
- Verified task commits `57eeee7` and `83d4c12` exist in git history.

---
*Phase: 05-run-summaries*
*Completed: 2026-04-06*
