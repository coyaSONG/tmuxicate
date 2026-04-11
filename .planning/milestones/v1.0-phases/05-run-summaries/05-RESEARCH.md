# Phase 5: Run Summaries - Research

**Researched:** 2026-04-06  
**Domain:** Coordinator-run summary aggregation and operator-visible rendering over existing durable run/task/review/blocker artifacts [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/protocol/coordinator.go]  
**Confidence:** MEDIUM [VERIFIED: codebase inspection][VERIFIED: targeted `go test ./internal/session ... -count=1`][VERIFIED: `rg` audit showing no existing run-summary/completion hook]

<user_constraints>
## User Constraints (from CONTEXT.md)

Verbatim copy from `.planning/phases/05-run-summaries/05-CONTEXT.md`. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md]

### Locked Decisions

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

### Claude's Discretion
- Exact section labels, bucket ordering, and ASCII formatting of the summary block, as long as the derived statuses and one-row-per-logical-item model remain intact.
- Exact field labels for message, task, review, and blocker references, as long as operators can trace each summary item back to durable artifacts.
- Exact detection of the one-time completion print hook, as long as it does not create a new durable summary truth source.

### Deferred Ideas (OUT OF SCOPE)
- Separate `run summary` command or alternate summary-only output mode.
- Persisted summary snapshots, JSON output, filtering, sorting, or historical reports.
- New workflow automation triggered from summary outcomes, such as auto-generating follow-up work after `changes_requested`.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SUM-01 | Operator can get an end-of-run summary that lists completed, waiting, blocked, under-review, and escalated work [VERIFIED: .planning/REQUIREMENTS.md] | Extend `LoadRunGraph`/`FormatRunGraph` with a logical-row summary projection over existing tasks, review handoffs, and blocker cases; keep the summary above the current task-local detail in `run show` and reuse the same formatter for one-time completion output [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/session/run_rebuild.go][VERIFIED: cmd/tmuxicate/main.go] |
| SUM-02 | Run summaries identify the responsible agent and related message or task references for each reported item [VERIFIED: .planning/REQUIREMENTS.md] | Derive the row owner and references from source task IDs plus folded-in `ReviewHandoff.Reviewer`, `BlockerCase.CurrentOwner`, `CurrentTaskID`, `CurrentMessageID`, `ReviewTaskID`, and `ResponseMessageID` rather than inventing new reference fields [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/session/run_rebuild.go] |
</phase_requirements>

## Summary

Phase 5 should be planned as a read-model extension, not as new workflow state. The existing rebuild path already has the right durable inputs: `LoadRunGraph` joins run/task YAML, message envelopes, owner receipts, review handoffs, and blocker cases into one in-memory graph, and `FormatRunGraph` already renders derived review and blocker blocks from that graph alone. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] The strongest plan is therefore to add a summary projection and formatter in the same file or directly adjacent helpers, then keep `run show` as the only required operator entrypoint. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: cmd/tmuxicate/main.go]

The main implementation risk is not storage; it is identity collapse. A naive summary over `graph.Tasks` will double-count review tasks and rerouted current tasks, because Phase 5 explicitly requires one logical row per source task while the rebuilt graph still contains the review task and any rerouted task as ordinary child tasks. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/protocol/coordinator.go] The summary builder therefore needs a task index and explicit exclusion/folding rules so source tasks remain the anchor and descendant workflow artifacts only enrich that row. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/protocol/coordinator.go]

The second major risk is operator output coupling. `newRunShowCmd` currently rejects any output that does not start with `Run: `, while Phase 5 locks the summary to the top of `run show`. [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] That means the integration plan must either place the new summary immediately after the existing `Run:` header or update the guard and command-level tests in the same change; otherwise the summary formatter can be correct and the CLI will still fail. [VERIFIED: cmd/tmuxicate/main.go]

