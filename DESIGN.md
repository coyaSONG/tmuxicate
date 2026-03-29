# tmuxicate Design Document

## 1. Overview

`tmuxicate` is a CLI for running multiple AI coding agents side by side in tmux and giving them a shared coordination layer. It does not replace tmux, and it does not depend on one model vendor. It uses tmux for visibility and process management, and a file-backed mailbox for reliable agent-to-agent communication.

The problem it solves is simple: multiple agents are useful, but without coordination they duplicate work, lose context, and get stuck in vague conversations. `tmuxicate` gives each agent a role, a pane, an inbox, and a common way to exchange tasks, reviews, questions, and status updates. One agent can act as the coordinator, while others implement, review, or research in parallel.

Under the hood, every message is written to disk as an immutable record. Each recipient gets a receipt in their inbox. A small runtime daemon watches those inboxes, checks whether a pane looks safe to notify, and injects a short instruction telling the agent to read the message with `tmuxicate read`. Agents reply with `tmuxicate reply`, and task progress is tracked with `tmuxicate task accept`, `wait`, `block`, and `done`. The filesystem is the source of truth; tmux is the operator interface.

For the human, the workflow is straightforward: define a session in `tmuxicate.yaml`, run `tmuxicate up`, send the coordinator a goal, and watch the team work. You can inspect status, follow logs, send ad-hoc instructions, or intervene directly in any pane. If the daemon dies or tmux crashes, the mailbox still exists on disk.

The key design choice is reliability over magic. `tmuxicate` keeps messages durable, delivery explicit, and coordination observable. It is not trying to turn terminal agents into a distributed operating system. It is a pragmatic collaboration tool: one binary, one tmux session, multiple agents, shared mailboxes, clear task ownership, and a human who can see and steer the whole system.

For v0.1, the design is intentionally narrower than some earlier ideas:
- The filesystem is the source of truth.
- The generic adapter is the baseline.
- Auto-notify is best-effort and optional.
- Threads are derived from `thread` and `reply_to` fields on messages, not from a separate authoritative thread database.
- Full reconcile, runtime add/remove agent, advanced layout editing, and vendor-specific adapter enhancements are deferred when they do not block the core value.

## 2. Architecture

### 2.1 Core concept

`tmuxicate` manages multiple AI agents in tmux panes and lets them communicate through a durable mailbox. Tmux is the presentation layer. The mailbox is the coordination layer.

Primary principles:
- Tmux is not the message bus.
- The filesystem is authoritative.
- Agents receive short notifications in the pane and read full content from disk.
- Messages are immutable and append-only.
- Delivery is explicit, observable, and idempotent.
- Human operators can always inspect and intervene.

### 2.2 Major components

1. `tmuxicate` CLI
   - human-facing commands: `up`, `down`, `send`, `status`, `log`
   - agent-facing commands: `inbox`, `read`, `reply`, `next`, `task`
2. File-backed mailbox
   - immutable messages
   - per-recipient receipts
   - atomic writes and updates
3. Runtime daemon: `tmuxicate serve`
   - watches receipts and state
   - retries notifications
   - performs health probes
   - updates current state
4. Adapter layer
   - starts agents
   - probes readiness
   - injects short notifications
   - handles bootstrap
5. Tmux integration
   - session and pane lifecycle
   - pane metadata
   - transcript capture with `pipe-pane`

### 2.3 Message delivery model

Chosen delivery model:
- canonical message is written to disk
- recipient receipt is written to `agents/<name>/inbox/unread/`
- daemon optionally injects a short notification into the pane
- agent explicitly runs `tmuxicate read <message-id>`

This combines:
- durability from a file-backed queue
- visibility from short injected notifications
- reliability from explicit agent reads

Rejected as the sole model:
- pure `send-keys` transport for full payloads
- pure polling without notifications
- a second message protocol inside tmux scrollback

### 2.4 Coordinator pattern

The preferred default is a coordinator pane plus specialist worker panes.

Example triad:
- coordinator: decomposes work and routes tasks
- backend: implements code changes and runs targeted verification
- reviewer: reviews diffs, finds bugs, and challenges assumptions

Coordinator responsibilities:
- break user goals into bounded tasks
- assign tasks to the right agent
- mediate disagreements
- escalate to human when required
- keep work moving without redundant chatter

### 2.5 Declared vs observed state

Tmuxicate tracks two distinct state types:

Declared state:
- set by the agent through `tmuxicate task ...`
- examples: `idle`, `busy`, `awaiting_reply`, `blocked`, `done`, `needs_human`

Observed state:
- inferred by tmuxicate from hooks, pane output, and process liveness
- examples: `starting`, `ready`, `active`, `unknown`, `exited`, `suspect_stuck`

This separation avoids conflating:
- what the agent says it is doing
- what tmuxicate can actually observe

## 3. Message Protocol

### 3.1 Canonical store

Canonical session store:

```text
.tmuxicate/sessions/<session>/
  messages/
    msg_000000000142/
      envelope.yaml
      body.md
  agents/
    <agent>/
      inbox/
        unread/
        active/
        done/
        dead/
```

Messages exist once. Receipts reference them by `message_id`. There are no copied message bodies per inbox.

### 3.2 `envelope.yaml`

Path:

```text
messages/msg_<id>/envelope.yaml
```

Required fields:

```yaml
schema: tmuxicate/message/v1
id: msg_000000000142
seq: 142
session: dev
thread: thr_000000000019
kind: review_request
from: coordinator
to:
  - reviewer
created_at: 2026-03-29T00:12:22.417Z
body_format: markdown
body_sha256: 8a5d4d7c5bf3a1f4f54abf1b7f70d3f3d95c2f5f7e82f4c0f33a0a2ec8714abc
body_bytes: 913
```

Optional fields:

```yaml
reply_to: msg_000000000141
subject: Review auth diff
priority: normal
requires_ack: true
requires_claim: false
deliver_after: 2026-03-29T00:12:22.417Z
expires_at: 2026-03-29T01:12:22.417Z
budget:
  max_turns: 1
  max_lines: 40
  respond_by: 2026-03-29T00:30:00Z
attachments:
  - path: artifacts/diff.patch
    media_type: text/x-diff
    sha256: 4e2b4f5a6874a1a26d3ce9fdb9f6d8bfa70cb737d8283e28c2a9338c40d0e734
meta:
  source: tmuxicate-send
```

Field notes:
- `id`: unique per session
- `seq`: monotonic session-local integer
- `thread`: thread identifier carried by messages, authoritative for grouping
- `kind`: semantic type
- `from`: logical sender identity
- `to`: one or more logical recipients
- `body_sha256`: verified on every read

### 3.3 Message kinds

v0.1 kinds:
- `task`
- `question`
- `review_request`
- `review_response`
- `decision`
- `status_request`
- `status_response`
- `note`

### 3.4 `body.md`

Path:

```text
messages/msg_<id>/body.md
```

Rules:
- UTF-8
- opaque Markdown
- no frontmatter
- trailing newline required
- tmuxicate does not parse headings

Recommended structure:

