# tmuxicate

Multi-agent collaboration in tmux, backed by a file-based mailbox.

`tmuxicate` is a CLI for running multiple AI coding agents side by side in `tmux` and giving them a shared coordination layer. It does not try to replace `tmux`, and it does not depend on one model vendor. It uses `tmux` for visibility and process management, and a file-backed mailbox for reliable agent-to-agent communication.

The problem it solves is simple: multiple agents are useful, but without coordination they duplicate work, lose context, and get stuck in vague conversations. `tmuxicate` gives each agent a role, a pane, an inbox, and a common way to exchange tasks, reviews, questions, and status updates. One agent can act as the coordinator, while others implement, review, or research in parallel.

Under the hood, every message is written to disk as an immutable record. Each recipient gets a receipt in their inbox. A small runtime daemon watches those inboxes, checks whether a pane looks safe to notify, and injects a short instruction telling the agent to read the message with `tmuxicate read`. Agents reply with `tmuxicate reply`, and task progress is tracked with `tmuxicate task accept`, `wait`, `block`, and `done`. The filesystem is the source of truth; `tmux` is the operator interface.

For the human, the workflow is straightforward: define a session in `tmuxicate.yaml`, run `tmuxicate up`, send the coordinator a goal, and watch the team work. You can inspect inboxes, follow transcripts, send ad-hoc instructions, or intervene directly in any pane. If the daemon dies or `tmux` crashes, the mailbox still exists on disk.

The key design choice is reliability over magic. `tmuxicate` keeps messages durable, delivery explicit, and coordination observable. It is not trying to turn terminal agents into a distributed operating system. It is a pragmatic collaboration tool: one binary, one `tmux` session, multiple agents, shared mailboxes, clear task ownership, and a human who can see and steer the whole system.

## Quick Start

Install prerequisites:

- `tmux`
- `go` 1.24+
- the agent CLIs you want to run (`codex`, `claude`, etc.)

Build or install:

```bash
go install github.com/coyaSONG/tmuxicate/cmd/tmuxicate@latest
```

For local development in this repo, build the actual CLI binary:

```bash
go build -o tmuxicate ./cmd/tmuxicate
```

Create `tmuxicate.yaml` from the example below, or adapt the repo’s local sample config if you are working from source.

Start a session:

```bash
tmuxicate up --config tmuxicate.yaml
```

Send work to the coordinator:

```bash
tmuxicate send pm "Implement X, keep tests green, ask reviewer for signoff before merge."
```

Inside an agent pane:

```bash
tmuxicate inbox
tmuxicate next
tmuxicate read msg_000000000142
printf 'Looks good. Two missing tests.\n' | tmuxicate reply msg_000000000142 --stdin
tmuxicate task accept msg_000000000142
tmuxicate task done msg_000000000142 --summary "Reviewed and replied"
```

Stop the session:

```bash
tmuxicate down --config tmuxicate.yaml
```

## Configuration

Minimal triad setup:

```yaml
version: 1

session:
  name: tmuxicate-dev
  workspace: .
  state_dir: .tmuxicate/sessions/dev
  window_name: agents
  layout: triad
  attach: false

delivery:
  mode: notify_then_read
  ack_timeout: 2m
  retry_interval: 30s
  max_retries: 3

transcript:
  mode: pipe-pane
  dir: .tmuxicate/sessions/dev/transcripts

routing:
  coordinator: coordinator

defaults:
  workdir: .
  notify:
    enabled: true

agents:
  - name: coordinator
    alias: pm
    adapter: codex
    command: codex
    role: Project coordinator
    pane:
      slot: main
    teammates: [backend, reviewer]

  - name: backend
    alias: api
    adapter: claude-code
    command: claude
    role: Backend implementer
    pane:
      slot: right-top
    teammates: [coordinator, reviewer]

  - name: reviewer
    alias: review
    adapter: codex
    command: codex
    role: Reviewer
    pane:
      slot: right-bottom
    teammates: [coordinator, backend]
```

The full schema and design rationale live in [DESIGN.md](./DESIGN.md).

## CLI Reference

| Command | Description |
| --- | --- |
| `tmuxicate up` | Create the state directory, start tmux panes, generate agent artifacts, and launch the daemon. |
| `tmuxicate down` | Stop the tmux session and preserve mailbox state. |
| `tmuxicate serve` | Run the minimal runtime daemon manually. |
| `tmuxicate send <agent> <message>` | Create a new mailbox message for an agent. |
| `tmuxicate inbox` | List inbox entries for the current agent. |
| `tmuxicate read <message-id>` | Read a message and move it from `unread` to `active` if needed. |
| `tmuxicate reply <message-id>` | Reply in the parent thread. |
| `tmuxicate next` | Read the next unread message by priority and sequence. |
| `tmuxicate task accept <message-id>` | Accept a task and mark the agent busy. |
| `tmuxicate task wait <message-id>` | Mark an active task as waiting. |
| `tmuxicate task block <message-id>` | Mark an active task as blocked. |
| `tmuxicate task done <message-id>` | Mark an active task as done. |
| `tmuxicate status` | Planned operator dashboard; currently stubbed. |
| `tmuxicate log` | Planned transcript/event viewer; currently stubbed. |
| `tmuxicate init` | Planned config/bootstrap helper; currently stubbed. |
| `tmuxicate pick` | Planned fzf/tmux picker; currently stubbed. |

## Architecture

`tmuxicate` has four core pieces:

- Mailbox: immutable messages on disk plus per-recipient receipts under `.tmuxicate/sessions/<id>/`.
- Adapters: a generic adapter layer that probes panes and injects short notifications.
- Daemon: `tmuxicate serve`, which watches unread inboxes, retries delivery, and writes heartbeat/observed state.
- tmux: the pane manager and human-visible interface.

The mailbox is the source of truth. `tmux` is the presentation layer.

## v0.1 Status

What works now:

- YAML config loading and validation
- durable message and receipt storage
- `tmux` session creation and pane metadata
- per-agent `run.sh` and `bootstrap.txt` generation
- unread mailbox delivery via a minimal daemon
- agent-facing commands: `inbox`, `read`, `reply`, `next`, and `task *`

What is planned for v0.2:

- richer adapter support for Codex and Claude-specific readiness signals
- operator dashboard and transcript viewer
- picker UI
- stronger reconciliation and recovery flows
- coordinator-friendly automation on top of the mailbox

## License

MIT
