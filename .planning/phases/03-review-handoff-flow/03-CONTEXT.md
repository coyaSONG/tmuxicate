# Phase 3: Review Handoff Flow - Context

**Gathered:** 2026-04-05
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase delivers a durable, inspectable review handoff inside an existing coordinator run. When review is required, completed implementation work should transition into a linked review task without losing run/task/message lineage, and the resulting reviewer response should be reconstructable from disk and visible through the existing operator inspection surface. Follow-up implementation branching, blocker/escalation policy, and richer summary UX remain separate later work.

</domain>

<decisions>
## Implementation Decisions

### Review creation trigger
- **D-01:** A `review_required=true` implementation task must trigger review-task creation automatically in code immediately after the implementation task's durable `done` transition is recorded.
- **D-02:** Automatic handoff must reuse `RouteChildTask(TaskClass=review)` and must not depend on coordinator prompt parsing, coordinator message reading, or implementer-authored ad hoc review requests.
- **D-03:** If review handoff creation fails, the original implementation task remains `done`; the system records a linked fail-loud handoff failure instead of rolling back completion.
- **D-04:** Duplicate review handoffs are prevented by a source-task linkage check, not by Phase 2's review fanout duplicate-key policy.

### Link model
- **D-05:** The canonical review-chain link is a dedicated `ReviewHandoff` artifact stored at `coordinator/runs/<run-id>/reviews/<source-task-id>.yaml`.
- **D-06:** Source implementation tasks and review tasks must not store redundant reverse-pointer fields to each other; the dedicated handoff artifact is the sole canonical linkage record.
- **D-07:** Review-response linkage and final review outcome must be recorded on the same `ReviewHandoff` artifact.
- **D-08:** Handoff uniqueness is enforced by the existence of `reviews/<source-task-id>.yaml`, not by scanning for generic review tasks or relying on duplicate-key semantics.

### Review outcome handling
- **D-09:** Both `approved` and `changes_requested` outcomes leave the source implementation task in its existing `done` state; review outcome does not reopen or retag the implementation task in Phase 3.
- **D-10:** Phase 3 scope ends at durable review-outcome recording plus operator visibility. Automatic follow-up implementation-task generation for `changes_requested` is explicitly out of scope.
- **D-11:** Reviewer outcome is submitted through a dedicated review-response CLI surface rather than by extending generic `task done` with outcome semantics.
- **D-12:** When a reviewer responds, `ReviewHandoff` records `response_message_id`, `outcome`, `responded_at`, and `status=responded`.

### Operator inspection surface
- **D-13:** Review-chain visibility is integrated into the existing `tmuxicate run show` output; each source implementation task renders a derived review-handoff block directly underneath the task.
- **D-14:** Review tasks remain visible as normal child tasks in the regular task list; the derived handoff block is an additional linkage view, not a replacement task view.
- **D-15:** The minimum review-handoff information shown to operators is `status`, `review_task_id`, reviewer owner, `response_message_id`, `outcome`, and a failure summary when routing/handoff failed.
- **D-16:** Phase 3 does not add a separate review-only command or filter such as `run show --reviews-only`; the existing `run show` surface remains the only required inspection entrypoint.

### the agent's Discretion
- Exact Go type names and helper-function names for `ReviewHandoff`, review-response request structs, and rebuild helpers, as long as the semantics above remain intact.
- Exact CLI flag names for the dedicated review-response command, provided the interface is clearly review-specific and does not overload generic `task done`.
- Exact YAML field ordering and formatting for review-handoff artifacts, provided they remain durable, validated, and readable from disk.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and product constraints
- `.planning/PROJECT.md` — Product philosophy, constraints, and active requirement for coordinator-managed review handoff without hiding workflow state.
- `.planning/REQUIREMENTS.md` — `REVIEW-01` and `REVIEW-02`, plus adjacent blocker/summary requirements that must not be absorbed into Phase 3.
- `.planning/ROADMAP.md` — Phase 3 goal, success criteria, and dependency order.
- `.planning/STATE.md` — Current project position and reminder that session/runtime coverage gaps matter for coordinator automation.

