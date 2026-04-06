# Phase 5: Run Summaries - Context

**Gathered:** 2026-04-06
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase delivers a trustworthy run-level summary for coordinator runs. Operators should be able to see completed, pending, waiting, blocked, under-review, and escalated logical work in one place without spelunking transcripts, while the existing task-local `run show` detail remains available underneath. The summary is a derived view over existing durable artifacts only; Phase 5 does not introduce new workflow artifacts, new state machines, or new coordinator automation.

</domain>

<decisions>
## Implementation Decisions

### Summary entrypoint
- **D-01:** Phase 5 summary output lives at the top of `tmuxicate run show` as a new summary section rather than as a new primary command or a flag-only alternate output mode.
- **D-02:** The summary is additive and does not replace the existing `run show` task-by-task detail view.
- **D-03:** Summary generation must reuse the existing `RunGraph` rebuild path instead of introducing a second aggregation model.

### Logical status derivation
- **D-04:** Summary status for a logical work item uses this precedence order: `escalated` > `blocked` > `waiting` > `under_review` > `completed`.
- **D-05:** A source task that is `done` with a pending review handoff is reported as `under_review`.
- **D-06:** A source task that is `done` with review outcome `changes_requested` is still reported as `under_review`; the outcome is shown explicitly on the summary item instead of inventing a new top-level `needs_work` bucket.
- **D-07:** A blocked task with an escalated blocker case is reported as `escalated`.
- **D-08:** Waiting work remains its own summary status and must not be folded into blocked or pending.

### Logical work item identity
- **D-09:** Each logical work item appears exactly once in the summary, anchored to the source task.
- **D-10:** Review tasks, rerouted current tasks, and blocker cases contribute metadata to the source task’s single summary row rather than appearing as separate rows.

### Summary row density
- **D-11:** Summary rows use medium density: derived status, responsible/current owner, source goal, and key references on the main line, with outcome or recommended action shown only when relevant.
- **D-12:** Phase 5 summary rows must stay meaningfully shorter than the existing full `run show` detail; the detailed task-local blocks remain the place for exhaustive workflow context.

### Summary timing
- **D-13:** Operators can access the summary on demand through `run show`.
- **D-14:** The system also prints the same run summary once when a coordinator run reaches completion so end-of-run output is visible without a second manual step.

### Scope boundary
- **D-15:** Phase 5 stops at a derived operator view over existing run, task, review, and blocker artifacts.
- **D-16:** New summary artifacts, new run-summary state machines, JSON/API summary formats, historical snapshots, filters, and follow-up automation remain out of scope for this phase.

### the agent's Discretion
- Exact section labels, bucket ordering, and ASCII formatting of the summary block, as long as the derived statuses and one-row-per-logical-item model remain intact.
- Exact field labels for message, task, review, and blocker references, as long as operators can trace each summary item back to durable artifacts.
- Exact detection of the one-time completion print hook, as long as it does not create a new durable summary truth source.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and product constraints
- `.planning/PROJECT.md` — Product philosophy, operator-visibility requirement, and the rule that automation stays explicit and inspectable.
- `.planning/REQUIREMENTS.md` — `SUM-01` and `SUM-02`, plus the adjacent blocker/review requirements that Phase 5 must summarize rather than reimplement.
- `.planning/ROADMAP.md` — Phase 5 goal, success criteria, and the explicit separation between summary aggregation and prior workflow phases.
- `.planning/STATE.md` — Current milestone position and the note that session/runtime test gaps still matter for coordinator surfaces.

### Prior phase decisions
- `.planning/phases/01-coordinator-foundations/01-CONTEXT.md` — Durable run/task artifacts and operator-facing lineage expectations that summaries must preserve.
- `.planning/phases/02-role-based-routing/02-CONTEXT.md` — Routing evidence and task metadata already persisted on task artifacts.
- `.planning/phases/03-review-handoff-flow/03-CONTEXT.md` — Review handoff artifact model and task-local review rendering that summaries must collapse into a run-level view.
- `.planning/phases/04-blocker-escalation/04-CONTEXT.md` — Blocker-case artifact model, escalation semantics, and task-local blocker rendering that summaries must reuse.

### Existing design and codebase guidance
- `DESIGN.md` — Overall reliability-over-magic philosophy and the operator-facing command model that Phase 5 must extend rather than replace.
- `README.md` — Current user-facing command expectations and operator workflow framing.
- `.planning/codebase/ARCHITECTURE.md` — Current layering, durable-state authority, and `run show` as the rebuild/inspection boundary.
- `.planning/codebase/CONCERNS.md` — Large command/session surfaces and missing session/runtime coverage that make narrow summary integration preferable to new orchestration paths.
- `.planning/codebase/TESTING.md` — Existing fake-based testing patterns and the need to add direct coverage in `internal/session`.

### Relevant implementation surfaces
- `internal/session/run_rebuild.go` — Existing `LoadRunGraph` and `FormatRunGraph` flow that should host the derived summary projection.
- `cmd/tmuxicate/main.go` — Current `run show` entrypoint that should continue to own the operator-facing command surface.
- `internal/protocol/coordinator.go` — Canonical task, review, and blocker structs whose existing fields drive derived summary status.
- `internal/mailbox/coordinator_store.go` — Durable artifact reader boundary for runs, tasks, reviews, and blockers.
- `internal/session/run_rebuild_test.go` — Existing run-show tests and the natural place to add summary assertions.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/session/run_rebuild.go`: already reconstructs a full `RunGraph` from durable run, task, review, and blocker artifacts.
- `internal/session/run_rebuild.go`: `FormatRunGraph` already renders task-local review and blocker detail, making it the right seam for a summary section plus shared formatter helpers.
- `internal/protocol/coordinator.go`: existing `ChildTask`, `ReviewHandoff`, and `BlockerCase` fields already contain the data needed to derive run-level summary status without new persistence.
- `internal/session/run_rebuild_test.go`: fixture builders already seed combined run/task/review/blocker scenarios that can be reused for summary-focused tests.

### Established Patterns
- `run show` is the existing durable inspection surface; workflow phases have consistently added derived detail there instead of creating separate read commands.
- Coordinator workflow truth lives on disk under run/task/review/blocker artifacts; projections are rebuilt from those artifacts rather than cached separately.
- Task-local workflow context stays visible under each source task, so Phase 5 should add aggregation above that detail rather than flattening or replacing it.

### Integration Points
- Add a summary projection step inside `FormatRunGraph` or a closely related helper, fed by the existing `RunGraph`.
- Derive one logical summary item per source task by combining task state with linked review handoff and blocker case data.
- Hook one-time completion printing into the existing coordinator/operator workflow without introducing a second summary truth source or a new command family.

</code_context>

<specifics>
## Specific Ideas

- Summary is an aggregate view; `run show` detail remains the task-local truth surface underneath.
- Status precedence is explicit and must stay deterministic: `escalated` > `blocked` > `waiting` > `under_review` > `completed`.
- `changes_requested` is not a new top-level bucket; it is shown as `under_review` with the outcome surfaced on the item.
- Small-team operator ergonomics matter more than generic reporting flexibility, so one logical item per source task is preferred over listing every workflow artifact separately.

</specifics>

<deferred>
## Deferred Ideas

- Separate `run summary` command or alternate summary-only output mode.
- Persisted summary snapshots, JSON output, filtering, sorting, or historical reports.
- New workflow automation triggered from summary outcomes, such as auto-generating follow-up work after `changes_requested`.

</deferred>

---

*Phase: 05-run-summaries*
*Context gathered: 2026-04-06*