```md
# Review auth diff

## Task
Review the attached patch for regressions and missing tests.

## Context
Coordinator wants a fast risk review before merge.

## Expected Reply
List findings first. Include file paths and missing tests.

## Artifacts
- artifacts/diff.patch
```

### 3.5 Receipt file

Path:

```text
agents/<agent>/inbox/unread/0000000142-msg_000000000142.yaml
```

Schema:

```yaml
schema: tmuxicate/receipt/v1
message_id: msg_000000000142
seq: 142
recipient: reviewer
folder_state: unread
revision: 3
acked_at: null
claimed_by: null
claimed_at: null
done_at: null
notify_attempts: 1
last_notified_at: 2026-03-29T00:12:23.101Z
next_retry_at: 2026-03-29T00:12:53.101Z
last_error: null
```

Folder semantics:
- `unread/`: message not yet read by recipient
- `active/`: read and in progress
- `done/`: complete
- `dead/`: expired, cancelled, or undeliverable

### 3.6 Atomic writes

Canonical message write algorithm:

1. Acquire `locks/sequence.lock` using `flock`.
2. Increment `state/next-seq`.
3. Create staging dir:

```text
messages/.staging/msg_000000000142.<pid>.tmp/
```

4. Write `envelope.yaml.tmp` and `body.md.tmp`.
5. `fsync` each file.
6. Rename temp files to final names inside staging.
7. `fsync` staging dir.
8. Atomically rename staging dir to:

```text
messages/msg_000000000142/
```

9. `fsync(messages/)`.
10. For each recipient, write receipt to `unread/` via temp file, `fsync`, rename, `fsync`.

Receipt update algorithm:
- lock `locks/receipts/<agent>/<message_id>.lock`
- rewrite full receipt to temp file
- `fsync`
- rename over existing file
- `fsync` parent directory

On disk-full or partial failure:
- if canonical message is not committed, fail with no visible final message
- if canonical message commits but receipt creation fails, move message dir to `messages/orphaned/` during recovery and emit operator-visible error

### 3.7 Message integrity

On every `read`:
- load `body.md`
- compute SHA-256
- compare to `body_sha256`

On mismatch:
- mark receipt `dead`
- set `last_error=body checksum mismatch`
- emit operator-visible alert

## 4. Adapter System

### 4.1 Adapter interface

V0.1 adapter contract:

```go
package adapter

import "context"

type ReadyState string

const (
	ReadyStateStarting     ReadyState = "starting"
	ReadyStateReady        ReadyState = "ready"
	ReadyStateActive       ReadyState = "active"
	ReadyStateUnknown      ReadyState = "unknown"
	ReadyStateExited       ReadyState = "exited"
	ReadyStateSuspectStuck ReadyState = "suspect_stuck"
)

type MessageRef struct {
	ID      string
	Thread  string
	From    string
	Subject string
}

type TranscriptCursor struct {
	Offset int64
}

type TranscriptDelta struct {
	Bytes []byte
}

type AgentHealth struct {
	Alive      bool
	PID        int
	PaneID     string
	LastError  string
}

type BootstrapContext struct {
	Session   string
	Agent     string
	Alias     string
	PaneID    string
	StateDir  string
	Bootstrap string
}

type Adapter interface {
	Bootstrap(ctx context.Context, bc BootstrapContext) error
	Probe(ctx context.Context) (ReadyState, error)
	Notify(ctx context.Context, msg MessageRef) error
	Interrupt(ctx context.Context, reason string) error
	Capture(ctx context.Context, since TranscriptCursor) (TranscriptDelta, TranscriptCursor, error)
	Health(ctx context.Context) (AgentHealth, error)
}
```

Important interface decisions:
- `Probe` returns more than a boolean
- `Notify` receives a message ref, not arbitrary free text
- `Interrupt` exists even if v0.1 operator workflows rely mostly on direct pane interaction
- `Health` is separate from `Probe`

### 4.2 Generic adapter

The generic adapter is the baseline for v0.1. Vendor-specific adapters are enhancements.

Generic config:

```go
type GenericConfig struct {
	Command         []string
	WorkDir         string
	ReadyRegex      string
	BusyRegex       string
	QuietPeriod     time.Duration
	BootstrapMode   string // "arg", "paste", "none"
	BootstrapArgPos int
}
```

Generic behavior:
- starts the configured command in a pane via generated `run.sh`
- uses quiet-period and pane snapshots for readiness heuristics
- injects short notifications only when pane looks safe
- bootstraps either by command argument, initial pasted message, or no-op

Universal no-hooks readiness heuristic:
1. pane process exists
2. transcript has been quiet for `>= 1500ms`
3. two consecutive `capture-pane -pJ` snapshots `500ms` apart are identical
4. if `ReadyRegex` is configured, snapshot must match
5. if `BusyRegex` matches, state is forced to `active`

When no regex is configured:
- generic adapter only claims `ready`, `unknown`, or `exited`

### 4.3 Claude Code adapter

As of local inspection on March 28, 2026:
- `claude --help` supports `--append-system-prompt`, `--settings`, `--agent`, `--agents`, and plugin support
- Anthropic docs support hooks including `SessionStart`, `UserPromptSubmit`, `Stop`, `StopFailure`, and `SessionEnd`

V0.1 decision:
- Claude-specific hook integration is documented and supported as an enhancement
- generic adapter remains sufficient for the minimum shipping system

Claude generated runner:

```bash
#!/usr/bin/env bash
set -euo pipefail
export TMUXICATE_SESSION=dev
export TMUXICATE_AGENT=backend
export TMUXICATE_STATE_DIR="/abs/.tmuxicate/sessions/dev"
exec claude \
  --append-system-prompt "$(cat '/abs/.tmuxicate/sessions/dev/agents/backend/adapter/bootstrap.txt')" \
  --settings "/abs/.tmuxicate/sessions/dev/agents/backend/adapter/settings.json" \
  -n "backend@dev"
```

Observed idle UI in this environment:
- Claude Code `v2.1.86`
- idle prompt line begins with `❯`

Claude fallback probe:
- process alive
- no transcript bytes for `>= 1200ms`
- bottom snapshot matches `^❯\s*$`

Safe notification:

```bash
tmux send-keys -t %7 -l "[tmuxicate] New message msg_000000000142 from coordinator. Please run \`tmuxicate read msg_000000000142\` using the shell tool, then reply through tmuxicate."
tmux send-keys -t %7 Enter
```

Hook usage:
- hooks are for state and telemetry
- not the primary mailbox transport

Claude `SendMessage` and subagents:
- not used as the tmuxicate mailbox transport
- they are Claude-internal constructs, not a vendor-neutral bus

### 4.4 Codex adapter

As of local inspection on March 28, 2026:
- `codex --help` supports `mcp`, `mcp-server`, `app-server`, `resume`, `fork`, `--no-alt-screen`
- `codex features list` exposes `codex_hooks` as under development and disabled

V0.1 decision:
- do not depend on any unstable Codex hook surface
- use `--no-alt-screen`
- use generic transcript and pane heuristics

Codex generated runner:

