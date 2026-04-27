# tmuxicate

## What This Is

`tmuxicate` is a Go CLI for running multiple AI coding agents side by side in `tmux` with a durable, file-backed coordination layer. It gives each agent a pane, mailbox, and task workflow so a human operator can watch work happen, intervene when needed, and keep coordination reliable rather than implicit. As of `v1.2`, that workflow now includes durable coordinator runs, deterministic and adaptive routing, linked review and blocker recovery flows, explicit execution-target placement, concrete non-pane target dispatch, durable target health, and operator target control built on the same mailbox-backed state model.

## Core Value

A human can coordinate multiple terminal agents through a reliable, observable workflow where the coordinator keeps work moving without hiding what happened.

## Current State

- Shipped `v1.0 Coordinator Automation` on 2026-04-11.
- Shipped `v1.1 Adaptive Coordination` on 2026-04-11.
- Shipped `v1.2 Remote Execution Foundations` on 2026-04-11.
- Started `v1.3 Runtime Trust & Honest Controls` planning on 2026-04-27.
- The latest shipped product supports operator-configured non-pane target dispatch, durable target heartbeat and availability state, explicit enable/disable and recovery flows, and target-aware routing evidence on top of the existing adaptive run graph and timeline model.
- The active milestone hardens target persistence, dispatch recovery, delivery policy, daemon lifecycle, artifact safety, and docs/UX alignment before expanding remote transport, worktree isolation, or multi-coordinator topology.
- The core architecture remains the same: `cmd/tmuxicate/main.go` wires the CLI, `internal/session/` owns user-facing workflows, `internal/mailbox/` persists durable state, `internal/runtime/daemon.go` handles local pane notifications, and `internal/adapter/` plus `internal/tmux/` isolate integration boundaries.
- The major remaining product pressure is now “how far can remote execution and multi-team topology expand without weakening inspectability or operational trust?”

## Requirements

### Validated

- ✓ Human operator can define a multi-agent session in `tmuxicate.yaml` and start it with `tmuxicate up`
- ✓ Agents can exchange durable mailbox messages with explicit read and reply flows backed by immutable message records and per-recipient receipts
- ✓ Agents can track task progress with accept, wait, block, and done state transitions
- ✓ Runtime daemon can watch unread inboxes and inject short pane notifications when an agent looks ready
- ✓ Operator can inspect collaboration state through `status`, `log`, inbox commands, and transcript/event files
- ✓ The system works across multiple agent CLIs through a generic adapter boundary plus Codex and Claude adapters
- ✓ Coordinator can start durable runs, decompose work into child tasks, and reconstruct that graph from disk
- ✓ Coordinator can route implementation, research, and review tasks deterministically through declared roles and teammate relationships
- ✓ Coordinator can manage linked review handoff and reviewer responses inside the same run graph
- ✓ Coordinator can react to waits and blockers through durable blocker cases, bounded reroutes, and explicit operator resolution
- ✓ Operator can inspect shared run summaries derived from the durable run graph
- ✓ Coordinator can rebuild inspectable adaptive routing preferences from completed runs and show why an owner was selected
- ✓ Coordinator can create bounded partial replans that preserve blocker and replacement lineage
- ✓ Coordinator can persist explicit execution-target placement for local, remote, and sandboxed workers without breaking current local workflows
- ✓ Operator can inspect filtered per-run timelines derived from durable artifacts and `state.jsonl`
- ✓ Coordinator can dispatch non-local work through operator-configured target commands while preserving canonical run and routing artifacts
- ✓ Operator can inspect durable target readiness, heartbeat, and capability state before or during routing
- ✓ Non-local workers can reuse the canonical task lifecycle contract so run summaries and timelines stay rebuildable
- ✓ Operator can explicitly disable, recover, and reroute around unhealthy targets while keeping routing decisions inspectable

### Active

- [x] Target health and dispatch artifacts are persisted with mailbox-grade durability under concurrent updates
- [ ] Non-pane dispatch recovery is intent-first and idempotent for each target/message pair
- [ ] Runtime notification behavior honors manual delivery, auto-notify, readiness, retry, and timeout policy explicitly
- [ ] Session startup, serving, status, and shutdown own daemon lifecycle and surface stale or duplicate daemon states
- [ ] Sensitive local artifacts and operator-facing docs/UX align with the reliability-first product contract

### Out of Scope

- Fully autonomous long-horizon planning without human steering
- Unbounded agent-to-agent chatter or opaque side channels outside the mailbox model
- Replacing the mailbox protocol with a separate orchestration backend
- Fully managed remote infrastructure provisioning baked into the coordinator
- Vendor-specific orchestration tied to a single model provider
- Coordinator directly replacing specialist agents as the primary implementer

