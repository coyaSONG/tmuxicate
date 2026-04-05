# Architecture Research

**Domain:** terminal-based multi-agent coding orchestration
**Researched:** 2026-04-05
**Confidence:** HIGH

## Standard Architecture

### System Overview

```text
┌─────────────────────────────────────────────────────────────┐
│                    Human / Operator Layer                   │
├─────────────────────────────────────────────────────────────┤
│  tmux panes  │  status/log views  │  explicit escalations   │
├─────────────────────────────────────────────────────────────┤
│                  Coordinator Workflow Layer                 │
├─────────────────────────────────────────────────────────────┤
│  Goal Intake  │  Task Planner  │  Router  │  Summarizer     │
│               │                │          │  Blocker Logic  │
├─────────────────────────────────────────────────────────────┤
│                 Durable Coordination Layer                  │
├─────────────────────────────────────────────────────────────┤
│  Envelopes  │  Receipts  │  Task Events  │  Run Artifacts   │
├─────────────────────────────────────────────────────────────┤
│                  Execution / Integration Layer              │
├─────────────────────────────────────────────────────────────┤
│  Agent adapters  │  tmux client  │  daemon notifier         │
└─────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| Goal intake | Convert a human objective into a coordinator run with explicit metadata | CLI command or coordinator session entrypoint in `internal/session/` |
| Task planner | Produce bounded child tasks linked to a parent objective | Deterministic planner using templates and current mailbox/project context |
| Router | Assign child tasks to the right specialist based on role/team metadata | Coordinator policy layer over resolved config agent roles and teammate graph |
| Blocker logic | Classify wait/block outcomes and choose reroute, clarification, or escalation | State-driven workflow transitions with explicit stop conditions |
| Summarizer | Produce a run-level outcome digest for the operator | Aggregation over durable task events, receipts, and reply threads |
| Durable store | Persist canonical work state and transitions | Existing `internal/mailbox` + protocol artifacts with small targeted extensions |

## Recommended Project Structure

```text
internal/
├── session/              # CLI-facing workflows and operator commands
│   ├── send.go
│   ├── task_cmd.go
│   ├── status.go
│   └── [coordinator entrypoints]
├── coordinator/          # New orchestration layer
│   ├── plan.go           # Goal -> child task decomposition
│   ├── route.go          # Role-based assignment policy
│   ├── review.go         # Review handoff logic
│   ├── block.go          # Wait/block classification and escalation
│   ├── summary.go        # Run-level summary generation
│   └── types.go          # Coordinator run/task graph types
├── mailbox/              # Canonical messages and receipts
├── protocol/             # Stable envelope/receipt schema
├── runtime/              # Notification daemon and observed state
├── adapter/              # Vendor-specific readiness + notify behavior
└── tmux/                 # Process/pane abstraction
```

### Structure Rationale

- **`internal/coordinator/`:** isolates orchestration policy from raw CLI plumbing and daemon mechanics, which the codebase map identified as currently too concentrated in large files.
- **`internal/session/`:** remains the human/agent command boundary so coordinator automation is still reachable through the same explicit CLI model.
- **`internal/mailbox/` + `internal/protocol/`:** remain authoritative because coordinator runs should be replayable and inspectable after failures.

## Architectural Patterns

### Pattern 1: Workflow-First Coordinator

**What:** Coordinator follows a bounded workflow graph instead of improvising open-ended group chat.
**When to use:** Default for implementation/review/research orchestration in a local coding team.
**Trade-offs:** Less "creative" autonomy, but far better debuggability and safety.

**Example:**
```text
human goal
  -> create coordinator run
  -> decompose into child tasks
  -> assign by role
  -> wait for state transition
  -> escalate or summarize
```

### Pattern 2: Handoff Messages, Not Shared Swarm Memory

**What:** Each task handoff is a durable message with parent linkage and expected output.
**When to use:** Whenever ownership must remain clear across agents.
**Trade-offs:** Slightly more explicit metadata, much less ambiguity.

**Example:**
```text
parent objective
  -> child task for backend
  -> completion receipt
  -> review request to reviewer
  -> review response linked to same thread tree
