# Stack Research

**Domain:** terminal-based multi-agent coding orchestration
**Researched:** 2026-04-05
**Confidence:** HIGH

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.26.1 | Primary implementation language for the coordinator workflow engine and CLI | The existing product is already a Go CLI, and the next milestone is workflow orchestration on top of current primitives rather than a platform rewrite |
| `github.com/spf13/cobra` | 1.10.2 | Command surface for human and agent entrypoints | Keeps coordinator automation accessible through explicit CLI commands and preserves the current operator workflow |
| Filesystem-backed mailbox (`internal/mailbox`, `internal/protocol`) | current repo implementation | Durable message, receipt, and task-state source of truth | Research across Anthropic, LangGraph, and AutoGen points toward explicit workflow state and debuggability; this repo already has that advantage |
| `tmux` | current runtime dependency | Visible execution surface for multiple live agents | `tmuxicate` differentiates on human-visible coordination; keep panes as the operator interface rather than replacing them with opaque background workers |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/fsnotify/fsnotify` | 1.9.0 | Watch inboxes and runtime files for work progression | Keep using it for orchestration events, but pair it with explicit state transitions rather than implicit chat-only coordination |
| `gopkg.in/yaml.v3` | 3.0.1 | Human-editable config and mailbox metadata | Retain for config and durable artifacts; it matches the repo’s current protocol and operator ergonomics |
| Go standard library (`context`, `time`, `os`, `path/filepath`, `encoding/json`) | stdlib | Workflow engine, deadlines, persistence, and summary output | Coordinator automation should stay simple and composable; there is no evidence that a heavyweight external workflow framework is needed here |
| Optional tracing/metrics package | defer decision | Structured observability for coordinator runs | Add only if the first automation slice needs richer run telemetry than current logs and transcripts provide |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `go test ./... -race` | Validate orchestration and session behavior | High priority because automation expands risk in the currently under-tested session/runtime areas |
| `golangci-lint` | Catch control-flow and API misuse in coordinator code | Keep current lint pipeline; add rules only if orchestration logic introduces repeated bug classes |
| transcript and JSONL fixtures | Regression coverage for adapter and coordinator behavior | Use captured mailbox + transcript scenarios to test routing, review handoff, and blocker escalation deterministically |

## Installation

```bash
# Existing core stack
go mod tidy

# Verification
go test ./... -count=1 -race
golangci-lint run
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| Explicit workflow engine in this Go repo | LangGraph/AutoGen supervisor framework | Only if `tmuxicate` stops being a tmux-native CLI and becomes a hosted orchestrator service |
| File-backed durable state | In-memory conversation-only team orchestration | Only for throwaway experiments where replay, recovery, and operator inspection do not matter |
| Role-based coordinator over mailbox primitives | Freeform broadcast group chat between all agents | Only for small brainstorming tasks where duplicate work and opaque context are acceptable |
| Incremental orchestration features in current packages | Ground-up rewrite around sandbox/runtime platform such as OpenHands | Only if product scope shifts toward isolated execution environments rather than terminal coordination |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Heavy supervisor framework as the new core runtime | Adds abstraction layers that obscure prompts, handoffs, and failure modes; Anthropic explicitly recommends starting with simple composable patterns | Implement coordinator logic directly on top of the existing mailbox/task model |
| Hidden shared memory as the authoritative run state | Makes runs hard to replay, inspect, or recover after daemon/pane failures | Keep durable receipts, envelopes, and explicit task events as the source of truth |
| Unbounded all-to-all agent messaging | Raises token cost, duplicates work, and destroys operator clarity | Coordinator-mediated handoffs with bounded recipients and expected outputs |
| Vendor-locked orchestration assumptions | `tmuxicate` is already positioned as multi-vendor and should stay that way | Keep adapter boundaries and role descriptions vendor-neutral |

## Stack Patterns by Variant

**If the workflow remains local and tmux-native:**
- Keep coordinator automation inside the current CLI/session/runtime packages
- Because the product’s value comes from visible local orchestration, not remote hidden execution

**If future work introduces remote sandboxes or cloud workers:**
- Add a separate execution boundary behind the existing mailbox protocol
- Because OpenHands-style runtimes solve isolation, but they should be a downstream execution target, not the first coordinator abstraction

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| `go 1.26.1` | current `go.mod` dependencies | Matches the repo baseline and avoids a distracting toolchain migration inside this milestone |
| `cobra 1.10.2` | existing CLI wiring in `cmd/tmuxicate/main.go` | Adequate for adding coordinator-oriented commands or flags without introducing a new command framework |
| `fsnotify 1.9.0` | current daemon event loop | Works for file-triggered orchestration, but do not let watcher callbacks become the only source of workflow state |

## Sources

- Anthropic, "Building Effective Agents" — workflow-vs-agent guidance, simple composable patterns, tool/interface design: https://www.anthropic.com/engineering/building-effective-agents
- Microsoft AutoGen Teams docs — role-based teams, observability, and selector-based coordination patterns: https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/tutorial/teams.html
- Microsoft AutoGen Selector Group Chat docs — coordinator/planner plus specialized worker pattern and termination conditions: https://microsoft.github.io/autogen/dev/user-guide/agentchat-user-guide/selector-group-chat.html
- LangGraph Supervisor docs — hierarchical supervisor with controlled handoffs and message-history policies: https://langchain-ai.github.io/langgraphjs/reference/modules/langgraph-supervisor.html
- OpenAI Codex product page — parallel coding workflows, always-on background work, and review-oriented agent usage: https://openai.com/codex/
- OpenHands runtime architecture docs — when sandboxed execution matters and why it should stay a separate concern from coordination: https://docs.openhands.dev/openhands/usage/architecture/runtime

---
*Stack research for: terminal-based multi-agent coding orchestration*
*Researched: 2026-04-05*
