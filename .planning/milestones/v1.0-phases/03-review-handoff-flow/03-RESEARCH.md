# Phase 3: Review Handoff Flow - Research

**Researched:** 2026-04-05  
**Domain:** Coordinator-driven review handoff inside the existing durable run/task/mailbox workflow [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/session/task_cmd.go][VERIFIED: internal/session/run.go]  
**Confidence:** HIGH [VERIFIED: codebase inspection][VERIFIED: go test ./... -count=1]

<user_constraints>
## User Constraints (from CONTEXT.md)

Verbatim copy from `.planning/phases/03-review-handoff-flow/03-CONTEXT.md`. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]

### Locked Decisions
- **D-01:** A `review_required=true` implementation task must trigger review-task creation automatically in code immediately after the implementation task's durable `done` transition is recorded.
- **D-02:** Automatic handoff must reuse `RouteChildTask(TaskClass=review)` and must not depend on coordinator prompt parsing, coordinator message reading, or implementer-authored ad hoc review requests.
- **D-03:** If review handoff creation fails, the original implementation task remains `done`; the system records a linked fail-loud handoff failure instead of rolling back completion.
- **D-04:** Duplicate review handoffs are prevented by a source-task linkage check, not by Phase 2's review fanout duplicate-key policy.
- **D-05:** The canonical review-chain link is a dedicated `ReviewHandoff` artifact stored at `coordinator/runs/<run-id>/reviews/<source-task-id>.yaml`.
- **D-06:** Source implementation tasks and review tasks must not store redundant reverse-pointer fields to each other; the dedicated handoff artifact is the sole canonical linkage record.
- **D-07:** Review-response linkage and final review outcome must be recorded on the same `ReviewHandoff` artifact.
- **D-08:** Handoff uniqueness is enforced by the existence of `reviews/<source-task-id>.yaml`, not by scanning for generic review tasks or relying on duplicate-key semantics.
- **D-09:** Both `approved` and `changes_requested` outcomes leave the source implementation task in its existing `done` state; review outcome does not reopen or retag the implementation task in Phase 3.
- **D-10:** Phase 3 scope ends at durable review-outcome recording plus operator visibility. Automatic follow-up implementation-task generation for `changes_requested` is explicitly out of scope.
- **D-11:** Reviewer outcome is submitted through a dedicated review-response CLI surface rather than by extending generic `task done` with outcome semantics.
- **D-12:** When a reviewer responds, `ReviewHandoff` records `response_message_id`, `outcome`, `responded_at`, and `status=responded`.
- **D-13:** Review-chain visibility is integrated into the existing `tmuxicate run show` output; each source implementation task renders a derived review-handoff block directly underneath the task.
- **D-14:** Review tasks remain visible as normal child tasks in the regular task list; the derived handoff block is an additional linkage view, not a replacement task view.
- **D-15:** The minimum review-handoff information shown to operators is `status`, `review_task_id`, reviewer owner, `response_message_id`, `outcome`, and a failure summary when routing/handoff failed.
- **D-16:** Phase 3 does not add a separate review-only command or filter such as `run show --reviews-only`; the existing `run show` surface remains the only required inspection entrypoint.

### Claude's Discretion
- Exact Go type names and helper-function names for `ReviewHandoff`, review-response request structs, and rebuild helpers, as long as the semantics above remain intact.
- Exact CLI flag names for the dedicated review-response command, provided the interface is clearly review-specific and does not overload generic `task done`.
- Exact YAML field ordering and formatting for review-handoff artifacts, provided they remain durable, validated, and readable from disk.

### Deferred Ideas (OUT OF SCOPE)
- Automatic creation of follow-up implementation tasks when review outcome is `changes_requested`.
- Review-outcome-driven workflow branching, retry, or escalation policy.
- Separate review-only inspection commands or filters such as `run show --reviews-only`.
- Any richer lifecycle state beyond current task `done` plus handoff/outcome metadata, such as introducing a new `reviewed` task state for implementation tasks.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REVIEW-01 | Coordinator can hand completed implementation work to a reviewer as a linked follow-up task [VERIFIED: .planning/REQUIREMENTS.md] | Extend `TaskDone` after the durable `done` transition, persist `ReviewHandoff`, and create the review child task through `RouteChildTask(TaskClass=review)` instead of prompt-driven routing [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/session/task_cmd.go][VERIFIED: internal/session/run.go] |
| REVIEW-02 | Reviewer response remains linked to the originating coordinator run so the operator can trace implementation and review in one flow [VERIFIED: .planning/REQUIREMENTS.md] | Add a dedicated review-response CLI, update the same `ReviewHandoff` artifact with `response_message_id/outcome/responded_at/status`, and teach `LoadRunGraph` plus `FormatRunGraph` to render the chain from disk alone [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/session/reply.go][VERIFIED: internal/session/run_rebuild.go] |
</phase_requirements>