```

### Pattern 3: Human Checkpoints on Exceptional Paths

**What:** Let the coordinator continue normal routing automatically, but require explicit escalation on ambiguity, repeated failure, or cross-agent conflict.
**When to use:** Any time the model would otherwise guess at a high-cost decision.
**Trade-offs:** Some pauses remain, but trust stays high.

## Data Flow

### Request Flow

```text
[Human Goal]
    ↓
[Coordinator Intake]
    ↓
[Task Planner] → [Child Task Messages] → [Worker Agent]
    ↓                                   ↓
[Run Graph] ← [Task Events / Replies] ← [Reviewer / Researcher]
    ↓
[Summary / Escalation]
```

### State Management

```text
[Mailbox + Task Events]
    ↓
[Coordinator Run State]
    ↓
[status/log/summaries]
    ↓
[Human operator]
```

### Key Data Flows

1. **Goal decomposition flow:** human goal becomes a coordinator run and one or more child task envelopes persisted before notification.
2. **Execution-to-review flow:** implementer completion triggers a review-request handoff and later feeds the run summary.
3. **Blocker escalation flow:** wait/block events are classified and turned into reroute, clarification request, or human escalation.

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| 2-5 agents | Single coordinator over current mailbox layout is fine |
| 5-15 agents | Add better run indexing, direct task graph lookup, and stronger summary aggregation to avoid filesystem rescans |
| 15+ agents or remote workers | Separate orchestration state from simple receipt scans and consider a queue/index layer, while preserving mailbox artifacts for auditability |

### Scaling Priorities

1. **First bottleneck:** status and summary aggregation over many receipts/files; fix with run-level indexes or cached counters.
2. **Second bottleneck:** routing ambiguity across many similar agents; fix with clearer role metadata and candidate narrowing rather than wider broadcast.

## Anti-Patterns

### Anti-Pattern 1: Coordinator as Another Chatty Agent

**What people do:** let the coordinator talk freely with every worker and infer state from conversation.
**Why it's wrong:** state becomes ambiguous, replay is poor, and operators lose confidence.
**Do this instead:** persist task graph transitions and keep coordinator decisions tied to explicit messages and task states.

### Anti-Pattern 2: Replace the Existing Store with Framework Memory

**What people do:** introduce a supervisor framework whose internal memory becomes the de facto source of truth.
**Why it's wrong:** breaks recovery and conflicts with the repo’s core design choice that the filesystem is authoritative.
**Do this instead:** treat external framework patterns as inspiration and keep durable artifacts first.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Agent CLIs (Codex, Claude, generic) | Existing adapter boundary with readiness + notify hooks | Coordinator logic should not depend on vendor-specific prompts beyond adapter contracts |
| `tmux` | Existing client abstraction | Use panes for visibility and delivery, not as the canonical task database |
| Optional future sandbox/runtime | Execution target behind mailbox tasks | Keep separate from initial coordinator milestone |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `session ↔ coordinator` | Direct API | Session commands should invoke orchestrator logic explicitly |
| `coordinator ↔ mailbox` | Envelope/receipt/task-event persistence | Child tasks and run summaries should be durable artifacts |
| `coordinator ↔ runtime` | Read observed state, avoid control coupling | Runtime notifies; coordinator decides workflow transitions |
| `coordinator ↔ adapter metadata` | Role/capability lookup | Routing policy should be based on declared capabilities, not transcript guesswork |

## Sources

- Anthropic, "Building Effective Agents": https://www.anthropic.com/engineering/building-effective-agents
- Microsoft AutoGen Teams tutorial: https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/tutorial/teams.html
- Microsoft AutoGen Selector Group Chat tutorial: https://microsoft.github.io/autogen/dev/user-guide/agentchat-user-guide/selector-group-chat.html
- LangGraph supervisor docs: https://langchain-ai.github.io/langgraphjs/reference/modules/langgraph-supervisor.html
- OpenAI Codex product page: https://openai.com/codex/
- OpenHands runtime architecture docs: https://docs.openhands.dev/openhands/usage/architecture/runtime

---
*Architecture research for: terminal-based multi-agent coding orchestration*
*Researched: 2026-04-05*
