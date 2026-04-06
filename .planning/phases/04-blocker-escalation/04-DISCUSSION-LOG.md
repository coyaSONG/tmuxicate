# Phase 4: Blocker Escalation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-06
**Phase:** 04-Blocker Escalation
**Areas discussed:** Blocker action model, Retry / reroute ceilings, Escalation payload + surface, Operator visibility without stealing Phase 5

---

## Blocker action model

| Option | Description | Selected |
|--------|-------------|----------|
| Keep coarse states and interpret freeform reason text | Reuse existing `reason` field only and let the coordinator infer meaning from prose. | |
| Keep `wait` / `block` and require structured subtype fields | Preserve the existing state split, but require `wait_kind` / `block_kind` so code can apply deterministic policy. | ✓ |
| Introduce many new declared states | Replace the current state model with more granular task states. | |

**User's choice:** Keep `wait` and `block` distinct, require structured subtype fields, and treat any human-input case as `block`.
**Notes:** The action set is locked to `watch`, `clarification_request`, `reroute`, and `escalate`. Action selection is code-driven through a policy table with no freeform interpretation; ambiguous cases escalate.

---

## Retry / reroute ceilings

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse a single global retry value | Share one retry ceiling across all blocker handling and transport concerns. | |
| Dedicated blocker config with global default and per-task-class overrides | Add blocker-specific ceiling config, with a default plus optional per-task-class overrides. | ✓ |
| Per-agent reroute ceilings | Stop conditions vary by the current assignee rather than by task intent. | |

**User's choice:** Add `blockers.max_reroutes_default` plus `blockers.max_reroutes_by_task_class`, and track attempts on a dedicated `BlockerCase` artifact.
**Notes:** `watch` and `clarification_request` do not consume reroute budget. `delivery.max_retries` stays transport-only. Ceiling reached means immediate escalation with no bonus reroute attempt.

---

## Escalation payload + surface

| Option | Description | Selected |
|--------|-------------|----------|
| Mailbox-to-human canonical escalation | Create a human mailbox delivery path and treat the message as the escalation truth. | |
| Canonical BlockerCase + derived run surface | Persist escalation canonically on a blocker artifact and show it through existing operator inspection surfaces. | ✓ |
| Terminal-only escalation output | Print escalation details without a durable workflow artifact. | |

**User's choice:** The `BlockerCase` artifact is canonical, with `status=escalated`, and `run show` is the primary operator read surface.
**Notes:** No human mailbox recipient model is added. Every escalated blocker must carry `recommended_action`. Operator resolution goes through `tmuxicate blocker resolve <run-id> <source-task-id> --action manual_reroute|clarify|dismiss`.

---

## Operator visibility without stealing Phase 5

| Option | Description | Selected |
|--------|-------------|----------|
| Separate blockers section or blockers-only command | Add a dedicated blocker reading surface for this phase. | |
| Task-local blocker block under `run show` | Render blocker information directly under the source task, matching the review-handoff pattern. | ✓ |
| Run-wide blocker summary now | Add aggregate blocked/escalated counts and run summary sections in Phase 4. | |

**User's choice:** Show blocker information only as a task-local derived block under the source task in `run show`.
**Notes:** Phase 4 explicitly does not add aggregate counts, blocker-only read commands, or broader run summary UX. Those remain Phase 5 work.

---

## the agent's Discretion

- Exact Go type names and YAML field ordering for blocker-case and operator-resolution artifacts
- Exact flag names for `blocker resolve`, as long as `manual_reroute`, `clarify`, and `dismiss` remain explicit actions
- Exact field ordering and label wording inside the derived blocker block in `run show`

## Deferred Ideas

- Run-level blocked/escalated counters and aggregate summary sections
- Blocker-only read commands
- Full operator-facing run summaries that combine completed, waiting, blocked, review, and escalated work