## Summary

Phase 3 should stay entirely inside the existing Go CLI, session, mailbox, and rebuild boundaries rather than introducing a second coordinator store or a prompt-parsed review workflow. `TaskDone` already owns the durable completion transition, `RouteChildTask` already owns deterministic review-task creation, `replyKind` already recognizes `review_request -> review_response`, and `run show` already rebuilds task state from disk. The plan should extend those seams instead of creating parallel coordinator logic. [VERIFIED: .planning/PROJECT.md][VERIFIED: internal/session/task_cmd.go][VERIFIED: internal/session/run.go][VERIFIED: internal/session/reply.go][VERIFIED: internal/session/run_rebuild.go]

The main structural gap is storage and lookup support around coordinator artifacts. `CoordinatorStore` currently creates and reads runs and only creates tasks; it does not yet read tasks by message ID, list tasks for a run, or update any existing coordinator artifact. Phase 3 needs that missing storage seam for both automatic review handoff after `task done` and outcome recording after reviewer response. [VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/session/task_cmd.go][VERIFIED: internal/session/run_rebuild.go]

The main design tension to resolve in planning is message kind, not ownership or storage. Current routed child tasks always emit `kind: task`, while reviewer reply specialization only happens when the parent message is `kind: review_request`. Phase 3 therefore needs an explicit plan for how review handoff creates or represents review-request messages so reviewer output can become a first-class `review_response` instead of a generic `note`. No `CLAUDE.md` or project-local skill overrides were present in the repo, so the effective constraints come from `AGENTS.md` plus the planning artifacts already loaded. [VERIFIED: internal/session/run.go][VERIFIED: internal/session/reply.go][VERIFIED: cmd/tmuxicate/main.go][VERIFIED: repo root]

**Primary recommendation:** Plan Phase 3 as one durable chain: `TaskDone` post-commit hook -> `ReviewHandoff` create/update -> routed review task creation -> dedicated review-response command -> `run show` rebuild/render, with no transcript parsing and no second source of truth. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/session/task_cmd.go][VERIFIED: internal/session/run.go][VERIFIED: internal/session/run_rebuild.go]