### Prior phase decisions
- `.planning/phases/01-coordinator-foundations/01-CONTEXT.md` — Durable coordinator-run/task artifact model and operator-visible lineage expectations established in Phase 1.
- `.planning/phases/02-role-based-routing/02-CONTEXT.md` — Deterministic `RouteChildTask`, explicit review fanout semantics, and durable routing evidence established in Phase 2.
- `.planning/phases/02-role-based-routing/02-VERIFICATION.md` — Verified Phase 2 behavior for routing, duplicate safeguards, and operator-visible task/routing metadata that Phase 3 must build on.

### Existing design and codebase guidance
- `DESIGN.md` — Mailbox message/thread authority, `review_request`/`review_response` kinds, and coordinator guidance to use focused review after implementation work.
- `.planning/codebase/ARCHITECTURE.md` — Current layering, durable-state authority, and `run show`/rebuild boundaries.
- `.planning/codebase/CONCERNS.md` — Reliability and session/runtime fragility concerns that make fail-loud handoff behavior and direct tests important.
- `.planning/codebase/TESTING.md` — Existing fake-based testing patterns and the need to add direct `internal/session` coverage for new orchestration behavior.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/session/task_cmd.go`: `TaskDone` already owns the implementation-task completion transition and is the natural hook for automatic review handoff after durable `done`.
- `internal/session/run.go`: `RouteChildTask`, `AddChildTask`, duplicate-policy helpers, and child-task body construction already provide the deterministic review-task creation seam.
- `internal/session/reply.go`: parent-kind-aware reply behavior already maps `review_request -> review_response`, which is the right message seam for reviewer feedback.
- `internal/session/run_rebuild.go`: `LoadRunGraph` and `FormatRunGraph` already rebuild and render run/task lineage from disk, making them the natural place to rehydrate and display review handoffs.
- `internal/mailbox/coordinator_store.go`: existing run/task artifact persistence is the right boundary to extend with review-handoff CRUD helpers instead of creating a second storage path.

### Established Patterns
- Filesystem artifacts under coordinator run directories are authoritative; mailbox messages and receipts are transport evidence, not the only workflow truth.
- Routing and ownership decisions are code-driven, deterministic, and persisted on task artifacts rather than inferred from model prompts.
- Threading is derived from message `thread` plus `reply_to`; new review linkage should validate against these fields instead of creating a second thread authority.
- Operator inspection prefers one durable view (`run show`) over requiring transcript reconstruction or hidden in-memory workflow state.

### Integration Points
- Extend `TaskDone` to load the source child task, check `review_required`, create/read the `ReviewHandoff` artifact, and invoke `RouteChildTask(TaskClass=review)` after the `done` transition is durable.
- Add review-handoff persistence and validation under the coordinator-store boundary, likely alongside run/task artifact paths.
- Add a dedicated review-response CLI that writes the `review_response` mailbox message and updates the linked `ReviewHandoff` artifact in one workflow.
- Extend run rebuild and formatting to read `coordinator/runs/<run-id>/reviews/*.yaml`, validate source/review/response linkage, and render the derived handoff block beneath the source task.

</code_context>

<specifics>
## Specific Ideas

- Keep review handoff code-driven and prompt-independent so the same run/task state produces the same review behavior across vendors.
- Treat `reviews/<source-task-id>.yaml` as the one canonical source of review-chain truth; do not spread reverse pointers across multiple task records.
- Keep `run show` task-centric: source implementation task first, then an indented handoff summary, while the review task also remains visible as a regular child task.
- Failures to route or create review handoff must remain inspectable from disk and must not silently downgrade into "review was skipped."

</specifics>

<deferred>
## Deferred Ideas

- Automatic creation of follow-up implementation tasks when review outcome is `changes_requested`.
- Review-outcome-driven workflow branching, retry, or escalation policy.
- Separate review-only inspection commands or filters such as `run show --reviews-only`.
- Any richer lifecycle state beyond current task `done` plus handoff/outcome metadata, such as introducing a new `reviewed` task state for implementation tasks.

</deferred>

---

*Phase: 03-review-handoff-flow*
*Context gathered: 2026-04-05*
