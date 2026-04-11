# Phase 3: Review Handoff Flow - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-05
**Phase:** 03-Review Handoff Flow
**Areas discussed:** Review creation trigger, Link model, Review outcome handling, Operator inspection surface

---

## Review creation trigger

### Trigger owner

| Option | Description | Selected |
|--------|-------------|----------|
| System automatic | After an implementation task reaches durable `done` and `review_required=true`, code automatically calls `RouteChildTask(TaskClass=review)` | ✓ |
| Coordinator explicit | Coordinator reads completion and manually creates the review task | |
| Implementer request | Implementer sends a review request directly when finishing work | |

**User's choice:** System automatic.
**Notes:** The trigger is coordinator workflow code, not prompt interpretation. This keeps review handoff deterministic, inspectable, and multi-vendor.

### Trigger timing

| Option | Description | Selected |
|--------|-------------|----------|
| Durable `done` transition | Fire immediately after the implementation task's durable `done` write succeeds | ✓ |
| Coordinator acknowledgment | Wait for coordinator to read/acknowledge completion first | |

**User's choice:** Durable `done` transition.
**Notes:** The current system has a real `done` transition but no separate coordinator-ack lifecycle. Adding one would widen scope.

### Trigger path

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse `RouteChildTask(TaskClass=review)` | Use Phase 2's deterministic routing path for review handoff | ✓ |
| Coordinator prompt logic | Let coordinator prompt or model behavior decide the review owner | |
| Implementer direct mailbox request | Allow the specialist implementer to originate review workflow directly | |

**User's choice:** Reuse `RouteChildTask(TaskClass=review)`.
**Notes:** This preserves the Phase 2 "code decides" rule and keeps routing behavior durable rather than conversational.

### Failure and duplicate behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Roll back implementation `done` on failure | Reopen or undo completion if review handoff cannot be created | |
| Keep `done`, record linked failure | Preserve the implementation completion and record an inspectable handoff failure | ✓ |
| Use review fanout duplicate key for uniqueness | Rely on generic fanout duplicate policy to prevent repeated handoffs | |
| Use source-task linkage uniqueness | Treat the implementation task as the unique handoff source | ✓ |

**User's choice:** Keep `done`, record linked failure, and enforce uniqueness from the source-task linkage.
**Notes:** Review fanout remains a separate routing policy; automatic handoff duplication is prevented by a dedicated source-task linkage check.

---

## Link model

### Canonical linkage storage

| Option | Description | Selected |
|--------|-------------|----------|
| ChildTask field(s) | Add source/review linkage directly onto child task records | |
| Dedicated `ReviewHandoff` artifact | Store canonical linkage in `reviews/<source-task-id>.yaml` | ✓ |
| Message meta only | Treat mailbox envelope metadata as the authoritative link record | |

**User's choice:** Dedicated `ReviewHandoff` artifact.
**Notes:** This follows the Phase 1 pattern of dedicated durable artifacts instead of overloading message metadata or duplicating linkage across task records.

### Reverse-pointer policy

| Option | Description | Selected |
|--------|-------------|----------|
| Source task only | Source implementation task stores the review task pointer | |
| Review task only | Review task stores the source implementation pointer | |
| Both | Duplicate reverse pointers on both tasks | |
| Neither | Keep the handoff artifact as the sole canonical link | ✓ |

**User's choice:** Neither.
**Notes:** Avoiding duplicated reverse pointers reduces drift risk and keeps review-chain authority in one file.

### Response/outcome storage

| Option | Description | Selected |
|--------|-------------|----------|
| Review task YAML | Persist response linkage and outcome on the review child task | |
| `ReviewHandoff` artifact | Persist response linkage and outcome with the canonical handoff record | ✓ |
| Message meta only | Infer final state only from the response message | |

**User's choice:** `ReviewHandoff` artifact.
**Notes:** `reply_to` and `thread` remain validation evidence, but the canonical response/outcome record lives on the handoff artifact.

### Uniqueness enforcement

| Option | Description | Selected |
|--------|-------------|----------|
| Review duplicate key | Infer uniqueness from generic routed-task duplicate checks | |
| Review task scan | Search tasks to find whether a review already exists | |
| `reviews/<source-task-id>.yaml` existence | Use the source-task handoff file as the uniqueness gate | ✓ |

