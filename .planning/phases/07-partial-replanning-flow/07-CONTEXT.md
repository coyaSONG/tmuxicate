# Phase 7: Partial Replanning Flow - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Allow coordinator runs to replace blocked work with a bounded partial replan that preserves durable lineage to the original task and blocker case, without introducing autonomous long-horizon replanning or hiding operator control.

</domain>

<decisions>
## Implementation Decisions

### Replan scope
- **D-01:** Partial replans must stay local to the blocked work they replace; they are not a license to reshape the whole coordinator run.
- **D-02:** Replacement or follow-up work must remain explicitly tied to the original source task, blocker case, and any superseded task nodes.

### Durable lineage
- **D-03:** Replan state must live in durable coordinator artifacts and rebuild through existing run inspection flows, not as runtime-only orchestration logic.
- **D-04:** Operators need to see which task was superseded, which replacement tasks were introduced, and why the replan happened from the same run graph.

### Recovery behavior
- **D-05:** Partial replanning extends the current blocker workflow; it does not replace explicit reroute ceilings, escalation, or operator resolution.
- **D-06:** Replanning remains bounded and deterministic. When the coordinator cannot recover safely, it must escalate instead of recursively re-planning.

### the agent's Discretion
- Exact artifact shape for replacement/replan lineage, as long as it is validated, durable, and readable from existing operator surfaces.
- Whether a partial replan is represented as one replacement task, a mini-plan artifact, or both, as long as the lineage and boundedness rules stay explicit.

</decisions>

<specifics>
## Specific Ideas

- The operator should be able to answer “what replaced this blocked task?” from `run show` without transcript review.
- Replan behavior should feel like an explicit extension of blocker resolution rather than a hidden orchestration loop.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope
- `.planning/PROJECT.md` — current milestone goal and non-negotiable operator-visibility constraints
- `.planning/REQUIREMENTS.md` — `REPLAN-01` and `REPLAN-02`
- `.planning/ROADMAP.md` — phase goal, dependency ordering, and success criteria

### Existing blocker and lineage behavior
- `.planning/milestones/v1.0-phases/04-blocker-escalation/04-01-SUMMARY.md` — blocker artifact contract and reroute ceiling expectations
- `.planning/milestones/v1.0-phases/04-blocker-escalation/04-02-SUMMARY.md` — deterministic blocker handling and `blocker resolve` operator workflow
- `.planning/milestones/v1.0-phases/04-blocker-escalation/04-03-SUMMARY.md` — task-local blocker rendering in `run show`
- `.planning/milestones/v1.0-phases/03-review-handoff-flow/03-02-SUMMARY.md` — durable follow-up lineage patterns already used for review handoffs
- `.planning/phases/06-adaptive-routing-signals/06-01-SUMMARY.md` — durable coordinator preference artifacts and root-only refresh pattern
- `.planning/phases/06-adaptive-routing-signals/06-02-SUMMARY.md` — additive routing evidence rendered from task YAML in operator surfaces

### Existing code seams
- `internal/session/task_cmd.go` — blocker handling and reroute action selection
- `internal/session/blocker_resolve.go` — operator-driven blocker resolutions
- `internal/session/run.go` — child-task creation, routing, and durable task persistence
- `internal/session/run_rebuild.go` — current run-graph reconstruction and operator rendering
- `internal/protocol/coordinator.go` — canonical run/task/blocker/review artifact types
- `internal/mailbox/coordinator_store.go` — YAML-backed coordinator artifact persistence patterns

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/session/task_cmd.go`: already owns automatic blocker action selection, reroute, and escalation paths
- `internal/session/blocker_resolve.go`: explicit operator-side recovery actions and blocker lifecycle mutations
- `internal/session/run_rebuild.go`: existing run graph and task-local workflow rendering surface where replan lineage can appear
- `internal/mailbox/coordinator_store.go`: coordinator artifact CRUD layer suitable for replacement/replan records

### Established Patterns
- Workflow-side state is persisted as dedicated coordinator artifacts and then rebuilt from disk into `RunGraph`.
- Blocker handling is source-task keyed and bounded by explicit ceilings; operator resolution remains final authority.
- Additive operator evidence is preferred over new top-level reporting commands.

### Integration Points
- `task wait` / `task block` policy flow where a replan action could be selected
- `blocker resolve` where operators may accept or trigger a bounded partial replan
- task persistence and `run show` rendering so superseded/replacement lineage survives restarts

</code_context>

<deferred>
## Deferred Ideas

- Run-wide replanning or multi-stage autonomous plan rewriting
- Cross-run optimization of replan strategies
- Timeline-specific visualization work that belongs in Phase 9

</deferred>

---

*Phase: 07-partial-replanning-flow*
*Context gathered: 2026-04-11*
