# Project Knowledge Index

## Decisions
- [File-based mailbox over tmux as message bus](decision-2026-03-28-file-based-mailbox.md) -- Immutable messages on disk, tmux as presentation layer only `architecture, mailbox, tmux`
- [Generic adapter as v0.1 default](decision-2026-03-28-generic-adapter-v01.md) -- Generic quiet-period+regex adapter for v0.1, Claude/Codex specific adapters added in v0.2 `adapter, claude-code, codex`
- [Codex (GPT-5.4) as design partner and implementer](decision-2026-03-28-codex-collaboration.md) -- 25-topic design discussion + orchestrated implementation via tmux `process, codex, collaboration`

## Discoveries
- [Claude Code and Codex CLI idle prompt patterns](discovery-2026-03-28-agent-cli-prompts.md) -- Ready regex and quiet periods for adapter probing `claude-code, codex, adapter`

## Gotchas
- [tmux send-keys requires separate Enter with delay](gotcha-2026-03-28-tmux-send-keys-enter.md) -- Send Enter 0.1s after text to avoid swallowed input `tmux, send-keys`
- [golangci-lint v2 requires action v7 and new config format](gotcha-2026-03-29-golangci-lint-v2-migration.md) -- v1 config incompatible, needs version:"2" + nested settings `ci, golangci-lint`
- [FakeClient needs mutex for concurrent test access](gotcha-2026-03-29-fakeclient-race.md) -- Race between daemon goroutine and test reads `testing, race-condition`

## Patterns
- [Index-based iteration for large structs](pattern-2026-03-29-large-struct-iteration.md) -- Use `&slice[i]` for AgentConfig(152B), Envelope(296B), Receipt(144B) `go, performance, linting`
