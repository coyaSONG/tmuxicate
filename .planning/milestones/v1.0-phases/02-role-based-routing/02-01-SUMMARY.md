---
phase: 02-role-based-routing
plan: 01
subsystem: api
tags: [routing, coordinator, cobra, yaml, go]
requires:
  - phase: 01-coordinator-foundations
    provides: durable coordinator runs, child-task persistence, and run root message contracts
provides:
  - structured agent role metadata with route priorities
  - deterministic `tmuxicate run route-task` owner selection
  - coordinator root-message routing instructions by task class and domain
affects: [02-02-PLAN, coordinator-run-workflow, config-validation]
tech-stack:
  added: []
  patterns: [RoleSpec metadata, deterministic route ranking, route-task before add-task]
key-files:
  created: []
  modified:
    - cmd/tmuxicate/main.go
    - internal/config/config.go
    - internal/config/loader.go
    - internal/config/loader_test.go
    - internal/protocol/coordinator.go
    - internal/protocol/validation.go
    - internal/session/run.go
    - internal/session/run_contracts.go
    - internal/session/run_test.go
key-decisions:
  - "Agent role metadata now uses RoleSpec with canonical task-class kinds and normalized domains."
  - "RouteChildTask ranks kind-matching candidates by route_priority descending, then config declaration order ascending."
  - "The coordinator root contract routes first via `tmuxicate run route-task` and only falls back to `run add-task` as the explicit-owner persistence path."
patterns-established:
  - "Structured routing metadata lives in config and is validated before session logic runs."
  - "Session routing returns a RoutingDecision on success and a structured RouteRejection on no-match."
requirements-completed: [ROUTE-01]
duration: 9min
completed: 2026-04-05
---

# Phase 02 Plan 01: Structured Routing Summary

**Structured coordinator routing with `RoleSpec`, deterministic `route-task` selection, and fail-loud route rejections**

## Performance

- **Duration:** 9 min
- **Started:** 2026-04-05T08:40:37Z
- **Completed:** 2026-04-05T08:50:08Z
- **Tasks:** 2
- **Files modified:** 12

## Accomplishments
- Added structured routing config with `RoleSpec`, `route_priority`, and per-class routing policy fields.
- Added canonical routing protocol types plus `RouteChildTask` session logic that selects one owner deterministically by class and domain coverage.
- Updated the coordinator root-message contract and CLI surface to drive routing through `tmuxicate run route-task`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add failing tests for structured routing config and deterministic route selection** - `bf0b07f` (`test`)
2. **Task 2: Implement structured role metadata, route-task CLI, and deterministic owner selection** - `604eb48` (`feat`)

**Plan metadata:** pending

## Files Created/Modified
- `cmd/tmuxicate/main.go` - Added the `run route-task` CLI entrypoint and flags for routed task creation.
- `internal/config/config.go` - Introduced `RoleSpec`, `route_priority`, and task-class routing config fields.
- `internal/config/loader.go` - Validated structured role metadata, normalized route domains, and copied the new routing config safely.
- `internal/config/loader_test.go` - Pinned structured role YAML parsing and routing policy fixture coverage.
- `internal/protocol/coordinator.go` - Added `TaskClass`, `RouteChildTaskRequest`, `RoutingDecision`, and `RouteRejection`.
- `internal/protocol/validation.go` - Added task-class validation, route-domain normalization, and request/decision/rejection validation.
- `internal/session/run.go` - Implemented deterministic routed owner selection and structured no-match handling.
- `internal/session/run_contracts.go` - Updated the coordinator contract to instruct `route-task` usage.
- `internal/session/run_test.go` - Added deterministic route selection and structured rejection tests.
- `internal/session/init_cmd.go` - Generated structured role metadata in default configs so the session package still compiles with `RoleSpec`.
- `internal/session/pick.go` - Rendered structured roles safely in picker output.
- `internal/session/up.go` - Rendered structured roles safely in bootstrap text.

## Decisions Made
- Stored role routing inputs as structured config metadata and kept run snapshots operator-readable by persisting the role kind string.
- Ranked `eligible_candidates` using the same priority and config-order rules as final owner selection so routing evidence stays deterministic.
- Normalized route domains in protocol validation so config parsing and runtime routing share one canonical matching rule.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated adjacent session helpers for `RoleSpec` compatibility**
- **Found during:** Task 2 (Implement structured role metadata, route-task CLI, and deterministic owner selection)
- **Issue:** Changing `AgentConfig.Role` from `string` to `RoleSpec` would leave `internal/session` unable to compile because init, picker, and bootstrap helpers still assumed string roles.
- **Fix:** Updated `internal/session/init_cmd.go`, `internal/session/pick.go`, and `internal/session/up.go` to create and render structured role metadata.
- **Files modified:** `internal/session/init_cmd.go`, `internal/session/pick.go`, `internal/session/up.go`
- **Verification:** `go test ./internal/config ./internal/session ./internal/protocol -count=1`
- **Committed in:** `604eb48`

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** The adjacent changes were required to keep the `internal/session` package compiling after the planned `RoleSpec` migration. No behavioral scope creep beyond that compile boundary.

## Issues Encountered
None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan 02-02 can build duplicate-task safeguards and durable routing evidence on top of the new `TaskClass`, `RouteChildTask`, and route-task CLI seams.
- Structured route rejections now expose the candidate/owner context that the next plan can persist into run/task inspection views.

## Self-Check: PASSED

- Found `.planning/phases/02-role-based-routing/02-01-SUMMARY.md`
- Found commit `bf0b07f`
- Found commit `604eb48`
