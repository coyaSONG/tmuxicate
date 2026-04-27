---
gsd_state_version: 1.0
milestone: v1.3
milestone_name: Runtime Trust & Honest Controls
current_phase: 13
current_phase_name: target-persistence-dispatch-safety
current_plan: 13-02-intent-first-dispatch-recovery
status: active
stopped_at: Milestone v1.3 Runtime Trust & Honest Controls defined
last_updated: "2026-04-27T04:55:27Z"
last_activity: 2026-04-27
progress:
  total_phases: 3
  completed_phases: 0
  total_plans: 6
  completed_plans: 1
  percent: 17
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-27)

**Core value:** A human can coordinate multiple terminal agents through a reliable, observable workflow where the coordinator keeps work moving without hiding what happened.
**Current focus:** v1.3 Runtime Trust & Honest Controls — harden target persistence, delivery policy, daemon lifecycle, artifact safety, and operator-facing docs before broader remote/worktree expansion.

## Current Position

Phase: 13 (target-persistence-dispatch-safety) — PLANNED
Current Phase: 13
Current Phase Name: target-persistence-dispatch-safety
Plan: 2 of 2
Current Plan: 13-02-intent-first-dispatch-recovery
Total Plans in Phase: 2
Total Phases: 3
Status: Milestone v1.3 Runtime Trust & Honest Controls active
Last activity: 2026-04-27
Last Activity Description: Phase 13 Plan 01 completed target store durability hardening with race-tested mailbox coverage

Progress: [██--------] 17%

## Performance Metrics

**Velocity:**

- Total plans completed: 24
- Average duration: -
- Total execution time: 0.0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 3 | - | - |
| 02 | 2 | - | - |
| 03 | 2 | - | - |
| 04 | 3 | - | - |
| 05 | 2 | - | - |
| 06 | 2 | 8m | 4m |
| 07 | 2 | 6m | 3m |
| 08 | 2 | 8m | 4m |
| 09 | 2 | 8m | 4m |
| 10 | 2 | 35m | 17.5m |
| 11 | 2 | 30m | 15m |
| 12 | 2 | 24m | 12m |

**Recent Trend:**

- Last 5 plans: 20min, 12min, 18min, 10min, 14min
- Trend: Stable execution speed with heavier runtime integration work

