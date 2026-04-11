# Phase 1: Coordinator Foundations - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-05
**Phase:** 01-Coordinator Foundations
**Areas discussed:** Run initiation, child task schema, routing baseline, operator visibility

---

## Run initiation

| Option | Description | Selected |
|--------|-------------|----------|
| New CLI command | Start coordinator runs through a dedicated workflow command instead of generic messaging | ✓ |
| Extend `send` | Reuse the generic mailbox send path with extra coordinator semantics | |
| Coordinator message convention | Keep the same CLI surface but require a special message format to start runs | |

**User's choice:** New CLI command
**Notes:** Recommended because it keeps coordinator runs first-class and avoids blurring normal messages with structured orchestration runs.

---

## Child task schema

| Option | Description | Selected |
|--------|-------------|----------|
| `owner goal expected-output depends-on review-required parent-run-id` | Minimal durable workflow schema for task graphing and later review/blocker phases | ✓ |
| Add `deadline` now | Include time-based scheduling in the foundation schema | |
| Keep only owner and goal | Lighter schema but weaker reconstruction and review linkage | |

**User's choice:** `owner goal expected-output depends-on review-required parent-run-id`
**Notes:** Deadline handling was intentionally deferred so Phase 1 stays focused on durable structure.

---

## Routing baseline

| Option | Description | Selected |
|--------|-------------|----------|
| `role only` | Assign solely from declared role names | |
| `role + teammate` | Use explicit role metadata plus teammate graph to narrow assignments | ✓ |
| Coordinator inference | Let the coordinator infer the best target without relying on explicit config relationships | |

**User's choice:** `role + teammate`
**Notes:** This keeps the first routing layer deterministic and aligned with the repo’s explicit-coordination philosophy.

---

## Operator visibility

| Option | Description | Selected |
|--------|-------------|----------|
| Run tree only | Show parent and child structure without deeper traceability | |
| Run tree + state summary | Show task graph and compact status rollup | |
| Run tree + state summary + message links | Preserve observability back to durable underlying artifacts | ✓ |

**User's choice:** Run tree + state summary + message links
**Notes:** This was chosen because Phase 1 should already reflect the product’s operator-visible reliability promise.

---

## the agent's Discretion

- Exact CLI flag naming
- Internal artifact layout for coordinator runs
- Whether durable references surface primarily as task IDs, message IDs, or both

## Deferred Ideas

- Rich review workflow details belong to Phase 3
- Blocker escalation policy belongs to Phase 4
- Advanced summaries belong to Phase 5
