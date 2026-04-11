# tmuxicate

## What This Is

`tmuxicate` is a Go CLI for running multiple AI coding agents side by side in `tmux` with a durable, file-backed coordination layer. It gives each agent a pane, mailbox, and task workflow so a human operator can watch work happen, intervene when needed, and keep coordination reliable rather than implicit. As of `v1.0`, that foundation now includes coordinator-driven run decomposition, deterministic routing, linked review handoff, blocker escalation, and operator-facing run summaries built on the same durable mailbox model.

## Core Value

A human can coordinate multiple terminal agents through a reliable, observable workflow where the coordinator keeps work moving without hiding what happened.

## Requirements

### Validated

- ✓ Human operator can define a multi-agent session in `tmuxicate.yaml` and start it with `tmuxicate up` — existing
- ✓ Agents can exchange durable mailbox messages with explicit read and reply flows backed by immutable message records and per-recipient receipts — existing
- ✓ Agents can track task progress with accept, wait, block, and done state transitions — existing
- ✓ Runtime daemon can watch unread inboxes and inject short pane notifications when an agent looks ready — existing
- ✓ Operator can inspect the collaboration state through `status`, `log`, inbox commands, and transcript/event files — existing
- ✓ The system works across multiple agent CLIs through a generic adapter boundary plus Codex and Claude adapters — existing
- ✓ Coordinator can turn a high-level human goal into a bounded set of child tasks with clear ownership and expected outputs — validated in Phase 1: Coordinator Foundations
- ✓ Coordinator can route implementation, research, and review tasks to the right agents using declared roles and team relationships — validated in Phase 2: Role-Based Routing
- ✓ Coordinator can manage a review handoff so completed implementation work reaches a reviewer and the resulting feedback stays linked to the coordinator run — validated in Phase 3: Review Handoff Flow
- ✓ Coordinator can react to `wait` and `block` states by requesting clarification, re-routing work, or escalating to the human operator through durable blocker cases and explicit operator resolution — validated in Phase 4: Blocker Escalation
- ✓ Coordinator can produce an end-of-run summary that explains what was completed, what is waiting, what is blocked, what is under review, and what still needs human attention — validated in Phase 5: Run Summaries

### Active

- [ ] Coordinator learns routing preferences from prior runs without hiding why an owner was selected
- [ ] Coordinator can partially re-plan a run after a blocker while preserving operator visibility and explicit escalation
- [ ] Coordinator can manage nested teams or multiple coordinators without collapsing the current durable workflow model
- [ ] Coordinator can target remote or sandboxed worker environments in addition to local `tmux` panes
- [ ] Operator can inspect richer coordinator dashboards with per-run timelines and filtering

### Out of Scope

- Fully autonomous long-horizon planning without human steering — this would violate the product's "reliability over magic" position
- Unbounded agent-to-agent chatter or opaque side channels outside the mailbox model — coordination must stay observable and durable
- Vendor-specific orchestration tied to a single model provider — `tmuxicate` needs to remain multi-agent and multi-vendor
- Coordinator directly replacing specialist agents as the primary implementer — the coordinator should orchestrate work, not collapse the role model

## Context

The existing codebase is a brownfield Go CLI with a layered structure: `cmd/tmuxicate/main.go` wires commands, `internal/session/` handles user-facing workflows, `internal/mailbox/` persists immutable messages and receipts, `internal/runtime/daemon.go` performs notification delivery, and `internal/adapter/` plus `internal/tmux/` isolate integration boundaries. The shipped `v1.0` product now proves the core mailbox and pane workflow, durable coordinator runs, deterministic role-based routing with duplicate-safe task assignment, a full implementation-to-review chain with linked reviewer responses rendered from durable artifacts, blocker escalation with durable blocker cases plus explicit operator resolution, and operator-facing run summaries rebuilt from the same durable run graph. The milestone shipped without introducing a replacement orchestration system.

## Current State

- Shipped `v1.0 Coordinator Automation` on 2026-04-11.
- The current milestone archive covers 5 phases, 12 plans, and 28 execution tasks.
- The repo now carries roughly 29k lines across Go, shell, YAML, and Markdown, with coordinator automation extending the existing mailbox runtime instead of replacing it.
- The main remaining product pressure is not feature correctness in `v1.0`, but how to safely expand coordination depth without reducing operator visibility.

## Next Milestone Goals

- Add smarter coordination that can learn from prior runs and re-plan limited portions of a workflow after blockers.
- Expand execution targets beyond local `tmux` panes while keeping the current durable mailbox and adapter contracts intact.
- Improve operator visibility with richer run timelines and filtering so more automation does not make the system harder to inspect.

## Constraints

- **Tech stack**: Stay within the existing Go CLI architecture and current tmux/mailbox runtime — the new work should extend current packages rather than introduce a second orchestration system
- **Product philosophy**: Reliability and operator visibility come before autonomy — automated behavior must remain inspectable and explicit
- **Compatibility**: Preserve the existing mailbox protocol and multi-vendor adapter model — current Codex/Claude/generic flows must not be broken by coordinator features
- **Operational model**: Human operator remains the final escalation point — coordinator automation should surface blocked or risky situations instead of hiding them
- **Quality**: New orchestration flows need direct test coverage in the currently under-tested session/runtime areas — otherwise automation will amplify regressions

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Keep the filesystem-backed mailbox as the authoritative coordination layer | Existing product value and recovery model depend on durable, inspectable state | ✓ Good |
| Build coordinator automation as an orchestration layer on top of current task/reply primitives | This extends the current system without turning it into a different product | — Pending |
| Focus the next milestone on coordinator-managed decomposition, routing, review, blocker handling, and summary generation | This is the smallest automation slice that materially improves collaboration without overreaching into full autonomy | — Pending |
| Exclude fully autonomous long-horizon behavior from v1 automation | Human-steerable reliability is a clearer fit than "autonomous swarm" behavior | ✓ Good |
| Route coordinator work through structured `RoleSpec` metadata plus deterministic `route-task` selection | Role/domain routing must stay inspectable, teammate-constrained, and vendor-independent | ✓ Good |
| Keep review linkage in a dedicated `ReviewHandoff` artifact keyed by source task ID | Review state must stay durable, explicit, and idempotent without mutating child-task contracts | ✓ Good |
| Record reviewer outcomes through `tmuxicate review respond` and surface them in `run show` | Review decisions should stay visible through existing operator workflows instead of transcript-only context | ✓ Good |
| Keep blocker handling on a dedicated `BlockerCase` artifact with code-driven action selection and explicit `blocker resolve` responses | Blocked work must remain durable, inspectable, ceiling-bounded, and human-escalatable without heuristic coordinator behavior | ✓ Good |
| Keep run summaries as a derived `RunGraph` projection rendered through `run show` and root completion output | Operator summaries must stay inspectable, additive to existing detail, and free of new summary persistence/state machines | ✓ Good |
| Ship coordinator automation as `v1.0` before attempting adaptive routing or remote worker expansion | The current foundation is strong enough to validate operator-facing workflow value before adding smarter or broader execution behavior | ✓ Good |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-11 after v1.0 milestone completion*