```bash
#!/usr/bin/env bash
set -euo pipefail
export TMUXICATE_SESSION=dev
export TMUXICATE_AGENT=reviewer
export TMUXICATE_STATE_DIR="/abs/.tmuxicate/sessions/dev"
exec codex --no-alt-screen "$(cat '/abs/.tmuxicate/sessions/dev/agents/reviewer/adapter/bootstrap.txt')"
```

Observed idle UI in this environment:
- Codex `v0.117.0`
- inline prompt line begins with `›`

Codex probe:
- process alive
- no transcript bytes for `>= 1500ms`
- bottom snapshot matches `^›(?:\s|$)`

Codex notification:

```bash
tmux send-keys -t %5 -l "[tmuxicate] New message msg_000000000142 from coordinator. Please use the shell tool to run \`tmuxicate read msg_000000000142\`, then respond via tmuxicate."
tmux send-keys -t %5 Enter
```

### 4.5 Bootstrap modes

Supported bootstrap modes:
- `arg`: pass bootstrap text as initial prompt or system-prompt argument
- `paste`: wait for first ready-ish state and paste bootstrap as the first message
- `none`: no bootstrap injection

### 4.6 Auto-notify risk and fallback

Riskiest assumption in the system:
- `tmux send-keys` can safely inject a short notification into an interactive agent at the right time

Fallback if this is wrong:
- set `delivery.mode=manual`
- daemon does not inject any notifications
- agents or humans explicitly run `tmuxicate inbox` or `tmuxicate next`

Auto-notify must be best-effort and optional from day one.

## 5. Session Lifecycle

### 5.1 `tmuxicate up`

Exact flow:

1. Parse `tmuxicate.yaml`.
2. Resolve absolute `workspace`, `state_dir`, and agent workdirs.
3. Validate config: unique names, aliases, adapters, pane slots.
4. Validate dependencies: `tmux`, `sh`, configured agent commands.
5. Acquire `locks/session.lock`.
6. Check `tmux has-session -t <session_name>`.
7. If session exists and state dir is healthy:
   - return “already running”
   - optionally attach
8. If session exists and state dir is stale:
   - fail with clear recovery guidance
   - v0.1 recovery path is `down --force` or manual cleanup, not full reconcile
9. Create state tree under `.tmuxicate/sessions/<id>/`.
10. Write `config.resolved.yaml`.
11. Generate per-agent `bootstrap.txt`.
12. Generate per-agent `run.sh`.
13. Generate vendor-specific helper files if configured, such as Claude `settings.json`.
14. Start first pane with:

```bash
tmux new-session -d -s tmuxicate-dev -n agents -c /abs/workspace \
  "bash -lc 'exec /abs/.tmuxicate/sessions/dev/agents/coordinator/adapter/run.sh'"
```

15. Capture returned pane ID using `-P -F '#{pane_id}'`.
16. Create remaining panes with `split-window -P -F '#{pane_id}'`.
17. Apply layout.
18. Set pane titles and metadata:

```bash
tmux select-pane -t %5 -T "coordinator(pm)"
tmux set-option -p -t %5 @tmuxicate-agent coordinator
tmux set-option -p -t %5 @tmuxicate-alias pm
tmux set-option -p -t %5 @tmuxicate-adapter codex
tmux set-option -p -t %5 @tmuxicate-pane-slot main
tmux set-option -p -t %5 @tmuxicate-session dev
tmux set-option -t tmuxicate-dev @tmuxicate-state-dir /abs/.tmuxicate/sessions/dev
```

19. Set `remain-on-exit`.
20. Attach transcript capture:

```bash
tmux pipe-pane -o -t %5 "cat >> '/abs/.tmuxicate/sessions/dev/agents/coordinator/transcripts/raw.ansi.log'"
```

21. Start daemon:

```bash
tmuxicate serve --state-dir /abs/.tmuxicate/sessions/dev
```

22. Wait for startup grace period.
23. Probe each pane for readiness.
24. If any agent exits before ready timeout:
   - fail startup
   - default behavior is fail-fast teardown
   - `--keep-failed` leaves panes for debugging
25. Write `runtime/ready.json`.
26. Attach to tmux if configured.

### 5.2 `tmuxicate send`

Exact flow:

1. Parse args.
2. Resolve session:
   - `--session`
   - `TMUXICATE_SESSION`
   - nearest session rooted at cwd
3. Resolve sender:
   - if inside a managed pane, pane metadata defines sender
   - otherwise sender is `human`
4. Resolve target alias.
5. Acquire `locks/sequence.lock`.
6. Allocate `seq`, `id`, and default `thread` if needed.
7. Build `body.md`.
8. Compute `body_sha256` and `body_bytes`.
9. Atomically write canonical message.
10. Atomically write recipient receipts.
11. Append runtime event.
12. Return success once message and receipts exist on disk.
13. Daemon notices via fsnotify or next sweep.
14. If adapter says pane is ready, inject short notification.
15. If pane is busy, leave receipt in `unread/` and schedule retry.

`send` guarantees:
- durable message commit
- durable receipt creation

`send` does not guarantee:
- immediate notification injection
- immediate agent acknowledgment

### 5.3 `tmuxicate down`

Graceful shutdown flow:

1. Resolve session.
2. Acquire `locks/session.lock`.
3. Write `runtime/shutdown.request.json`.
4. Daemon sets `shutting_down=true` and stops new notifications.
5. Inject shutdown notice to ready panes.
6. Wait grace period, default `10s`.
7. Requeue every `active/` receipt back to `unread/`, clear claims, set `last_error=session_stopped`.
8. Flush daemon heartbeat and logs.
9. Close transcript pipes.
10. Kill tmux session.
11. Stop daemon.
12. Write `runtime/last_shutdown.json`.

Force mode:
- skips notification and grace wait

Purge mode:
- removes session state after stop

## 6. Runtime Daemon

### 6.1 Process model

`tmuxicate serve` is a separate long-lived process, not a goroutine inside `up`.

Reasons:
- survives `up` exit
- easier attach/detach semantics
- simpler operator lifecycle
- crash isolation

### 6.2 Event loop

The daemon uses a hybrid event model:
- `fsnotify` for fast wakeups
- timer heap for retries and lease expiry
- periodic full sweep for missed events and drift correction

Skeleton:

```go
type Daemon struct {
    cfg      *config.Resolved
    tmux     tmux.Client
    store    *mailbox.Store
    watcher  *fsnotify.Watcher
    timers   *timerheap.Queue
    log      *slog.Logger
    adapters map[string]adapter.Adapter
}

func (d *Daemon) Run(ctx context.Context) error
func (d *Daemon) handleFSEvent(ctx context.Context, ev fsnotify.Event) error
func (d *Daemon) handleTimer(ctx context.Context, now time.Time) error
func (d *Daemon) fullSweep(ctx context.Context, now time.Time) error
```

Main loop:
1. watch `agents/*/inbox/unread`
2. watch `agents/*/inbox/active`
3. watch `agents/*/events`
4. watch `runtime/control`
5. seed timer heap from existing receipts
6. `select` on:
   - context cancellation
   - fsnotify events
   - next due timer
   - periodic full sweep

### 6.3 Timers

V0.1 timers:
- notification retry
- agent health probe every `2s`
- lease expiry sweep every `5s`
- full sweep every `15s`
- daemon heartbeat every `5s`
- transcript rotation or size check every `60s`

