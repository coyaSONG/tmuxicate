---
phase: 03-review-handoff-flow
plan: 01
subsystem: workflow
tags: [coordinator, review-handoff, routing, mailbox]
requires:
  - phase: 02-role-based-routing
    provides: deterministic RouteChildTask ownership and durable routing metadata
provides:
  - canonical review handoff artifacts keyed by source task id
  - automatic review task routing after durable implementation completion
  - direct TaskDone coverage for create, idempotent, and fail-loud handoff paths
affects: [blocker-escalation, run-summaries, review-response]
tech-stack:
  added: []
  patterns:
    - task completion triggers follow-up review work only after the source receipt is durable in done
    - review linkage lives in reviews/<source-task-id>.yaml instead of reverse pointers on task records
key-files:
  created:
    - internal/session/task_cmd_test.go
  modified:
    - internal/protocol/coordinator.go
    - internal/protocol/validation.go
    - internal/mailbox/paths.go
    - internal/mailbox/coordinator_store.go
    - internal/session/task_cmd.go
    - internal/session/run.go
key-decisions:
  - "TaskDone reads parent_run_id and task_id from durable message metadata before attempting review routing."
  - "Existing review handoff artifacts are the only idempotency guard; repeated TaskDone calls do not re-route review work."
  - "Review child tasks emit review_request messages while keeping the source implementation task in done on handoff failures."
patterns-established:
  - "Canonical review-chain state uses ReviewHandoff YAML under coordinator/runs/<run-id>/reviews/."
  - "Coordinator automation records handoff_failed with a readable failure summary instead of rolling back durable task completion."
requirements-completed: [REVIEW-01]
duration: 1h
completed: 2026-04-06
---

# Phase 03: Review Handoff Flow Summary

**Implementation task completion now produces one canonical review handoff artifact and one linked review_request task without losing coordinator run lineage.**

## Performance

- **Duration:** 1h
- **Started:** 2026-04-06T08:30:00Z
- **Completed:** 2026-04-06T09:27:30Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Added `ReviewHandoff`, review outcome/status validation, and run-scoped review artifact paths in the existing protocol/mailbox model.
- Extended `TaskDone` to route review follow-up work only after the source receipt is durable in `done`, with artifact-based idempotency.
- Added direct `internal/session` coverage for successful handoff creation, repeated completion attempts, and fail-loud routing failures.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add review handoff contracts, store seams, and red session tests** - `31ca64b` (test)
2. **Task 2: Implement post-done review handoff creation and fail-loud routing** - `20c5718` (feat)

## Files Created/Modified
- `internal/protocol/coordinator.go` - review handoff schema plus review outcome and status enums
- `internal/protocol/validation.go` - validation rules for pending, responded, and handoff_failed review artifacts
- `internal/mailbox/paths.go` - canonical `reviews/` path helpers under each run
- `internal/mailbox/coordinator_store.go` - review handoff CRUD plus task read helper
- `internal/session/task_cmd.go` - post-done review handoff orchestration and fail-loud artifact updates
- `internal/session/run.go` - emits `review_request` for routed review child tasks
- `internal/session/task_cmd_test.go` - direct coverage for create, idempotency, and fail-loud completion paths

## Decisions Made

- Reused durable message metadata (`parent_run_id`, `task_id`) as the trigger source instead of inferring coordinator context from transcripts or prompt text.
- Treated the existence of `reviews/<source-task-id>.yaml` as the only handoff uniqueness gate.
- Left review-routing failures as best-effort handoff artifact updates so the source implementation task remains complete.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- The initial executor worker stalled before producing usable test or implementation output, so the plan was completed locally to keep wave execution moving.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Review requests now exist as durable follow-up work and are linked from a canonical handoff artifact.
- Wave 2 can build the dedicated `review respond` command and `run show` rendering on top of the persisted handoff schema without changing the completion path again.

---
*Phase: 03-review-handoff-flow*
*Completed: 2026-04-06*