| Phase 01 P03 | 8min | 2 tasks | 3 files |
| Phase 02 P01 | 9min | 2 tasks | 12 files |
| Phase 02 P02 | 14 min | 2 tasks | 9 files |
| Phase 03 P01 | 1h | 2 tasks | 7 files |
| Phase 03 P02 | 1h | 3 tasks | 6 files |
| Phase 06 P01 | 4min | 2 tasks | 11 files |
| Phase 06 P02 | 4min | 2 tasks | 7 files |
| Phase 07 P01 | 2min | 2 tasks | 6 files |
| Phase 07 P02 | 4min | 3 tasks | 8 files |
| Phase 08 P01 | 4m | 3 tasks | 8 files |
| Phase 08 P02 | 4m | 3 tasks | 9 files |
| Phase 09 P01 | 5m | 2 tasks | 2 files |
| Phase 09 P02 | 3m | 2 tasks | 4 files |
| Phase 10 P01 | 15m | 3 tasks | 5 files |
| Phase 10 P02 | 20m | 3 tasks | 4 files |
| Phase 11 P01 | 12m | 3 tasks | 3 files |
| Phase 11 P02 | 18m | 3 tasks | 5 files |
| Phase 12 P01 | 10m | 3 tasks | 2 files |
| Phase 12 P02 | 14m | 3 tasks | 3 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Initialization: Build coordinator automation as a workflow layer on top of the existing mailbox/task model
- Initialization: Exclude full autonomous swarm behavior from the first milestone
- [Phase 01]: Run membership is derived from tasks/*.yaml under each run directory instead of a child_task_ids index on the run record.
- [Phase 01]: Coordinator state uses dedicated run/task records instead of Envelope.Meta so dependency and ownership fields stay explicit.
- [Phase 01]: Child task messages reuse the run root thread and reply-to linkage for transcript-free reconstruction.
- [Phase 01]: Run rebuild validates child task message threads against the run root thread so artifact drift fails loudly.
- [Phase 01]: Operator inspection renders task IDs, owners, expected output, state, and message IDs as the durable run debugging surface.
- [Phase 02]: Agent role metadata now uses RoleSpec with canonical task-class kinds and normalized domains.
- [Phase 02]: RouteChildTask ranks kind-matching candidates by route_priority descending, then config declaration order ascending.
- [Phase 02]: Coordinator decomposition now routes through tmuxicate run route-task before using run add-task as the explicit-owner persistence path.
- [Phase 02]: Duplicate routing now blocks by default unless the task class is explicitly listed in routing.fanout_task_classes.
- [Phase 02]: Routed child tasks persist normalized domains, duplicate keys, and routing decisions directly on canonical task YAML so run show explains routes from disk alone.
- [Phase 03]: Review linkage is canonicalized in `coordinator/runs/<run-id>/reviews/<source-task-id>.yaml` instead of reverse pointers on task records.
- [Phase 03]: Review tasks use dedicated `review_request` and `review_response` message kinds, and `run show` rebuilds the review chain from durable artifacts.
- [Phase 04]: Blocker handling uses durable `BlockerCase` artifacts with explicit reroute, escalation, and operator resolution paths.
- [Phase 05]: Run summaries are a derived `RunGraph` view rendered at the top of `run show`, not a new persisted artifact or command family.
- [Phase 05]: Completing the coordinator root task prints the same shared summary once using canonical root-message metadata.
- [Phase 06]: Adaptive routing inputs now live in one coordinator-keyed YAML artifact under the existing coordinator tree.
- [Phase 06]: Adaptive preference rebuilds reuse RunGraph plus RunSummary instead of transcript scanning or a second reporting backend.
- [Phase 06]: Adaptive routing only changes selection when a unique exact-match preference beats the deterministic baseline; ties fall back to route_priority desc, config_order asc.
- [Phase 06]: Adaptive explanations are additive RoutingDecision fields rendered from task YAML in both route-task output and run show.
- [Phase 07]: Partial replans are durable source-task keyed artifacts with one superseded task and one replacement task.
- [Phase 07]: partial_replan only runs from escalated blocker cases and still creates replacement work through RouteChildTask guardrails.
- [Phase 07]: run show and run summaries rebuild partial replan lineage from disk and fail loudly on blocker/replan link drift.
- [Phase 08]: Implicit local placement is synthesized as explicit target metadata; owner-derived placement remains the only selection path in 08-01.
- [Phase 08]: Dry-run preview shares route selection with persisted routing, and only pane-backed local agents participate in tmux lifecycle and daemon notifications.
- [Phase 09]: Timeline rebuild validates TaskEvent ownership and thread linkage against canonical run artifacts before rendering.
- [Phase 09]: Timeline filters derive owner, state, class, and target fields strictly from durable task metadata with a stable local fallback.
- [Phase 09]: Run show remains the canonical inspection surface; timeline rendering is additive and timeline-only reuses the same formatter path.
- [Phase 09]: Any timeline filter flag implies timeline mode so operators can narrow output without adding a second reporting command.
- [Phase 10]: Non-local execution dispatch is modeled as a target-scoped command contract plus durable dispatch records under the existing state tree.
- [Phase 10]: Dispatch runs only after canonical task artifacts are persisted, so remote launch failure cannot erase routed work.
- [Phase 11]: Target health is derived from durable heartbeat state and timeout policy instead of tmux-native remote probes.
- [Phase 11]: Remote lifecycle parity stays on the existing mailbox/task/state event contract; target-aware routing evidence is additive.
- [Phase 12]: Operator target control lives in a dedicated `tmuxicate target` command family with durable enable/disable state.
- [Phase 12]: Re-enabling a target redispatches unread pending work only, keeping recovery bounded and inspectable.
- [Phase 13]: Target state and dispatch records now use target-scoped locks plus atomic JSON writes so concurrent target operations preserve valid artifacts.

### Pending Todos

- Execute Phase 13 Plan 02: intent-first dispatch recovery.
- Preserve pre-existing code WIP in `cmd/tmuxicate/main.go`, `cmd/tmuxicate/main_test.go`, `internal/mailbox/target_store.go`, `internal/session/target.go`, `internal/session/target_test.go`, and `internal/session/up.go` unless explicitly asked otherwise.

### Blockers/Concerns

- Target state and dispatch persistence currently need stronger lock/atomic-write semantics before remote execution expands further.
- Delivery policy config exists beyond what runtime currently enforces, so operator expectations can drift from actual behavior.
- Daemon lifecycle ownership remains incomplete until `down` can stop the daemon and `up`/`serve` can prevent stale or duplicate daemons.
- Remote authenticated transport, worktree isolation, cross-run attention, and multi-coordinator topology remain intentionally deferred until v1.3 trust work lands.

## Session Continuity

Last session: 2026-04-27T04:55:27Z
Stopped at: Phase 13 Plan 01 complete; Plan 02 not started
Resume file: .planning/milestones/v1.3-phases/13-target-persistence-dispatch-safety/13-02-PLAN.md