### 6.4 Crash and restart

Volatile:
- fsnotify handles
- timer heap
- in-memory caches

Persisted:
- messages
- receipts
- state history
- current state file
- daemon pid
- heartbeat

Restart procedure:
- rebuild timer heap by scanning receipts
- rebuild current state from `state.current.json` and event streams
- probe all panes
- resume retry schedule

### 6.5 Current state files

Event logs are history. Current state needs a cheap read path.

Each agent has:

```text
agents/<agent>/events/state.jsonl
agents/<agent>/state.current.json
```

`state.current.json` is atomically rewritten whenever current observed or declared state changes.

### 6.6 CLI to daemon communication

V0.1 policy:
- ordinary commands do not talk to daemon over a socket
- commands write files
- daemon observes them via fsnotify and periodic sweep

Reserved for future:
- `runtime/control/*.json` for operator control messages like `shutdown`, `rescan`, `interrupt`

### 6.7 Logging

Daemon logs:

```text
state_dir/logs/serve.jsonl
state_dir/logs/serve.stderr.log
```

JSONL example:

```json
{"ts":"2026-03-29T00:10:11.222Z","level":"INFO","event":"notify.injected","agent":"reviewer","message_id":"msg_000000000142","pane_id":"%7","attempt":1}
```

## 7. Agent CLI Commands

### 7.1 Session and agent resolution

Session resolution:
1. `--session`
2. `TMUXICATE_SESSION`
3. nearest repo root containing `tmuxicate.yaml` and `.tmuxicate/current-session`

Agent resolution:
1. `--agent`
2. if inside tmux pane, pane metadata `@tmuxicate-agent`
3. `TMUXICATE_AGENT`

Agents must not be able to impersonate other agents through env vars alone.

### 7.2 `tmuxicate inbox [--unread] [--all]`

Default is `--unread`.

Outside a tmuxicate session:
- exit `1`
- print `not in a tmuxicate session`

Output:

```text
SEQ    PRI    STATE   KIND             FROM         THREAD          AGE   SUBJECT
142    high   unread  review_request   coordinator  thr_000000019   2m    Review auth diff
143    normal unread  question         backend      thr_000000020   8s    Need schema decision
```

Sorting:
- unread only: `priority DESC`, then `seq ASC`
- all: state order `unread`, `active`, `done`, then `priority DESC`, then `seq ASC`

### 7.3 `tmuxicate read <message-id>`

Behavior:
- load current agent’s receipt and canonical message
- if receipt is `unread`, move `unread -> active` and set `acked_at`
- if already `active` or `done`, still print message
- if receipt missing, exit `2`

Output:

```text
Message: msg_000000000142
Seq: 142
Thread: thr_000000000019
From: coordinator
To: reviewer
Kind: review_request
Priority: high
Subject: Review auth diff
Created: 2026-03-29T00:12:22Z
Requires-Claim: false
Attachments: artifacts/diff.patch (text/x-diff)

--- body.md ---
# Review auth diff

Please review the attached patch for regressions and missing tests.
```

### 7.4 `tmuxicate reply <message-id> [--body-file <path>] [--stdin]`

Body source precedence:
1. `--body-file`
2. `--stdin`
3. implicit stdin if stdin is not a TTY

If stdin is a TTY and no source is provided:
- exit `1`
- print `reply body required`

Reply semantics:
- `thread = parent.thread`
- `reply_to = parent.id`
- `to = parent.from`

Reply kind mapping:
- `review_request -> review_response`
- `status_request -> status_response`
- otherwise `note`

Success output:

```text
created msg_000000000144 in thread thr_000000000019
```

### 7.5 `tmuxicate next`

Selects first unread receipt by:
- `priority DESC`
- then `seq ASC`

Equivalent behavior to:
- pick best unread receipt
- perform `read`

If none:
- print `no unread messages`
- exit `3`

### 7.6 `tmuxicate task accept <message-id>`

Valid for:
- `task`
- `review_request`
- `question`
- `status_request`

Behavior:
- ensure receipt is at least active
- acquire claim if `requires_claim=true`
- set declared state to `busy`

If already claimed by another agent:
- exit `2`

### 7.7 `tmuxicate task wait <message-id> --on <target> --reason <text>`

Behavior:
- requires active receipt
- if claimable, current agent must own claim
- set declared state to `awaiting_reply`
- append state event with `waiting_on` and `reason`
- keep receipt in `active/`
- default also emits status update to coordinator in same thread

### 7.8 `tmuxicate task block <message-id> --reason <text> [--on <target>]`

Behavior:
- requires active receipt
- set declared state to `blocked`
- append state event with `blocked_on` and `reason`
- keep receipt in `active/`
- default emits escalation status update to coordinator

### 7.9 `tmuxicate task done <message-id> [--summary <text>]`

Behavior:
- requires active receipt
- if claimable, agent must own claim
- move `active -> done`
- set `done_at`
- clear claim fields
- set declared state to `idle`
- optional summary emits status update to coordinator before completion

## 8. State Management

### 8.1 Authoritative state

Authoritative state is always on disk:
- canonical messages
- receipts
- current state files
- state event logs
- tmux pane metadata

No in-memory daemon state is authoritative.

### 8.2 Reconciliation

The design discussed richer reconcile flows, but full automatic reconcile is cut from v0.1.

V0.1 policy:
- if state dir exists but tmux session does not, `up` may reuse preserved durable state and start fresh panes
- if tmux session exists but state dir is stale or missing, fail fast and require operator cleanup or `down --force`

Deferred richer reconcile behavior includes:
- adopting orphaned tmux sessions
- rebuilding state from pane titles
- full restart journals

### 8.3 Pane identity

Pane metadata is authoritative for runtime identity:

```bash
tmux set-option -p -t %7 @tmuxicate-agent reviewer
tmux set-option -p -t %7 @tmuxicate-alias review
tmux set-option -p -t %7 @tmuxicate-adapter codex
tmux set-option -p -t %7 @tmuxicate-pane-slot right-bottom
tmux set-option -p -t %7 @tmuxicate-session dev
tmux set-option -t tmuxicate-dev @tmuxicate-state-dir /abs/.../.tmuxicate/sessions/dev
```

Fallback match methods like PID or title are not sufficient for v0.1.

### 8.4 Edge cases

Agent process crashes mid-task:
- observed state becomes `exited`
- active receipts move back to unread
- claims cleared
- coordinator receives synthetic alert or operator sees it in status

Tmux server crashes:
- daemon sees tmux calls fail
- session marked degraded
- active receipts requeued to unread
- operator restarts with `up`

Disk full during write:
- atomic staging fails before visible commit
- command returns non-zero

Two humans sending concurrently:
- `sequence.lock` serializes allocation

Extremely long output:
- transcript files are authoritative
- tmux scrollback is not

Network interruption during Claude spinner:
- observed state may become `suspect_stuck`
- daemon must not inject into a visibly active spinner by default

## 9. Operator UX

### 9.1 Human workflow

Typical operator journey:

