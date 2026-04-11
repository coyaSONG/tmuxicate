---
phase: 02-role-based-routing
plan: 02
subsystem: api
tags: [routing, duplicate-policy, coordinator, yaml, go]
requires:
  - phase: 02-role-based-routing
    provides: deterministic `route-task` selection and structured task-class routing metadata
provides:
  - duplicate-safe routed task persistence keyed by run, task class, and normalized domains
  - run-scoped locking plus duplicate rechecks inside `AddChildTask`
  - durable `run show` rendering of candidates, duplicate keys, and override reasons
affects: [03-review-handoff-flow, coordinator-run-inspection, task-persistence]
tech-stack:
  added: []
  patterns: [run-scoped route lock, routed duplicate identity, disk-backed routing evidence]
key-files:
  created: []
  modified:
    - internal/mailbox/coordinator_store.go
    - internal/mailbox/paths.go
    - internal/protocol/coordinator.go
    - internal/protocol/validation.go
    - internal/session/run.go
    - internal/session/run_contracts.go
    - internal/session/run_rebuild.go
    - internal/session/run_test.go
    - internal/session/run_rebuild_test.go
key-decisions:
  - "Duplicate routing now blocks by default and only permits repeat work for task classes explicitly listed in `routing.fanout_task_classes`."
  - "Routed child tasks persist normalized domains, duplicate keys, and routing decisions directly on canonical task YAML so `run show` can explain routes from disk alone."
patterns-established:
  - "Route selection and task persistence share one run-scoped lock to close duplicate scan/write races."
  - "Operator-visible routing evidence lives on child-task artifacts instead of logs or transcript reconstruction."
requirements-completed: [ROUTE-01, ROUTE-02]
duration: 14 min
completed: 2026-04-05
---

# Phase 02 Plan 02: Duplicate-Safe Routing Summary

**Duplicate-safe route-task persistence with run-scoped locking, override guardrails, and operator-visible routing evidence**

## Performance

- **Duration:** 14 min
- **Started:** 2026-04-05T08:54:42Z
- **Completed:** 2026-04-05T09:09:31Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Added duplicate identity enforcement on `(run_id, task_class, normalized_domains)` with a run-scoped route lock to close race windows.
- Rechecked duplicate policy inside `AddChildTask` so explicit-owner persistence cannot bypass routed safeguards.
- Persisted routing decisions, duplicate keys, normalized domains, candidate lists, and override reasons onto child-task artifacts and surfaced them in `run show`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add failing tests for duplicate policy, override gates, and routing-evidence output** - `2499350` (`test`)
2. **Task 2: Implement duplicate-policy enforcement and durable routing evidence** - `05f122c` (`feat`)

**Plan metadata:** pending

## Files Created/Modified
- `internal/mailbox/coordinator_store.go` - Added the shared run-route lock helper over the coordinator run directory.
- `internal/mailbox/paths.go` - Added the canonical `coordinator/runs/<run-id>/locks/route.lock` path helper.
- `internal/protocol/coordinator.go` - Extended child-task artifacts with duplicate identity and persisted routing evidence fields.
- `internal/protocol/validation.go` - Validated routed child-task metadata, duplicate-key shape, and routing decision contents.
- `internal/session/run.go` - Added duplicate scans, fanout policy handling, override-safe routing, and the locked `AddChildTask` persistence path.
- `internal/session/run_contracts.go` - Extended `ChildTaskRequest` so routed duplicate metadata can reach the explicit-owner persistence boundary.
- `internal/session/run_rebuild.go` - Rendered task class, domains, duplicate key, routing decision, candidates, and override reason in `run show`.
- `internal/session/run_test.go` - Added and updated direct session coverage for duplicate blocking, review fanout, override rules, and direct add-task duplicate rechecks.
- `internal/session/run_rebuild_test.go` - Added coverage that proves routing evidence survives disk rebuild and appears in `FormatRunGraph`.

## Decisions Made
- Defaulted duplicate handling to "block unless explicitly fanout" so `ROUTE-02` is conservative even for task classes that are not pre-listed in `exclusive_task_classes`.
- Stored routing evidence on `protocol.ChildTask` rather than in message metadata or logs so operators can reconstruct route choices from canonical task YAML alone.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Extended the explicit-owner request contract for routed metadata**
- **Found during:** Task 2 (Implement duplicate-policy enforcement and durable routing evidence)
- **Issue:** The RED tests required `AddChildTask` to receive `task_class`, normalized domains, duplicate key, and routing decision data, but `ChildTaskRequest` could not carry that metadata to the persistence boundary.
- **Fix:** Added routed metadata fields and validation to [`internal/session/run_contracts.go`](/Users/chsong/Developer/Personal/tmuxicate/internal/session/run_contracts.go) so the explicit-owner path can re-check duplicate policy before writing task YAML.
- **Files modified:** `internal/session/run_contracts.go`
- **Verification:** `go test ./internal/session -run 'TestRouteChildTaskBlocksExclusiveDuplicate|TestRouteChildTaskAllowsFanoutReviewClass|TestRouteChildTaskRequiresOverrideReason|TestAddChildTaskRejectsDuplicateWithoutRouteDecision|TestRunShowIncludesRoutingDecisionEvidence' -count=1 && go test ./internal/config ./internal/session ./internal/protocol -count=1`
- **Committed in:** `05f122c`

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** The adjacent contract change was required to complete the planned duplicate recheck inside `AddChildTask`. No scope expansion beyond the persistence boundary.

## Issues Encountered
None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 2 is complete and `ROUTE-02` is now backed by duplicate-safe persistence and operator-visible routing artifacts.
- Phase 3 can build review handoff behavior on top of persisted `task_class`, `duplicate_key`, and `routing_decision` data without adding another observability channel.

## Self-Check: PASSED

- Found `.planning/phases/02-role-based-routing/02-02-SUMMARY.md`
- Found commit `2499350`
- Found commit `05f122c`
