# Phase 4: Blocker Escalation - Context

**Gathered:** 2026-04-06
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase delivers explicit, durable coordinator handling for blocked and waiting child tasks inside an existing run. Coordinator-run work must classify `wait` and `block` states into deterministic next actions, stop reroute loops at configured ceilings, escalate unresolved work to the human operator with concrete context and a recommended action, and expose task-local blocker chains through the existing `run show` surface. Run-level aggregate summaries remain Phase 5.

</domain>

<decisions>
## Implementation Decisions

### Blocker state model
- **D-01:** Keep `wait` and `block` as distinct declared task states. `wait` means the resume condition is already known; `block` means intervention is required to create the next step.
- **D-02:** Coordinator-run tasks must persist structured subtype data for blocker handling. `wait` requires `wait_kind`; `block` requires `block_kind`. Freeform `reason` text alone is not sufficient input for coordinator policy.
- **D-03:** Any case that requires human input is modeled as `block`, never `wait`.

### Coordinator action policy
- **D-04:** Phase 4 supports exactly four coordinator actions for blocker handling: `watch`, `clarification_request`, `reroute`, and `escalate`.
- **D-05:** The coordinator code selects the action through a deterministic policy table driven by structured blocker fields and durable attempt history. It must not infer actions from freeform text or model output.
- **D-06:** Ambiguous or unsupported blocker cases fail safe to `escalate` rather than falling back to hidden heuristics.

### Retry and reroute ceilings
- **D-07:** Reroute ceilings live in a dedicated blocker config surface: `blockers.max_reroutes_default` plus optional `blockers.max_reroutes_by_task_class` overrides.
- **D-08:** Reroute attempts are tracked on a dedicated `BlockerCase` artifact stored at `coordinator/runs/<run-id>/blockers/<source-task-id>.yaml`, keyed to the original blocked logical work rather than to a single owner or a broad `(task_class, domains)` aggregate.
- **D-09:** `watch` and `clarification_request` do not consume reroute budget.
- **D-10:** When `reroute_count >= resolved ceiling`, the next action is immediate `escalate`. There is no bonus reroute after the ceiling is reached.
- **D-11:** `delivery.max_retries` remains transport-level daemon policy and must not be reused for coordinator blocker reroute ceilings.

### Escalation artifact and operator response
- **D-12:** The canonical truth for human escalation is the `BlockerCase` artifact with `status=escalated`. Phase 4 must not add a human mailbox recipient model.
- **D-13:** Every escalated `BlockerCase` must include a structured `recommended_action` so the operator sees a concrete next step rather than only a failure report.
- **D-14:** The operator responds through a dedicated command: `tmuxicate blocker resolve <run-id> <source-task-id> --action manual_reroute|clarify|dismiss`.
- **D-15:** `blocker resolve` records the canonical resolution on the `BlockerCase` artifact and may reuse existing routing/mailbox primitives as side effects.

### Operator visibility boundary
- **D-16:** `tmuxicate run show <run-id>` renders blocker information as a derived block directly under the source task, matching the existing task-local review-handoff presentation pattern.
- **D-17:** Phase 4 blocker visibility is task-local only. Aggregate blocked/escalated counts, run-wide summary sections, and blocker-only read commands remain out of scope for this phase and belong in Phase 5.
- **D-18:** Phase 4 adds the `blocker resolve` write path only. It does not add a separate read-only command such as `blocker show` or `blockers list`.

### the agent's Discretion
- Exact Go type names, YAML field ordering, and helper-function names for `BlockerCase`, action history records, and resolution structs, as long as the semantics above remain intact.
- Exact CLI flag names for `blocker resolve`, provided the command still expresses `manual_reroute`, `clarify`, and `dismiss` explicitly.
- Exact `run show` label text and field ordering for blocker blocks, provided the output stays task-local, scan-friendly, and durable-artifact-backed.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and product constraints
- `.planning/PROJECT.md` — Product philosophy, human escalation constraint, and requirement to keep automation inspectable instead of autonomous-magic.
- `.planning/REQUIREMENTS.md` — `BLOCK-01`, `BLOCK-02`, and `BLOCK-03`, plus the separation from Phase 5 summary requirements.
- `.planning/ROADMAP.md` — Phase 4 goal, success criteria, and the explicit split between blocker escalation and later run summaries.
- `.planning/STATE.md` — Current project position and the reminder that runtime/session coverage gaps are still active concerns.

### Prior phase decisions
- `.planning/phases/01-coordinator-foundations/01-CONTEXT.md` — Durable coordinator artifact model and operator-visible lineage expectations that blocker handling must preserve.
- `.planning/phases/02-role-based-routing/02-CONTEXT.md` — Deterministic code-driven routing, fail-loud rejection policy, and explicit owner overrides that blocker rerouting must reuse.
- `.planning/phases/03-review-handoff-flow/03-CONTEXT.md` — Durable workflow-side artifacts, derived `run show` blocks, and fail-loud transition handling that blocker escalation should mirror.