1. `git clone <project> && cd <project>`
2. install `tmux`, `fzf`, desired agent CLIs, and `tmuxicate`
3. run `tmuxicate init --template triad`
4. edit `tmuxicate.yaml`
5. run `tmuxicate up`
6. send initial goal to coordinator:

```bash
tmuxicate send pm "Implement X, keep tests green, ask reviewer for signoff before merge."
```

7. watch progress with:
   - pane switching
   - `tmuxicate status`
   - `tmuxicate log --all --follow`
8. intervene with:
   - `tmuxicate send <agent> ...`
   - direct pane interaction
   - `tmuxicate down`

### 9.2 Observing conversations

Primary:
- switch tmux panes

Secondary:
- `tmuxicate log --all --follow`

Dashboard:
- `tmuxicate status`

### 9.3 Ad-hoc instructions

Preferred path:

```bash
tmuxicate send backend "Stop refactor. Only fix the failing test."
```

This preserves mailbox history and threadability better than raw typing.

### 9.4 Operator interrupts and loops

The design discussed richer `interrupt` and `cancel` commands. They are useful but not required for the v0.1 core.

V0.1 operator guidance for loops:
1. inspect `tmuxicate thread show <id>` if available or `log --all`
2. send a decisive human message to coordinator
3. if necessary, type directly into the pane
4. use `down --force` if the session is irrecoverably wedged

### 9.5 `tmuxicate status`

Purpose:
- human operator dashboard

Output:

```text
Session: tmuxicate-dev   State: running   Uptime: 18m   Daemon: healthy
Window: agents           Layout: main-vertical

AGENT        PANE   OBSERVED  DECLARED        UNREAD  ACTIVE  LAST-EVENT   LAST-ERROR
coordinator  %5     ready     busy            0       2       4s           -
backend      %6     active    busy            1       1       1s           -
reviewer     %7     ready     awaiting_reply  0       1       12s          -

FLOW
sent=14  acked=11  done=8  pending=3  retrying=1  failed=0

THREADS
open=3  resolved=0  closed=0
```

Notes:
- context window or token metrics are best-effort and omitted in v0.1

### 9.6 `tmuxicate log`

Commands:
- `tmuxicate log <agent> [--tail N] [--follow] [--raw] [--events]`
- `tmuxicate log --all [--tail N] [--follow]`

Default view:
- merged normalized transcript stream plus structured tmuxicate events

Example:

```text
2026-03-29T01:10:12Z [reviewer] [notify] msg_000000000143 injected
2026-03-29T01:10:16Z [reviewer] Please use the shell tool to run `tmuxicate read msg_000000000143`
2026-03-29T01:10:30Z [reviewer] [state] observed=ready declared=busy
2026-03-29T01:11:04Z [reviewer] Found two risks in auth middleware...
```

Flags:
- `--raw`: show `raw.ansi.log`
- `--events`: show structured events only
- `--tail N`: default `100`
- `--follow`: follow mode

Correlation:
- notification events include `message_id`
- injected notification text includes `message_id`
- replies and task transitions emit structured events with `message_id` and `thread`

### 9.7 `tmuxicate pick`

The picker is useful, but it is cut from the minimum v0.1 core. The design is preserved here for later implementation.

Input rows:

```text
%7	review	reviewer	ready	idle	2	Reviewer pane
%5	pm	coordinator	busy	active	0	Coordinator pane
```

Exact `fzf` invocation:

```bash
tmuxicate __list-panes --session "$SESSION" |
fzf --ansi \
  --delimiter=$'\t' \
  --with-nth=2,3,4,5,6,7 \
  --nth=2,3,7 \
  --prompt='agent> ' \
  --height=100% \
  --layout=reverse \
  --border=rounded \
  --info=inline-right \
  --no-sort \
  --bind 'ctrl-r:reload(tmuxicate __list-panes --session '"$SESSION"')' \
  --preview 'tmuxicate __preview-pane --session '"$SESSION"' --pane {1} --alias {2}' \
  --preview-window 'right,65%,wrap,border-left'
```

Preview content:
- alias, agent name, pane id, title
- declared and observed state
- unread and active counts
- last notification time
- last 20 lines of transcript
- top unread subjects

Selected value insertion:

```bash
sel="$(tmuxicate pick --session dev --emit alias)"
tmux set-buffer -- "@${sel}"
tmux paste-buffer -p -t "$TMUX_PANE"
```

Suggested tmux binding:

```tmux
bind-key A display-popup -E -w 80% -h 70% -T 'tmuxicate pick' \
  "TMUXICATE_PICK_TARGET='#{pane_id}' tmuxicate pick --session '#S' --insert send-target"
```

## 10. Thread Model

### 10.1 V0.1 scope

Threads in v0.1 are derived from message fields only.

Authoritative fields:
- `thread`
- `reply_to`

There is no separate thread authority in v0.1.

This explicitly cuts earlier ideas about separate persisted thread lifecycle metadata as a v0.1 requirement.

### 10.2 Thread creation

New thread is created when:
- `tmuxicate send` is called without `--thread` and without `--reply-to`

Existing thread is reused when:
- `tmuxicate reply` is called
- `tmuxicate send --thread <id>` is used
- `tmuxicate send --reply-to <message-id>` is used

### 10.3 Derived lifecycle

Derived thread statuses:
- `open`: at least one receipt in `unread/` or `active/`
- `resolved`: all known receipts are in `done/`
- `closed`: not represented explicitly in v0.1; archival is deferred

### 10.4 Thread views

`tmuxicate thread list`:

```text
THREAD          STATUS    OPEN  LAST-ACTIVITY   PARTICIPANTS                    SUBJECT
thr_000000019   open      2     12s             coordinator,backend,reviewer    Review auth diff
thr_000000020   resolved  0     3m              coordinator,backend             Schema decision
```

`tmuxicate thread show <id>`:

```text
Thread: thr_000000000019
Status: open
Subject: Review auth diff
Participants: coordinator, reviewer
Open-Receipts: reviewer=active

[142] coordinator -> reviewer  review_request  high  2m
  Subject: Review auth diff

[144] reviewer -> coordinator  review_response normal  20s
  Subject: Findings on auth diff
```

## 11. Security

### 11.1 Identity

Threat:
- malicious or confused agent attempts to impersonate another by changing env vars or passing `--agent`

Policy:
- commands inside managed panes resolve sender from tmux pane metadata first
- `TMUXICATE_AGENT` is advisory only
- commands outside a managed pane are always `from=human`

Every agent-facing command verifies:
- `$TMUX_PANE` exists
- `@tmuxicate-agent` matches inferred sender
- `@tmuxicate-session` matches session

On mismatch:
- exit `1`
- print `pane identity mismatch`

### 11.2 Message integrity

`body_sha256` is verified on read and on selected daemon operations.

### 11.3 Cross-inbox access

Policy:
- agents may only read their own inbox through the tmuxicate CLI
- humans may inspect any inbox through operator commands

This is a policy boundary, not host-level isolation. Anyone with shell access can still inspect the filesystem directly.

### 11.4 Filesystem trust model

Tmuxicate does not provide host sandboxing. It assumes:
- all agents and humans share a working directory
- tmuxicate reduces accidental misuse
- tmuxicate does not defend against a fully malicious local process