**Primary recommendation:** Add `BuildRunSummary(graph *RunGraph)` plus `FormatRunSummary(...)` beside the existing rebuild/format helpers, render the summary directly under the `Run:` header in `FormatRunGraph`, and treat the coordinator root message moving to `done` as the preferred one-time auto-print trigger because it is the only existing unique completion artifact for a run. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/run.go][VERIFIED: internal/session/run_contracts.go][VERIFIED: cmd/tmuxicate/main.go][ASSUMED]

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go toolchain | `1.26.1` [VERIFIED: go.mod][VERIFIED: `go version`] | Build and test the summary projection inside the existing CLI/runtime stack. [VERIFIED: go.mod] | Phase 5 is a brownfield extension of the current Go CLI, and the repo already runs targeted and full tests on this toolchain. [VERIFIED: Makefile][VERIFIED: targeted `go test ./internal/session ... -count=1`] |
| `internal/session/run_rebuild.go` | repo current [VERIFIED: internal/session/run_rebuild.go] | Canonical rebuild boundary for run/task/review/blocker state and the correct home for summary aggregation helpers. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/session/run_rebuild.go] | D-03 explicitly locks summary generation to the existing `RunGraph` rebuild path. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] |
| `internal/mailbox.CoordinatorStore` | repo current [VERIFIED: internal/mailbox/coordinator_store.go] | Canonical read boundary for run, task, review, and blocker artifacts that summary rows need to reference. [VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/session/run_rebuild.go] | No new summary storage is allowed, so reads must stay on the existing durable artifact boundary. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] |
| `github.com/spf13/cobra` | `v1.10.2`, published 2025-12-03 [VERIFIED: go.mod][VERIFIED: `go list -m -json github.com/spf13/cobra`] | Preserve `run show`, `task done`, `review respond`, and `blocker resolve` as the operator-visible command surfaces. [VERIFIED: cmd/tmuxicate/main.go] | The CLI already owns adjacent workflow output, and Phase 5 explicitly keeps summary visibility in the existing command family. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: cmd/tmuxicate/main.go] |
| `gopkg.in/yaml.v3` | `v3.0.1`, published 2022-05-27 [VERIFIED: go.mod][VERIFIED: `go list -m -json gopkg.in/yaml.v3`] | Continue reading the canonical YAML artifacts that drive summary state. [VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/session/run_rebuild.go] | Summary data is derived from the same human-readable run/task/review/blocker records already used elsewhere. [VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/protocol/validation.go] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/session/task_cmd.go` | repo current [VERIFIED: internal/session/task_cmd.go] | Existing task mutation seam, including `TaskDone` and task-state event writes. [VERIFIED: internal/session/task_cmd.go] | Use it only as a completion-print trigger or run-ID lookup seam; do not move summary truth into task events. [VERIFIED: internal/session/task_cmd.go][VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] |
| `internal/session/review_response.go` | repo current [VERIFIED: internal/session/review_response.go] | Existing review terminal transition that can affect summary status. [VERIFIED: internal/session/review_response.go] | Use it to verify under-review and approved/changes-requested transitions in summary tests. [VERIFIED: internal/session/review_response.go][VERIFIED: internal/session/review_response_test.go] |
| `internal/session/blocker_resolve.go` | repo current [VERIFIED: internal/session/blocker_resolve.go] | Existing operator-resolution seam for escalated blockers. [VERIFIED: internal/session/blocker_resolve.go] | Use it in completion/output planning if the auto-print strategy needs to run after blocker resolution. [VERIFIED: internal/session/blocker_resolve.go][ASSUMED] |
| Go `testing` package | stdlib [VERIFIED: .planning/codebase/TESTING.md] | Direct unit coverage for summary aggregation and command-output seams. [VERIFIED: .planning/codebase/TESTING.md] | Existing repo practice is co-located tests with `t.Parallel()`, temp dirs, and hand-rolled fakes. [VERIFIED: .planning/codebase/TESTING.md] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Summary derived from `RunGraph` inside `run_rebuild.go` [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/session/run_rebuild.go] | A second summary scanner or daemon-side cache [VERIFIED: `rg -n "RunSummary|summary section" internal cmd`] | Rejected because D-03 forbids a second aggregation model and the codebase already treats rebuild-on-read as the operator truth path. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: .planning/codebase/ARCHITECTURE.md] |
| Summary block at the top of `run show` [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | New `run summary` command or summary-only flag mode [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | Rejected because D-01 and deferred ideas explicitly keep the existing command surface as the primary operator entrypoint. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] |
| One logical row per source task with folded-in review/reroute metadata [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | Separate rows for review tasks, rerouted tasks, and blocker cases [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/protocol/coordinator.go] | Rejected because it breaks D-09 and D-10 and makes the summary longer than the existing detailed task view. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] |
| One-time print triggered from an existing completion artifact [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: `rg -n "run complete|RunSummary|summary printed" internal cmd`] | New persisted “summary printed” marker [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | Rejected because D-16 excludes new summary artifacts and the repo currently has no summary-specific storage path. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: `rg -n "RunSummary|summary section" internal cmd`] |

**Installation:** No new external module is recommended for Phase 5; stay on the current module graph and validate with targeted `go test` plus the existing full-suite command. [VERIFIED: go.mod][VERIFIED: Makefile]

```bash
go test ./internal/session -count=1
go test ./... -count=1 -race
```

**Version verification:** The repo-pinned baseline is already present locally: Go `1.26.1`, Cobra `v1.10.2`, fsnotify `v1.9.0`, Koanf `v2.3.4`, YAML v3 `v3.0.1`, and `x/sys` `v0.42.0`. [VERIFIED: go.mod][VERIFIED: `go version`][VERIFIED: `go list -m -json github.com/spf13/cobra github.com/fsnotify/fsnotify github.com/knadh/koanf/v2 gopkg.in/yaml.v3 golang.org/x/sys`] Phase 5 does not require adding any dependency beyond that baseline. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md]

## Architecture Patterns

### Recommended Project Structure

```text
cmd/tmuxicate/main.go                     # run show render call + completion-print hook wiring
internal/session/run_rebuild.go          # BuildRunSummary / FormatRunSummary beside LoadRunGraph / FormatRunGraph
internal/session/run_rebuild_test.go     # summary aggregation + collapse tests
internal/session/task_cmd.go             # optional run-completion trigger lookup for root task done
internal/session/review_response.go      # optional post-review completion trigger lookup
internal/session/blocker_resolve.go      # optional post-resolution completion trigger lookup
```

This keeps aggregation in the existing rebuild/read-model layer and keeps operator-visible printing at the CLI boundary, which matches the current repo layering. [VERIFIED: .planning/codebase/ARCHITECTURE.md][VERIFIED: internal/session/run_rebuild.go][VERIFIED: cmd/tmuxicate/main.go]

### Pattern 1: Summary Is a Projection of the Existing Run Graph

**What:** Build summary items from the already-validated `RunGraph` rather than scanning the filesystem again or creating a new cache. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/session/run_rebuild.go]  
**When to use:** Every `tmuxicate run show <run-id>` call and any one-time completion print. [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md]  
**Example:**

```go
graph, err := session.LoadRunGraph(cfg.Session.StateDir, protocol.RunID(args[0]))
if err != nil {
	return err
}