### Existing design and codebase guidance
- `.planning/codebase/ARCHITECTURE.md` — Current layering, durable-state authority, and the `run show` rebuild boundary.
- `.planning/codebase/CONCERNS.md` — Existing retry-ceiling gaps, daemon fragility, and the need to avoid silent loops.
- `.planning/codebase/TESTING.md` — Established fake-based testing patterns and the need for direct `internal/session` coverage.
- `DESIGN.md` — Existing `task wait` / `task block` semantics, human-escalation guidance, and operator-visible workflow philosophy.
- `README.md` — Current operator workflow expectations and the product's reliability-over-magic framing.

### Relevant implementation surfaces
- `internal/session/task_cmd.go` — Current `task wait`, `task block`, and `task done` lifecycle hooks where structured blocker events and coordinator transitions will attach.
- `internal/session/run.go` — Deterministic `RouteChildTask`, owner override rules, and durable child-task creation flow reused by reroutes.
- `internal/session/run_rebuild.go` — Current durable `run show` rebuild path and task-local derived block rendering pattern.
- `internal/session/review_response.go` — Existing dedicated workflow-resolution command shape that Phase 4 should mirror for operator blocker resolution.
- `internal/mailbox/coordinator_store.go` — Canonical run/task/review artifact persistence boundary to extend with blocker-case CRUD.
- `internal/config/config.go` — Existing config surface where blocker-specific reroute ceiling settings should be added without reusing delivery settings.
- `internal/runtime/daemon.go` — Existing transport retry behavior that must remain separate from coordinator blocker ceilings.
- `cmd/tmuxicate/main.go` — Existing CLI wiring patterns for `run show`, `run route-task`, and workflow-specific subcommands.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/session/task_cmd.go`: already records task lifecycle state transitions and is the natural entrypoint for adding structured blocker subtype fields plus coordinator blocker handling hooks.
- `internal/session/run.go`: `RouteChildTask` already gives deterministic rerouting with explicit owner overrides and durable routing evidence.
- `internal/session/run_rebuild.go`: `LoadRunGraph` and `FormatRunGraph` already rebuild workflow-side artifacts from disk and render derived task-local blocks for operator inspection.
- `internal/session/review_response.go`: demonstrates the preferred pattern for a dedicated workflow-resolution command that validates durable artifacts and records a canonical outcome.
- `internal/mailbox/coordinator_store.go`: the existing durable coordinator artifact store is the correct persistence boundary for `BlockerCase` records.
- `internal/config/config.go` and `internal/config/loader.go`: already validate adjacent retry-related config, making them the right home for blocker-specific reroute ceiling config.

### Established Patterns
- Durable coordinator artifacts under `coordinator/runs/<run-id>/...` are authoritative; mailbox messages are workflow transport, not the sole source of truth.
- Workflow transitions are code-driven, deterministic, and fail loud instead of being delegated to prompt interpretation.
- `run show` remains task-centric and adds derived workflow context beneath source tasks rather than introducing separate dashboards for every workflow phase.
- The current recipient model is agent-config-based; there is no special human inbox and Phase 4 should not invent one.

### Integration Points
- Extend `task wait` / `task block` flows to require structured subtype data for coordinator-run tasks and trigger blocker-case creation or updates.
- Add `BlockerCase` persistence and validation under the coordinator artifact boundary alongside runs, tasks, and reviews.
- Extend run rebuild and formatting to load `coordinator/runs/<run-id>/blockers/*.yaml`, validate linkage to source/current tasks, and render the derived blocker block under the source task.
- Add `tmuxicate blocker resolve` as the operator resolution entrypoint, recording canonical blocker resolution while reusing `RouteChildTask` and existing mailbox send/reply flows as side effects.

</code_context>

<specifics>
## Specific Ideas

- Follow the Phase 3 pattern closely: durable workflow artifact first, `run show` derived block second.
- Treat `BlockerCase` as the canonical place to store reroute history, current owner, recommended action, and operator resolution outcome.
- Keep the operator surface task-local and explicit; do not pull Phase 5 summary work forward into Phase 4.
- Preserve the existing agent-recipient mailbox model; escalation is for the human operator but does not require a synthetic human inbox.

</specifics>

<deferred>
## Deferred Ideas

- Run-level blocked/escalated counters and aggregate summary sections — Phase 5
- Blocker-only list or read commands such as `tmuxicate blocker show` or `tmuxicate blockers list` — Phase 5 or later if still needed
- Broader run-summary UX that groups completed, waiting, blocked, review, and escalated work into one operator-facing report — Phase 5

</deferred>

---
*Phase: 04-blocker-escalation*
*Context gathered: 2026-04-06*
