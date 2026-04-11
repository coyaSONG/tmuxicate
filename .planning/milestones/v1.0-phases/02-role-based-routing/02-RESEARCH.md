# Phase 02: Role-Based Routing - Research

**Researched:** 2026-04-05
**Domain:** Deterministic coordinator routing over structured role metadata, teammate boundaries, and duplicate-assignment safeguards inside the existing Go CLI/mailbox architecture.
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Replace the freeform routing role string with `RoleSpec{Kind, Domains, Description}` so `Kind` and `Domains` are authoritative routing inputs while `Description` remains operator-facing context.
- **D-02:** Introduce a structured `TaskClass` for routing intent. It is distinct from `protocol.Kind`, which remains a mailbox/message kind.
- **D-03:** Add a dedicated `RouteChildTask` policy step that matches candidates from `RoleSpec.Kind + Domains`. `AddChildTask` stays an explicit-owner persistence writer.
- **D-04:** Routing decisions must be code-driven rather than model-driven so the same config and run state produce the same candidate set.
- **D-05:** Tie-breaking order is `route_priority` descending, then config declaration order ascending.
- **D-06:** Load balancing and round-robin state are out of scope for Phase 2.
- **D-08:** Duplicate identity is `(run_id, task_class, normalized_domains)` and intentionally excludes owner.
- **D-09:** Duplicate policy is defined on `TaskClass`, not on `protocol.Kind`.
- **D-10:** `RouteChildTask` must block duplicates before owner selection, and `AddChildTask` must repeat the check before persistence so direct CLI calls and race windows still fail safely.
- **D-11:** `fanout_task_classes` are the only normal path for parallel routing. `exclusive_task_classes` permit only one active task per duplicate key.
- **D-12:** Any duplicate-policy override requires an explicit reason.
- **D-13:** The default policy for `research` remains configurable rather than hidden in code.
- **D-14:** No-match is fail-loud. Routing must not silently widen candidates by dropping domains or broadening to arbitrary teammates.
- **D-15:** `OwnerOverride` may bypass role/domain no-match only with an explicit reason, and it still must respect teammate boundaries and duplicate safeguards.
- **D-16:** Routing failures must return structured coordinator-facing data rather than plain text only.
- **D-17:** `RoutingDecision` must include duplicate status plus structured tie-break evidence.
- **D-18:** Routing artifacts should preserve the candidate set and winner rationale in durable run/task state rather than requiring transcript reconstruction.

### The Agent's Discretion
- Exact Go type names and YAML field names for route requests, routing decisions, tie-break evidence, and rejection payloads.
- Whether routing helpers remain in `internal/session/` or move into a narrow coordinator policy file/package, provided the CLI/session/mailbox layering stays intact.
- The exact domain normalization routine, as long as it is deterministic, documented, and covered by tests.

### Deferred Ideas (OUT OF SCOPE)
- Load balancing by active task count.
- Round-robin routing state.
- Automatic no-match fallback by dropping domains or widening to arbitrary teammates.
- Goal-text similarity as a duplicate heuristic.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ROUTE-01 | Coordinator assigns implementation, research, and review tasks using configured agent roles and teammate relationships | `## Architecture Patterns` recommends `RoleSpec`, `TaskClass`, and `RouteChildTask` over freeform owner selection while preserving teammate boundaries. |
| ROUTE-02 | Coordinator does not assign the same execution task to multiple agents unless duplication is an explicit workflow step such as review | `## Architecture Patterns` and `## Validation Architecture` require duplicate keys, per-class policy, and direct `internal/session` tests for duplicate blocking and fanout. |
</phase_requirements>

## Summary

Phase 2 should extend the Phase 1 run workflow instead of inventing a new coordinator subsystem. The current seam is explicit: `internal/session/run.go` already snapshots `allowed_owners`/`team_snapshot`, `AddChildTask` is the persistence boundary for explicit owners, and `cmd/tmuxicate/main.go` already exposes the `run` command tree. The routing upgrade belongs at that same session boundary.

The current implementation is too weak for the Phase 2 decisions. `AgentConfig.Role` is still a freeform string, `RoutingConfig` still keys duplicate policy off mailbox `protocol.Kind`, and `AddChildTask` only checks teammate membership plus a non-empty role string. That supports Phase 1 foundations, but it cannot express structured routing intent, deterministic domain matching, or duplicate identity independent of owner.