output := session.FormatRunGraph(graph)
```

Source: `cmd/tmuxicate/main.go`. [VERIFIED: cmd/tmuxicate/main.go]

### Pattern 2: Anchor Rows to Source Tasks and Fold Descendants Back In

**What:** Treat a logical work item as the source task plus any linked workflow descendants: review task, review response, blocker case, and rerouted current task. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/protocol/coordinator.go]  
**When to use:** While building the row set, exclude review tasks and exclude any task referenced as `BlockerCase.CurrentTaskID` when it differs from `SourceTaskID`; then use those excluded tasks only as metadata sources for the source row. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/protocol/coordinator.go][ASSUMED]  
**Example:**

```go
taskByID := make(map[protocol.TaskID]*RunGraphTask, len(tasks))
for _, task := range tasks {
	...
	taskByID[task.Task.TaskID] = &graph.Tasks[len(graph.Tasks)-1]
}
```

Source: `internal/session/run_rebuild.go`. [VERIFIED: internal/session/run_rebuild.go]

### Pattern 3: Derive Status From Task-Local Workflow Artifacts, Not Agent-Global State

**What:** Use blocker and review artifacts first, then fall back to receipt state for ordinary work. `RunGraphTask.DeclaredState` should not be the primary status source for summaries because it comes from `agents/<owner>/events/state.current.json`, which is only the latest agent-global state snapshot. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/status.go]  
**When to use:** Every summary row, especially if the same agent can own multiple tasks in a run. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: .planning/codebase/CONCERNS.md]  
**Recommended rule table:**

| Condition | Summary status | Confidence |
|-----------|----------------|------------|
| `BlockerCase.Status == escalated` [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/protocol/validation.go] | `escalated` [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | HIGH |
| `BlockerCase.Status == active` and `DeclaredState == "block"` [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/protocol/validation.go] | `blocked` [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | HIGH |
| `BlockerCase.Status == active` and `DeclaredState == "wait"` [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/protocol/validation.go] | `waiting` [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | HIGH |
| `ReviewHandoff.Status == pending` [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/protocol/validation.go] | `under_review` [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | HIGH |
| `ReviewHandoff.Status == responded` and `Outcome == changes_requested` [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/protocol/validation.go] | `under_review` [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | HIGH |
| No blocker/review override and source receipt is `unread` or `active` [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/protocol/receipt.go] | `pending` [ASSUMED] | LOW |
| No blocker/review override and source receipt is `done` [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/protocol/validation.go] | `completed` [ASSUMED] | LOW |
| `ReviewHandoff.Status == responded` and `Outcome == approved` [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/protocol/validation.go] | `completed` [ASSUMED] | LOW |

### Pattern 4: Keep `run show` Header Compatibility Intact

**What:** Render the summary directly after the existing `Run:` header or update the CLI guard and tests in the same change. [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md]  
**When to use:** Any change to `FormatRunGraph` output ordering. [VERIFIED: cmd/tmuxicate/main.go]  
**Example:**

```go
if !strings.HasPrefix(output, "Run: ") {
	return fmt.Errorf("run show output must start with Run: header")
}
```

Source: `cmd/tmuxicate/main.go`. [VERIFIED: cmd/tmuxicate/main.go]

### Pattern 5: Use an Existing Completion Artifact for One-Time Auto-Print

**What:** Prefer the coordinator root message transitioning to `done` as the one-time auto-print edge, because the run record already stores `RootMessageID`, the root message is unique per run, and the repo has no existing run-level completion marker or summary-print state. [VERIFIED: internal/session/run.go][VERIFIED: internal/session/run_contracts.go][VERIFIED: `rg -n "run complete|RunSummary|summary printed" internal cmd`][ASSUMED]  
**When to use:** When `task done` is invoked on the run root message, print the same summary formatter that `run show` uses. [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: internal/session/run.go][ASSUMED]  
**Why this is the narrowest fit:** A fully automatic “graph is complete now” detector would need an explicit terminal-state policy for `pending`, `waiting`, `escalated`, `handoff_failed`, and `changes_requested`, and the current durable artifacts do not encode that policy anywhere. [VERIFIED: internal/protocol/coordinator.go][VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][ASSUMED]

### Anti-Patterns to Avoid

- **Second summary truth source:** Do not add `summary.yaml`, cached counters, or daemon-owned summary files. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md]
- **Review-task rows in the summary:** Do not let `TaskClassReview` tasks or rerouted current tasks appear as separate summary rows. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/protocol/coordinator.go]
- **Primary reliance on `state.current.json`:** Do not derive blocked/waiting/pending from `RunGraphTask.DeclaredState` alone. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/status.go]
- **Transcript-derived references:** Do not scrape message IDs, owners, or outcomes from transcript logs. [VERIFIED: .planning/PROJECT.md][VERIFIED: .planning/codebase/ARCHITECTURE.md]
- **Command-only formatter duplication:** Do not build one summary formatter for `run show` and another for completion prints. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Run summary storage | A new `summary.yaml` or summary snapshot directory [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | Derived `BuildRunSummary(graph)` over `LoadRunGraph` output [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/session/run_rebuild.go] | D-03 and D-16 explicitly keep summary as a derived operator view over existing artifacts only. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] |
| Logical work identity | Independent rows for review tasks or rerouted tasks [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/protocol/coordinator.go] | One source-task row with folded-in review/blocker/reroute metadata [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | D-09 and D-10 lock one-row-per-logical-item behavior. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] |
| Owner/reference derivation | New reference fields written back onto source tasks [VERIFIED: internal/protocol/coordinator.go] | Existing `ReviewHandoff` and `BlockerCase` fields plus source `Task`/`MessageID` [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/session/run_rebuild.go] | Phase 5 is additive and must preserve Phase 1-4 artifact contracts. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: .planning/STATE.md] |
| Auto-print deduplication | New persistent “printed” marker [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | Existing unique completion edge, preferably root task `done` [VERIFIED: internal/session/run.go][ASSUMED] | D-16 rules out new summary-state artifacts, so deduplication must piggyback on existing workflow transitions. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] |

**Key insight:** The difficult part of Phase 5 is not producing text; it is faithfully collapsing multiple task-level artifacts back into one logical work row without inventing new truth. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/session/run_rebuild.go]

## Common Pitfalls

### Pitfall 1: Breaking `run show` by Moving `Run:` Off the First Line

**What goes wrong:** The new summary formatter works, but `newRunShowCmd` returns `run show output must start with Run: header` because the CLI still asserts the old prefix invariant. [VERIFIED: cmd/tmuxicate/main.go]  
**Why it happens:** Phase 5 locks the summary to the top of `run show`, while the current command assumes the detail formatter starts with `Run:`. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: cmd/tmuxicate/main.go]  
**How to avoid:** Keep `Run:` first and insert the summary immediately after it, or change the guard and add command-level tests in the same plan. [VERIFIED: cmd/tmuxicate/main.go][ASSUMED]  
**Warning signs:** The formatter output starts with `Summary:` or a bucket header instead of `Run:`. [VERIFIED: cmd/tmuxicate/main.go][ASSUMED]

### Pitfall 2: Using Agent-Global Declared State as Task-Local Summary Truth

**What goes wrong:** Multiple tasks owned by the same agent all show the same pending/waiting/blocked state, even when only one task actually changed. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/status.go]  
**Why it happens:** `LoadRunGraph` reads `agents/<owner>/events/state.current.json`, which stores the latest event for the agent, not a per-message/per-task snapshot. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/status.go]  
**How to avoid:** Use `BlockerCase` and `ReviewHandoff` as the primary task-local workflow signals and use receipt state for the normal pending/completed fallback. [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/session/run_rebuild.go][ASSUMED]  
**Warning signs:** The design mentions `DeclaredState` before it mentions blocker or review artifacts. [VERIFIED: internal/session/run_rebuild.go][ASSUMED]

### Pitfall 3: Missing the Rerouted-Task Review Fold-Back

**What goes wrong:** A rerouted current task acquires its own review handoff, but the summary either shows two rows or drops the review outcome entirely because the review handoff is attached to the rerouted task ID rather than the original source task ID. [VERIFIED: internal/session/task_cmd.go][VERIFIED: internal/protocol/coordinator.go][ASSUMED]  
**Why it happens:** `BlockerCase` tracks `SourceTaskID` and `CurrentTaskID`, while `ReviewHandoff` is keyed to the task that actually completed review-required work. [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/session/task_cmd.go]  
**How to avoid:** When a source row has a blocker case with `CurrentTaskID != SourceTaskID`, also inspect the current-task node for a review handoff and fold that metadata back onto the source row. [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/session/run_rebuild.go][ASSUMED]  
**Warning signs:** The implementation only ever reads `sourceTask.ReviewHandoff` when building a summary row. [VERIFIED: internal/session/run_rebuild.go][ASSUMED]

### Pitfall 4: Treating `task done --summary` Text as Canonical Summary Data

**What goes wrong:** The summary depends on optional human-authored text and loses correctness when `--summary` is omitted. [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: internal/session/task_cmd.go]  
**Why it happens:** `TaskDone` records the optional summary only in `state.jsonl` / `state.current.json`, not in canonical task YAML or workflow artifacts. [VERIFIED: internal/session/task_cmd.go]  
**How to avoid:** Use the optional completion summary as display enrichment only, never as required input for status, owner, or reference derivation. [VERIFIED: internal/session/task_cmd.go][ASSUMED]  
**Warning signs:** A summary row cannot be built when `summary == ""`. [VERIFIED: cmd/tmuxicate/main.go][ASSUMED]

### Pitfall 5: Double-Printing the Completion Summary

**What goes wrong:** The same run summary prints multiple times after retries, repeated commands, or different terminal transitions. [VERIFIED: `rg -n "summary printed|RunSummary" internal cmd`][ASSUMED]  
**Why it happens:** The repo has no persisted “printed already” marker and no existing run-level completion state. [VERIFIED: `rg -n "summary printed|RunSummary|run complete" internal cmd`][VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md]  
**How to avoid:** Choose one explicit completion edge and centralize printing there; the narrowest current fit is root run message `done`. [VERIFIED: internal/session/run.go][VERIFIED: internal/session/run_contracts.go][ASSUMED]  
**Warning signs:** The plan proposes printing after both `task done` and `review respond` without a shared edge detector. [VERIFIED: cmd/tmuxicate/main.go][ASSUMED]

### Pitfall 6: Assuming All Terminal Review Outcomes Are Already Specified

**What goes wrong:** The implementation silently guesses what to do with `ReviewHandoffStatusHandoffFailed` or `responded + approved` because the locked precedence only explicitly calls out pending and changes-requested review states. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/protocol/coordinator.go]  
**Why it happens:** D-05 and D-06 define only the under-review cases, not every fallback branch. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md]  
**How to avoid:** Lock the fallback mapping in planning before coding; otherwise different helpers can classify the same row differently. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][ASSUMED]  
**Warning signs:** Helper code special-cases `pending` and `changes_requested` but leaves `approved` or `handoff_failed` to incidental fallthrough. [VERIFIED: internal/protocol/coordinator.go][ASSUMED]

## Code Examples

Verified patterns from the current repo:

### Rebuild Once, Render Once

```go
graph, err := session.LoadRunGraph(cfg.Session.StateDir, protocol.RunID(args[0]))
if err != nil {
	return err
}

