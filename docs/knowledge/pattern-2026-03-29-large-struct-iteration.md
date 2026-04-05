---
title: Index-based iteration for large structs
category: pattern
status: active
date: 2026-03-29
tags: [go, performance, linting, convention]
---

# Index-based iteration for large structs

golangci-lint v2 with gocritic flags `rangeValCopy` for any struct >= 96 bytes iterated by value. This project has several large types:

| Type | Size | Where |
|------|------|-------|
| `config.AgentConfig` | 152B | everywhere |
| `protocol.Envelope` | 296B | mailbox, session |
| `protocol.Receipt` | 144B | mailbox, session |
| `tmux.PaneInfo` | 120B | session, pick |
| `AgentStatus` | 104B | status |
| `tmux.PopupSpec` | 96B | tmux client |

## Convention

Use index-based iteration with pointer alias:

```go
// Good
for i := range cfg.Agents {
    agent := &cfg.Agents[i]
    // use agent.Name, etc.
}

// Also good (when few field accesses)
for i := range cfg.Agents {
    name := cfg.Agents[i].Name
}

// Bad - copies 152 bytes per iteration
for _, agent := range cfg.Agents {
    // ...
}
```

For function parameters, pass large structs by pointer:

```go
func CreateMessage(env *protocol.Envelope, body []byte) error { ... }
func DisplayPopup(ctx context.Context, spec *PopupSpec) error { ... }
```
