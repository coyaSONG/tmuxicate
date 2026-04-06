# Phase 4: Blocker Escalation - Research

**Researched:** 2026-04-06  
**Domain:** Coordinator-driven blocker handling inside the existing durable run/task/mailbox workflow [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/session/task_cmd.go][VERIFIED: internal/session/run.go][VERIFIED: internal/session/run_rebuild.go]  
**Confidence:** HIGH [VERIFIED: codebase inspection][VERIFIED: go test ./... -count=1 -race]

<user_constraints>
## User Constraints (from CONTEXT.md)

Verbatim copy from `.planning/phases/04-blocker-escalation/04-CONTEXT.md`. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]

### Locked Decisions

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

### Claude's Discretion
- Exact Go type names, YAML field ordering, and helper-function names for `BlockerCase`, action history records, and resolution structs, as long as the semantics above remain intact.
- Exact CLI flag names for `blocker resolve`, provided the command still expresses `manual_reroute`, `clarify`, and `dismiss` explicitly.
- Exact `run show` label text and field ordering for blocker blocks, provided the output stays task-local, scan-friendly, and durable-artifact-backed.

### Deferred Ideas (OUT OF SCOPE)
- Run-level blocked/escalated counters and aggregate summary sections — Phase 5
- Blocker-only list or read commands such as `tmuxicate blocker show` or `tmuxicate blockers list` — Phase 5 or later if still needed
- Broader run-summary UX that groups completed, waiting, blocked, review, and escalated work into one operator-facing report — Phase 5
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BLOCK-01 | Coordinator reacts to child task `wait` and `block` states with an explicit next step instead of silently stalling [VERIFIED: .planning/REQUIREMENTS.md] | Extend `TaskWait` and `TaskBlock` so coordinator-run tasks create or update a durable `BlockerCase`, classify the structured subtype through a deterministic policy table, and record the selected next action instead of writing a freeform state event only [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/session/task_cmd.go] |
| BLOCK-02 | Coordinator can escalate blocked or ambiguous work to the human operator with current owner, blocker reason, and recommended action [VERIFIED: .planning/REQUIREMENTS.md] | Persist escalation on the canonical `BlockerCase`, render it under the source task in `run show`, and add a dedicated `tmuxicate blocker resolve` command mirroring the Phase 3 `review respond` pattern [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/session/review_response.go][VERIFIED: internal/session/run_rebuild.go][VERIFIED: cmd/tmuxicate/main.go] |
| BLOCK-03 | Coordinator stops retrying or rerouting after defined limits and surfaces the unresolved task instead of looping indefinitely [VERIFIED: .planning/REQUIREMENTS.md] | Add a dedicated blocker config surface and source-task-scoped reroute history on `BlockerCase`; keep those ceilings separate from daemon transport retries in `delivery.max_retries` [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/config/config.go][VERIFIED: internal/runtime/daemon.go] |
</phase_requirements>

## Summary

Phase 4 should be planned as a close sibling of Phase 3, not as a new subsystem. The existing repo already has the right workflow shape: `TaskDone` performs a durable state transition, `CoordinatorStore` owns run-scoped artifacts, `ReviewRespond` provides the model for a dedicated resolution command, and `LoadRunGraph` plus `FormatRunGraph` rebuild task-local derived blocks from disk. The blocker phase should reuse those seams for `task wait`, `task block`, escalation, and operator resolution instead of inventing a second coordinator store, a human mailbox, or prompt-parsed routing logic. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/session/task_cmd.go][VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/session/review_response.go][VERIFIED: internal/session/run_rebuild.go]

The current implementation gap is narrow and concrete. `TaskWait` and `TaskBlock` only append agent state events with freeform `on` and `reason` fields, `mailbox/paths.go` has no blocker subtree, `CoordinatorStore` has no blocker CRUD, `config.Config` has no blocker config surface, and `run show` only knows how to rehydrate review handoffs. That means Phase 4 planning needs four explicit additions: a validated `BlockerCase` protocol artifact, blocker-specific path/store helpers, deterministic policy evaluation at the session layer, and run-show rebuild/render support for blocker blocks. [VERIFIED: internal/session/task_cmd.go][VERIFIED: internal/mailbox/paths.go][VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/config/config.go][VERIFIED: internal/session/run_rebuild.go]

The most important planning caution is that current task-state rendering is not fully task-local. `LoadRunGraph` reads declared state from `agents/<owner>/events/state.current.json`, which is an agent-global latest-state snapshot; if the same owner has multiple tasks, every task owned by that agent can inherit the same declared state in `run show`. Because Phase 4 explicitly requires task-local blocker chains, blocker visibility should come from the durable `BlockerCase` artifact and any per-message state lookup, not from the agent-global `state.current.json` file alone. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/status.go]

