# Feature Research

**Domain:** terminal-based multi-agent coding orchestration
**Researched:** 2026-04-05
**Confidence:** HIGH

## Feature Landscape

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Goal decomposition into bounded tasks | A coordinator is not credible if the human still has to manually split every objective | MEDIUM | Coordinator should emit child tasks with ownership, acceptance criteria, and reply expectations |
| Role-based routing | Multi-agent systems only help if the right specialist gets the right work | MEDIUM | Use existing agent roles, aliases, and teammate relationships instead of open-ended speaker selection |
| Explicit task lifecycle | Operators expect to see what is queued, active, waiting, blocked, and done | LOW | Leverage current `task accept|wait|block|done` semantics instead of inventing a second state model |
| Review handoff | Coding teams expect implementation to move into review before the run is considered complete | MEDIUM | Review requests should be durable first-class messages, not ad hoc chat |
| Human escalation | Operators need a clear moment where the system asks for help instead of guessing | LOW | Escalation should identify blocker, current owner, and recommended next action |
| Run summary | Users expect a concise end state after delegation finishes | LOW | Final summary should aggregate completed work, pending work, blockers, and review outcomes |

### Differentiators (Competitive Advantage)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Durable mailbox-backed orchestration | Recovery and auditability survive pane crashes and daemon restarts | LOW | This is already a repo advantage; automation should deepen it |
| tmux-native observability | Human can watch the team live instead of trusting a hidden service | MEDIUM | Preserve panes, transcripts, and status views as the main interface |
| Vendor-neutral agent coordination | Teams can mix Codex, Claude, and generic CLIs | MEDIUM | Avoid prompts or routing logic that assume one vendor’s tool surface |
| Structured blocker response | Coordinator reacts differently to ambiguity, dependency wait, and failed execution | MEDIUM | This separates a real orchestrator from a glorified fan-out bot |
| Replayable run history | Users can inspect why work was routed, blocked, or escalated | MEDIUM | Summaries should point back to concrete message IDs and task transitions |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Freeform all-agent group chat | Feels powerful and "agentic" | Blurs ownership, duplicates work, and hides decision boundaries | Coordinator-mediated point-to-point tasking with explicit recipients |
| Full autonomy without human checkpoints | Sounds efficient | Compounds errors and conflicts with the product’s reliability positioning | Controlled automation with clear escalation and stop conditions |
| Hidden planning memory outside mailbox state | Seems convenient for richer context | Makes runs irreproducible and fragile after restarts | Persist plans, assignments, and summaries to durable files/messages |
| Coordinator directly coding everything | Appears faster to implement | Collapses specialist roles and removes the reason to run multiple panes | Coordinator orchestrates; specialists execute and review |

## Feature Dependencies

```text
Goal decomposition
    └──requires──> explicit task schema
                           └──requires──> durable child-task persistence

Role-based routing
    └──requires──> reliable agent role metadata

Review handoff
    └──requires──> task completion signals
                           └──enhances──> final run summary

Blocker escalation
    └──requires──> durable wait/block states

Run summary
    └──requires──> coordinator-visible task graph and state aggregation
```

### Dependency Notes

- **Goal decomposition requires explicit task schema:** the coordinator needs a predictable message format to create child tasks that workers can act on consistently.
- **Role-based routing requires reliable agent metadata:** routing decisions are only meaningful if names, roles, and teammate graphs are explicit in config.
- **Review handoff requires task completion signals:** without a trustworthy "implementation done" event, review requests will fire too early or not at all.
- **Run summary depends on the task graph:** the coordinator cannot summarize accurately if work only exists as loose reply chains.

## MVP Definition

### Launch With (v1)

- [ ] Coordinator-generated child tasks with ownership, expected output, and parent linkage — essential because it turns high-level goals into executable work
- [ ] Automatic routing to implementer/reviewer/research roles using existing config metadata — essential because it operationalizes the multi-agent model
- [ ] Blocker and wait handling with explicit human escalation path — essential because autonomy without safe failure behavior will erode trust quickly
- [ ] End-of-run summary with completed, waiting, blocked, and review states — essential because users need closure and visibility

### Add After Validation (v1.x)

- [ ] Smarter routing heuristics or scoring — add once the baseline coordinator loop is stable and real usage shows misrouting patterns
- [ ] Partial auto-replanning when a task is blocked — add once blocker categories and safe reroute behaviors are validated
- [ ] Richer operator dashboard views for coordinator runs — add after the first automation flow proves useful

### Future Consideration (v2+)

- [ ] Multi-level coordinators or nested teams — defer until single-coordinator workflows are proven and observable
- [ ] Remote or sandbox-backed worker environments — defer until local tmux-native coordination is solid
- [ ] Cross-run learning or persistent agent memory beyond project files — defer until there is a clear need and safe persistence model

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Coordinator task decomposition | HIGH | MEDIUM | P1 |
| Role-based routing | HIGH | MEDIUM | P1 |
| Review handoff | HIGH | MEDIUM | P1 |
| Blocker escalation | HIGH | MEDIUM | P1 |
| Final run summary | HIGH | LOW | P1 |
| Smarter routing heuristics | MEDIUM | MEDIUM | P2 |
| Rich coordinator dashboard | MEDIUM | MEDIUM | P2 |
| Nested supervisors | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for launch
- P2: Should have, add when possible
- P3: Nice to have, future consideration

## Competitor Feature Analysis

| Feature | Competitor A | Competitor B | Our Approach |
|---------|--------------|--------------|--------------|
| Supervisor/coordinator | LangGraph offers hierarchical supervisors | AutoGen offers team presets and selector/swarm patterns | Keep a coordinator, but make mailbox state authoritative and visible in tmux |
| Collaboration flow | AutoGen team chat shares context broadly | OpenAI Codex emphasizes parallel work and background task execution | Use bounded receipts and tasks so parallel work stays inspectable |
| Runtime model | OpenHands emphasizes isolated execution runtimes | OpenAI/Anthropic coding agents often run in controlled workspaces | Stay local/tmux-first for now; treat execution isolation as a separate future layer |
| Human control | Frameworks usually expose termination conditions and HITL hooks | Codex/Anthropic emphasize review and checkpointing in agent workflows | Build escalation and summary directly into coordinator transitions |

## Sources

- Anthropic engineering guidance on effective agent patterns: https://www.anthropic.com/engineering/building-effective-agents
- OpenAI Codex product page for parallel coding and always-on automation expectations: https://openai.com/codex/
- Microsoft AutoGen Teams tutorial: https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/tutorial/teams.html
- Microsoft AutoGen Selector Group Chat tutorial: https://microsoft.github.io/autogen/dev/user-guide/agentchat-user-guide/selector-group-chat.html
- LangGraph supervisor docs: https://langchain-ai.github.io/langgraphjs/reference/modules/langgraph-supervisor.html
- OpenHands runtime architecture docs: https://docs.openhands.dev/openhands/usage/architecture/runtime

---
*Feature research for: terminal-based multi-agent coding orchestration*
*Researched: 2026-04-05*
