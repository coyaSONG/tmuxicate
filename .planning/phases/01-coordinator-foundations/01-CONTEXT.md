# Phase 1: Coordinator Foundations - Context

**Gathered:** 2026-04-05
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase delivers the durable foundation for coordinator-driven runs in `tmuxicate`. A human should be able to start a coordinator run from a high-level goal and have the system create reconstructable child tasks with explicit ownership and expected outputs. Review flow, blocker escalation, and richer summary behavior remain separate later phases.

</domain>

<decisions>
## Implementation Decisions

### Run initiation
- **D-01:** Coordinator runs should start through a dedicated CLI command rather than overloading the existing generic `send` flow.
- **D-02:** The initial command shape should support an explicit coordinator target, for example `tmuxicate run "goal..." --coordinator pm`, so coordinator-run semantics are distinct from plain mailbox messages.

### Child task schema
- **D-03:** Every child task must record `owner`, `goal`, `expected-output`, `depends-on`, `review-required`, and `parent-run-id`.
- **D-04:** Deadlines are intentionally out of scope for Phase 1; the first milestone should establish durable structure before adding time-based workflow policy.

### Routing baseline
- **D-05:** Phase 1 should use `role + teammate` metadata as the routing baseline rather than role-only assignment or unconstrained model inference.
- **D-06:** Freeform coordinator inference is not a foundation-phase behavior; routing should prefer explicit config relationships so the initial system stays predictable and debuggable.

### Operator visibility
- **D-07:** Phase 1 visibility should include the coordinator run tree, a compact state summary, and direct links or references back to the underlying messages/tasks.
- **D-08:** The foundation is not complete if operators must inspect raw transcripts to reconstruct who owns what.

### the agent's Discretion
- Exact CLI flag names beyond the dedicated run entrypoint
- Whether run/message references surface as message IDs, task IDs, or both, as long as they are durable and traceable
- Internal file layout for coordinator-run artifacts, provided it preserves mailbox authority and restart reconstruction

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase and scope
- `.planning/PROJECT.md` — Project scope, constraints, and the decision to build coordinator automation on top of the existing mailbox/task model
- `.planning/REQUIREMENTS.md` — Phase 1 requirement set, especially `PLAN-01`, `PLAN-02`, and `PLAN-03`
- `.planning/ROADMAP.md` — Phase 1 boundary, dependency order, and success criteria
- `.planning/STATE.md` — Current project position and known concerns affecting Phase 1

### Existing system context
- `.planning/codebase/ARCHITECTURE.md` — Current layering, durable state model, and integration boundaries
- `.planning/codebase/STACK.md` — Current Go/tmux/fsnotify/Cobra baseline and tooling constraints
- `.planning/codebase/CONCERNS.md` — Known reliability and testing gaps that Phase 1 must avoid amplifying
- `README.md` — Current user-facing workflow and product framing
- `DESIGN.md` — Product philosophy and coordinator pattern rationale already stated by the project

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/protocol/envelope.go` and `internal/protocol/receipt.go`: existing durable message and receipt schema that Phase 1 should extend or align with rather than bypass
- `internal/mailbox/store.go`: canonical persistence layer for immutable messages and mutable receipt state
- `internal/session/send.go` and `internal/session/task_cmd.go`: current command-side message creation and task lifecycle primitives
- `internal/session/status.go` and `internal/session/log_view.go`: operator-facing views that can later surface coordinator-run state

### Established Patterns
- Filesystem state is authoritative; `tmux` is the operator interface, not the message bus
- Session commands are thin orchestration functions under `internal/session/`
- Boundary packages (`internal/tmux`, `internal/adapter`) isolate infrastructure concerns through small interfaces and fakes
- Current code style favors explicit structs, wrapped errors, and durable on-disk artifacts over hidden in-memory workflow state

### Integration Points
- A new coordinator workflow layer will likely connect at the `internal/session/` command boundary and persist through `internal/mailbox/` plus `internal/protocol/`
- Existing agent role and teammate data comes from resolved config in `internal/config/`
- Operator visibility should eventually surface through `status`, logs, or related session views instead of a separate opaque dashboard

</code_context>

<specifics>
## Specific Ideas

- Prefer a dedicated command like `tmuxicate run "goal..." --coordinator pm` so the workflow is explicit from day one
- The first slice should feel like a structured orchestration layer, not a freeform swarm
- Operator-visible references back to underlying messages/tasks are important enough to treat as part of Phase 1, not an afterthought

</specifics>

<deferred>
## Deferred Ideas

- Review handoff behavior beyond the minimum child-task foundation — covered in Phase 3
- Blocker escalation and retry policy — covered in Phase 4
- Rich run summaries beyond the foundation visibility requirement — covered in Phase 5
- Smarter inference-based routing or adaptive learning — future milestone, not Phase 1

</deferred>

---
*Phase: 01-coordinator-foundations*
*Context gathered: 2026-04-05*
