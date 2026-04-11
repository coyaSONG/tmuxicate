# Phase 2: Role-Based Routing - Context

**Gathered:** 2026-04-05
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase delivers deterministic coordinator routing for child tasks inside a run. The coordinator should choose suitable agents from declared teammate relationships and structured role metadata, prevent accidental duplicate execution, and fail loudly with inspectable reasons when no safe route exists. Review handoff, blocker escalation, load balancing, and rich run summaries remain outside this phase.

</domain>

<decisions>
## Implementation Decisions

### Role matching
- **D-01:** Replace the freeform routing role string with `RoleSpec{Kind, Domains, Description}` so `Kind` and `Domains` become the authoritative routing inputs while `Description` remains operator-facing context.
- **D-02:** Introduce a structured `TaskClass` for routing intent. `TaskClass` is distinct from `protocol.Kind`, which remains a message/workflow kind rather than a routing signal.
- **D-03:** Add a dedicated `RouteChildTask` policy step that matches candidates from `RoleSpec.Kind + Domains`. `AddChildTask` stays an explicit-owner persistence writer rather than becoming the routing engine.
- **D-04:** Routing decisions must be code-driven rather than model-driven so the same config and run state produce the same candidate set across vendors.

### Tie-breaking
- **D-05:** Once role/domain filtering and duplicate policy have passed, owner selection must be strictly deterministic for the same config and run state.
- **D-06:** Tie-breaking order is `route_priority` descending, then config declaration order ascending.
- **D-07:** Load balancing and round-robin state are explicitly out of scope for Phase 2.

### Duplicate safeguards
- **D-08:** Duplicate identity is `(run_id, task_class, normalized_domains)`. `owner` is intentionally excluded so the same work cannot be sent to multiple agents by accident.
- **D-09:** Duplicate policy is defined on `TaskClass`, not on `protocol.Kind`.
- **D-10:** `RouteChildTask` must block duplicates before owner selection, and `AddChildTask` must repeat the check before persistence so direct CLI calls and race windows still fail safely.
- **D-11:** `fanout_task_classes` represent normal, policy-approved parallel routing. `exclusive_task_classes` permit only one active task per duplicate key.
- **D-12:** Any duplicate-policy override requires an explicit reason.
- **D-13:** The default policy for `research` remains intentionally undecided in discussion; implementation should keep that behavior explicitly configurable instead of hard-coding hidden heuristics.

### No-match behavior
- **D-14:** No-match is fail-loud. Routing must not automatically retry by dropping domains or widening to arbitrary teammates.
- **D-15:** `OwnerOverride` may bypass role/domain no-match only with an explicit reason, and it still must respect teammate boundaries and duplicate safeguards.
- **D-16:** Routing failures must return structured coordinator-facing data rather than plain text only. The rejection payload should include the requested `TaskClass`, requested domains, kind-level eligible candidates, allowed owners, and retry suggestions.

### Routing observability
- **D-17:** `RoutingDecision` must include duplicate status plus structured tie-break evidence so an operator can inspect why a route was accepted, blocked, or chosen.
- **D-18:** Routing artifacts should preserve the candidate set and winner rationale in durable run/task state rather than requiring transcript reconstruction.

### the agent's Discretion
- Exact Go type names and YAML field names for `RouteChildTask`, `RoutingDecision`, tie-break detail structs, and rejection structs, as long as the semantics above remain intact.
- Whether routing helpers stay inside `internal/session/` or move into a narrowly scoped coordinator policy package, as long as the existing Go CLI architecture and durable mailbox model remain authoritative.
- The exact normalization routine for domains, provided it is deterministic, documented, and stable in tests.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and product constraints
- `.planning/PROJECT.md` — Project philosophy, constraints, and the requirement to keep routing reliable, inspectable, and multi-vendor.
- `.planning/REQUIREMENTS.md` — `ROUTE-01` and `ROUTE-02`, plus adjacent review/blocker requirements that Phase 2 must not accidentally absorb.
- `.planning/ROADMAP.md` — Phase 2 goal, success criteria, and dependency order.
- `.planning/STATE.md` — Current project state and the reminder that session/runtime coverage gaps matter when adding coordinator automation.