**User's choice:** `reviews/<source-task-id>.yaml` existence.
**Notes:** The source implementation task is the unique anchor for automatic review handoff.

---

## Review outcome handling

### Effect on source task state

| Option | Description | Selected |
|--------|-------------|----------|
| New `reviewed` state on approval | Advance the implementation task into a new review-complete lifecycle state | |
| Reopen on `changes_requested` | Move the implementation task out of `done` when review requests changes | |
| Keep source task `done` | Leave the implementation task state unchanged and record review outcome separately | ✓ |

**User's choice:** Keep source task `done`.
**Notes:** Review semantics are carried by the handoff artifact rather than by changing the generic task lifecycle in Phase 3.

### Phase 3 scope

| Option | Description | Selected |
|--------|-------------|----------|
| Record and display outcome only | Durable review-outcome capture plus operator visibility | ✓ |
| Auto-create follow-up implementation tasks | Turn `changes_requested` into a new implementation branch automatically | |

**User's choice:** Record and display outcome only.
**Notes:** Automatic follow-up work creation is deferred outside Phase 3.

### Outcome submission interface

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated review-response CLI | Reviewer submits outcome through a review-specific command surface | ✓ |
| Extend `task done` with `--outcome` | Overload generic task completion with review semantics | |
| Freeform response body only | Infer outcome from prose in `review_response` content | |

**User's choice:** Dedicated review-response CLI.
**Notes:** Outcome is review semantics, not generic task lifecycle data.

### Handoff updates on response

| Option | Description | Selected |
|--------|-------------|----------|
| Update review task only | Keep outcome on the review child task | |
| Update `ReviewHandoff` artifact | Record response message ID, outcome, response timestamp, and responded status on the handoff | ✓ |
| Message body only | Treat the response message as sufficient | |

**User's choice:** Update `ReviewHandoff` artifact.
**Notes:** The response message remains the human-readable findings channel, while the handoff artifact is the durable workflow summary.

---

## Operator inspection surface

### Display location

| Option | Description | Selected |
|--------|-------------|----------|
| Indented under source task | Render review handoff directly beneath the implementation task that triggered it | ✓ |
| Separate "Review Chain" section | Show review linkage away from the task list | |
| Review task only | Rely on the review task's normal task-row presence without extra linkage view | |

**User's choice:** Indented under source task.
**Notes:** This preserves a task-centric operator view while making the implementation-to-review chain explicit.

### Review task visibility

| Option | Description | Selected |
|--------|-------------|----------|
| Hide review task from normal task list | Only show it through the source task's handoff block | |
| Keep review task as a normal child task | Review task remains part of the regular run graph | ✓ |

**User's choice:** Keep review task as a normal child task.
**Notes:** The derived handoff block supplements the normal task list; it does not replace it.

### Minimum displayed fields

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal | Status only | |
| Balanced | Status, review task ID, reviewer, response message ID, outcome, failure summary | ✓ |
| Rich | Include expanded routing details, timestamps, and full response summary inline | |

**User's choice:** Balanced.
**Notes:** This is enough to inspect the chain without transcript spelunking while avoiding summary-surface creep.

### Separate inspection command

| Option | Description | Selected |
|--------|-------------|----------|
| Add review-only command/filter | Introduce `run show --reviews-only` or equivalent | |
| Use existing `run show` only | Keep Phase 3 visibility inside the current operator surface | ✓ |

**User's choice:** Use existing `run show` only.
**Notes:** Review-only filtering is deferred until a later richer-inspection phase if it proves necessary.

---

## the agent's Discretion

- Exact naming for `ReviewHandoff` helper types, validation helpers, and storage helpers.
- Exact review-response CLI flag names, as long as the interface remains review-specific.
- Exact ordering/formatting of review-handoff blocks in `run show`, as long as the minimum required fields remain visible.

## Deferred Ideas

- Automatic follow-up implementation-task generation for `changes_requested`.
- Review-only filters or commands.
- Any richer implementation-task lifecycle state such as `reviewed`.
- Outcome-driven workflow branching, escalation, or retry policy beyond durable recording and display.
