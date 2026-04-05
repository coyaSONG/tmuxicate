# Project Research Summary

**Project:** tmuxicate
**Domain:** terminal-based multi-agent coding orchestration
**Researched:** 2026-04-05
**Confidence:** HIGH

## Executive Summary

Current guidance from Anthropic, AutoGen, LangGraph, OpenAI, and OpenHands all points in the same direction: the next valuable step for `tmuxicate` is not a more magical swarm, but a more explicit coordinator workflow. The repo already has the right substrate for that: durable mailbox state, clear task verbs, visible tmux panes, and an operator-centered workflow. The best move is to add coordinator automation as a bounded orchestration layer on top of those primitives.

For this project, the recommended approach is to keep the filesystem-backed mailbox authoritative, implement coordinator logic as direct Go workflows, and prioritize task decomposition, role-based routing, review handoff, blocker escalation, and final summary generation. The main risk is drifting into opaque chat-based autonomy that bypasses explicit state and human checkpoints. The roadmap should therefore front-load durable task graph foundations and test coverage before adding richer automation behaviors.

## Key Findings

### Recommended Stack

Coordinator automation should stay inside the current Go CLI architecture rather than introducing a heavy supervisor framework. Research strongly favors simple, composable workflows with carefully designed tools and clear state transitions over layered abstractions that are harder to debug.

**Core technologies:**
- **Go 1.26.1**: implement workflow logic inside the existing CLI/runtime codebase
- **Filesystem-backed mailbox**: keep runs replayable, inspectable, and recoverable
- **`tmux` + current adapters**: preserve the operator-visible execution surface and multi-vendor agent model
- **Explicit task events and summaries**: treat them as first-class coordinator outputs, not incidental logs

### Expected Features

The must-have feature set is consistent across current multi-agent patterns: a planner/coordinator, specialized workers, explicit handoffs, termination conditions, and observability. For `tmuxicate`, those patterns map naturally onto the existing mailbox/task model.

**Must have (table stakes):**
- Coordinator decomposes a human goal into bounded child tasks
- Coordinator routes tasks by declared role/capability
- Coordinator manages implementation-to-review handoff
- Coordinator escalates blocked or ambiguous work to the human
- Coordinator emits a final run summary with clear status breakdown

**Should have (competitive):**
- Durable run history with message/task linkage
- tmux-native live observability for coordinator runs
- Structured blocker handling rather than simple retry spam

**Defer (v2+):**
- Nested supervisors or multi-level teams
- Remote sandbox/runtime expansion
- Persistent cross-run agent memory beyond project artifacts

### Architecture Approach

The recommended architecture is a workflow-first coordinator layer over the current mailbox store. Add a focused `internal/coordinator/` package that plans child tasks, routes them, handles review and blocker transitions, and produces summaries, while leaving `internal/mailbox/`, `internal/protocol/`, `internal/runtime/`, and `internal/tmux/` as the durable coordination and execution boundaries.

**Major components:**
1. **Goal intake and planner** — converts a human objective into a durable run plus child tasks
2. **Router and review flow** — assigns work to the right agent and pushes completed implementation into review
3. **Blocker/escalation logic** — decides when to re-route, ask clarifying questions, or stop and ask the human
4. **Run summarizer** — aggregates explicit work state into a trustworthy operator-facing outcome

### Critical Pitfalls

1. **Freeform swarm drift** — avoid by keeping tasks bounded and single-owner
2. **Hidden state divergence** — avoid by persisting run metadata and summaries durably
3. **Over-autonomy without safe stops** — avoid by defining escalation thresholds and retry ceilings
4. **Misrouting by vague prompt inference** — avoid by using explicit role metadata first
5. **Demo-only completion** — avoid by making review, blocker handling, and final summaries part of MVP

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Coordinator Foundations
**Rationale:** The task graph and durable workflow semantics must exist before higher-level automation is trustworthy.
**Delivers:** Coordinator run model, child-task generation, parent/child linkage, deterministic routing inputs
**Addresses:** goal decomposition, role-based routing
**Avoids:** hidden state divergence and swarm drift

### Phase 2: Execution and Review Handoffs
**Rationale:** Once tasks are routable, the first real value comes from moving implementation into structured review.
**Delivers:** implementer assignment, review-request generation, linked review responses, routing tests
**Uses:** existing task lifecycle and mailbox receipts
**Implements:** router and review components

### Phase 3: Blockers and Human Escalation
**Rationale:** Automation without safe failure behavior will not be trusted in practice.
**Delivers:** wait/block classification, escalation triggers, reroute vs human-help decisions
**Addresses:** blocker handling and safe stop conditions

### Phase 4: Run Summary and Operator Confidence
**Rationale:** The workflow is not complete until operators can see the final state without spelunking transcripts.
**Delivers:** end-of-run summary, improved status integration, verification for completed/waiting/blocked coverage

### Phase Ordering Rationale

- Goal decomposition must come before review or summary because there is nothing reliable to aggregate otherwise.
- Review handoff belongs before blocker sophistication because it validates the core multi-agent loop in a concrete way.
- Blocker handling should be explicit before richer automation because retries without policy quickly damage trust.
- Summary and operator views close the loop and make the workflow usable beyond demos.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1:** routing/task-schema details, because the current mailbox model may need careful protocol extension without losing simplicity
- **Phase 3:** escalation policy and retry ceilings, because they interact with current daemon/session reliability gaps

Phases with standard patterns (skip research-phase):
- **Phase 2:** review handoff flow, because the pattern is well established and already fits the current mailbox/task model
- **Phase 4:** run summaries and operator-facing aggregation, because the main challenge is local integration, not uncertain domain patterns

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Strong alignment between external guidance and the repo’s existing Go/tmux/mailbox foundation |
| Features | HIGH | Current multi-agent systems consistently emphasize planner/worker/reviewer structure, handoffs, and observability |
| Architecture | HIGH | Supervisor/workflow patterns map cleanly onto the current layered codebase |
| Pitfalls | HIGH | Risks are both externally documented and already hinted at by this repo’s current concerns map |

**Overall confidence:** HIGH

### Gaps to Address

- **Task graph storage shape:** decide whether child-task linkage can live in current protocol artifacts or needs a small new run artifact model
- **Coordinator command surface:** decide whether automation starts as new CLI commands, coordinator-side conventions, or both
- **Verification depth:** strengthen tests in `internal/session/` and `internal/runtime/` so orchestration does not rest on happy-path demos

## Sources

### Primary (HIGH confidence)
- Anthropic, "Building Effective Agents" — workflow simplicity, tool/interface design, stop conditions: https://www.anthropic.com/engineering/building-effective-agents
- Microsoft AutoGen Teams tutorial — team patterns, observability, and collaboration guidance: https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/tutorial/teams.html
- Microsoft AutoGen Selector Group Chat tutorial — planning agent plus specialized worker pattern: https://microsoft.github.io/autogen/dev/user-guide/agentchat-user-guide/selector-group-chat.html
- LangGraph supervisor docs — hierarchical supervisor, controlled handoffs, message-history policies: https://langchain-ai.github.io/langgraphjs/reference/modules/langgraph-supervisor.html
- OpenAI Codex product page — parallel coding workflows, review-oriented background work: https://openai.com/codex/
- OpenHands runtime architecture docs — isolation/runtime concerns and why they are a separate layer: https://docs.openhands.dev/openhands/usage/architecture/runtime

### Secondary (MEDIUM confidence)
- Existing repo docs and codebase map in `.planning/codebase/` — concrete baseline for what `tmuxicate` already validates today

---
*Research completed: 2026-04-05*
*Ready for roadmap: yes*
