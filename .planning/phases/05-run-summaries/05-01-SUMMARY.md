---
phase: 05-run-summaries
plan: 01
subsystem: infra
tags: [go, coordinator, run-summary, session]
requires:
  - phase: 03-review-handoff-flow
    provides: durable ReviewHandoff artifacts folded into source-task summary rows
  - phase: 04-blocker-escalation
    provides: durable BlockerCase artifacts and reroute linkage folded into source-task summary rows
provides:
  - RunSummary contracts for logical source-task reporting
  - deterministic status derivation over RunGraph review and blocker state
  - grouped summary formatting for future run-show and completion-hook reuse
affects: [run-show, coordinator-reporting]
tech-stack:
  added: []
  patterns:
    - derive operator summary views from RunGraph instead of rescanning durable artifacts
    - fold descendant review and reroute workflow nodes back into source-task rows
key-files:
  created:
    - internal/session/run_summary.go
    - internal/session/run_summary_test.go
  modified: []
key-decisions:
  - "Summary rows are keyed to source tasks only; review tasks and blocker current tasks never render as standalone logical items."
  - "Effective owner follows the reviewer when review is active, otherwise the current task owner."
patterns-established:
  - "RunSummary is a pure in-memory projection over RunGraph."
  - "Summary formatting keeps dense refs on the main line and optional detail on a second line only."
requirements-completed: [SUM-01, SUM-02]
duration: 5min
completed: 2026-04-06
---

# Phase 05 Plan 01: Run Summary Read Model Summary

**Source-task run summary projection with blocker/review fold-back and grouped operator formatting**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-06T15:01:19Z
- **Completed:** 2026-04-06T15:05:58Z
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments
- Added `RunSummary`, `RunSummaryItem`, and explicit run-summary status contracts in `internal/session`.
- Implemented source-task-only aggregation that reuses `RunGraph` and folds review/reroute/blocker artifacts back into the logical source row.
- Added grouped summary formatting plus direct tests for precedence, fold-back, and compact operator-facing output.

## Task Commits

Each task was committed atomically:

1. **Task 1: Define run-summary contracts and red aggregation tests** - `9787901` (test)
2. **Task 2: Implement logical summary aggregation over source tasks only** - `24ba2fb` (feat)
3. **Task 3: Implement grouped summary formatting for operator-visible reuse** - `d567050` (feat)

## Files Created/Modified
- `internal/session/run_summary.go` - Run summary contracts, source-row aggregation, and grouped formatter helpers.
- `internal/session/run_summary_test.go` - Summary precedence, descendant fold-back, and compact formatting coverage.

## Decisions Made

- Summary identity is anchored to the source task; review tasks and rerouted current tasks are excluded from the logical row set and only enrich the source row.
- Effective owner is derived from the active reviewer when a review handoff exists; otherwise the formatter falls back to the current task owner.
- Compact refs stay on the main line as `task=`, `msg=`, `current=`, `review=`, and `response=` tokens, while review/blocker detail moves to an optional second line.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Summary test fixtures needed valid blocker-state documents for waiting and resolved scenarios so the red/green checks exercised summary behavior instead of artifact-schema validation.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `BuildRunSummary` and `FormatRunSummary` are ready for Phase 05-02 command/output wiring.
- No session-package blockers remain from this plan.
- Shared phase artifact updates were intentionally left untouched for the orchestrator.

## Self-Check: PASSED

- Verified `.planning/phases/05-run-summaries/05-01-SUMMARY.md` exists.
- Verified `internal/session/run_summary.go` exists.
- Verified `internal/session/run_summary_test.go` exists.
- Verified task commits `9787901`, `24ba2fb`, and `d567050` exist in git history.

---
*Phase: 05-run-summaries*
*Completed: 2026-04-06*
