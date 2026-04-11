# Phase 6: Adaptive Routing Signals - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Persist adaptive routing signals from prior coordinator runs and expose the reasoning behind adaptive owner selection without replacing the current deterministic `route-task` workflow or mailbox-backed task graph.

</domain>

<decisions>
## Implementation Decisions

### Adaptive signal storage
- **D-01:** Adaptive routing data must be stored as durable coordinator artifacts tied to existing run/task lineage, not as daemon-only memory or a separate orchestration backend.
- **D-02:** Adaptive signals must augment the existing routing model (`RoleSpec`, domains, route priority, duplicate policy), not bypass it.

### Routing behavior
- **D-03:** Baseline deterministic routing remains authoritative; adaptive inputs may influence ranking but must never make the final owner selection opaque.
- **D-04:** Every adaptive routing decision must preserve an inspectable explanation of what historical evidence or preference shifted the outcome.

### Operator visibility
- **D-05:** Operators need to inspect both the persisted adaptive preference inputs and the decision-time explanation from existing run inspection surfaces rather than transcript-only output.
- **D-06:** Any new routing evidence should compose with current `run show` / routing evidence patterns instead of creating a disconnected reporting command first.

### the agent's Discretion
- Exact scoring or weighting model for adaptive inputs, as long as the result stays deterministic and explainable.
- Whether adaptive preference state is attached to runs, tasks, agents, or a dedicated coordinator preference artifact, as long as mailbox compatibility and inspectability are preserved.

</decisions>

<specifics>
## Specific Ideas

- Adaptive routing should feel like an extension of the current routing evidence model, not a second “AI decides” subsystem.
- Preference learning is milestone scope; fully automatic self-tuning without operator-readable evidence is not.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope
- `.planning/PROJECT.md` — current milestone goal, active requirements, and product-level constraints
- `.planning/REQUIREMENTS.md` — `ADAPT-01` and `ADAPT-02` scope for this phase
- `.planning/ROADMAP.md` — phase goal, dependency ordering, and success criteria

### Existing routing behavior
- `.planning/milestones/v1.0-phases/02-role-based-routing/02-01-SUMMARY.md` — structured routing metadata, deterministic ranking, and `route-task` operator contract
- `.planning/milestones/v1.0-phases/02-role-based-routing/02-02-SUMMARY.md` — duplicate-safe routing evidence and persisted routing decision expectations
- `internal/session/run.go` — current `RouteChildTask` selection and routing evidence persistence
- `internal/protocol/coordinator.go` — routed task artifacts and routing-decision protocol surface
- `internal/config/config.go` — `RoleSpec`, `route_priority`, and routing config structure

### Existing operator visibility
- `.planning/milestones/v1.0-phases/05-run-summaries/05-01-SUMMARY.md` — derived operator summary/read-model constraints
- `.planning/milestones/v1.0-phases/05-run-summaries/05-02-SUMMARY.md` — current operator-facing summary surfaces
- `internal/session/run_rebuild.go` — `run show` rebuild and formatting surface

[If the project has no external specs: "No external specs — requirements are fully captured in decisions above"]

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/session/run.go`: current routing path, duplicate safeguards, and persisted routing evidence fields
- `internal/session/run_rebuild.go`: existing operator-visible run inspection surface where adaptive explanations can be rendered
- `internal/protocol/coordinator.go`: canonical place for new durable coordinator artifact types or routing evidence fields
- `internal/mailbox/coordinator_store.go`: existing durable YAML-backed coordinator storage patterns

### Established Patterns
- Routing decisions are deterministic and explicitly explain tie-break behavior (`route_priority desc, config_order asc`).
- Operator-visible workflow evidence is persisted to canonical task/run artifacts and then rebuilt from disk instead of cached separately.
- New coordinator behavior is expected to land with direct `internal/session` test coverage and fail-loud validation.

### Integration Points
- `tmuxicate run route-task` owner selection and routing-decision persistence
- coordinator run/task artifact storage under `.tmuxicate/.../coordinator/runs/`
- `run show` formatting paths that already display routing, review, blocker, and summary evidence

</code_context>

<deferred>
## Deferred Ideas

- Cross-run self-tuning or autonomous preference updates without explicit operator controls
- Multi-coordinator or nested-team routing heuristics
- Timeline/dashboard-specific visualizations beyond what Phase 9 will handle

</deferred>

---

*Phase: 06-adaptive-routing-signals*
*Context gathered: 2026-04-11*