**Primary recommendation:** Plan Phase 4 as one durable blocker workflow: `task wait|block` validates active coordinator-run task -> create/update `BlockerCase` -> deterministic action selection (`watch|clarification_request|reroute|escalate`) -> enforce `blockers.max_reroutes_*` ceilings -> render the blocker block under the source task in `run show` -> resolve through `tmuxicate blocker resolve`, with all canonical state on the same artifact. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/session/task_cmd.go][VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/review_response.go]

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go toolchain | `1.26.1` [VERIFIED: go.mod][VERIFIED: go version] | Compile, validate, and test the blocker workflow inside the existing CLI [VERIFIED: go.mod] | The repo is pinned to this toolchain and the full suite is currently green on it locally [VERIFIED: go test ./... -count=1 -race] |
| `github.com/spf13/cobra` | `v1.10.2` published 2025-12-03 [VERIFIED: go.mod][VERIFIED: go list -m -u -json github.com/spf13/cobra] | Add `tmuxicate blocker resolve` as a first-class nested workflow command [VERIFIED: cmd/tmuxicate/main.go] | The CLI already uses Cobra command trees, subcommands, and required flags for adjacent workflows, and Cobra’s documented `AddCommand` and `MarkFlagRequired` APIs directly fit the new command surface [VERIFIED: cmd/tmuxicate/main.go][CITED: https://pkg.go.dev/github.com/spf13/cobra] |
| `gopkg.in/yaml.v3` | `v3.0.1` published 2022-05-27 [VERIFIED: go.mod][VERIFIED: go list -m -u -json gopkg.in/yaml.v3] | Persist human-readable blocker artifacts in the same run-scoped YAML format as run, task, and review documents [VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/session/run_rebuild.go] | Existing coordinator artifacts already use YAML and are validated on readback, so Phase 4 should extend that pattern rather than mixing formats [VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/protocol/validation.go] |
| `internal/mailbox.CoordinatorStore` | repo current [VERIFIED: internal/mailbox/coordinator_store.go] | Remain the authoritative persistence boundary for blocker-case CRUD [VERIFIED: internal/mailbox/coordinator_store.go] | Project constraints explicitly reject a second orchestration system and Phase 3 already proved this store pattern works for review handoffs [VERIFIED: .planning/PROJECT.md][VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/session/review_response.go] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/session.TaskWait` and `internal/session.TaskBlock` | repo current [VERIFIED: internal/session/task_cmd.go] | Current active-task validation seam for declared wait/block state changes [VERIFIED: internal/session/task_cmd.go] | Use these as the only entrypoints that create or update blocker workflow state for coordinator-run tasks [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] |
| `internal/session.RouteChildTask` | repo current [VERIFIED: internal/session/run.go] | Deterministic reroute engine with existing duplicate safeguards and owner overrides [VERIFIED: internal/session/run.go] | Reuse it for blocker-driven reroutes instead of building a second routing engine [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] |
| `internal/session.ReviewRespond` | repo current [VERIFIED: internal/session/review_response.go] | Existing pattern for “validate artifact -> write side-effect message -> update canonical workflow artifact” [VERIFIED: internal/session/review_response.go] | Mirror this pattern for `blocker resolve` [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] |
| `internal/session.LoadRunGraph` and `FormatRunGraph` | repo current [VERIFIED: internal/session/run_rebuild.go] | Existing rebuild-and-render surface for task-local derived workflow blocks [VERIFIED: internal/session/run_rebuild.go] | Extend it to load `blockers/*.yaml` and render each source task’s blocker state under that task only [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] |
| `internal/config` validation | repo current [VERIFIED: internal/config/config.go][VERIFIED: internal/config/loader.go] | Existing place to add `blockers.max_reroutes_default` and task-class overrides [VERIFIED: internal/config/config.go] | Phase 4 explicitly locks blocker ceilings to a dedicated config surface, not daemon delivery config [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Canonical `coordinator/runs/<run-id>/blockers/<source-task-id>.yaml` artifact [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Add blocker fields directly onto task YAML [VERIFIED: internal/protocol/coordinator.go] | Rejected because Phase 4 locks blocker truth to a dedicated artifact, and task YAML already holds routing/task-contract data rather than mutable blocker histories and operator resolutions [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/protocol/coordinator.go] |
| Dedicated blocker config surface [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Reuse `delivery.max_retries` [VERIFIED: internal/config/config.go][VERIFIED: internal/runtime/daemon.go] | Rejected because daemon retries are transport-level receipt notification policy, while blocker reroutes are coordinator workflow policy over logical work items [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/runtime/daemon.go] |
| Dedicated `blocker resolve` command [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Synthetic human mailbox recipient or freeform note reply [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: DESIGN.md] | Rejected because the locked operator response path is artifact-backed and write-only for Phase 4; no human inbox model is allowed [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] |
| Deterministic policy table over `wait_kind`/`block_kind` [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Parse freeform `reason` text or model output [VERIFIED: internal/session/task_cmd.go] | Rejected because the project philosophy favors explicit, inspectable workflow state and Phase 4 explicitly forbids freeform-text policy inference [VERIFIED: .planning/PROJECT.md][VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] |

**Installation:** No new external dependency is recommended for Phase 4; stay on the current module graph and validate with the existing Go test commands. [VERIFIED: go.mod][VERIFIED: Makefile]

```bash
go test ./internal/session -count=1
go test ./... -count=1 -race
```

**Version verification:** The recommended stack is the repo-pinned baseline. `go list -m -u -json` reported Cobra `v1.10.2`, fsnotify `v1.9.0`, and YAML v3 `v3.0.1` without a newer `Update` field in the checked output, and the local suite passes on Go `1.26.1`. [VERIFIED: go list -m -u -json github.com/spf13/cobra][VERIFIED: go list -m -u -json github.com/fsnotify/fsnotify][VERIFIED: go list -m -u -json gopkg.in/yaml.v3][VERIFIED: go version][VERIFIED: go test ./... -count=1 -race]

## Architecture Patterns

### Recommended Project Structure

```text
cmd/tmuxicate/main.go                         # blocker command wiring
internal/protocol/                           # BlockerCase schema + validation
internal/mailbox/                            # blocker paths + CRUD helpers
internal/config/                             # blockers.* config surface + validation
internal/session/                            # task wait/block policy + blocker resolve + run show rebuild

<state-dir>/coordinator/runs/<run-id>/
  run.yaml
  tasks/<task-id>.yaml
  reviews/<source-task-id>.yaml
  blockers/<source-task-id>.yaml
```

This matches the current CLI/session/mailbox/protocol layering and the locked blocker-artifact path under the existing run directory. `mailbox/paths.go` does not yet expose any blocker helpers, so planning should include path additions there first. [VERIFIED: .planning/codebase/ARCHITECTURE.md][VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/mailbox/paths.go]

### Pattern 1: Artifact-First Blocker Case

**What:** Treat `BlockerCase` as the canonical mutable record for blocker subtype, current owner, selected coordinator action, reroute history, escalation payload, and operator resolution outcome. This mirrors the Phase 3 `ReviewHandoff` pattern instead of spreading mutable state across task YAML, mailbox bodies, and agent event logs. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/mailbox/coordinator_store.go]  
**When to use:** Every coordinator-run `task wait` or `task block`, every reroute attempt, every escalation, and every operator resolution. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]  
**Example:** The current repo already uses a dedicated workflow artifact rather than reverse pointers:

```go
if err := coordinatorStore.UpdateReviewHandoff(runID, handoff.SourceTaskID, func(existing *protocol.ReviewHandoff) error {
	existing.ResponseMessageID = responseMessageID
	existing.Outcome = outcome
	existing.RespondedAt = &now
	existing.Status = protocol.ReviewHandoffStatusResponded
	return nil
}); err != nil {
	return "", err
}
```

Source: `internal/session/review_response.go`. [VERIFIED: internal/session/review_response.go]

### Pattern 2: Deterministic Policy Table at the Session Boundary

**What:** Evaluate blocker actions in code immediately after active-task validation, using structured subtype fields plus durable attempt history. The policy table should return only the locked four actions: `watch`, `clarification_request`, `reroute`, or `escalate`. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/session/task_cmd.go]  
**When to use:** Inside or immediately beneath `TaskWait` and `TaskBlock`, after the source task/message/run linkage is loaded. [VERIFIED: internal/session/task_cmd.go]  
**Example:** The repo’s routing flow is already deterministic and code-driven rather than model-driven:

```go
duplicateKey := duplicateKeyForRoute(req.RunID, req.TaskClass, req.Domains)
existingDuplicate, err := findActiveDuplicateTask(cfg.Session.StateDir, req.RunID, duplicateKey)
...
kindCandidates, domainCandidates := routeCandidates(cfg, run.AllowedOwners, req.TaskClass, req.Domains)
rankCandidates(kindCandidates)
rankCandidates(domainCandidates)
```

Source: `internal/session/run.go`. [VERIFIED: internal/session/run.go]

### Pattern 3: Separate Workflow Ceilings From Transport Retries

**What:** Keep reroute ceilings in `config.Blockers`, and keep daemon retry behavior where it already is. The two systems protect different failure modes and must not share counters or config fields. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/config/config.go][VERIFIED: internal/runtime/daemon.go]  
**When to use:** Config schema design, blocker policy evaluation, and tests around repeated failure. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]  
**Example:** The current daemon retry path updates receipt transport metadata only:

```go
if err := d.store.UpdateReceipt(agentName, msgID, func(r *protocol.Receipt) {
	r.NextRetryAt = &nextRetry
	r.LastError = &lastErr
	r.Revision++
}); err != nil {
	return err
}
```

Source: `internal/runtime/daemon.go`. [VERIFIED: internal/runtime/daemon.go]

### Pattern 4: Task-Local Rebuild and Rendering

**What:** Load blocker artifacts during `LoadRunGraph`, validate their linkage to the source task and any rerouted task/message references, then render a derived blocker block directly under the source task in `FormatRunGraph`. The source task remains the anchor. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/session/run_rebuild.go]  
**When to use:** Every `tmuxicate run show <run-id>`. [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: internal/session/run_rebuild.go]  
**Example:** Review handoffs already follow this exact show pattern:

```go
if task.ReviewHandoff != nil {
	fmt.Fprintf(&builder, "Review Handoff: %s\n", task.ReviewHandoff.Status)
	fmt.Fprintf(&builder, "Review Task: %s\n", displayTaskID(task.ReviewHandoff.ReviewTaskID))
	fmt.Fprintf(&builder, "Reviewer: %s\n", normalizeDisplayValue(string(task.ReviewHandoff.Reviewer)))
	fmt.Fprintf(&builder, "Response: %s\n", displayMessageID(task.ReviewHandoff.ResponseMessageID))
	fmt.Fprintf(&builder, "Outcome: %s\n", normalizeDisplayValue(string(task.ReviewHandoff.Outcome)))
	fmt.Fprintf(&builder, "Failure: %s\n", normalizeDisplayValue(task.ReviewHandoff.FailureSummary))
}
```

Source: `internal/session/run_rebuild.go`. [VERIFIED: internal/session/run_rebuild.go]

### Pattern 5: Dedicated Operator Resolution Command

**What:** Add `tmuxicate blocker resolve` as a review-response-style workflow command that validates the canonical artifact, records the operator action on that artifact, and then performs any routing/mailbox side effects. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/session/review_response.go][VERIFIED: cmd/tmuxicate/main.go]  
**When to use:** Human intervention after a blocker reaches `status=escalated`. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]  
**Example:** Cobra already exposes this shape for adjacent workflows:

```go
reviewCmd := &cobra.Command{
	Use:   "review",
	Short: "Manage review workflows",
	Run:   stubRun,
}

reviewCmd.AddCommand(newReviewRespondCmd())
```

Source: `cmd/tmuxicate/main.go`, using Cobra’s documented subcommand APIs. [VERIFIED: cmd/tmuxicate/main.go][CITED: https://pkg.go.dev/github.com/spf13/cobra]

### Anti-Patterns to Avoid

- **Freeform blocker inference:** Do not decide `watch` vs `reroute` vs `escalate` from `reason` text or model output. Use `wait_kind` and `block_kind` plus durable attempt history only. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]
- **Reuse of `delivery.max_retries`:** Do not couple logical reroute ceilings to daemon notification retries. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/runtime/daemon.go]
- **Synthetic human inbox:** Do not add a new mailbox recipient model for escalations in Phase 4. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]
- **Agent-global task rendering:** Do not treat `agents/<owner>/events/state.current.json` as the blocker truth for a specific task. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/status.go]
- **Second workflow store:** Do not bypass `CoordinatorStore` or write blocker truth into ad hoc JSON files outside the run artifact tree. [VERIFIED: .planning/PROJECT.md][VERIFIED: internal/mailbox/coordinator_store.go]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Blocker truth | Ad hoc fields on task YAML or state-event logs [VERIFIED: internal/session/task_cmd.go] | Dedicated `BlockerCase` artifact under `coordinator/runs/<run-id>/blockers/` [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Phase 4 explicitly locks blocker truth to a durable run-scoped artifact, and Phase 3 already proved this pattern with `ReviewHandoff` [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/session/review_response.go] |
| Reroute engine | A second owner-selection or retry system [VERIFIED: internal/session/run.go] | Existing `RouteChildTask` with source-task-scoped blocker history around it [VERIFIED: internal/session/run.go][VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | The repo already has deterministic routing, duplicate safeguards, and override semantics [VERIFIED: internal/session/run.go][VERIFIED: internal/session/run_test.go] |
| Human escalation transport | Synthetic human mailbox recipient [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | `BlockerCase status=escalated` + task-local `run show` + `blocker resolve` write path [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Phase 4 explicitly excludes a human mailbox model and read-only blocker commands [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] |
| Retry ceilings | Shared daemon receipt retry counters [VERIFIED: internal/runtime/daemon.go] | Dedicated `blockers.max_reroutes_default` and per-task-class overrides [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Transport failure and logical workflow failure are different failure domains [VERIFIED: internal/runtime/daemon.go][VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] |
| Task-local blocker display | Agent-global `state.current.json` snapshot [VERIFIED: internal/session/run_rebuild.go] | Blocker artifact plus task/message-linked validation during rebuild [VERIFIED: internal/session/run_rebuild.go][VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Multiple tasks owned by the same agent can otherwise collapse to the same displayed state [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/status.go] |

**Key insight:** The repo already has the exact architectural precedent needed for Phase 4 in Phase 3. The planning job is to generalize the “dedicated workflow artifact + dedicated resolution command + `run show` derived block” pattern to blockers, while correcting the current agent-global state rendering limitation for task-local blocker visibility. [VERIFIED: internal/session/review_response.go][VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/status.go]

## Common Pitfalls

### Pitfall 1: Using Agent-Global Declared State as Task Truth

**What goes wrong:** `run show` can display the same declared state for every task owned by one agent, even if only one task is blocked or waiting. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/status.go]  
**Why it happens:** `LoadRunGraph` reads `agents/<owner>/events/state.current.json`, which stores only the latest agent-wide state event. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/status.go]  
**How to avoid:** Make blocker blocks artifact-backed and keyed to the source task, and only use agent-level state files for coarse status surfaces like `status`, not blocker-chain truth. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/session/status.go]  
**Warning signs:** A design references `state.current.json` as the primary source for task-local blocker chains. [VERIFIED: internal/session/status.go]

### Pitfall 2: Letting Freeform `reason` Text Drive Policy

**What goes wrong:** Slight wording changes can flip coordinator behavior or create vendor-dependent blocker handling. [VERIFIED: internal/session/task_cmd.go][VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]  
**Why it happens:** The current wait/block commands only capture `on` plus `reason`, and no structured subtype fields exist yet. [VERIFIED: internal/session/task_cmd.go]  
**How to avoid:** Add validated subtype enums and make `reason` explanatory only. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]  
**Warning signs:** The plan says things like “inspect the reason text for clues” or “ask the coordinator model to decide the action.” [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]

### Pitfall 3: Spending Reroute Budget on Non-Reroute Actions

**What goes wrong:** Benign wait cases can exhaust the budget even though no reroute was attempted, causing premature escalation. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]  
**Why it happens:** `watch`, `clarification_request`, and `reroute` are all next actions, but only one of them represents logical rerouting. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]  
**How to avoid:** Increment the reroute counter only when a new child task is actually created or a manual reroute is recorded. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/session/run.go]  
**Warning signs:** The implementation increments the same counter for every policy evaluation or every wait/block event. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]

### Pitfall 4: Keying Retry History to the Current Owner Instead of the Source Task

**What goes wrong:** Ownership changes can reset the budget or smear history across unrelated blocked work. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]  
**Why it happens:** `RouteChildTask` already thinks in terms of owner selection, so it is tempting to attach blocker attempts to the newest routed child task instead of the original blocked logical work. [VERIFIED: internal/session/run.go]  
**How to avoid:** Keep the blocker artifact keyed by `source-task-id` exactly as locked in context, and append attempts there. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]  
**Warning signs:** Attempt history is stored on the rerouted child task or grouped only by `(task_class, domains)`. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]

### Pitfall 5: Reusing Daemon Transport Retries for Workflow Escalation

**What goes wrong:** A pane-readiness or adapter problem can look like a logical blocker, or a logical blocker can consume receipt retry budget meant for notification transport. [VERIFIED: internal/runtime/daemon.go][VERIFIED: .planning/codebase/CONCERNS.md]  
**Why it happens:** The repo already has `delivery.max_retries` and `markNotifyFailure`, but that path only touches unread receipt transport metadata. [VERIFIED: internal/config/config.go][VERIFIED: internal/runtime/daemon.go]  
**How to avoid:** Keep a clean split: daemon retry state stays on receipts; coordinator blocker state stays on `BlockerCase`. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/runtime/daemon.go]  
**Warning signs:** Planning language treats “notify retry” and “reroute retry” as the same budget or config knob. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]

### Pitfall 6: Making Escalation Visible Only Through Side Effects

**What goes wrong:** Operators see a message or stdout line once, but there is no durable canonical record to inspect later or rebuild after restart. [VERIFIED: .planning/PROJECT.md][VERIFIED: internal/session/review_response.go]  
**Why it happens:** The mailbox and tmux layers are visible, so it is tempting to treat the escalation message as the workflow truth. [VERIFIED: README.md][VERIFIED: DESIGN.md]  
**How to avoid:** Record escalation canonically on the blocker artifact first, then treat any routed message or console output as a side effect only. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]  
**Warning signs:** The design has no single source that can answer “why was this task escalated, to whom, and what is the recommended action?” after restart. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]

## Code Examples

Verified patterns from the current repo and official Cobra docs:

### Workflow Artifact Update Pattern

```go
if err := coordinatorStore.UpdateReviewHandoff(runID, handoff.SourceTaskID, func(existing *protocol.ReviewHandoff) error {
	existing.ResponseMessageID = responseMessageID
	existing.Outcome = outcome
	existing.RespondedAt = &now
	existing.Status = protocol.ReviewHandoffStatusResponded
	return nil
}); err != nil {
	return "", err
}
```

Source: `internal/session/review_response.go`. This is the closest verified pattern for `UpdateBlockerCase(...)` in Phase 4. [VERIFIED: internal/session/review_response.go]

### Deterministic Routed Side-Effect Pattern

```go
reviewTask, _, err := RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
	RunID:          sourceTask.ParentRunID,
	TaskClass:      protocol.TaskClassReview,
	Domains:        append([]string(nil), sourceTask.NormalizedDomains...),
	Goal:           fmt.Sprintf("Review implementation task %s: %s", sourceTask.TaskID, sourceTask.Goal),
	ExpectedOutput: fmt.Sprintf("Submit approved or changes_requested for %s via tmuxicate review respond", sourceTask.TaskID),
	ReviewRequired: false,
})
```

Source: `internal/session/task_cmd.go`. This is the verified routing seam to reuse for blocker-triggered reroutes. [VERIFIED: internal/session/task_cmd.go]

### Cobra Subcommand Pattern

```go
reviewCmd := &cobra.Command{
	Use:   "review",
	Short: "Manage review workflows",
	Run:   stubRun,
}

reviewCmd.AddCommand(newReviewRespondCmd())
```

Source: `cmd/tmuxicate/main.go`, using Cobra’s documented command-tree API. [VERIFIED: cmd/tmuxicate/main.go][CITED: https://pkg.go.dev/github.com/spf13/cobra]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `task wait` / `task block` only append freeform agent state events [VERIFIED: internal/session/task_cmd.go] | Phase 4 now requires structured subtype data plus deterministic action selection on a durable blocker artifact [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Locked in Phase 4 context on 2026-04-06 [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Wait/block states become explicit workflow transitions instead of passive status logs [VERIFIED: .planning/REQUIREMENTS.md] |
| `run show` currently derives review linkage from disk but blocker state is absent [VERIFIED: internal/session/run_rebuild.go] | `run show` should render a blocker block directly under the source task, following the review-handoff pattern [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Planned in Phase 4 after Phase 3 delivered review blocks [VERIFIED: .planning/ROADMAP.md][VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Operators can inspect unresolved blocker chains without transcript spelunking [VERIFIED: .planning/PROJECT.md] |
| `delivery.max_retries` is the only retry-like config currently present [VERIFIED: internal/config/config.go] | Blocker reroute ceilings move to a new `blockers.*` config surface and must stay separate [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Locked in Phase 4 context on 2026-04-06 [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Prevents transport failures from being confused with logical work reroutes [VERIFIED: internal/runtime/daemon.go] |
| `LoadRunGraph` uses agent-global `state.current.json` for declared state [VERIFIED: internal/session/run_rebuild.go] | Task-local blocker chains should be artifact-backed because agent-global state is not precise enough for multiple tasks per owner [VERIFIED: internal/session/run_rebuild.go][VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Recommended for Phase 4 planning based on current code shape [VERIFIED: internal/session/run_rebuild.go] | Avoids false blocker displays when one owner has multiple concurrent tasks [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/status.go] |

**Deprecated/outdated:**

- Treating freeform `reason` text as sufficient coordinator policy input is outdated by locked Phase 4 decisions `D-02` and `D-05`. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]
- Reusing `delivery.max_retries` for logical blocker reroutes is explicitly forbidden by locked decision `D-11`. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]
- `.planning/codebase/CONCERNS.md` still describes `internal/session` as untested, but the repo now contains multiple `internal/session/*_test.go` files and `go test ./... -count=1 -race` passes. [VERIFIED: .planning/codebase/CONCERNS.md][VERIFIED: internal/session/task_cmd_test.go][VERIFIED: internal/session/review_response_test.go][VERIFIED: internal/session/run_rebuild_test.go][VERIFIED: go test ./... -count=1 -race]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|

All substantive claims in this research were verified from the repo, planning artifacts, local tool checks, or official Cobra docs; no `[ASSUMED]` claims remain. [VERIFIED: codebase inspection][CITED: https://pkg.go.dev/github.com/spf13/cobra]

## Open Questions

1. **What exact side effect should `blocker resolve --action clarify` emit?**
   - What we know: The canonical resolution must be recorded on `BlockerCase`, and mailbox/routing side effects are optional reuse, not the source of truth. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]
   - What's unclear: Whether clarification should be sent as a reply to the current task message, a new coordinator `decision`/`note`, or only an artifact update plus operator note. [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: internal/session/review_response.go][VERIFIED: internal/session/reply.go]
   - Recommendation: Keep the artifact update mandatory, and if a message side effect is added, keep it in the existing run thread and record the emitted message ID on the blocker artifact for rebuild visibility. [VERIFIED: internal/session/review_response.go][VERIFIED: internal/session/run.go]

2. **How much of general task-state rendering should Phase 4 fix versus only blocker rendering?**
   - What we know: Current `run show` declared-state loading is agent-global, while Phase 4 blocker visibility is explicitly task-local. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]
   - What's unclear: Whether Phase 4 should broaden that fix to all task states or keep the correction limited to blocker-chain rendering. [VERIFIED: internal/session/run_rebuild.go]
   - Recommendation: Make blocker blocks fully task-local in Phase 4, and leave any broader per-task state-history redesign to Phase 5 unless the implementation can tighten both without scope creep. [VERIFIED: .planning/ROADMAP.md][VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | build and test commands [VERIFIED: Makefile] | ✓ [VERIFIED: go version] | `go1.26.1` [VERIFIED: go version] | — |
| `make` | repo task wrappers (`test`, `lint`, `fmt`, `ci`) [VERIFIED: Makefile] | ✓ [VERIFIED: make --version] | `GNU Make 3.81` [VERIFIED: make --version] | Direct `go test` invocation [VERIFIED: Makefile] |
| `tmux` | integration testing and manual runtime validation [VERIFIED: README.md][VERIFIED: internal/tmux/real_test.go] | ✓ [VERIFIED: tmux -V] | `3.6a` [VERIFIED: tmux -V] | Skip integration/manual pane checks for unit-only validation [VERIFIED: .planning/codebase/TESTING.md] |
| `golangci-lint` | `make lint` / CI parity [VERIFIED: Makefile] | ✗ [VERIFIED: command -v golangci-lint] | — | No repo-parity fallback; install required for lint gate [VERIFIED: Makefile] |
| `gofumpt` | `make fmt` [VERIFIED: Makefile] | ✗ [VERIFIED: command -v gofumpt] | — | Partial `gofmt` fallback exists operationally but does not meet repo formatting convention [VERIFIED: Makefile][VERIFIED: AGENTS.md instructions] |
| `goimports` | `make fmt` [VERIFIED: Makefile] | ✗ [VERIFIED: command -v goimports] | — | No repo-parity fallback for import normalization; install required [VERIFIED: Makefile] |

**Missing dependencies with no fallback:**

- `golangci-lint` blocks `make lint` and full CI-parity validation. [VERIFIED: Makefile][VERIFIED: command -v golangci-lint]
- `goimports` blocks repo-standard import normalization. [VERIFIED: Makefile][VERIFIED: command -v goimports]

**Missing dependencies with fallback:**

- `gofumpt` is missing; `gofmt` can keep code runnable, but it does not satisfy the repo’s declared formatting convention. [VERIFIED: Makefile][VERIFIED: .planning/codebase/CONVENTIONS.md]

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package [VERIFIED: .planning/codebase/TESTING.md] |
| Config file | none; repo-standard commands live in `Makefile` [VERIFIED: Makefile] |
| Quick run command | `go test ./internal/session -count=1` [VERIFIED: local run pattern from go test ./... -count=1] |
| Full suite command | `go test ./... -count=1 -race` [VERIFIED: Makefile][VERIFIED: go test ./... -count=1 -race] |

Current local baseline is green for both `go test ./... -count=1` and `go test ./... -count=1 -race`. [VERIFIED: go test ./... -count=1][VERIFIED: go test ./... -count=1 -race]

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BLOCK-01 | `task wait` and `task block` create/update a blocker artifact and always record an explicit next action [VERIFIED: .planning/REQUIREMENTS.md][VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | unit | `go test ./internal/session -run 'TestTask(Wait|Block).*Blocker' -count=1` | ✅ extend [`internal/session/task_cmd_test.go`](/Users/chsong/Developer/Personal/tmuxicate/internal/session/task_cmd_test.go) |
| BLOCK-02 | escalated blocker artifacts include owner, reason, and recommended action, and `blocker resolve` records the canonical operator resolution [VERIFIED: .planning/REQUIREMENTS.md][VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | unit + CLI | `go test ./internal/session -run 'TestBlocker(Resolve|Escalation|RunShow)' -count=1` | ⚠️ session files exist; CLI wiring file does not yet ([`cmd/tmuxicate/main.go`](/Users/chsong/Developer/Personal/tmuxicate/cmd/tmuxicate/main.go) has no test peer) [VERIFIED: go test ./... -cover] |
| BLOCK-03 | reroute ceilings stop loops, `watch` / `clarification_request` do not consume budget, and unresolved work is surfaced clearly [VERIFIED: .planning/REQUIREMENTS.md][VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | unit | `go test ./internal/session -run 'TestBlocker(Reroute|Ceiling|Unresolved)' -count=1` | ✅ extend [`internal/session/task_cmd_test.go`](/Users/chsong/Developer/Personal/tmuxicate/internal/session/task_cmd_test.go) and [`internal/session/run_rebuild_test.go`](/Users/chsong/Developer/Personal/tmuxicate/internal/session/run_rebuild_test.go) |

### Sampling Rate

- **Per task commit:** `go test ./internal/session -count=1` [VERIFIED: local repo test shape]
- **Per wave merge:** `go test ./... -count=1 -race` [VERIFIED: Makefile][VERIFIED: go test ./... -count=1 -race]
- **Phase gate:** Full suite green before `/gsd-verify-work` [VERIFIED: .planning/config.json]

### Wave 0 Gaps

- [ ] [`cmd/tmuxicate/main_test.go`](/Users/chsong/Developer/Personal/tmuxicate/cmd/tmuxicate/main_test.go) — add Cobra wiring and flag validation for `tmuxicate blocker resolve` because `cmd/tmuxicate` still has `0.0%` coverage. [VERIFIED: go test ./... -cover]
- [ ] [`internal/config/loader_test.go`](/Users/chsong/Developer/Personal/tmuxicate/internal/config/loader_test.go) — add validation coverage for `blockers.max_reroutes_default` and per-task-class overrides. [VERIFIED: internal/config/loader_test.go][VERIFIED: internal/config/loader.go]
- [ ] [`internal/session/task_cmd_test.go`](/Users/chsong/Developer/Personal/tmuxicate/internal/session/task_cmd_test.go) — add unhappy-path coverage for ambiguous blocker escalation, watch/clarify budget semantics, and reroute ceiling exhaustion. [VERIFIED: internal/session/task_cmd_test.go]
- [ ] [`internal/session/run_rebuild_test.go`](/Users/chsong/Developer/Personal/tmuxicate/internal/session/run_rebuild_test.go) — add blocker artifact linkage and render validation under `run show`. [VERIFIED: internal/session/run_rebuild_test.go]

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no [VERIFIED: README.md][VERIFIED: DESIGN.md] | No new authenticated user flow is introduced in this local CLI phase. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] |
| V3 Session Management | no [VERIFIED: README.md][VERIFIED: DESIGN.md] | `tmux` sessions are runtime/process containers here, not authenticated application sessions. [VERIFIED: DESIGN.md] |
| V4 Access Control | no [VERIFIED: README.md][VERIFIED: DESIGN.md] | There is workflow ownership validation, but no application-level authz surface is introduced by Phase 4. [VERIFIED: internal/session/run.go] |
| V5 Input Validation | yes [VERIFIED: internal/protocol/validation.go][VERIFIED: internal/config/loader.go] | Follow the existing enum-and-`Validate()` pattern for blocker actions, blocker kinds, reroute ceilings, and resolution actions. [VERIFIED: internal/protocol/validation.go][VERIFIED: internal/config/loader.go] |
| V6 Cryptography | no [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Phase 4 does not require any new cryptographic primitive; keep the existing message integrity model unchanged. [VERIFIED: DESIGN.md] |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Freeform blocker text triggers the wrong workflow action [VERIFIED: internal/session/task_cmd.go] | Tampering | Use validated `wait_kind` / `block_kind` enums and a deterministic policy table. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/protocol/validation.go] |
| Infinite reroute loops consume operator attention and keep work hidden in churn [VERIFIED: .planning/REQUIREMENTS.md][VERIFIED: .planning/codebase/CONCERNS.md] | Denial of Service | Enforce source-task-scoped reroute ceilings and escalate unresolved work immediately once the limit is met. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] |
| Blocker artifact drift from source task or rerouted task linkage [VERIFIED: internal/session/run_rebuild.go] | Tampering | Validate blocker linkage during `LoadRunGraph`, following the same pattern used for review handoff mismatch detection. [VERIFIED: internal/session/run_rebuild.go] |
| Unsupported operator resolution action or malformed blocker subtype enters the artifact [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md] | Tampering | Validate CLI action enums and artifact schema before any side effects run. [VERIFIED: internal/protocol/validation.go][CITED: https://pkg.go.dev/github.com/spf13/cobra] |

## Sources

### Primary (HIGH confidence)

- `.planning/phases/04-blocker-escalation/04-CONTEXT.md` - locked blocker model, action set, artifact path, config boundary, and operator visibility boundary. [VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md]
- `.planning/REQUIREMENTS.md` - `BLOCK-01`, `BLOCK-02`, and `BLOCK-03`. [VERIFIED: .planning/REQUIREMENTS.md]
- `.planning/ROADMAP.md` - Phase 4 success criteria and plan split. [VERIFIED: .planning/ROADMAP.md]
- `internal/session/task_cmd.go` - current wait/block/done behavior and Phase 3 post-done hook model. [VERIFIED: internal/session/task_cmd.go]
- `internal/session/run.go` - deterministic reroute engine and routing evidence model. [VERIFIED: internal/session/run.go]
- `internal/session/run_rebuild.go` - current run-show rebuild boundary and review-block rendering. [VERIFIED: internal/session/run_rebuild.go]
- `internal/session/review_response.go` - dedicated workflow-resolution command pattern. [VERIFIED: internal/session/review_response.go]
- `internal/mailbox/coordinator_store.go` and `internal/mailbox/paths.go` - canonical artifact persistence and current missing blocker subtree helpers. [VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/mailbox/paths.go]
- `internal/protocol/coordinator.go` and `internal/protocol/validation.go` - current workflow schemas and validation style to extend. [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/protocol/validation.go]
- `internal/config/config.go` and `internal/config/loader.go` - current config surface and validation boundary. [VERIFIED: internal/config/config.go][VERIFIED: internal/config/loader.go]
- `cmd/tmuxicate/main.go` - current Cobra command tree and adjacent workflow command shapes. [VERIFIED: cmd/tmuxicate/main.go]
- `go test ./... -count=1`, `go test ./... -count=1 -race`, and `go test ./... -cover` - current validation baseline and coverage posture. [VERIFIED: go test ./... -count=1][VERIFIED: go test ./... -count=1 -race][VERIFIED: go test ./... -cover]
- `go version`, `go list -m -u -json ...`, `tmux -V`, and `Makefile` - environment availability and repo/toolchain baseline. [VERIFIED: go version][VERIFIED: go list -m -u -json github.com/spf13/cobra][VERIFIED: go list -m -u -json github.com/fsnotify/fsnotify][VERIFIED: go list -m -u -json gopkg.in/yaml.v3][VERIFIED: tmux -V][VERIFIED: Makefile]
- Cobra official docs - subcommands and required-flag APIs. [CITED: https://pkg.go.dev/github.com/spf13/cobra]

### Secondary (MEDIUM confidence)

- `.planning/PROJECT.md` - product philosophy and coordination constraints. [VERIFIED: .planning/PROJECT.md]
- `.planning/codebase/TESTING.md` - established fake-based test patterns and Go test commands. [VERIFIED: .planning/codebase/TESTING.md]
- `DESIGN.md` and `README.md` - current operator-facing task wait/block semantics and system philosophy. [VERIFIED: DESIGN.md][VERIFIED: README.md]

### Tertiary (LOW confidence)

- None. [VERIFIED: research session source log]

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH - repo-pinned toolchain and modules were verified locally, and no new external dependency is needed. [VERIFIED: go.mod][VERIFIED: go version][VERIFIED: go list -m -u -json github.com/spf13/cobra]
- Architecture: HIGH - Phase 4 maps directly onto already-implemented Phase 3 seams (`CoordinatorStore`, dedicated workflow artifact, dedicated resolution command, `run show` rebuild). [VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/session/review_response.go][VERIFIED: internal/session/run_rebuild.go]
- Pitfalls: HIGH - the highest-risk failures are visible in the current code shape today, especially freeform blocker inputs, the transport/workflow retry split, and agent-global declared-state rendering. [VERIFIED: internal/session/task_cmd.go][VERIFIED: internal/runtime/daemon.go][VERIFIED: internal/session/run_rebuild.go]

**Research date:** 2026-04-06  
**Valid until:** 2026-05-06
