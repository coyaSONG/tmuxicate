---
phase: 06-adaptive-routing-signals
plan: 02
subsystem: api
tags: [go, coordinator, adaptive-routing, cobra, cli]
requires:
  - phase: 06-adaptive-routing-signals
    provides: coordinator-scoped adaptive preference artifacts and validated adaptive routing config
  - phase: 02-role-based-routing
    provides: deterministic route-task baseline ranking and durable routed task evidence
provides:
  - adaptive owner reordering inside the existing eligible candidate set
  - persisted adaptive routing explanations on canonical routing decisions
  - operator-visible adaptive evidence in `run route-task` output and `run show`
affects: [07-partial-replanning-flow, run-show, route-task]
tech-stack:
  added: []
  patterns:
    - apply adaptive scoring only after the deterministic routing baseline has selected eligible candidates
    - render routing explanations from durable task YAML instead of a separate reporting backend
key-files:
  created: []
  modified:
    - cmd/tmuxicate/main.go
    - internal/session/run.go
    - internal/session/run_adaptive.go
    - internal/session/run_rebuild.go
    - cmd/tmuxicate/main_test.go
    - internal/session/run_test.go
    - internal/session/run_rebuild_test.go
key-decisions:
  - "Adaptive routing only changes owner selection when a unique exact-match preference beats the baseline winner; ties fall back to `route_priority desc, config_order asc`."
  - "Adaptive explanations are additive fields on `RoutingDecision` and are rendered from task YAML in both CLI routing output and `run show`."
patterns-established:
  - "Baseline route filtering and duplicate guards remain authoritative; adaptive scoring is an overlay inside the already-eligible candidate list."
  - "Operator-visible adaptive evidence uses the same labels across immediate CLI output and rebuilt run inspection."
requirements-completed: [ADAPT-01, ADAPT-02]
duration: 4min
completed: 2026-04-11
---

# Phase 6 Plan 02: Adaptive Routing Application Summary

**Adaptive `route-task` overlay that reorders eligible owners by exact-match preference scores and renders disk-backed routing evidence in both CLI routing output and `run show`**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-11T10:24:24Z
- **Completed:** 2026-04-11T10:24:27Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Applied adaptive preference rows only after duplicate checks and baseline eligibility filtering, preserving deterministic fallback on ties or missing matches.
- Persisted structured adaptive explanations on `RoutingDecision` with baseline owner, score breakdown, stable reason text, and copied evidence refs.
- Exposed the same adaptive explanation labels in both `tmuxicate run route-task` output and `FormatRunGraph` task detail blocks.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add red tests for adaptive ranking, deterministic fallback, and operator-visible evidence** - `0303edf` (`test`)
2. **Task 2: Implement adaptive-aware route-task ranking and operator-visible decision evidence** - `e03e8ba` (`feat`)

**Plan metadata:** pending

## Files Created/Modified
- `cmd/tmuxicate/main.go` - Extended `run route-task` to print the selected owner plus adaptive routing reason, baseline, score, and evidence.
- `cmd/tmuxicate/main_test.go` - Added CLI coverage that adaptive decision evidence appears in route-task output.
- `internal/session/run.go` - Added exact-match adaptive overlay logic on top of the deterministic candidate baseline and persisted adaptive explanation data.
- `internal/session/run_adaptive.go` - Added helpers for loading and matching coordinator-scoped adaptive preference rows by task class, normalized domains, and owner.
- `internal/session/run_rebuild.go` - Rendered adaptive routing evidence under each task’s existing routing detail block.
- `internal/session/run_rebuild_test.go` - Added rebuild coverage that adaptive routing explanation survives disk round-trip and appears in `run show`.
- `internal/session/run_test.go` - Added direct session coverage for adaptive ranking, baseline fallback on tie/miss, and persisted explanation fields.

## Decisions Made

- Kept adaptive overlay opt-in behind `routing.adaptive.enabled` so stale preference artifacts do not affect routing when the feature is disabled.
- Used exact-match preference keys and score ties as the hard boundary for baseline fallback, which keeps selection deterministic and inspectable.
- Reused the same human-readable labels across route-time output and run rebuild output to avoid a split explanation model.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- The initial CLI red test accidentally hit the existing duplicate safeguard; the fixture was narrowed to a fresh run so the test exercised adaptive output instead of duplicate rejection.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 6 is complete: coordinator routing now has durable adaptive inputs and operator-visible adaptive explanations without breaking mailbox compatibility or duplicate safeguards.
- Phase 7 can build bounded partial replanning on top of the richer routing evidence and coordinator-scoped preference artifacts.

## Self-Check: PASSED

- Verified `.planning/phases/06-adaptive-routing-signals/06-02-SUMMARY.md` exists.
- Verified commits `0303edf` and `e03e8ba` exist in git history.

---
*Phase: 06-adaptive-routing-signals*
*Completed: 2026-04-11*