output := session.FormatRunGraph(graph)
_, err = fmt.Fprint(cmd.OutOrStdout(), output)
return err
```

Source: `cmd/tmuxicate/main.go`. [VERIFIED: cmd/tmuxicate/main.go]

### Durable Completion Happens Before Follow-Up Workflow Logic

```go
if err := store.UpdateReceipt(agentName, msgID, func(r *protocol.Receipt) {
	r.ClaimedBy = nil
	r.ClaimedAt = nil
	r.DoneAt = &now
	r.Revision++
}); err != nil {
	return err
}
if err := store.MoveReceipt(agentName, msgID, protocol.FolderStateActive, protocol.FolderStateDone); err != nil {
	return err
}

if err := appendStateEvent(stateDir, agentName, &TaskEvent{ ... }); err != nil {
	return err
}

return createReviewHandoffAfterTaskDone(stateDir, store, msgID)
```

Source: `internal/session/task_cmd.go`. [VERIFIED: internal/session/task_cmd.go]

### Canonical Review and Blocker Fields Already Exist for Summary Rows

```go
type ReviewHandoff struct {
	SourceTaskID      TaskID
	ReviewTaskID      TaskID
	ReviewMessageID   MessageID
	Reviewer          AgentName
	Status            ReviewHandoffStatus
	ResponseMessageID MessageID
	Outcome           ReviewOutcome
}

