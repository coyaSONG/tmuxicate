# Phase 8: Remote Execution Targets - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Extend coordinator dispatch so routed child tasks can target explicit remote or sandboxed execution environments in addition to today's local `tmux` panes, while keeping mailbox ownership, agent identity, and operator-visible placement evidence intact.

</domain>

<decisions>
## Implementation Decisions

### Target model
- **D-01:** Execution placement must be modeled explicitly and durably; it cannot remain an implied property of the local `tmux` pane layout.
- **D-02:** Task ownership and execution target are separate concerns. The existing owner/agent mailbox semantics remain canonical, while target metadata explains where that owner's work is expected to run.
- **D-03:** Local `tmux` behavior must remain the default path when no remote or sandbox target is configured.

### Compatibility and dispatch boundaries
- **D-04:** Phase 8 must preserve the existing mailbox protocol, message envelope/receipt flow, and agent adapter model.
- **D-05:** Remote execution support should extend current config, coordinator artifacts, and runtime seams rather than introduce a second orchestration backend.
- **D-06:** Dispatch decisions must remain deterministic and inspectable. Operators should be able to see why a task was placed on a specific target before or immediately when the route is persisted.

### Operator visibility
- **D-07:** Execution target capabilities must be visible from the same durable run/task surfaces already used for routing and blocker inspection.
- **D-08:** Placement evidence belongs on persisted coordinator artifacts and `run show` / `route-task` output, not on transcript-only or daemon-only state.

### the agent's Discretion
- Exact config shape for execution targets and capability metadata, as long as local-only configs continue to validate and behave the same.
- Whether target selection is expressed through agent-scoped default targets, explicit target catalogs, or both, as long as routing and operator inspection stay durable and explicit.

</decisions>

<specifics>
## Specific Ideas

- An operator should be able to answer “which environment will this task run in?” from the route result and `run show` without opening transcripts.
- The current `tmuxicate up` flow assumes every configured agent gets a local pane and adapter-backed readiness probe; remote-target support needs a bounded escape hatch instead of silently faking that assumption.
- Capability metadata should be concrete enough to support future routing or filtering work in Phase 9, but Phase 8 should not try to build the full timeline/filter feature yet.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope
- `.planning/PROJECT.md` — current milestone goal and non-negotiable compatibility constraints
- `.planning/REQUIREMENTS.md` — `EXEC-01` and `EXEC-02`
- `.planning/ROADMAP.md` — Phase 8 goal, dependency ordering, and success criteria

### Recently established coordinator patterns
- `.planning/phases/06-adaptive-routing-signals/06-02-SUMMARY.md` — additive routing evidence rendered from durable task YAML in `route-task` output and `run show`
- `.planning/phases/07-partial-replanning-flow/07-02-SUMMARY.md` — bounded workflow extension that preserved lineage without introducing a second backend
- `.planning/milestones/v1.0-phases/02-role-based-routing/02-02-SUMMARY.md` — routed task persistence, duplicate-safe routing decisions, and operator-visible route evidence

### Existing code seams
- `internal/config/config.go` and `internal/config/loader.go` — agent/session config model and validation rules
- `internal/session/up.go` — local pane/bootstrap generation and current assumption that every agent gets a tmux pane
- `internal/runtime/daemon.go` — adapter construction and unread-notify loop keyed to pane-backed agents
- `internal/session/run.go` — routing baseline, team snapshots, and child-task persistence
- `internal/protocol/coordinator.go` — canonical run/task/routing artifact types where placement metadata can become durable
- `internal/mailbox/coordinator_store.go` — YAML-backed coordinator artifact persistence patterns
- `internal/session/run_rebuild.go` and `cmd/tmuxicate/main.go` — existing operator-facing run inspection and route output surfaces
- `DESIGN.md` section 11.4 — filesystem trust model and current lack of host sandboxing

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `routingBaseline()` already snapshots allowed owners into durable run metadata; this is the natural place to include target-aware placement context.
- `RouteChildTask()` already persists structured `RoutingDecision` data and prints operator-facing placement feedback through existing CLI and run-rebuild surfaces.
- `CoordinatorStore` already owns durable workflow-side YAML artifacts, so target metadata can stay on the same coordinator graph instead of inventing a new state store.

### Current Gaps
- `session.Up()` and `runtime.buildAdapters()` are local-only today: they assume every configured agent maps to a local pane ID and tmux-backed adapter.
- `AgentSnapshot` and `ChildTask` do not currently carry execution-target or capability metadata, so `run show` cannot explain placement beyond owner identity.
- Config validation knows about agents, adapters, roles, and pane slots, but not about non-local execution targets or placement capabilities.

### Integration Points
- Config parsing and validation for target catalogs or agent-default target bindings
- Run snapshot construction so coordinator runs preserve target capabilities at creation time
- Route/add-task persistence so each child task records selected execution placement
- Operator-facing output in `run route-task` and `run show`
- Runtime/session startup boundaries where local panes should be skipped, adapted, or marked differently for non-local targets

</code_context>

<deferred>
## Deferred Ideas

- Full remote transport/orchestration beyond the current mailbox model
- Cross-target load balancing or automatic target scoring
- Timeline/filter visualization work that belongs in Phase 9

</deferred>

---

*Phase: 08-remote-execution-targets*
*Context gathered: 2026-04-11*