Environment availability audit was intentionally skipped as a standalone section because Phase 3 is a code-and-artifact change inside the existing Go/Cobra/YAML/mailbox stack; no new external service, runtime, or CLI dependency was identified beyond the repo baseline already validated locally. [VERIFIED: go.mod][VERIFIED: cmd/tmuxicate/main.go][VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: go test ./... -count=1]

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go toolchain | `1.26.1` [VERIFIED: go.mod][VERIFIED: go version] | Compile and test all Phase 3 changes in the existing CLI/runtime [VERIFIED: go.mod] | The repo is pinned to this toolchain and the current full suite is green on it [VERIFIED: go test ./... -count=1] |
| `github.com/spf13/cobra` | `v1.10.2` [VERIFIED: go.mod] | Add the dedicated review-response CLI surface next to existing `run`, `reply`, and `task` commands [VERIFIED: cmd/tmuxicate/main.go] | The command tree already owns neighboring workflows, so Phase 3 should extend it instead of inventing a second entrypoint layer [VERIFIED: cmd/tmuxicate/main.go] |
| `gopkg.in/yaml.v3` | `v3.0.1` [VERIFIED: go.mod] | Persist `ReviewHandoff` artifacts as human-readable run-scoped YAML [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Existing canonical run/task artifacts already use YAML and are validated on reload [VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/session/run_rebuild.go] |
| `internal/mailbox.CoordinatorStore` | repo current [VERIFIED: internal/mailbox/coordinator_store.go] | Remain the authoritative persistence boundary for run/task/review artifacts [VERIFIED: internal/mailbox/coordinator_store.go] | Project constraints explicitly reject a second orchestration system [VERIFIED: .planning/PROJECT.md] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/session.TaskDone` | repo current [VERIFIED: internal/session/task_cmd.go] | Existing post-completion seam after durable `done` receipt transition [VERIFIED: internal/session/task_cmd.go] | Use it as the only automatic handoff trigger point [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] |
| `internal/session.RouteChildTask` | repo current [VERIFIED: internal/session/run.go] | Deterministic owner selection and child-task persistence for review work [VERIFIED: internal/session/run.go] | Use it for the review task itself, not for source-task uniqueness checks [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] |
| `internal/session.Reply` and `replyKind` | repo current [VERIFIED: internal/session/reply.go] | Existing reply transport and parent-kind-aware message typing [VERIFIED: internal/session/reply.go] | Reuse or mirror this seam when building the dedicated review-response workflow [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] |
| `internal/session.LoadRunGraph` and `FormatRunGraph` | repo current [VERIFIED: internal/session/run_rebuild.go] | Current operator inspection surface rebuilt from disk [VERIFIED: internal/session/run_rebuild.go] | Extend it to load and render review-handoff blocks under source tasks [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Canonical `reviews/<source-task-id>.yaml` handoff artifact [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Reverse pointers on source and review task YAML [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Rejected because it duplicates truth and makes rebuild validation drift-prone [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] |
| Dedicated review-response CLI [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Extend generic `tmuxicate task done` with outcome flags [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Rejected because task completion and review outcome are different workflow events and current `task done` is intentionally generic [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: internal/session/task_cmd.go] |
| Code-driven handoff via `RouteChildTask(TaskClass=review)` [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Prompt parsing or freeform coordinator mail reading [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Rejected because the project philosophy favors deterministic, inspectable workflow state over model-dependent behavior [VERIFIED: .planning/PROJECT.md] |

**Installation:** No new external dependency is recommended for Phase 3; stay on the current module graph and validate with the existing test commands. [VERIFIED: go.mod][VERIFIED: Makefile]

```bash
go test ./internal/session -count=1
make test
```

**Version verification:** The recommended stack is the repo-pinned baseline, not a new package search: Go `1.26.1`, Cobra `v1.10.2`, and YAML v3 `v3.0.1` are already present, and the local full suite currently passes. [VERIFIED: go.mod][VERIFIED: go version][VERIFIED: go test ./... -count=1]

## Architecture Patterns

### Recommended Project Structure

```text
cmd/tmuxicate/main.go                         # dedicated review-response CLI wiring
internal/protocol/                           # ReviewHandoff schema + validation
internal/mailbox/                            # ReviewHandoff CRUD + lookup helpers
internal/session/                            # post-done handoff orchestration + run show rebuild

<state-dir>/coordinator/runs/<run-id>/
  run.yaml
  tasks/<task-id>.yaml
  reviews/<source-task-id>.yaml
```

The code location split follows the current CLI/session/mailbox/protocol layering, and the new on-disk subtree follows the locked `reviews/<source-task-id>.yaml` contract. [VERIFIED: .planning/codebase/ARCHITECTURE.md][VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]

### Pattern 1: Post-Done Handoff Hook

**What:** Extend `TaskDone` only after the receipt update and `active -> done` move have succeeded, because that is the durable completion point already used by the current workflow. [VERIFIED: internal/session/task_cmd.go]  
**When to use:** Trigger only for implementation tasks that belong to a coordinator run and have `review_required=true`. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/protocol/coordinator.go]  
**Example:** Current routed review-task creation already exists as a verified pattern:

```go
task, decision, err := RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
	RunID:          run.RunID,
	TaskClass:      protocol.TaskClassReview,
	Domains:        []string{"session", "protocol"},
	Goal:           "Review the implementation routing outcome",
	ExpectedOutput: "A review artifact for the same work item",
	ReviewRequired: true,
})
```

Source: `internal/session/run_test.go` routed review fanout pattern. [VERIFIED: internal/session/run_test.go]

### Pattern 2: Canonical ReviewHandoff Artifact

**What:** Keep source-task linkage, review-task linkage, response linkage, outcome, and failure state on one `ReviewHandoff` artifact keyed by source task ID; do not add reverse pointers to either task record. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]  
**When to use:** Create or update this artifact at automatic handoff time and again when the reviewer responds. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]  
**Example:** Minimum locked semantics:

```yaml
status: responded
review_task_id: task_000000000222
response_message_id: msg_000000000333
outcome: approved
responded_at: 2026-04-05T12:34:56Z
```

Exact field names beyond these semantics remain discretionary, but the path and response fields above are locked. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]

### Pattern 3: Task-Centric Rebuild and Show

**What:** `LoadRunGraph` should load and validate review handoffs from disk, and `FormatRunGraph` should render a derived handoff block directly beneath the source implementation task while still leaving the review task in the normal task list. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/session/run_rebuild.go]  
**When to use:** Every `tmuxicate run show <run-id>` call. [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: internal/session/run_rebuild.go]  
**Example:** Target output shape:

```text
Task: task_000000000111
Owner: backend
...
Review Handoff: responded
Review Task: task_000000000222
Reviewer: qa
Response: msg_000000000333
Outcome: approved
```

This preserves the current task-centric operator surface instead of requiring a second inspection command. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]

### Pattern 4: Dedicated Review-Response Command

**What:** Add a review-specific command next to `reply` and `task done`, and reuse the existing reply-body input helper shape rather than overloading generic task completion semantics. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: cmd/tmuxicate/main.go]  
**When to use:** Reviewer submits `approved` or `changes_requested` for a linked review task. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]  
**Example:** Command surface shape to plan for:

```bash
tmuxicate review respond <review-message-id> --outcome approved --stdin
```

Exact flag names are discretionary, but the command must remain review-specific and must not overload `task done`. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]

### Anti-Patterns to Avoid

- **Prompt-derived review routing:** Do not infer reviewer, task class, or handoff timing from coordinator prose or transcripts; use `TaskDone` plus `RouteChildTask` plus durable artifacts. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]
- **Reverse-pointer fields on task YAML:** Do not write `review_task_id` back onto source tasks or `source_task_id` back onto review tasks as canonical state. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]
- **Rollback on handoff failure:** Do not undo the original `done` transition if review creation fails; record the failure on the handoff artifact instead. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]
- **Separate review-only inspection surfaces:** Do not add `run show --reviews-only` or a second review chain command in Phase 3. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Reviewer selection | A second reviewer-selection engine [VERIFIED: .planning/PROJECT.md] | `RouteChildTask(TaskClass=review)` [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/session/run.go] | Phase 2 already solved deterministic owner selection and duplicate policy within the current run model [VERIFIED: .planning/phases/02-role-based-routing/02-VERIFICATION.md] |
| Run/review linkage | Transcript parsing or ad hoc body scraping [VERIFIED: .planning/PROJECT.md] | `ReviewHandoff` + task YAML + immutable message thread/reply metadata [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/protocol/envelope.go] | The product value is durable, inspectable workflow state, not transcript reconstruction [VERIFIED: .planning/PROJECT.md][VERIFIED: DESIGN.md] |
| Review outcome transport | Outcome flags on generic `task done` [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Dedicated review-response command and review-specific artifact update [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Generic task completion is already shared by non-review work and should stay generic [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: internal/session/task_cmd.go] |
| Artifact authority | Reverse indexes or a second coordinator DB [VERIFIED: .planning/PROJECT.md] | Existing coordinator run directory under the session state tree [VERIFIED: internal/mailbox/paths.go][VERIFIED: internal/mailbox/coordinator_store.go] | The repo already treats the filesystem under the configured state dir as authoritative [VERIFIED: .planning/codebase/ARCHITECTURE.md] |

**Key insight:** Phase 3 fits the existing architecture cleanly, but only if the plan adds narrow storage/update helpers instead of improvising lookup logic inside CLI handlers or transcript readers. [VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: cmd/tmuxicate/main.go][VERIFIED: .planning/PROJECT.md]

## Common Pitfalls

### Pitfall 1: Guessing the Source Task Instead of Looking It Up

**What goes wrong:** `TaskDone` currently knows only `stateDir`, `agent`, `message_id`, and receipt/message state, so review handoff code can drift into parsing body text or transcripts to recover task linkage. [VERIFIED: internal/session/task_cmd.go]  
**Why it happens:** `CoordinatorStore` has no current helper to read tasks by `message_id` or to list tasks for a run. [VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/session/run_rebuild.go]  
**How to avoid:** Plan explicit coordinator-store lookup helpers before implementing automatic handoff logic. [VERIFIED: code gap from internal/mailbox/coordinator_store.go]  
**Warning signs:** Any design that mentions transcript parsing, `body.md` regexes, or coordinator prompt text as the source of task identity. [VERIFIED: .planning/PROJECT.md]

### Pitfall 2: Starting Review Handoff Before Completion Is Durable

**What goes wrong:** A routed review task may exist even though the source task never durably reached `done`, which makes the chain ambiguous to operators. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]  
**Why it happens:** `TaskDone` currently performs a receipt update, then a folder move, then an event append; inserting handoff creation before that sequence finishes would break the locked ordering. [VERIFIED: internal/session/task_cmd.go][VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]  
**How to avoid:** Hook handoff creation after the `active -> done` move succeeds and keep failures fail-loud on the handoff artifact. [VERIFIED: internal/session/task_cmd.go][VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]  
**Warning signs:** Review artifacts appear when the source receipt is still `active` or the task.done event is missing. [VERIFIED: internal/session/task_cmd.go]

### Pitfall 3: Treating `task done --summary` as Required Review Input

**What goes wrong:** Review request creation becomes brittle or empty because the source implementation summary is optional, not required. [VERIFIED: cmd/tmuxicate/main.go]  
**Why it happens:** `newTaskDoneCmd` accepts `--summary` as an optional flag and `TaskDone` persists it only in the state event log, not in canonical task YAML. [VERIFIED: cmd/tmuxicate/main.go][VERIFIED: internal/session/task_cmd.go]  
**How to avoid:** Build review handoff from durable task/run/message identifiers first; treat the completion summary as optional enrichment only. [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/session/task_cmd.go]  
**Warning signs:** The proposed review body cannot be built when `summary == ""`. [VERIFIED: internal/session/task_cmd.go]

### Pitfall 4: Non-Routed Source Tasks Have No Deterministic Review Domains

**What goes wrong:** Automatic review routing cannot determine `domains` for `RouteChildTask` when the source implementation task was created through plain `add-task` without route metadata. [VERIFIED: internal/session/run_contracts.go][VERIFIED: internal/protocol/validation.go]  
**Why it happens:** `RouteChildTaskRequest.Validate()` requires non-empty domains, but `ChildTask` only requires `domains/normalized_domains` when routing metadata is present. [VERIFIED: internal/protocol/validation.go]  
**How to avoid:** The safest Phase 3 plan is fail-loud: if the source task lacks routed domains, persist a handoff failure summary instead of inventing domains. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/protocol/validation.go]  
**Warning signs:** Source task YAML has empty `task_class`, `domains`, and `normalized_domains`, but the design still assumes automatic review routing can proceed. [VERIFIED: internal/protocol/coordinator.go][VERIFIED: internal/protocol/validation.go]

### Pitfall 5: Review Message Kind Mismatch

**What goes wrong:** Reviewer output lands as a generic `note` instead of `review_response`, which weakens traceability and makes handoff-specific behavior harder to validate. [VERIFIED: internal/session/reply.go][VERIFIED: internal/session/run.go]  
**Why it happens:** `addChildTaskWithResolvedOwner()` always creates `kind: task` messages today, and `replyKind()` only upgrades replies when the parent is `kind: review_request`. [VERIFIED: internal/session/run.go][VERIFIED: internal/session/reply.go]  
**How to avoid:** Make message-kind handling an explicit Wave 0 design task: either allow routed review-task creation to emit `KindReviewRequest`, or let the dedicated review-response command create `KindReviewResponse` directly. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/session/reply.go]  
**Warning signs:** A proposed implementation still uses generic `Reply()` against a review task whose envelope kind is `task`. [VERIFIED: internal/session/reply.go][VERIFIED: internal/session/run.go]

### Pitfall 6: Confusing Review Fanout Policy With Review Handoff Uniqueness

**What goes wrong:** Phase 3 accidentally relies on Phase 2 duplicate-key fanout rules to decide whether a source task already has a review handoff. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: .planning/phases/02-role-based-routing/02-VERIFICATION.md]  
**Why it happens:** Review tasks may legitimately fan out under Phase 2, but Phase 3 explicitly locks handoff uniqueness to `reviews/<source-task-id>.yaml`. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]  
**How to avoid:** Gate idempotency on handoff artifact existence first, then use routing duplicate policy only for the review task creation step itself. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/session/run.go]  
**Warning signs:** The implementation scans generic review tasks or duplicate keys instead of checking the canonical handoff file path. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]

## Code Examples

Verified patterns from the current repo:

### Durable Completion Happens Before Any New Handoff Logic

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
```