### Prior phase decisions
- `.planning/phases/01-coordinator-foundations/01-CONTEXT.md` — Phase 1 decisions that routing must build on, especially the explicit `role + teammate` baseline and durable operator visibility expectations.

### Codebase guidance
- `.planning/codebase/ARCHITECTURE.md` — Existing layering, durable-state authority, and command/session boundaries that routing should extend instead of bypassing.
- `.planning/codebase/CONCERNS.md` — Current reliability and testing gaps that make fail-loud routing and direct session-package coverage important.
- `.planning/codebase/TESTING.md` — Established test patterns and the uncovered `internal/session` package that Phase 2 should target directly.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config/config.go` and `internal/config/loader.go`: existing routing config surface and validation boundary where `RoleSpec`, `RoutePriority`, `exclusive_task_classes`, and `fanout_task_classes` can be introduced and checked.
- `internal/protocol/coordinator.go` and `internal/protocol/validation.go`: canonical coordinator run/task records and validation hooks that can carry `TaskClass`, routing decisions, and rejection metadata.
- `internal/session/run.go`: current routing baseline snapshot, explicit-owner `AddChildTask`, and durable message creation flow that Phase 2 should extend with `RouteChildTask`.
- `internal/mailbox/coordinator_store.go`: durable coordinator artifact writer that can remain the persistence boundary after routing chooses an owner.
- `internal/session/run_test.go` and `internal/session/run_rebuild_test.go`: direct, local tests that already pin coordinator artifacts and can be extended for routing, duplicate rejection, and inspectable decisions.

### Established Patterns
- Filesystem artifacts and validated structs are the source of truth; routing should remain explicit in durable state rather than hidden in prompts.
- Session-layer orchestration in `internal/session/` is the current workflow boundary, with config/protocol/mailbox packages enforcing invariants underneath it.
- The codebase prefers explicit validation and fail-fast errors over silent coercion, which aligns with no-match hard failures and duplicate blocking.
- Tests rely on temp dirs and direct package calls rather than live tmux or LLM behavior, so routing policy should stay fully unit-testable.

### Integration Points
- `cmd/tmuxicate/main.go` currently exposes `run add-task --owner ...`; Phase 2 can add a routing-aware entry path while keeping explicit-owner writes available beneath it.
- `Run` currently snapshots `allowed_owners` and `team_snapshot`; `RouteChildTask` should consume that durable routing baseline rather than recomputing team membership from transcripts.
- `LoadRunGraph` and related run inspection output can later surface routing decisions, duplicate status, and rejection reasons without adding a second observability channel.

</code_context>

<specifics>
## Specific Ideas

- Routing should use structured metadata as the authoritative input and keep prose descriptions explanatory rather than decisive.
- Fail-loud behavior is preferred over graceful-but-opaque degradation for no-match and duplicate scenarios.
- Duplicate safeguards should reason about task intent, not message transport kind.
- Operators should be able to understand routing outcomes from durable artifacts alone, including why a candidate was selected, blocked, or overridden.

</specifics>

<deferred>
## Deferred Ideas

- Load balancing by active task count — valuable but outside Phase 2 because it weakens strict determinism.
- Round-robin routing state — deferred because it adds extra mutable coordinator state for limited benefit in a 2-5 agent team.
- Automatic no-match fallback by dropping domains or widening to any teammate — rejected for Phase 2 and should only return if product philosophy changes.
- Goal-text similarity as a duplicate heuristic — deferred because it would reintroduce model-dependent routing behavior.

</deferred>

---
*Phase: 02-role-based-routing*
*Context gathered: 2026-04-05*
