# Pitfalls Research

**Domain:** terminal-based multi-agent coding orchestration
**Researched:** 2026-04-05
**Confidence:** HIGH

## Critical Pitfalls

### Pitfall 1: Freeform Swarm Drift

**What goes wrong:**
The coordinator turns into a chat relay and agents start broadcasting partial thoughts rather than executing bounded work.

**Why it happens:**
Multi-agent systems feel more powerful when everyone can talk to everyone, and frameworks often showcase open-ended collaboration patterns.

**How to avoid:**
Keep the coordinator workflow-first: decompose goals into child tasks, assign explicit owners, and use replies/review requests as bounded handoffs.

**Warning signs:**
- Multiple agents discussing the same task without clear ownership
- Long reply chains with no state transition
- Operator cannot tell who is currently responsible for progress

**Phase to address:**
Phase 1, when defining coordinator task graph and routing rules.

---

### Pitfall 2: Hidden State Divergence

**What goes wrong:**
The "real" orchestration state lives in coordinator memory or prompt history while mailbox receipts show something different.

**Why it happens:**
It is tempting to treat LLM conversation context as the run state because it is easy to prototype.

**How to avoid:**
Persist run metadata, child task linkage, and final summaries to durable artifacts. Use in-memory state only as a cache over canonical files.

**Warning signs:**
- Restarting the process loses task graph understanding
- `status` disagrees with what the coordinator "thinks" happened
- Summaries cannot point to concrete message IDs or state transitions

**Phase to address:**
Phase 1, before higher-level automation is added.

---

### Pitfall 3: Over-Autonomy Without Safe Stops

**What goes wrong:**
The coordinator keeps retrying, rerouting, or making product decisions after workers block instead of escalating.

**Why it happens:**
Agent demos reward continued action, but production collaboration tools need safe boundaries.

**How to avoid:**
Define stop conditions for ambiguity, repeated failure, missing reviewer approval, and retry ceilings. Escalate to the human with context and a recommended next action.

**Warning signs:**
- Same task is reassigned repeatedly with no new information
- Review is bypassed because the coordinator "felt confident"
- Blocked tasks remain active with no operator notification

**Phase to address:**
Phase 3, when implementing blocker handling and escalation policy.

---

### Pitfall 4: Role Mismatch and Misrouting

**What goes wrong:**
The coordinator sends work to the wrong agent because routing is based on loose prompt inference instead of explicit role data.

**Why it happens:**
Model-selected speaker patterns are flexible, but they depend heavily on agent descriptions and shared context quality.

**How to avoid:**
Base initial routing on explicit config roles and teammate graphs, then add smarter heuristics only after observable baseline behavior exists.

**Warning signs:**
- Reviewer receives raw implementation work
- Researcher receives tasks requiring filesystem mutation
- Similar agents duplicate the same subtask

**Phase to address:**
Phase 1 and Phase 2.

---

### Pitfall 5: Demo-Only Completion

**What goes wrong:**
Coordinator automation appears to work in a happy-path demo, but there is no durable review trail, no blocker story, and no trustworthy end summary.

**Why it happens:**
Teams optimize for the visible "task got delegated" moment and skip the less glamorous closure paths.

**How to avoid:**
Treat review handoff, blocked-task handling, and final summary generation as first-class deliverables in the MVP.

**Warning signs:**
- No test coverage for blocked/review scenarios
- Summary ignores waiting or blocked tasks
- Operator has to inspect raw transcripts to know if the run succeeded

