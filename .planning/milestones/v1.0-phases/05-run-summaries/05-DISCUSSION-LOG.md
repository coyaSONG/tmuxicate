# Phase 5: Run Summaries - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-06
**Phase:** 05-run-summaries
**Areas discussed:** Summary entrypoint, Summary status rules, Per-item detail level, When operators see it

---

## A. Summary entrypoint

| Option | Description | Selected |
|--------|-------------|----------|
| `run show --summary` | Keep the existing command but switch to a summary-focused output mode. | |
| `run summary` | Add a dedicated summary command separate from `run show`. | |
| `run show` top summary section | Keep `run show` as the canonical surface and add a summary section above the existing detailed task view. | ✓ |

**User's choice:** Add a summary section at the top of `run show`.
**Notes:** Summary stays on the existing inspection surface because `run show` already rebuilds `RunGraph` and renders task-local detail.

---

## B. Relationship to `run show`

| Option | Description | Selected |
|--------|-------------|----------|
| Replace detail view | Summary becomes the new primary output and detailed task rendering is removed or sidelined. | |
| Complement detail view using the same `RunGraph` | Summary is an aggregate layer above the current detailed output and reuses the same rebuild path. | ✓ |
| Parallel implementation path | Summary uses a new aggregation path while `run show` keeps the current rebuild logic. | |

**User's choice:** Summary complements the existing detailed output and reuses the same `RunGraph`.
**Notes:** Phase 5 must preserve consistency with Phases 1-4 and avoid a second aggregation model.

---

## C. Logical work item final status

| Option | Description | Selected |
|--------|-------------|----------|
| Precedence model | Use explicit precedence `escalated > blocked > waiting > under_review > completed`; show `changes_requested` as `under_review` with outcome surfaced. | ✓ |
| Outcome-specific bucket | Add a new top-level `needs_work` summary bucket for `changes_requested`. | |
| Raw-state passthrough | Report each artifact's current raw state without deriving a final logical status. | |

**User's choice:** Use explicit precedence with `changes_requested` reported as `under_review` plus explicit outcome.
**Notes:** The mapping is: `done + review approved -> completed`, `done + review pending -> under_review`, `done + review changes_requested -> under_review`, `blocked + escalated -> escalated`, and waiting stays separate.

---

## D. Logical work item duplication

| Option | Description | Selected |
|--------|-------------|----------|
| Separate rows per artifact | Source task, review task, and blocker/reroute activity each get separate summary rows. | |
| One row per source task | A logical work item appears once, anchored to the source task, with review/blocker metadata folded into that row. | ✓ |
| Hybrid | Some artifact types collapse while others remain separate rows. | |

**User's choice:** One summary row per logical work item, anchored to the source task.
**Notes:** Review tasks and blocker cases become metadata on the source-task row rather than standalone summary items.

---

## E. Summary row density

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal | Status, owner, and goal only on a single line. | |
| Medium | Status, owner, and goal plus outcome or recommended action when relevant. | ✓ |
| Full | Repeat nearly all `run show` detail on every summary item. | |

**User's choice:** Medium density.
**Notes:** The summary must remain meaningfully shorter than the existing detailed task-local view while still surfacing the reason an operator cares.

---

## F. When operators see the summary

| Option | Description | Selected |
|--------|-------------|----------|
| On-demand only | Summary appears only when the operator explicitly asks for it. | |
| Automatic only | Summary is printed only when the run completes. | |
| Both | Summary is available on demand and is also printed once at run completion. | ✓ |

**User's choice:** Both on-demand and one-time automatic completion output.
**Notes:** The automatic output is a convenience view, not a second source of truth.

---

## Scope Boundary

- Phase 5 stops at a derived operator view over existing durable artifacts.
- No new summary artifact, no new summary-specific state machine, and no new workflow automation are added in this phase.
- Separate summary commands, JSON output, filters, historical reports, and follow-up orchestration remain future work.

## the agent's Discretion

- Exact ASCII layout and section labels for the summary block.
- Exact field labels for task, message, review, and blocker references.
- Exact hook point for printing the summary once at run completion.

## Deferred Ideas

- Dedicated `run summary` command.
- Persisted summary snapshots or machine-readable summary output.
- Automation that reacts to summary outcomes such as `changes_requested`.