Source: `internal/session/task_cmd.go`. [VERIFIED: internal/session/task_cmd.go]

### Routed Review Task Creation Already Exists

```go
task, decision, err := RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
	RunID:          run.RunID,
	TaskClass:      protocol.TaskClassReview,
	Domains:        []string{"session", "protocol"},
	Goal:           "Review the implementation routing outcome",
	ExpectedOutput: "A review artifact for the same work item",
	ReviewRequired: true,
})
```

Source: `internal/session/run_test.go`. [VERIFIED: internal/session/run_test.go]

### Parent Kind Controls Reply Kind

```go
func replyKind(parent protocol.Kind) protocol.Kind {
	switch parent {
	case protocol.KindReviewRequest:
		return protocol.KindReviewResponse
	case protocol.KindStatusRequest:
		return protocol.KindStatusResponse
	default:
		return protocol.KindNote
	}
}
```

Source: `internal/session/reply.go`. [VERIFIED: internal/session/reply.go]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Transcript reconstruction or generic task scanning for review linkage [VERIFIED: .planning/PROJECT.md] | Canonical `ReviewHandoff` artifact keyed by source task ID [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Phase 3 decisions gathered on 2026-04-05 [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | One inspectable linkage record replaces inference and duplicate truth [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] |
| Generic `task done` semantics for all outcomes [VERIFIED: cmd/tmuxicate/main.go] | Dedicated review-response CLI for `approved` and `changes_requested` [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Phase 3 decisions gathered on 2026-04-05 [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Keeps task completion generic and review outcome explicit [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] |
| Separate or implicit operator surfaces [VERIFIED: .planning/PROJECT.md] | `tmuxicate run show` remains the single required review-chain inspection surface [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Phase 3 decisions gathered on 2026-04-05 [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Operators can inspect the full chain without transcript spelunking or extra commands [VERIFIED: .planning/ROADMAP.md][VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] |

**Deprecated/outdated:**
- Prompt-dependent or transcript-dependent review routing is not compatible with the current project philosophy or the locked Phase 3 decisions. [VERIFIED: .planning/PROJECT.md][VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]

## Assumptions Log

All substantive claims in this research were verified against local planning artifacts, repo code, or local command output; no user confirmation is required for unstated assumptions. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/session/run.go][VERIFIED: go test ./... -count=1]

## Open Questions (RESOLVED)

1. **How should the review task message be typed?** [VERIFIED: internal/session/run.go][VERIFIED: internal/session/reply.go]  
   Resolution: routed review child-task creation should emit `kind: review_request` instead of generic `task`, and the dedicated review-response command can then reuse the existing `Reply()` path so the response message is typed `review_response` through `replyKind()`. This keeps message kinds explicit, operator-visible, and consistent with the dedicated review-response CLI decision from `03-CONTEXT.md`. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/session/reply.go][VERIFIED: cmd/tmuxicate/main.go]

2. **What should happen for source implementation tasks that lack routed domains?** [VERIFIED: internal/protocol/validation.go][VERIFIED: internal/session/run_contracts.go]  
   Resolution: Phase 3 should fail loudly. If a source implementation task has no usable `normalized_domains`, automatic handoff must record `status: handoff_failed` plus a failure summary on the canonical `ReviewHandoff` artifact and must not invent review-routing domains or roll back the source task's `done` state. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: .planning/PROJECT.md]

3. **How broad should new coordinator-store helpers be?** [VERIFIED: internal/mailbox/coordinator_store.go]  
   Resolution: keep Phase 3 narrow and review-focused. Add the minimum helpers the plans require, such as `ReadTask`, targeted review-handoff CRUD/update helpers, and lookup support for response recording. Do not generalize the coordinator store into a second orchestration layer in this phase. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/session/run_rebuild.go]

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard library `testing` on Go `1.26.1` [VERIFIED: .planning/codebase/TESTING.md][VERIFIED: go.mod][VERIFIED: go version] |
| Config file | none; test conventions are package-local `*_test.go` plus `Makefile` commands [VERIFIED: .planning/codebase/TESTING.md][VERIFIED: Makefile] |
| Quick run command | `go test ./internal/session -count=1` [VERIFIED: go test ./internal/session -count=1] |
| Full suite command | `make test` [VERIFIED: Makefile] |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REVIEW-01 | Completing a review-required implementation task creates or updates a linked review handoff and routes a review child task without losing run linkage [VERIFIED: .planning/REQUIREMENTS.md][VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | unit [VERIFIED: .planning/codebase/TESTING.md] | `go test ./internal/session -run TestTaskDoneCreatesReviewHandoffAndRoutesReview -count=1` [VERIFIED: recommended test shape from internal/session/task_cmd.go] | ❌ Wave 0 [VERIFIED: rg -n "TaskDone|task.done" internal/session/*_test.go] |
| REVIEW-02 | Reviewer response records `response_message_id/outcome/responded_at/status` on the canonical handoff and becomes visible through `run show` [VERIFIED: .planning/REQUIREMENTS.md][VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | unit [VERIFIED: .planning/codebase/TESTING.md] | `go test ./internal/session -run 'TestReviewRespondRecordsOutcome|TestRunShowIncludesReviewHandoffBlock' -count=1` [VERIFIED: recommended test shape from internal/session/reply.go][VERIFIED: internal/session/run_rebuild.go] | ❌ Wave 0 [VERIFIED: internal/session/run_rebuild_test.go][VERIFIED: rg -n "TaskDone|task.done" internal/session/*_test.go] |

### Sampling Rate

- **Per task commit:** `go test ./internal/session -count=1` [VERIFIED: go test ./internal/session -count=1]
- **Per wave merge:** `make test` [VERIFIED: Makefile]
- **Phase gate:** Full suite green before `/gsd-verify-work` [VERIFIED: .planning/config.json]

### Wave 0 Gaps

- [ ] `internal/session/task_cmd_test.go` or equivalent targeted session test coverage for post-`done` handoff creation, idempotency, and fail-loud failure recording [VERIFIED: internal/session/task_cmd.go][VERIFIED: rg -n "TaskDone|task.done" internal/session/*_test.go]
- [ ] `internal/session/review_response_test.go` or equivalent coverage for dedicated review-response recording and `review_response` message creation semantics [VERIFIED: internal/session/reply.go][VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]
- [ ] `internal/session/run_rebuild_test.go` additions for review-handoff rendering and mismatched source/review/response links [VERIFIED: internal/session/run_rebuild_test.go][VERIFIED: internal/session/run_rebuild.go]
- [ ] Optional CLI wiring test for the new review-response command, because `cmd/tmuxicate` still has no direct tests today [VERIFIED: go test ./... -count=1][VERIFIED: cmd/tmuxicate/main.go]

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no [VERIFIED: .planning/codebase/ARCHITECTURE.md] | The current product is a local CLI with config-based agent identities, not a networked auth subsystem [VERIFIED: .planning/codebase/ARCHITECTURE.md] |
| V3 Session Management | no [VERIFIED: .planning/codebase/ARCHITECTURE.md] | There is no web or token session layer in the current architecture [VERIFIED: .planning/codebase/ARCHITECTURE.md] |
| V4 Access Control | yes [VERIFIED: internal/session/run.go][VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Keep allowed-owner checks, reviewer ownership validation, and fail-loud linkage validation around response recording [VERIFIED: internal/session/run.go][VERIFIED: internal/session/run_rebuild.go] |
| V5 Input Validation | yes [VERIFIED: internal/protocol/validation.go][VERIFIED: cmd/tmuxicate/main.go] | Reuse protocol validation and explicit CLI/request validation for review-handoff fields, outcomes, task IDs, and message IDs [VERIFIED: internal/protocol/validation.go] |
| V6 Cryptography | yes [VERIFIED: internal/protocol/envelope.go][VERIFIED: internal/mailbox/store.go] | Reuse existing immutable message body hashing and do not invent a second integrity scheme for review linkage [VERIFIED: internal/mailbox/store.go] |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Review-handoff artifact drift between source task, review task, and response message [VERIFIED: internal/session/run_rebuild.go][VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Tampering | Validate all links during `LoadRunGraph`, just as current rebuild already fails loudly on task/root-message mismatches [VERIFIED: internal/session/run_rebuild.go] |
| Wrong agent records review outcome for a review task they do not own [VERIFIED: current owner linkage model from internal/session/task_cmd.go][VERIFIED: internal/session/run.go] | Spoofing | Require response recording to validate reviewer ownership against the linked review task and receipt state before updating `ReviewHandoff` [VERIFIED: existing owner/receipt model in internal/session/task_cmd.go][VERIFIED: internal/session/run.go] |
| Duplicate review handoff creation on repeated `task done` calls or retries [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] | Tampering | Gate idempotency on `reviews/<source-task-id>.yaml` existence and keep creation/update under a run-scoped lock [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md][VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/mailbox/paths.go] |
| Outcome trace only exists in transcripts or pane text [VERIFIED: .planning/PROJECT.md] | Repudiation | Persist `response_message_id/outcome/responded_at/status` on the handoff artifact and render them in `run show` [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md] |

## Sources

### Primary (HIGH confidence)

- `.planning/phases/03-review-handoff-flow/03-CONTEXT.md` - locked review-handoff decisions, scope, and operator-surface requirements. [VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]
- `.planning/REQUIREMENTS.md` - `REVIEW-01` and `REVIEW-02` requirement definitions. [VERIFIED: .planning/REQUIREMENTS.md]
- `.planning/STATE.md` - current project position and reminder that session/runtime coverage still matters for coordinator automation. [VERIFIED: .planning/STATE.md]
- `.planning/PROJECT.md` and `.planning/ROADMAP.md` - product constraints, phase goal, and success criteria. [VERIFIED: .planning/PROJECT.md][VERIFIED: .planning/ROADMAP.md]
- `internal/session/task_cmd.go` - current durable `task done` transition and missing automatic handoff behavior. [VERIFIED: internal/session/task_cmd.go]
- `internal/session/run.go` and `internal/session/run_test.go` - routed child-task creation, review task fanout behavior, and duplicate policy. [VERIFIED: internal/session/run.go][VERIFIED: internal/session/run_test.go]
- `internal/session/reply.go` - current reply-kind specialization for `review_request` and `review_response`. [VERIFIED: internal/session/reply.go]
- `internal/session/run_rebuild.go` and `internal/session/run_rebuild_test.go` - current rebuild and operator inspection surface. [VERIFIED: internal/session/run_rebuild.go][VERIFIED: internal/session/run_rebuild_test.go]
- `internal/mailbox/coordinator_store.go` and `internal/mailbox/paths.go` - authoritative coordinator artifact persistence boundary and current run/task paths. [VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/mailbox/paths.go]
- `go.mod`, `go version`, `Makefile`, and `go test ./... -count=1` - current toolchain baseline and validation commands. [VERIFIED: go.mod][VERIFIED: go version][VERIFIED: Makefile][VERIFIED: go test ./... -count=1]

### Secondary (MEDIUM confidence)

- None. All important claims were verified directly from local planning artifacts, source code, or local command output. [VERIFIED: local repo inspection]

### Tertiary (LOW confidence)

- None. [VERIFIED: local repo inspection]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Phase 3 should use the repo-pinned Go/Cobra/YAML stack and current internal packages; no new external dependency is warranted. [VERIFIED: go.mod][VERIFIED: cmd/tmuxicate/main.go][VERIFIED: internal/mailbox/coordinator_store.go]
- Architecture: HIGH - The exact implementation seams are already present in `TaskDone`, `RouteChildTask`, `Reply`, and `run show`; the remaining work is durable linkage plus validation. [VERIFIED: internal/session/task_cmd.go][VERIFIED: internal/session/run.go][VERIFIED: internal/session/reply.go][VERIFIED: internal/session/run_rebuild.go]
- Pitfalls: HIGH - The main failure modes are directly observable from current code gaps and locked phase decisions, especially message-kind mismatch, missing task lookup helpers, and non-routed domain gaps. [VERIFIED: internal/mailbox/coordinator_store.go][VERIFIED: internal/protocol/validation.go][VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]

**Research date:** 2026-04-05 [VERIFIED: current session date]  
**Valid until:** 2026-04-12 or until `03-CONTEXT.md` changes, because the repo and phase decisions are still moving during active planning. [VERIFIED: .planning/STATE.md][VERIFIED: .planning/phases/03-review-handoff-flow/03-CONTEXT.md]