**Phase to address:**
Phase 2 through Phase 4.

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Store coordinator run state only in memory | Faster prototype | Breaks replay, recovery, and operator trust | Never |
| Encode routing rules only in prompts | Quick iteration | Behavior drifts and becomes hard to test | Only for very early experiments, not committed code |
| Skip session/runtime tests for new automation | Faster shipping | Regressions in highest-coupling flows | Never for launch scope |
| Reuse generic chat replies as task schema | Less code upfront | Ambiguous ownership and missing acceptance criteria | Only for temporary manual workflows |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| `tmux` panes | Assuming pane text is canonical proof of progress | Use pane output for operator visibility only; persist actual state separately |
| Agent adapters | Parsing brittle prompt strings as workflow truth | Keep readiness probing in adapters, but route/summary logic on explicit task events |
| Mailbox store | Adding orchestration metadata without preserving immutability boundaries | Extend with linked artifacts or event files rather than mutating canonical message content |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Full filesystem scan for every summary | Slow `status` and long final summaries | Maintain run-level indexes or cached aggregates | Noticeable once runs fan out across many child tasks |
| Repeated rerouting loops | Many notifications, no forward progress | Add retry ceilings and "same evidence" detection | Breaks trust quickly even at small team sizes |
| Broadcast-style notifications | Pane spam and duplicated work | Narrow recipients per task and prevent unnecessary fan-out | Breaks as soon as more than 3-4 agents are active |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Escalation summaries exposing secrets from task context | Sensitive repo or env data leaks into logs and transcripts | Redact secrets in summaries and prefer references to file paths/message IDs |
| Coordinator generating shell-ready commands from untrusted message text | Command injection or unintended execution | Keep coordinator outputs as task descriptions, not executable shell strings unless explicitly bounded |
| Treating local machine execution as harmless | Host compromise or accidental destructive changes | Preserve current explicit operator control and avoid silent self-directed execution leaps |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Coordinator acts without explaining why a task moved | Operator loses confidence and interrupts manually | Record routing rationale in task metadata or summary |
| Too many intermediate coordinator messages | Team feels noisy and hard to follow | Prefer stateful task updates over conversational chatter |
| Final summary only lists successes | Real work remains hidden | Always include waiting, blocked, and escalated items explicitly |

## "Looks Done But Isn't" Checklist

- [ ] **Task decomposition:** Often missing ownership and expected output — verify every child task names an owner and completion contract
- [ ] **Routing:** Often missing deterministic fallback — verify coordinator behavior when multiple agents could qualify
- [ ] **Review handoff:** Often missing review completion linkage — verify reviewer outcomes feed the same parent run
- [ ] **Blocker handling:** Often missing escalation thresholds — verify repeated waits/blocks cannot loop indefinitely
- [ ] **Run summary:** Often missing incomplete work — verify blocked and pending tasks appear alongside completed ones

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Freeform swarm drift | MEDIUM | Freeze new fan-out, identify active owner per task, reissue bounded assignments, summarize current state |
| Hidden state divergence | HIGH | Reconstruct run graph from durable artifacts, discard in-memory assumptions, backfill missing metadata |
| Over-autonomy without safe stops | MEDIUM | Add escalation event, halt reroutes, require human acknowledgment before further delegation |
| Misrouting | LOW | Reassign using explicit role data, annotate why the first route failed, improve routing tests |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Freeform swarm drift | Phase 1 | Child tasks are durable and single-owner by default |
| Hidden state divergence | Phase 1 | Restart/reload preserves coordinator run understanding |
| Over-autonomy without safe stops | Phase 3 | Repeated blocks escalate instead of looping |
| Misrouting | Phase 2 | Routing tests cover role-specific assignments and review handoff |
| Demo-only completion | Phase 4 | Final summary includes complete, waiting, blocked, and escalated items |

## Sources

- Anthropic, "Building Effective Agents": https://www.anthropic.com/engineering/building-effective-agents
- Microsoft AutoGen Teams tutorial: https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/tutorial/teams.html
- Microsoft AutoGen Selector Group Chat tutorial: https://microsoft.github.io/autogen/dev/user-guide/agentchat-user-guide/selector-group-chat.html
- LangGraph supervisor docs: https://langchain-ai.github.io/langgraphjs/reference/modules/langgraph-supervisor.html
- OpenAI Codex product page: https://openai.com/codex/
- OpenHands runtime architecture docs: https://docs.openhands.dev/openhands/usage/architecture/runtime

---
*Pitfalls research for: terminal-based multi-agent coding orchestration*
*Researched: 2026-04-05*