**Primary recommendation:** Add a routing-aware subcommand and session entrypoint, for example `tmuxicate run route-task`, backed by a structured `TaskClass`, `RoleSpec`, deterministic candidate selection, and durable routing decision data persisted on each child task. `AddChildTask` should remain the explicit-owner writer, but it should accept enough routing metadata to repeat duplicate-policy checks before persistence.

## Current Seam

### What already exists
- `internal/session/run.go` already has the run-level baseline: it resolves the coordinator, snapshots `allowed_owners`, writes run/task artifacts, and emits mailbox-compatible task messages.
- `internal/session/run_contracts.go` already owns the coordinator root-message contract and is the right place to change the decomposition instructions from explicit owner guessing to routing-aware commands.
- `internal/protocol/coordinator.go` already defines canonical run/task records, so routing metadata should extend these structs rather than hide in `Envelope.Meta`.
- `internal/session/run_rebuild.go` already reconstructs run/task lineage from disk and is the natural place to surface routing evidence to operators.

### What is missing
- Structured routing metadata in config (`RoleSpec`, `route_priority`, per-class duplicate policy).
- Structured routing intent in protocol/session (`TaskClass`, route request, route decision, route rejection).
- Duplicate checking keyed by `(run_id, task_class, normalized_domains)` instead of owner or mailbox kind.
- Durable operator-visible routing evidence beyond who currently owns the task.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.26.1 | Routing logic, config parsing, durable state, and tests | Phase 2 stays inside the existing CLI/runtime architecture. |
| `github.com/spf13/cobra` | v1.10.2 | Add a routing-aware `run` subcommand without changing the CLI framework | The run workflow is already wired through Cobra. |
| `gopkg.in/yaml.v3` | v3.0.1 | Persist structured routing metadata in config and coordinator artifacts | Canonical records in this repo are YAML-backed and operator-readable. |
| `golang.org/x/sys/unix` | v0.42.0 | Reuse existing file-locking patterns when duplicate checks need a small critical section | Avoids adding a second locking or storage strategy. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Go `testing` package | stdlib | Add direct `internal/session` and `internal/config` coverage for routing behavior | Phase 2 should close the highest-priority session coverage gap while extending config semantics. |
| Existing mailbox store/helpers | in-repo | Keep mailbox delivery authoritative after routing picks an owner | Routing chooses the owner; message/receipt creation still uses the existing mailbox discipline. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Extend config/protocol/session structs in place | Hide routing logic in prompt instructions only | Violates the requirement for code-driven determinism and inspectable routing. |
| Add `TaskClass` beside `protocol.Kind` | Reuse `protocol.Kind` as routing intent | `protocol.Kind` is mailbox transport state, not routing policy; Phase 2 decisions explicitly separate them. |
| Persist routing evidence in canonical task YAML | Store it only in logs or transcript text | Breaks the operator-visibility requirement and makes restarts lose reasoning context. |

## Architecture Patterns

### Pattern 1: Structured Agent Role Metadata

Replace the string role field with an explicit `RoleSpec` shape:

```yaml
agents:
  - name: backend
    alias: api
    route_priority: 20
    role:
      kind: implementation
      domains: [session, protocol]
      description: Owns run/session logic and protocol changes
```

Why: the planner needs deterministic inputs it can compare directly in code. `kind` matches the task class, `domains` acts as a capability set, and `description` stays human-facing.

### Pattern 2: Dedicated Route Entry Point

Add a routing-aware session API and CLI surface, for example:

```text
tmuxicate run route-task --run <run-id> \
  --task-class implementation \
  --domain session \
  --domain protocol \
  --goal "..." \
  --expected-output "..."
```

Why: this keeps `AddChildTask` as the explicit-owner writer while giving the coordinator a canonical way to request deterministic routing instead of freeform owner guesses.

### Pattern 3: Deterministic Candidate Selection

Recommended selection order:
1. Start from the run's `allowed_owners` snapshot.
2. Resolve each candidate's `RoleSpec`.
3. Filter to `RoleSpec.Kind == TaskClass`.
4. Filter to candidates whose normalized domains are a superset of the request domains.
5. If an owner override is present, allow it only with an explicit reason and only if the override still appears in the run's teammate-constrained baseline.
6. Tie-break by `route_priority` descending, then config declaration order ascending.

This preserves the "same config + same run state = same route" guarantee and avoids hidden model behavior.