## 12. Layout System

### 12.1 Named slots

Triad slot mapping:
- `main`: initial pane from `new-session`
- `right-top`: `split-window -h -p 35`
- `right-bottom`: `split-window -v -t <right-top-pane> -p 50`

Then:

```bash
tmux select-layout -t tmuxicate-dev:agents main-vertical
```

### 12.2 Layout strategies

Supported strategies:
- `triad`
- `tiled`
- `main-vertical`
- `main-horizontal`
- `even-horizontal`
- `even-vertical`

For `N > 3`, default recommendation is `tiled`.

### 12.3 Custom tmux layout strings

The design discussed raw `tmux select-layout` strings. They are useful, but support beyond the built-in named strategies can be deferred from v0.1 if needed.

Config shape:

```yaml
session:
  layout: custom
  tmux_layout: "b25d,237x63,0,0[158x63,0,0,0,78x63,159,0{78x31,159,0,1,78x31,159,32,2}]"
```

### 12.4 Runtime layout changes

The design discussed runtime `agent add/remove`. This is deferred from v0.1.

V0.1 assumption:
- agent set is static for the lifetime of a session

## 13. Bootstrap & Hooks

### 13.1 Common bootstrap template

Example bootstrap:

```text
tmuxicate bootstrap

You are running inside a tmuxicate-managed tmux pane.

Identity
- Agent name: backend
- Alias: api
- Session: tmuxicate-dev
- Role: Backend implementer. Make code changes, run targeted verification, and report diffs, risks, and blockers.

Team
- coordinator (alias: pm): project coordinator, task router, conflict resolver
- reviewer (alias: review): reviewer for bugs, regressions, tests, and design feedback

Communication model
- The tmuxicate mailbox is the source of truth.
- Short lines injected into this pane are notifications only.
- Do not communicate with teammates by manually pasting large text into other panes.
- Read a message with: tmuxicate read <message-id>
- List unread messages with: tmuxicate inbox --unread
- Reply with: tmuxicate reply <message-id> --stdin
- Accept a task with: tmuxicate task accept <message-id>
- Mark waiting with: tmuxicate task wait <message-id> --on <agent> --reason "<reason>"
- Mark blocked with: tmuxicate task block <message-id> --on human --reason "<reason>"
- Mark done with: tmuxicate task done <message-id> --summary "<one line>"

Working rules
- Stay within your role unless explicitly reassigned.
- Keep replies concise and specific. Reference files, commands, tests, and decisions.
- If instructions conflict, ask the coordinator instead of choosing silently.
- If you suspect pending work and have no notification, run: tmuxicate inbox --unread
- If you need a second opinion, send a mailbox message through tmuxicate, not raw pane text.

Startup action
- Acknowledge this bootstrap silently and wait for mailbox work.
```

### 13.2 Coordinator prompt

Complete coordinator bootstrap prompt:

```text
You are the coordinator agent in a tmuxicate-managed multi-agent session.

Your job is to turn user goals into clear, bounded work for the team, keep threads moving, prevent duplicate effort, resolve conflicts, and escalate to the human only when necessary.

Team
- You are: {agent_name} ({alias})
- Backend implementer: {backend_alias}. Strong at code changes and targeted verification.
- Reviewer: {reviewer_alias}. Strong at bug-finding, regression review, test gaps, and design critique.

Operating model
- The tmuxicate mailbox is the source of truth.
- Use tmuxicate commands through the shell tool.
- Prefer short, explicit assignments with clear expected outputs.
- Keep one owner per implementation task unless the task is explicitly parallelizable.
- Use threads to preserve context. Reply within an existing thread whenever possible.

Core responsibilities
1. Decompose work into the smallest useful independent tasks.
2. Route each task to the right agent based on role and current load.
3. Track open threads, waiting states, and blockers.
4. Make decisions when two agents disagree, or escalate to the human if the decision is product- or policy-sensitive.
5. Summarize progress for the human without flooding them.

tmuxicate commands
- Check status: tmuxicate status
- Check your inbox: tmuxicate inbox --unread
- Read next task: tmuxicate next
- Read a specific message: tmuxicate read <message-id>
- Send a new task: tmuxicate send <alias> --subject "<subject>" --stdin
- Reply in-thread: tmuxicate reply <message-id> --stdin
- Inspect a thread: tmuxicate thread show <thread-id>
- Mark a task done: tmuxicate task done <message-id> --summary "<one line>"
- Mark waiting: tmuxicate task wait <message-id> --on <agent> --reason "<reason>"
- Mark blocked: tmuxicate task block <message-id> --on human --reason "<reason>"

Routing rules
- Send implementation work to the backend agent.
- Send review, validation, and risk analysis to the reviewer.
- Do not ask both agents to solve the same implementation task unless you explicitly want competing proposals.
- Use the reviewer after backend changes when correctness matters.
- If a task is ambiguous, first narrow it before assigning it.

Conflict handling
- If two agents disagree, do not let them argue indefinitely.
- Read both positions, decide if the answer is technical and local.
- If yes, choose one direction and explain why in one short decision message.
- If no, escalate to the human with the minimum context needed for a decision.

Escalate to the human when
- Requirements are ambiguous and the ambiguity changes the implementation materially.
- A decision affects product behavior, policy, security posture, or irreversible data changes.
- An agent is blocked by missing credentials, external services, or failing infrastructure.
- The team is looping without new evidence.

Good coordination patterns
- Good: assign one concrete task with expected output, deadline, and thread continuity.
- Good: ask the reviewer for a focused review after the backend agent finishes a patch.
- Good: close a thread once the decision is made and the task is done.
- Bad: broadcast the same vague task to everyone.
- Bad: ask another agent to “figure it out” without files, scope, or success criteria.
- Bad: let backend and reviewer debate the same issue for multiple round-trips when you can decide.
- Bad: escalate to the human before you have synthesized the disagreement.

Examples
- Good assignment:
  Backend: “In thread thr_19, patch the auth middleware null-check bug in src/auth.ts, run targeted tests, and reply with changed files plus test results.”
- Good review request:
  Reviewer: “In thread thr_19, review the backend patch for regressions, missing tests, and unsafe assumptions. Findings first.”
- Good escalation:
  Human: “Backend proposes rejecting expired tokens with 401; reviewer suggests silent refresh. This changes user-facing auth behavior. Which policy do you want?”

Behavioral rules
- Be concise.
- Prefer one message with clear intent over many small pings.
- Always include file paths, commands, or concrete next actions when relevant.
- Keep the team moving. If a thread stalls, either decide or escalate.
```

### 13.3 Claude hook script: `emit-state.sh`

Exact script:

