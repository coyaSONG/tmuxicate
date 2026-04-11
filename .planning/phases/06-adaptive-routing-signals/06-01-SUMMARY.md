---
phase: 06-adaptive-routing-signals
plan: 01
subsystem: api
tags: [go, coordinator, adaptive-routing, yaml, mailbox]
requires:
  - phase: 02-role-based-routing
    provides: deterministic route-task selection, duplicate-safe routed task artifacts, and task-class/domain metadata
  - phase: 05-run-summaries
    provides: RunGraph-derived summary status used for cross-run preference rebuilds
provides:
  - coordinator-scoped adaptive routing preference artifacts
  - validated adaptive routing config with exact-match manual preferences
  - root-only preference refresh after completed coordinator runs
affects: [06-02-PLAN, route-task, run-show, coordinator-artifacts]
tech-stack:
  added: []
  patterns:
    - derive adaptive routing inputs from existing RunGraph and RunSummary read models
    - persist coordinator-wide adaptive state under the existing coordinator artifact tree
key-files:
  created:
    - internal/session/run_adaptive.go
    - internal/session/run_adaptive_test.go
  modified:
    - cmd/tmuxicate/main.go
    - internal/config/config.go
    - internal/config/loader.go
    - internal/mailbox/coordinator_store.go
    - internal/mailbox/paths.go
    - internal/protocol/coordinator.go
    - internal/protocol/validation.go
key-decisions:
  - "Adaptive routing inputs live in one coordinator-keyed YAML artifact under `coordinator/preferences/adaptive-routing/`, not in envelopes, receipts, or daemon memory."
  - "Preference rebuilds reuse completed RunGraph plus RunSummary output so cross-run signals stay inspectable and transcript-free."
patterns-established:
  - "Manual adaptive boosts are exact-match keyed by task class, normalized domains, and preferred owner."
  - "Only root coordinator completion refreshes cross-run adaptive state; child-task completion does not mutate coordinator-wide preferences."
requirements-completed: [ADAPT-01]
duration: 4min
completed: 2026-04-11
---

# Phase 6 Plan 01: Adaptive Preference Inputs Summary

**Coordinator-scoped adaptive routing preference artifacts rebuilt from completed runs with exact-match manual boosts and a root-only refresh hook**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-11T10:20:24Z
- **Completed:** 2026-04-11T10:20:31Z
- **Tasks:** 2
- **Files modified:** 11

## Accomplishments
- Added validated adaptive routing config fields for lookback, score weights, penalties, and exact-match manual owner preferences.
- Added canonical adaptive preference/evidence protocol contracts plus coordinator-store path helpers for one durable coordinator-scoped artifact.
- Rebuilt adaptive preferences from completed root runs using `LoadRunGraph` and `BuildRunSummary`, then refreshed that artifact only on root coordinator completion.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add red tests for adaptive routing config, durable preference artifacts, and root-run refresh** - `6d8d599` (`test`)
2. **Task 2: Implement adaptive routing config, durable preference artifacts, and root-run refresh logic** - `901a425` (`feat`)

**Plan metadata:** pending

## Files Created/Modified
- `cmd/tmuxicate/main.go` - Extended the root-only `task done` path to rebuild and persist coordinator adaptive preferences after summary rebuild succeeds.
- `cmd/tmuxicate/main_test.go` - Added root-versus-child completion coverage for adaptive preference refresh behavior.
- `internal/config/config.go` - Added the adaptive routing config surface and manual preference type.
- `internal/config/loader.go` - Validated adaptive config values, preferred owners, and normalized exact-match domains.
- `internal/config/loader_test.go` - Pinned YAML parsing for all adaptive config keys and the manual preference fixture.
- `internal/mailbox/paths.go` - Added canonical coordinator preference paths under the existing artifact tree.
- `internal/mailbox/coordinator_store.go` - Added read/write support for adaptive routing preference artifacts.
- `internal/protocol/coordinator.go` - Added adaptive routing preference and evidence contracts shared by session and storage code.
- `internal/protocol/validation.go` - Validated adaptive preference sets, rows, and evidence ordering/shape.
- `internal/session/run_adaptive.go` - Implemented completed-run scanning, score aggregation, evidence persistence, and exact-match manual boosts.
- `internal/session/run_adaptive_test.go` - Added session coverage for aggregation, lookback trimming, and evidence lineage refs.

## Decisions Made

- Kept adaptive routing state coordinator-scoped and file-backed so later routing decisions can be explained directly from disk.
- Reused the existing summary/read-model stack for rebuilds instead of inventing a second cross-run reporting path.
- Treated the root task as the only safe write boundary for coordinator-wide preference refreshes.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Session tests that exercised review-aware scoring had to persist a valid `config.resolved.yaml` fixture because the existing review flow reloads resolved config from disk during `TaskDone`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Coordinator runs now persist reusable adaptive routing inputs that `route-task` can consume without changing mailbox compatibility.
- Phase 06-02 can layer adaptive owner ranking and operator-visible explanations on top of the new durable preference artifact.

## Self-Check: PASSED

- Verified `.planning/phases/06-adaptive-routing-signals/06-01-SUMMARY.md` exists.
- Verified commits `6d8d599` and `901a425` exist in git history.

---
*Phase: 06-adaptive-routing-signals*
*Completed: 2026-04-11*
