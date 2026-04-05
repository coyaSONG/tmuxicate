---
title: Claude Code and Codex CLI idle prompt patterns
category: discovery
status: active
date: 2026-03-28
tags: [claude-code, codex, adapter, readiness]
---

# Claude Code and Codex CLI idle prompt patterns

Discovered during design discussion (March 2026). These patterns are used by adapters to detect when an agent pane is idle and safe to inject a notification.

## Claude Code (v2.1.86+)
- Idle prompt: `❯` (Unicode right-pointing triangle)
- Ready regex: `(?m)^❯\s*$`
- Quiet period: 1200ms (no transcript output for this duration)
- Bootstrap: `--append-system-prompt` flag + optional `--settings` for adapter config
- Notification phrasing: "Please run `tmuxicate read <id>` **using the shell tool**, then reply through tmuxicate."

## Codex (v0.117.0+)
- Idle prompt: `›` (Unicode single right-pointing angle quotation mark)
- Ready regex: `(?m)^›(?:\s|$)`
- Quiet period: 1500ms
- Bootstrap: `--no-alt-screen` flag + initial prompt argument
- Notification phrasing: "Please **use the shell tool** to run `tmuxicate read <id>`, then respond via tmuxicate."

## Notes
- These patterns may change with CLI updates -- verify against current versions
- Both adapters embed `GenericAdapter` and only override `Notify` for custom message phrasing
- Factory function `adapter.NewAdapter(type, client, paneID)` instantiates the right adapter