```bash
#!/usr/bin/env bash
set -euo pipefail

phase="${1:-unknown}"
state_dir="${TMUXICATE_STATE_DIR:-}"
agent="${TMUXICATE_AGENT:-unknown}"

# Always consume stdin so Claude's hook pipeline cannot wedge.
tmp="$(mktemp "${TMPDIR:-/tmp}/tmuxicate-hook.XXXXXX.json")"
trap 'rm -f "$tmp"' EXIT
cat >"$tmp" || true

# Never break Claude if tmuxicate state is unavailable.
if [[ -z "$state_dir" || ! -d "$state_dir" ]]; then
  exit 0
fi

events_dir="$state_dir/agents/$agent/events"
mkdir -p "$events_dir" 2>/dev/null || exit 0

if ! command -v tmuxicate >/dev/null 2>&1; then
  exit 0
fi

tmuxicate internal emit-state \
  --state-dir "$state_dir" \
  --agent "$agent" \
  --phase "$phase" \
  --hook-json "$tmp" \
  >>"$events_dir/emit-state.stderr.log" 2>&1 || true

exit 0
```

### 13.4 State event schema

Hook-derived state events:

```json
{
  "schema": "tmuxicate/state-event/v1",
  "ts": "2026-03-29T00:40:12.123Z",
  "agent": "backend",
  "source": "claude-hook",
  "phase": "ready",
  "observed_state": "ready",
  "hook_event_name": "Stop",
  "claude_session_id": "abc123",
  "claude_agent_id": null,
  "claude_agent_type": null,
  "cwd": "/Users/chsong/Developer/Personal/tmuxicate",
  "transcript_path": "/Users/chsong/.claude/projects/.../transcript.jsonl"
}
```

Phase mapping:
- `session_start -> starting`
- `busy -> active`
- `ready -> ready`
- `ready_error -> ready`
- `exited -> exited`
- else `unknown`

## 14. Configuration

### 14.1 YAML shape

```yaml
version: 1

session:
  name: tmuxicate-dev
  workspace: .
  state_dir: .tmuxicate/sessions/dev
  window_name: agents
  layout: triad
  attach: true

delivery:
  mode: notify_then_read
  ack_timeout: 2m
  retry_interval: 30s
  max_retries: 3
  safe_notify_only_when_ready: true
  auto_notify: true

transcript:
  mode: pipe-pane
  dir: .tmuxicate/sessions/dev/transcripts

routing:
  coordinator: coordinator
  exclusive_task_kinds:
    - task
  fanout_task_kinds:
    - review_request
    - question
    - status_request

defaults:
  workdir: .
  env:
    TMUXICATE_SESSION: tmuxicate-dev
  bootstrap_template: default
  notify:
    enabled: true

agents:
  - name: coordinator
    alias: pm
    adapter: codex
    command: codex
    role: >
      Project coordinator. Break work down, route tasks, resolve conflicts,
      and escalate to the human when needed.
    pane:
      slot: main
    teammates:
      - backend
      - reviewer
    bootstrap:
      extra_instructions: |
        You own task routing and final decision-making. Prefer short, explicit assignments.

  - name: backend
    alias: api
    adapter: claude-code
    command: claude
    workdir: .
    role: >
      Backend implementer. Make code changes, run targeted verification,
      and report diffs, risks, and blockers.
    pane:
      slot: right-top
    teammates:
      - coordinator
      - reviewer
    bootstrap:
      extra_instructions: |
        Focus on implementation. Escalate ambiguous product decisions to coordinator.

  - name: reviewer
    alias: review
    adapter: codex
    command: codex
    workdir: .
    role: >
      Reviewer. Review designs, patches, and plans for bugs, regressions,
      missing tests, and unclear assumptions.
    pane:
      slot: right-bottom
    teammates:
      - coordinator
      - backend
    bootstrap:
      extra_instructions: |
        Findings first. Keep reviews concise and risk-focused.
```

### 14.2 V0.1 config notes

In v0.1:
- static agent set per session
- no runtime add/remove
- no automatic restart policy unless added explicitly later

## 15. Directory Structure

### 15.1 Go project layout

```text
cmd/
  tmuxicate/
    main.go

internal/
  app/              # CLI wiring and dependency assembly
  config/           # YAML parsing, resolution, validation
  mailbox/          # message and receipt store
  protocol/         # envelope, receipt, thread, state types
  session/          # up/down lifecycle
  tmux/             # tmux client wrapper
  pane/             # pane/window layout and metadata
  adapter/          # generic + vendor-specific adapters
  transcript/       # pipe-pane management and transcript reads
  runtime/          # serve daemon
  state/            # declared/observed state management
  lock/             # flock and atomic file helpers
  logx/             # slog setup
  testutil/         # fakes and fixtures
```

### 15.2 Example session tree

Example with 3 agents, 5 messages, 2 derived threads:

```text
.tmuxicate/sessions/dev/
├── config.resolved.yaml (2.9 KB)
├── logs/
│   ├── serve.jsonl (18 KB)
│   └── serve.stderr.log (0 B)
├── locks/
│   ├── session.lock (0 B)
│   ├── sequence.lock (0 B)
│   └── receipts/
│       ├── backend/
│       │   ├── msg_000000000141.lock (0 B)
│       │   └── msg_000000000145.lock (0 B)
│       ├── coordinator/
│       │   ├── msg_000000000142.lock (0 B)
│       │   └── msg_000000000144.lock (0 B)
│       └── reviewer/
│           └── msg_000000000143.lock (0 B)
├── runtime/
│   ├── daemon.pid (6 B)
│   ├── daemon.heartbeat.json (196 B)
│   ├── events.jsonl (7.4 KB)
│   ├── ready.json (148 B)
│   └── last_shutdown.json (absent while running)
├── state/
│   └── next-seq (4 B)
├── messages/
│   ├── msg_000000000141/
│   │   ├── envelope.yaml (512 B)
│   │   └── body.md (428 B)
│   ├── msg_000000000142/
│   │   ├── envelope.yaml (476 B)
│   │   └── body.md (211 B)
│   ├── msg_000000000143/
│   │   ├── envelope.yaml (498 B)
│   │   └── body.md (306 B)
│   ├── msg_000000000144/
│   │   ├── envelope.yaml (484 B)
│   │   └── body.md (389 B)
│   └── msg_000000000145/
│       ├── envelope.yaml (472 B)
│       └── body.md (156 B)
└── agents/
    ├── coordinator/
    │   ├── adapter/
    │   │   ├── bootstrap.txt (3.8 KB)
    │   │   └── run.sh (242 B)
    │   ├── events/
    │   │   └── state.jsonl (1.1 KB)
    │   ├── state.current.json (188 B)
    │   ├── inbox/
    │   │   ├── unread/
    │   │   ├── active/
    │   │   │   └── 0000000144-msg_000000000144.yaml (236 B)
    │   │   ├── done/
    │   │   │   └── 0000000142-msg_000000000142.yaml (228 B)
    │   │   └── dead/
    │   └── transcripts/
    │       └── raw.ansi.log (24 KB)
    ├── backend/
    │   ├── adapter/
    │   │   ├── bootstrap.txt (3.4 KB)
    │   │   ├── run.sh (231 B)
    │   │   └── settings.json (1.2 KB)
    │   ├── events/
    │   │   ├── state.jsonl (1.4 KB)
    │   │   └── emit-state.stderr.log (0 B)
    │   ├── state.current.json (191 B)
    │   ├── inbox/
    │   │   ├── unread/
    │   │   │   └── 0000000145-msg_000000000145.yaml (214 B)
    │   │   ├── active/
    │   │   ├── done/
    │   │   │   └── 0000000141-msg_000000000141.yaml (226 B)
    │   │   └── dead/
    │   └── transcripts/
    │       └── raw.ansi.log (38 KB)
    └── reviewer/
        ├── adapter/
        │   ├── bootstrap.txt (3.2 KB)
        │   └── run.sh (208 B)
        ├── events/
        │   └── state.jsonl (942 B)
        ├── state.current.json (182 B)
        ├── inbox/
        │   ├── unread/
        │   ├── active/
        │   ├── done/
        │   │   └── 0000000143-msg_000000000143.yaml (227 B)
        │   └── dead/
        └── transcripts/
            └── raw.ansi.log (19 KB)
```

