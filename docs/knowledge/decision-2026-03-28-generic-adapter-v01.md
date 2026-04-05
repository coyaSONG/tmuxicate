---
title: Generic adapter as v0.1 default
category: decision
status: active
date: 2026-03-28
tags: [adapter, claude-code, codex, architecture]
---

# Generic adapter as v0.1 default

## Context
Need to detect when an AI agent pane is ready to receive a notification and inject the message instruction. Different CLI agents (Claude Code, Codex, Gemini) have different prompt patterns and readiness signals.

## Considered Options
1. **Agent-specific adapters only** -- Tight integration with each CLI's hooks/APIs. Best UX but high coupling and maintenance.
2. **Generic adapter only** -- Quiet-period + regex probing on tmux capture-pane output. Works with any CLI but less precise.
3. **Generic default + specific overrides** -- Generic adapter for v0.1, add Claude/Codex specific adapters later.

## Decision
Option 3. v0.1 ships with GenericAdapter only (quiet period + regex). v0.2 adds ClaudeCodeAdapter and CodexAdapter as thin wrappers with preset configs.

Adapter specifics (added in v0.2):
- **Claude Code**: Ready regex `(?m)^❯\s*$`, quiet period 1200ms, notification says "using the shell tool"
- **Codex**: Ready regex `(?m)^›(?:\s|$)`, quiet period 1500ms, notification says "use the shell tool"
- **Factory**: `adapter.NewAdapter(type, client, paneID)` maps "generic"/"claude-code"/"codex" to constructors

## Impact
- Any CLI agent works out of the box with generic adapter
- Claude/Codex get better readiness detection with specific adapters
- `adapter` field in tmuxicate.yaml selects which adapter to use
