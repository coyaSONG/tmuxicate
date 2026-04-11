---
phase: 04-blocker-escalation
plan: 01
subsystem: workflow
tags: [blockers, coordinator, protocol, config, yaml]
requires:
  - phase: 02-role-based-routing
    provides: routed task classes and deterministic task ownership metadata reused by blocker reroute ceilings
  - phase: 03-review-handoff-flow
    provides: durable workflow artifact and coordinator-store patterns mirrored by blocker cases
provides:
  - typed blocker enums and `BlockerCase` validation rules for wait, block, escalation, and resolution state
  - canonical blocker artifact paths plus coordinator-store CRUD keyed to `source_task_id`
  - dedicated `blockers.max_reroutes_*` config defaults and validation independent from daemon delivery retries
affects: [04-02, 04-03, blocker-resolution, run-show]
tech-stack:
  added: []
  patterns: [artifact-first workflow contracts, source-task keyed blocker persistence, separate blocker and transport retry ceilings]
key-files:
  created:
    - internal/mailbox/coordinator_store_test.go
    - .planning/phases/04-blocker-escalation/04-01-SUMMARY.md
  modified:
    - internal/protocol/coordinator.go
    - internal/protocol/validation.go
    - internal/protocol/protocol_test.go
    - internal/mailbox/paths.go
    - internal/mailbox/coordinator_store.go
    - internal/config/config.go
    - internal/config/loader.go
    - internal/config/loader_test.go
key-decisions:
  - "Escalated blocker cases carry a `recommended_action` using the operator resolution enum so escalation and resolve flows share one explicit contract."
  - "Blocker artifacts stay keyed to `source_task_id`, with current-task lookup implemented as a scan layer instead of changing the canonical filename."
  - "Blocker reroute defaults use an unmarshal sentinel so an explicit `0` ceiling survives YAML loading while the unset default still resolves to `1`."
patterns-established:
  - "Blocker workflow artifacts follow the same YAML-backed coordinator-store pattern as review handoffs."
  - "Workflow reroute ceilings are configured under `blockers.*` and validated separately from daemon delivery retry settings."
requirements-completed: [BLOCK-01, BLOCK-02, BLOCK-03]
duration: 11m
completed: 2026-04-06
---

# Phase 4 Plan 01: Blocker Contract Summary

**Source-task keyed blocker case artifacts with typed escalation contracts and dedicated blocker reroute ceilings**

## Performance

- **Duration:** 11 min
- **Started:** 2026-04-06T12:31:11Z
- **Completed:** 2026-04-06T12:42:02Z
- **Tasks:** 3
- **Files modified:** 10

## Accomplishments

- Added the blocker protocol surface in `internal/protocol` with explicit wait, block, action, status, and resolution enums plus a validated `BlockerCase` artifact.
- Added canonical `coordinator/runs/<run-id>/blockers/<source-task-id>.yaml` paths and coordinator-store CRUD with lookup by current task for reroute continuity.
- Added a dedicated `blockers.max_reroutes_default` and `blockers.max_reroutes_by_task_class` config surface with defaults, deep-copy support, and loader validation.

## Task Commits

Each TDD task was committed in red and green phases:

1. **Task 1: Add blocker protocol enums, artifact schema, and validation tests**
   `8827bac` (`test`) and `1b09537` (`feat`)
2. **Task 2: Add canonical blocker-case paths, CRUD, and lookup-by-current-task tests**
   `24efc47` (`test`) and `afc9bcc` (`feat`)
3. **Task 3: Add blocker reroute ceiling config with dedicated defaults and loader tests**
   `6218056` (`test`) and `580490b` (`feat`)

Summary metadata was committed separately after this file was created.

## Files Created/Modified

- `internal/protocol/coordinator.go` - blocker enums, escalation structs, and the canonical `BlockerCase` artifact.
- `internal/protocol/validation.go` - state-specific blocker validation and enum validation helpers.
- `internal/protocol/protocol_test.go` - protocol coverage for structured blocker kinds, escalation requirements, and resolution actions.
- `internal/mailbox/paths.go` - canonical blocker artifact directory and file path helpers.
- `internal/mailbox/coordinator_store.go` - blocker case create/read/update/find persistence APIs.
- `internal/mailbox/coordinator_store_test.go` - CRUD and reroute continuity coverage for blocker artifacts.
- `internal/config/config.go` - `BlockersConfig` schema and YAML unmarshal handling for explicit zero ceilings.
- `internal/config/loader.go` - blocker config defaults, validation, and deep-copy handling.
- `internal/config/loader_test.go` - loader coverage for valid blocker ceilings and invalid blocker config inputs.

## Decisions Made

- `RecommendedAction.Kind` uses `BlockerResolutionAction` values so escalations point directly at the operator actions later exposed by `blocker resolve`.
- `ReadBlockerCase` and `UpdateBlockerCase` validate that the YAML document’s `run_id` and `source_task_id` still match the source-keyed path, keeping the filename authoritative.
- `BlockersConfig` tracks whether `max_reroutes_default` was explicitly set so `0` remains a valid configured ceiling instead of being overwritten by the default.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. The only non-plan workflow adjustment was honoring the execution constraint to leave `.planning/STATE.md` and `.planning/ROADMAP.md` unchanged.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Phase `04-02` can now implement deterministic wait/block policy and `blocker resolve` against stable blocker contracts, store APIs, and reroute ceilings.

Phase `04-03` can rebuild blocker visibility in `run show` from the canonical blocker artifact paths without redefining protocol or config semantics.

## Self-Check: PASSED

- Verified `internal/mailbox/coordinator_store_test.go` exists.
- Verified `.planning/phases/04-blocker-escalation/04-01-SUMMARY.md` exists.
- Verified task commits `8827bac`, `1b09537`, `24efc47`, `afc9bcc`, `6218056`, and `580490b` exist in git history.

---
*Phase: 04-blocker-escalation*
*Completed: 2026-04-06*