## 16. Testing Strategy

### 16.1 Layers

1. unit tests for mailbox and state logic
2. tmux integration tests using fake agents
3. optional manual or e2e tests for real Claude/Codex sessions

### 16.2 Tmux abstraction

```go
type Client interface {
    NewSession(ctx context.Context, spec SessionSpec) (paneID string, err error)
    SplitPane(ctx context.Context, spec SplitSpec) (paneID string, err error)
    SendKeys(ctx context.Context, paneID string, text string, enter bool) error
    CapturePane(ctx context.Context, paneID string, lines int) (string, error)
    PipePane(ctx context.Context, paneID string, cmd string) error
    SetPaneOption(ctx context.Context, paneID, key, value string) error
    ShowPaneOption(ctx context.Context, paneID, key string) (string, error)
    ListPanes(ctx context.Context, session string) ([]PaneInfo, error)
}
```

Unit tests use a fake implementation that records calls and returns canned snapshots.

### 16.3 Mailbox tests

Critical tests:

```go
func TestCreateMessage_AtomicVisibility(t *testing.T)
func TestReceiptUpdate_RequiresLock(t *testing.T)
func TestClaim_IsExclusive(t *testing.T)
func TestDaemonRestart_RequeuesUnread(t *testing.T)
func TestConcurrentSend_ProducesUniqueSeqs(t *testing.T)
```

### 16.4 Fake agent integration harness

Use real tmux and fake agents.

Example fake agent:

```bash
#!/usr/bin/env bash
echo "READY>"
while IFS= read -r line; do
  echo "GOT:$line" >> "$TMUXICATE_TEST_LOG"
  echo "BUSY"
  sleep 0.2
  echo "READY>"
done
```

Integration harness:
- create temp state dir
- create real tmux session
- run `tmuxicate up`
- run `tmuxicate send`
- verify receipt transitions, transcript content, and notification behavior

### 16.5 Critical invariants

Must test:
- canonical message exists before any visible receipt
- no partial envelope or body reads
- sequence numbers are unique under concurrency
- claims are exclusive
- daemon restart loses no unread messages
- retries do not duplicate receipts
- notification is not injected while adapter is strongly busy
- pane metadata maps panes to agents reliably
- transcript pipes are attached after startup

## 17. Distribution

### 17.1 Installation

Source install:

```bash
go install github.com/coyaSONG/tmuxicate/cmd/tmuxicate@latest
```

### 17.2 Homebrew

Planned considerations:
- install single binary
- install shell completions and man page
- hard dependency: `tmux`
- soft dependency: `fzf`
- do not require `jq`

### 17.3 Shell completion

Provide:
- `tmuxicate completion bash`
- `tmuxicate completion zsh`
- `tmuxicate completion fish`

### 17.4 `tmux.conf` snippet

Suggested popup binding:

```tmux
bind-key A display-popup -E -w 80% -h 70% -T 'tmuxicate pick' \
  "TMUXICATE_PICK_TARGET='#{pane_id}' tmuxicate pick --session '#S' --insert send-target"
```

### 17.5 `tmuxicate init`

First-run experience:
- finds repo root or uses cwd
- detects installed CLIs like `codex`, `claude`, `gemini`, `aider`
- writes starter `tmuxicate.yaml`
- adds `.tmuxicate/` to `.gitignore` if missing
- supports `--template minimal|triad`

If no known CLI is found:
- generate a generic template with `adapter: generic`

## 18. v0.1 Scope

### 18.1 In scope

Ship v0.1 with:
- YAML config
- `up`
- `down`
- `serve`
- `send`
- `status`
- `log`
- `inbox`
- `read`
- `reply`
- `next`
- `task accept|wait|block|done`
- file-backed mailbox
- atomic writes
- tmux pane metadata
- raw transcript capture
- generic adapter
- conservative safe notification heuristics

### 18.2 Deferred from v0.1

Cut for faster shipping:
- full automatic reconcile of stale tmux and state mismatch
- first-class persisted thread lifecycle metadata
- runtime add/remove agents
- advanced custom layouts if they slow shipping
- picker popup
- Claude hooks as required functionality
- Codex-specific enhancements as required functionality
- context-window metrics
- automatic restart policy
- Homebrew polish if it blocks core delivery

### 18.3 Build order

1. protocol types and path layout
2. mailbox store with atomic writes
3. config loader and validation
4. tmux client wrapper
5. CLI skeleton
6. generic adapter
7. minimum `up/down/send`
8. transcript capture
9. minimum daemon
10. agent-facing commands
11. status and log
12. optional vendor-specific adapters
13. optional picker and polish

### 18.4 Minimum end-to-end slice

The smallest usable slice:
1. `up` creates panes
2. mailbox writes messages and receipts
3. daemon notices unread receipt
4. daemon injects short notification if safe
5. agent runs `next` or `read`
6. agent replies with `reply`
7. human watches via `status` and `log`

If this works reliably, the product is real.

## 19. Risks & Mitigations

### 19.1 Final review findings

Key design corrections from the final review:
- choose a smaller v0.1 boundary
- do not maintain two sources of truth for threads
- do not make vendor-specific adapters mandatory
- do not over-promise reconcile in v0.1
- ensure cheap current-state files exist, not only event logs

### 19.2 Contradictions resolved

Resolved decisions:
- `send` guarantees durable commit, not guaranteed immediate notify
- threads are derived from message fields in v0.1
- generic adapter is the baseline
- full reconcile is deferred

### 19.3 Riskiest assumption

Single riskiest assumption:
- short notification injection via `tmux send-keys` can be done safely enough

Why risky:
- prompt surfaces vary
- readiness is heuristic
- a mistimed injection can confuse or corrupt an active agent turn

Fallback:
- `delivery.mode=manual`
- daemon stops injecting notifications
- agents and humans explicitly run `tmuxicate inbox` and `tmuxicate next`

### 19.4 Other risks

Risk: tmux or daemon crash
- mitigation: filesystem remains source of truth

Risk: local process impersonation
- mitigation: pane metadata identity checks

Risk: disk-full partial commit
- mitigation: atomic staging and orphan recovery

Risk: over-engineering delays delivery
- mitigation: hard v0.1 cuts and staged roadmap

### 19.5 Decision rule

Whenever implementation choices conflict, prefer:
1. durable filesystem truth
2. explicit operator visibility
3. conservative behavior over clever automation
4. smaller shippable v0.1 over broader speculative scope
