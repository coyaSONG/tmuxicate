---
phase: 01-coordinator-foundations
plan: 03
subsystem: orchestration
tags: [cobra, yaml, mailbox, coordinator, rebuild]
requires:
  - phase: 01-02
    provides: dedicated run/task artifacts, root-thread linkage, and mailbox-backed child task delivery
provides:
  - disk-only coordinator run graph reconstruction from durable artifacts
  - `tmuxicate run show <run-id>` operator inspection for run/task/message references
  - loud mismatch detection for missing dependencies, task artifacts, and message links
affects: [phase-02-01, coordinator-routing, operator-inspection]
tech-stack:
  added: []
  patterns:
    - rebuild joins coordinator YAML, mailbox receipts, and declared agent state without transcript parsing
    - operator output renders compact durable references and fails loudly on lineage divergence
key-files:
  created:
    - internal/session/run_rebuild.go
    - internal/session/run_rebuild_test.go
  modified:
    - cmd/tmuxicate/main.go
key-decisions:
  - "Run rebuild validates every task message against the run root thread so hidden lineage drift is surfaced as a coordinator artifact mismatch."
  - "Operator inspection shows task IDs, owners, expected output, state, and message IDs as the durable debugging surface instead of transcript-derived context."
patterns-established:
  - "Disk-only reconstruction lives in the session layer and reuses existing scan helpers rather than introducing daemon caches."
  - "Receipt folder state and per-agent declared state are merged into a compact run tree for operator-facing inspection."
requirements-completed: [PLAN-03]
duration: 8min
completed: 2026-04-05
---

# Phase 01 Plan 03: Coordinator Run Rebuild Summary

**Restart-safe coordinator run reconstruction with disk-only mismatch detection and `tmuxicate run show` inspection output**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-05T06:35:20Z
- **Completed:** 2026-04-05T06:43:21Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Added red tests for rebuild fidelity, operator-visible run output, and loud mismatch handling before implementation.
- Implemented `LoadRunGraph` and `FormatRunGraph` to reconstruct coordinator runs from run/task YAML, mailbox receipts, message threads, and declared state files.
- Wired `tmuxicate run show <run-id>` so operators can inspect run lineage and compact task state without transcript review.

## Task Commits

Each task was committed atomically:

1. **Task 1: Write failing reconstruction and operator-view tests** - `63f185f` (`test`)
2. **Task 2: Implement disk-scan rebuild and `run show` output** - `d9b2dd0` (`feat`)

Plan metadata is recorded in the final docs commit after summary/state updates.

## Files Created/Modified
- `internal/session/run_rebuild.go` - loads coordinator run graphs from durable artifacts and renders operator-facing summaries.
- `internal/session/run_rebuild_test.go` - covers rebuild fidelity, `run show` output, and mismatch failures.
- `cmd/tmuxicate/main.go` - adds the `run show` command wiring for operator inspection.

## Decisions Made
- Used the run root thread as an integrity check for every child task message so rebuild surfaces drift instead of tolerating it silently.
- Kept inspection output focused on durable IDs and compact state labels, avoiding transcript content and runtime-only context.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed done-receipt transitions in the new rebuild fixture**
- **Found during:** Task 2 (Implement disk-scan rebuild and `run show` output)
- **Issue:** The new red test helper moved unread receipts directly to `done`, which violated mailbox invariants requiring the real `unread -> active -> done` path plus `done_at`.
- **Fix:** Updated the helper to follow the real receipt lifecycle before asserting rebuild output.
- **Files modified:** `internal/session/run_rebuild_test.go`
- **Verification:** `go test ./internal/session -run 'TestRebuildRunGraphFromDisk|TestRunShowSummarizesReceiptAndDeclaredState|TestRunShowRejectsMissingOrMismatchedArtifacts' -count=1` and `go test ./internal/session ./internal/mailbox ./internal/protocol -count=1`
- **Committed in:** `d9b2dd0` (part of Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** The fix stayed inside the planned test surface and aligned the new fixture with existing mailbox behavior. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 1 foundations are complete: coordinator runs can now be started, persisted, rebuilt, and inspected from disk.
- Phase 2 can build routing behavior on top of the new reconstructable run graph and operator inspection surface.

## Verification

- `go test ./internal/session -run 'TestRebuildRunGraphFromDisk|TestRunShowSummarizesReceiptAndDeclaredState|TestRunShowRejectsMissingOrMismatchedArtifacts' -count=1`
- `go test ./internal/session ./internal/mailbox ./internal/protocol -count=1`

## Self-Check: PASSED

- Found `.planning/phases/01-coordinator-foundations/01-03-SUMMARY.md`
- Found task commit `63f185f`
- Found task commit `d9b2dd0`

---
*Phase: 01-coordinator-foundations*
*Completed: 2026-04-05*