type BlockerCase struct {
	SourceTaskID     TaskID
	CurrentTaskID    TaskID
	CurrentMessageID MessageID
	CurrentOwner     AgentName
	DeclaredState    string
	SelectedAction   BlockerAction
	Status           BlockerStatus
	RecommendedAction *RecommendedAction
}
```

Source: `internal/protocol/coordinator.go` (abridged). [VERIFIED: internal/protocol/coordinator.go]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Phase 1 `run show` exposed only run/task ownership, compact state, and message references [VERIFIED: .planning/phases/01-coordinator-foundations/01-CONTEXT.md][VERIFIED: internal/session/run_rebuild.go] | Review and blocker phases already extend the same task-local rebuild surface with derived workflow blocks [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: .planning/phases/04-blocker-escalation/04-CONTEXT.md][VERIFIED: internal/session/run_rebuild.go] | 2026-04-06 across Phases 3 and 4 [VERIFIED: .planning/STATE.md] | Phase 5 should continue the same pattern by adding a run-level summary section above the detailed blocks instead of creating a new dashboard or store. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] |
| Operator had to scan detailed task blocks to understand overall run posture [VERIFIED: internal/session/run_rebuild.go][VERIFIED: .planning/ROADMAP.md] | Phase 5 adds a compact logical-work summary while preserving the existing detail below it [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | Phase 5 planning target, gathered 2026-04-06 [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | Better operator scanability without changing the canonical artifact model. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] |

**Deprecated/outdated:**

- Separate summary-only commands or persisted summary snapshots are explicitly out of scope for this phase. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md]
- Treating transcripts as the place to reconstruct run-level status is inconsistent with the repo’s filesystem-first, artifact-backed operator model. [VERIFIED: .planning/PROJECT.md][VERIFIED: .planning/codebase/ARCHITECTURE.md][VERIFIED: DESIGN.md]

## Assumptions Log (Resolved by Planning)

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Plain source tasks with no blocker/review override and receipt state `unread`/`active` summarize as `pending`. [RESOLVED: 05-01-PLAN.md] | Architecture Patterns | Ordinary in-progress work would be mislabeled or omitted if implementation drifts from the locked fallback bucket. |
| A2 | `ReviewHandoff.Status == responded` with `Outcome == approved` falls through to `completed`, while `changes_requested` remains `under_review`. [RESOLVED: 05-01-PLAN.md] | Architecture Patterns | Approved review items could remain under-review forever or changes-requested items could be misreported as complete. |
| A3 | If a rerouted current task later creates a review handoff, Phase 5 folds that review metadata back onto the original source row by following `BlockerCase.CurrentTaskID`. [RESOLVED: 05-01-PLAN.md] | Architecture Patterns | The summary could double-count or hide logical work after reroute + review chains. |
| A4 | The one-time completion hook is the coordinator root message moving to `done`; no graph-derived completion state or new summary artifact is introduced in Phase 5. [RESOLVED: 05-02-PLAN.md] | Architecture Patterns | Summary auto-print could fire on the wrong edge or require out-of-scope persistence. |

## Open Questions (RESOLVED)

1. **What bucket should `handoff_failed` use in the run summary?**
   Resolution: Treat `handoff_failed` as `pending` and surface the handoff failure note on the summary item instead of inventing a new top-level bucket. This aligns with the locked Phase 5 scope and the planner's explicit Task 2 mapping in `05-01-PLAN.md`. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: .planning/phases/05-run-summaries/05-01-PLAN.md]

2. **Should completion auto-print be tied to root-task completion or a derived graph transition?**
   Resolution: Tie the one-time automatic print to the coordinator root message moving to `done`. That is the explicit Phase 5 integration decision encoded in `05-02-PLAN.md`, and it avoids inventing a second completion-state artifact. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: .planning/phases/05-run-summaries/05-02-PLAN.md]

3. **Is `pending` only a bucket label, or a first-class derived status?**
   Resolution: Treat `pending` as the fallback derived status for ordinary unread/active logical work that does not match a higher-precedence blocker or review rule. This keeps `SUM-01` aligned with the roadmap wording and the explicit status set defined in `05-01-PLAN.md`. [VERIFIED: .planning/ROADMAP.md][VERIFIED: .planning/REQUIREMENTS.md][VERIFIED: .planning/phases/05-run-summaries/05-01-PLAN.md]

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build, unit tests, full suite | ✓ [VERIFIED: `command -v go`] | `go1.26.1` [VERIFIED: `go version`] | — |
| `tmux` | Optional integration/manual validation paths | ✓ [VERIFIED: `command -v tmux`] | `3.6a` [VERIFIED: `tmux -V`] | Skip integration-tag tests and rely on unit coverage if tmux-backed validation is not needed immediately. [VERIFIED: .planning/codebase/TESTING.md] |
| `golangci-lint` | `make ci` lint phase | ✗ [VERIFIED: `command -v golangci-lint`] | — | No in-repo fallback for the exact lint gate; planning should treat full lint verification as environment setup or a human-run step. [VERIFIED: Makefile][VERIFIED: .planning/codebase/CONCERNS.md] |
| `gofumpt` | `make fmt` exact repo formatting | ✗ [VERIFIED: `command -v gofumpt`] | — | No exact repo-equivalent fallback is installed locally; `gofmt` would be partial only. [VERIFIED: Makefile][ASSUMED] |
| `goimports` | `make fmt` import normalization | ✗ [VERIFIED: `command -v goimports`] | — | No installed fallback for the exact repo formatter. [VERIFIED: Makefile] |

**Missing dependencies with no fallback:**

- `golangci-lint` for the repo’s exact `make ci` lint gate. [VERIFIED: Makefile][VERIFIED: `command -v golangci-lint`]
- `gofumpt` and `goimports` for the repo’s exact `make fmt` behavior. [VERIFIED: Makefile][VERIFIED: `command -v gofumpt`][VERIFIED: `command -v goimports`]

**Missing dependencies with fallback:**

- None beyond optional use of plain `gofmt`, which is not the repo’s exact formatting contract. [VERIFIED: Makefile][ASSUMED]

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard library `testing` package [VERIFIED: .planning/codebase/TESTING.md] |
| Config file | none — repo uses `Makefile` targets and package-local tests [VERIFIED: .planning/codebase/TESTING.md][VERIFIED: Makefile] |
| Quick run command | `Use the task-specific command from the verification map for the file you just changed; keep commit-level sampling on focused summary tests rather than package-wide runs.` [RESOLVED: 05-VALIDATION.md] |
| Full suite command | `go test ./... -count=1 -race` [VERIFIED: Makefile] |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SUM-01 | `run show` prints a compact run summary section that classifies logical work into completed/pending/waiting/blocked/under_review/escalated while keeping detail below it. [VERIFIED: .planning/REQUIREMENTS.md][VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | unit + cmd | `rg -n "type RunSummary|type RunSummaryItem|type RunSummaryStatus|func BuildRunSummary|func FormatRunSummary|TestBuildRunSummaryDerivesStatusBucketsAndReferences|TestBuildRunSummaryCollapsesReviewAndRerouteArtifactsIntoSourceRows|TestFormatRunSummaryGroupsItemsWithoutTaskDetailSprawl" internal/session/run_summary.go internal/session/run_summary_test.go && rg -n "TestFormatRunGraphIncludesSummaryBeforeTaskDetails|TestRunShowCommandPrintsSummaryUnderHeader" internal/session/run_rebuild_test.go cmd/tmuxicate/main_test.go` for red-test creation tasks, then the focused `go test` commands from later plan tasks once implementation exists. [RESOLVED: 05-01-PLAN.md][RESOLVED: 05-02-PLAN.md] | ❌ Wave 0 [VERIFIED: internal/session/run_rebuild_test.go currently has no summary tests][VERIFIED: `cmd/tmuxicate` currently has no tests] |
| SUM-02 | Each summary row shows the responsible/current owner plus task/message/review/blocker references for traceability. [VERIFIED: .planning/REQUIREMENTS.md][VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | unit + cmd | `go test ./internal/session -run 'TestBuildRunSummaryDerivesStatusBucketsAndReferences|TestBuildRunSummaryCollapsesReviewAndRerouteArtifactsIntoSourceRows' -count=1 && go test ./cmd/tmuxicate -run 'TestTaskDoneCommandPrintsSummaryOnlyForRootRunCompletion' -count=1` [RESOLVED: 05-01-PLAN.md][RESOLVED: 05-02-PLAN.md] | ❌ Wave 0 [VERIFIED: internal/session/run_rebuild_test.go currently has no summary tests][VERIFIED: `cmd/tmuxicate` currently has no tests] |

### Sampling Rate

- **Per task commit:** Run the exact task-level command from the verification map entry for the task you just changed; red-test creation tasks use `rg` existence checks first, then later implementation tasks use focused `go test` commands. [RESOLVED: 05-VALIDATION.md]
- **Per wave merge:** `go test ./... -count=1 -race` [VERIFIED: Makefile]
- **Phase gate:** Full suite green plus operator-visible confirmation that `run show` still starts with `Run:` and that the completion print fires only on the chosen edge. [VERIFIED: cmd/tmuxicate/main.go][ASSUMED]

### Wave 0 Gaps

- [ ] `internal/session/run_summary_test.go` — add red tests for summary contracts, precedence, descendant folding, and grouped formatting. [RESOLVED: 05-01-PLAN.md]
- [ ] `internal/session/run_rebuild_test.go` — add summary-ordering coverage for `FormatRunGraph`. [RESOLVED: 05-02-PLAN.md]
- [ ] `cmd/tmuxicate/main_test.go` — add Cobra execution tests for `run show` header/summary ordering and root-only completion printing. [RESOLVED: 05-02-PLAN.md]
- [x] Fallback semantics for `pending`, `approved`, and `handoff_failed` are now locked in the plans and no longer require discovery. [VERIFIED: .planning/phases/05-run-summaries/05-01-PLAN.md]
- [ ] Install `golangci-lint`, `gofumpt`, and `goimports` locally if the phase requires exact `make ci` / `make fmt` verification. [VERIFIED: Makefile][VERIFIED: environment availability audit]

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no [VERIFIED: .planning/codebase/ARCHITECTURE.md] | Local CLI workflow only; no networked auth subsystem is introduced by Phase 5. [VERIFIED: .planning/codebase/ARCHITECTURE.md] |
| V3 Session Management | no [VERIFIED: .planning/codebase/ARCHITECTURE.md] | No new session/token model; phase reuses existing local session state directories only. [VERIFIED: .planning/codebase/ARCHITECTURE.md][VERIFIED: internal/mailbox/paths.go] |
| V4 Access Control | no [VERIFIED: .planning/codebase/ARCHITECTURE.md] | No new access-control surface; summary is a local read-model over existing artifacts. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] |
| V5 Input Validation | yes [VERIFIED: internal/protocol/validation.go][VERIFIED: cmd/tmuxicate/main.go] | Keep all summary inputs artifact-backed and validated via existing run/task/review/blocker schema validation and Cobra arg handling. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/protocol/validation.go][VERIFIED: cmd/tmuxicate/main.go] |
| V6 Cryptography | no [VERIFIED: .planning/codebase/ARCHITECTURE.md] | Phase 5 introduces no new cryptographic surface; never hand-roll if later requirements change. [VERIFIED: .planning/codebase/ARCHITECTURE.md] |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Artifact drift between source task, review handoff, blocker case, and current task references [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/protocol/coordinator.go] | Tampering | Keep summary derived from `LoadRunGraph`, which already rejects broken task/message/review/blocker linkages instead of rendering partial truth. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/run_rebuild_test.go] |
| Operator-visible summary lies because it scrapes transcripts or optional freeform text [VERIFIED: .planning/PROJECT.md][VERIFIED: internal/session/task_cmd.go] | Repudiation | Restrict summary inputs to canonical task/review/blocker artifacts plus receipts; treat transcript text and `task done --summary` as non-authoritative enrichment only. [VERIFIED: .planning/PROJECT.md][VERIFIED: internal/session/task_cmd.go][ASSUMED] |
| Command-output ambiguity hides the real detail view [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md] | Information Disclosure / Integrity | Keep summary additive and shorter than the detail view, and preserve the durable task-local blocks below it. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/session/run_rebuild.go] |

## Sources

### Primary (HIGH confidence)

- `.planning/phases/05-run-summaries/05-CONTEXT.md` — locked decisions, scope boundary, and required operator surface. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md]
- `.planning/REQUIREMENTS.md` and `.planning/ROADMAP.md` — `SUM-01` / `SUM-02` and the two-plan phase split. [VERIFIED: .planning/REQUIREMENTS.md][VERIFIED: .planning/ROADMAP.md]
- `internal/session/run_rebuild.go` and `internal/session/run_rebuild_test.go` — existing rebuild seam, current task-local review/blocker rendering, and reusable fixtures. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/run_rebuild_test.go]
- `cmd/tmuxicate/main.go` — current `run show` command, `Run:` header guard, and adjacent operator-facing mutation commands. [VERIFIED: cmd/tmuxicate/main.go]
- `internal/session/task_cmd.go`, `internal/session/review_response.go`, and `internal/session/blocker_resolve.go` — current workflow mutation points and completion transitions. [VERIFIED: internal/session/task_cmd.go][VERIFIED: internal/session/review_response.go][VERIFIED: internal/session/blocker_resolve.go]
- `internal/protocol/coordinator.go` and `internal/protocol/validation.go` — canonical task/review/blocker fields and validation invariants. [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/protocol/validation.go]
- `.planning/codebase/TESTING.md`, `.planning/codebase/ARCHITECTURE.md`, and `.planning/codebase/CONCERNS.md` — test strategy, layering constraints, and known session/CLI coverage gaps. [VERIFIED: .planning/codebase/TESTING.md][VERIFIED: .planning/codebase/ARCHITECTURE.md][VERIFIED: .planning/codebase/CONCERNS.md]
- Local command outputs: targeted `go test` runs, `go version`, `go list -m -json`, and environment-availability probes. [VERIFIED: command outputs captured during this research session]

### Secondary (MEDIUM confidence)

- None. [VERIFIED: this research relied on local codebase artifacts and local tool output only]

### Tertiary (LOW confidence)

- None. [VERIFIED: this research did not rely on unverified community posts or secondary web search]

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH - Phase 5 reuses the repo’s existing Go/Cobra/YAML/session stack and adds no new dependency decisions. [VERIFIED: go.mod][VERIFIED: internal/session/run_rebuild.go][VERIFIED: cmd/tmuxicate/main.go]
- Architecture: MEDIUM - The aggregation seam is clear, but fallback semantics for `pending`, `approved`, `handoff_failed`, and the preferred completion-print edge still need explicit planning choices. [VERIFIED: .planning/phases/05-run-summaries/05-CONTEXT.md][VERIFIED: internal/protocol/coordinator.go][ASSUMED]
- Pitfalls: HIGH - The main failure modes are directly observable in current code (`Run:` header guard, agent-global state file, descendant task duplication risk, missing summary tests). [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: internal/session/run_rebuild.go][VERIFIED: .planning/codebase/CONCERNS.md]

**Research date:** 2026-04-06  
**Valid until:** 2026-05-06 for this codebase state, or until Phase 5 implementation materially changes `run show`, workflow completion semantics, or artifact contracts. [VERIFIED: git status showed a clean worktree before writing this file][ASSUMED]