## Context

The codebase is now a brownfield Go CLI with three shipped coordinator milestones. `v1.0` proved durable coordinator decomposition, routing, review handoff, blocker escalation, and run summaries. `v1.1` extended that same run graph with adaptive routing evidence, bounded partial replans, execution-target placement metadata, and timeline projections without introducing a replacement backend. `v1.2` turned non-local target metadata into a concrete command-dispatch path with durable target health and operator control, while keeping remote workers on the same mailbox and task-state contract. The product remains intentionally conservative: durable artifacts and operator visibility outrank hidden autonomy.

## Next Milestone Goals

- Harden target state and dispatch persistence before expanding non-pane execution capabilities
- Make delivery configuration truthful by wiring manual mode, notification disablement, readiness checks, retry ceilings, and timeout visibility into runtime behavior
- Own daemon lifecycle across `up`, `serve`, `status`, and `down` so operators can recover from stale or duplicate runtime processes
- Tighten sensitive local artifact handling and update README/CLI UX to match shipped `run`, `target`, and picker behavior
- Keep authenticated remote transport, worktree isolation, cross-run attention, and multi-coordinator topology deferred until the trust foundation is stronger

## Constraints

- **Tech stack**: Stay within the existing Go CLI architecture and current tmux/mailbox runtime
- **Product philosophy**: Reliability and operator visibility come before autonomy
- **Compatibility**: Preserve the existing mailbox protocol and multi-vendor adapter model
- **Operational model**: Human operator remains the final escalation point
- **Quality**: New orchestration flows need direct test coverage in session/runtime areas because automation amplifies regressions

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Keep the filesystem-backed mailbox as the authoritative coordination layer | Existing product value and recovery model depend on durable, inspectable state | ✓ Good |
| Build coordinator automation as a workflow layer on top of the current task/reply primitives | This extends the current system without turning it into a different product | ✓ Good |
| Route coordinator work through structured `RoleSpec` metadata plus deterministic `route-task` selection | Role/domain routing must stay inspectable and teammate-constrained | ✓ Good |
| Keep review linkage in a dedicated `ReviewHandoff` artifact keyed by source task ID | Review state must stay durable and explicit without mutating child-task contracts | ✓ Good |
| Keep blocker handling on a dedicated `BlockerCase` artifact with explicit `blocker resolve` actions | Blocked work must remain durable, bounded, and human-escalatable | ✓ Good |
| Keep run summaries as a derived `RunGraph` projection rendered through `run show` and root completion output | Operator summaries must stay additive and artifact-driven | ✓ Good |
| Adaptive routing only changes selection when a unique exact-match preference beats the deterministic baseline | Learned behavior must remain deterministic and inspectable | ✓ Good |
| Partial replans are durable source-task keyed artifacts with one superseded task and one replacement task | Recovery must stay bounded and explicit rather than recursively autonomous | ✓ Good |
| Implicit local placement is synthesized as explicit execution-target metadata | Placement and filtering need durable target fields even for local tasks | ✓ Good |
| Only pane-backed local agents participate in tmux lifecycle and daemon notifications | Non-local targets should not pretend to be local panes | ✓ Good |
| Timeline rebuild validates TaskEvent ownership and thread linkage against canonical artifacts | Operator history must fail loudly on drift instead of guessing | ✓ Good |
| `run show` remains the single inspection surface; timeline rendering is additive and timeline-only reuses the same formatter path | Visibility should deepen without fragmenting workflows into parallel tools | ✓ Good |
| Non-local execution dispatch is a target-scoped command contract layered onto the existing mailbox/run graph | Remote execution should extend current artifacts rather than introduce a second coordinator backend | ✓ Good |
| Target health is persisted as durable target state plus heartbeat logs | Operators need inspectable health history without depending on tmux-only probes | ✓ Good |
| Remote lifecycle parity reuses the existing task lifecycle and `state.jsonl` contract | Summaries and timelines must rebuild from one canonical event model | ✓ Good |
| Target recovery redispatches only unread pending work when a target is re-enabled | Recovery must stay bounded and explicit instead of replaying arbitrary historical work | ✓ Good |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition:**
1. Move shipped requirements into Validated
2. Update current state and constraints if reality changed
3. Record durable architectural decisions that now shape future work

**After each milestone:**
1. Re-evaluate Core Value and Out of Scope items
2. Update Current State and Next Milestone Goals
3. Archive milestone-scoped roadmap and requirements context

---
*Last updated: 2026-04-27 after defining v1.3 Runtime Trust & Honest Controls*