### Pattern 4: Duplicate Policy Before and During Persistence

Recommended duplicate key:

```text
<run-id>|<task-class>|<normalized-domain-list>
```

Where normalized domains are lowercased, trimmed, sorted, and deduplicated.

Recommended enforcement:
- `RouteChildTask` computes the duplicate key and checks existing task artifacts before owner selection.
- `AddChildTask` receives the routed task metadata and re-checks the same duplicate key immediately before writing the task artifact.
- Use a small run-scoped lock path under `coordinator/runs/<run-id>/` so the scan-and-write sequence cannot race.

### Pattern 5: Durable Routing Evidence

Each routed child task should persist enough evidence to answer:
- what task class and domains were requested,
- which candidates were considered,
- why the winner was chosen,
- whether the task was blocked as a duplicate or routed through an override,
- which duplicate key guarded the write.

The right persistence location is the canonical child-task YAML plus `run show` output, not logs alone.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Delivery after route selection | A second queue or daemon-only dispatch path | Existing mailbox message + receipt creation in `internal/session/run.go` | Keeps mailbox protocol authoritative. |
| Config parsing | Ad-hoc YAML maps in session code | `internal/config/config.go` + `internal/config/loader.go` | Preserves one config validation boundary. |
| Durable routing visibility | Prompt-only explanation or logs only | Canonical task fields + `run show` output | Survives restarts and matches operator-visibility goals. |
| Duplicate coordination | In-memory maps only | File-backed task scan plus a small on-disk lock | Matches the repo's filesystem-authoritative design. |

## Common Pitfalls

### Pitfall 1: Keeping `role` as a freeform string

That preserves Phase 1 behavior but makes domain-aware routing impossible and leaves the coordinator to infer capability from prose.

### Pitfall 2: Using `protocol.Kind` as routing intent

Mailbox kind is still useful for receipts and views, but Phase 2 decisions explicitly require a separate `TaskClass`. Conflating them weakens duplicate policy and obscures intent.

### Pitfall 3: Blocking duplicates only in the route helper

That still leaves direct CLI or future internal callers able to bypass the guard. The persistence boundary must repeat the check.

### Pitfall 4: Explaining routing only in message bodies or logs

That breaks the requirement that operators can inspect why a route was accepted or rejected from durable artifacts.

## Validation Architecture

Phase 2 should add direct unit/integration-with-tempdir coverage in `internal/session` and `internal/config` before implementation changes land. The minimal verification matrix is:

- Config parsing/validation:
  - `TestLoadValidConfigWithStructuredRoles`
- Deterministic routing:
  - `TestRunRootMessageContractUsesRouteTaskCommand`
  - `TestRouteChildTaskSelectsDeterministicOwner`
  - `TestRouteChildTaskRejectsNoMatchWithStructuredReason`
- Duplicate safeguards and routing evidence:
  - `TestRouteChildTaskBlocksExclusiveDuplicate`
  - `TestRouteChildTaskAllowsFanoutReviewClass`
  - `TestRouteChildTaskRequiresOverrideReason`
  - `TestAddChildTaskRejectsDuplicateWithoutRouteDecision`
  - `TestRunShowIncludesRoutingDecisionEvidence`

Recommended commands:

```bash
go test ./internal/config ./internal/session ./internal/protocol -count=1
go test ./... -count=1 -race
```

These tests stay inside the current fake/tempdir strategy and avoid adding live tmux or daemon dependencies.

## Recommended File Targets

- `cmd/tmuxicate/main.go`
- `internal/config/config.go`
- `internal/config/loader.go`
- `internal/config/loader_test.go`
- `internal/protocol/coordinator.go`
- `internal/protocol/validation.go`
- `internal/mailbox/paths.go`
- `internal/mailbox/coordinator_store.go`
- `internal/session/run_contracts.go`
- `internal/session/run.go`
- `internal/session/run_test.go`
- `internal/session/run_rebuild.go`
- `internal/session/run_rebuild_test.go`

## Plan Implications

The most coherent phase split is:

1. **Plan 02-01:** Introduce structured routing metadata plus the deterministic routing entrypoint and root-message contract.
2. **Plan 02-02:** Add duplicate policy, override handling, and durable routing evidence with direct tests.

That keeps the first plan focused on "how a route is chosen" and the second on "how unsafe or ambiguous routing is prevented and explained."

---
*Phase: 02-role-based-routing*
*Research completed: 2026-04-05*
