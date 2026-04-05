---
title: File-based mailbox over tmux as message bus
category: decision
status: active
date: 2026-03-28
tags: [architecture, mailbox, tmux, messaging]
---

# File-based mailbox over tmux as message bus

## Context
Designing how multiple AI agents running in tmux panes should communicate. The question was whether to use tmux itself (pipes, buffers, send-keys) as the message transport or to build a separate mechanism.

## Considered Options
1. **tmux as message bus** -- Use tmux pipes/buffers directly for agent-to-agent communication. Simple but fragile: messages lost on crash, no delivery guarantees, no threading.
2. **File-based mailbox** -- Immutable messages on disk (envelope.yaml + body.md) with per-recipient receipts. tmux used only for notification injection. More complex but durable and observable.
3. **Database-backed** -- SQLite or similar. Reliable but heavier dependency, harder to inspect.

## Decision
File-based mailbox. Codex (GPT-5.4) provided the critical architectural insight during design discussion: "tmux should be presentation layer, not message bus."

Core design:
- Messages are immutable files under `.tmuxicate/sessions/<id>/messages/`
- Each recipient gets a receipt with folder state (unread/active/done/dead)
- Atomic writes via staging dir -> fsync -> rename -> fsync parent
- flock-based sequence allocation for unique message IDs (msg_000000000142)
- SHA256 body verification on read

## Impact
- Mailbox survives daemon crashes and tmux restarts
- All state is inspectable on disk (human debuggable)
- tmux send-keys used only for short notification injection, not message transport
- Daemon watches unread dirs via fsnotify + timer sweep
