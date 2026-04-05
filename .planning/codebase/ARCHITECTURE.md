# Architecture

**Analysis Date:** 2026-04-05

## Pattern Overview

**Overall:** Layered Go CLI with filesystem-backed messaging and a thin tmux integration boundary.

**Key Characteristics:**
- `cmd/tmuxicate/main.go` is the single executable entrypoint and wires every subcommand with `cobra`.
- `internal/session/*.go` is the application layer: each file maps closely to one user action such as `up`, `send`, `read`, `reply`, `status`, or `pick`.
- Durable state lives on disk under the session state directory, while `tmux` is used as the operator-facing pane/process layer rather than the message bus.

## Layers

**CLI Layer:**
- Purpose: Parse flags, resolve defaults, print user-facing output, and delegate to internal services.
- Location: `cmd/tmuxicate/main.go`
- Contains: Cobra command constructors such as `newUpCmd`, `newSendCmd`, `newServeCmd`, `newStatusCmd`, and hidden picker helpers.
- Depends on: `internal/config`, `internal/session`, `internal/runtime`, `internal/mailbox`, `internal/protocol`, `internal/tmux`.
- Used by: The compiled binary launched from `./cmd/tmuxicate`.

**Application/Use-Case Layer:**
- Purpose: Implement session lifecycle and mailbox workflows as plain functions.
- Location: `internal/session/up.go`, `internal/session/down.go`, `internal/session/send.go`, `internal/session/read_msg.go`, `internal/session/reply.go`, `internal/session/task_cmd.go`, `internal/session/status.go`, `internal/session/log_view.go`, `internal/session/pick.go`, `internal/session/init_cmd.go`
- Contains: Orchestration logic, file writes for runtime artifacts, state transitions, dashboard aggregation, and picker UX.
- Depends on: `internal/config`, `internal/mailbox`, `internal/protocol`, `internal/tmux`, `internal/runtime`.
- Used by: `cmd/tmuxicate/main.go`.

**Configuration Layer:**
- Purpose: Load YAML config, apply defaults, resolve relative paths, and validate agent/session definitions.
- Location: `internal/config/config.go`, `internal/config/loader.go`
- Contains: `Config`, `ResolvedConfig`, duration parsing, and validation helpers for layouts, adapters, and task kinds.
- Depends on: `internal/protocol` and `gopkg.in/yaml.v3`.
- Used by: CLI commands, session functions, and daemon startup.

**Protocol Layer:**
- Purpose: Define canonical message and receipt schemas that all other packages exchange.
- Location: `internal/protocol/envelope.go`, `internal/protocol/receipt.go`, `internal/protocol/ids.go`, `internal/protocol/validation.go`
- Contains: `Envelope`, `Receipt`, message/thread IDs, folder states, kinds, priorities, and validation rules.
- Depends on: Standard library only.
- Used by: `internal/session`, `internal/mailbox`, and `internal/runtime`.

**Storage Layer:**
- Purpose: Persist immutable messages and mutable per-agent receipts using atomic filesystem operations.
- Location: `internal/mailbox/store.go`, `internal/mailbox/paths.go`
- Contains: Sequence allocation, receipt moves, receipt locking, body hash verification, and path helpers.
- Depends on: `internal/protocol` and `golang.org/x/sys/unix` for `flock`.
- Used by: `internal/session` and `internal/runtime`.

**Runtime/Delivery Layer:**
- Purpose: Watch unread inboxes, probe panes, inject notifications, and publish heartbeat/observed state.
- Location: `internal/runtime/daemon.go`
- Contains: `Daemon`, fsnotify watcher loop, periodic sweep, retry bookkeeping, and JSON event logging.
- Depends on: `internal/adapter`, `internal/config`, `internal/mailbox`, `internal/protocol`, `internal/tmux`, `github.com/fsnotify/fsnotify`.
- Used by: `tmuxicate serve` and background daemon startup in `internal/session/up.go`.

**Integration Boundary:**
- Purpose: Hide agent-specific notification behavior and tmux command execution behind interfaces.
- Location: `internal/adapter/*.go`, `internal/tmux/*.go`
- Contains: `adapter.Adapter`, `tmux.Client`, `GenericAdapter`, `CodexAdapter`, `ClaudeCodeAdapter`, `RealClient`, and `FakeClient`.
- Depends on: Standard library plus internal protocol/session state where needed.
- Used by: `internal/runtime/daemon.go`, `internal/session/up.go`, `internal/session/down.go`, `internal/session/status.go`, `internal/session/pick.go`.

## Data Flow

**Session Startup Flow:**
1. `cmd/tmuxicate/main.go` calls `config.LoadResolved` and then `session.Up`.
2. `internal/session/up.go` creates the on-disk state tree, writes `config.resolved.yaml`, and generates per-agent `adapter/bootstrap.txt` and `adapter/run.sh`.
3. `internal/session/up.go` creates the tmux session through `tmux.Client`, annotates panes with `@tmuxicate-*` options, starts transcript piping, writes `runtime/ready.json`, and launches the background `serve` process.

**Message Creation and Read Flow:**
1. `internal/session/send.go` resolves the target agent from config and allocates a monotonic sequence number via `mailbox.Store.AllocateSeq`.
2. `internal/mailbox/store.go` writes `messages/<msg-id>/envelope.yaml` and `body.md` atomically, then creates one unread receipt in `agents/<agent>/inbox/unread/`.
3. `internal/session/read_msg.go` loads the envelope and receipt, marks unread receipts as acked, and moves them from `unread` to `active`.

**Notification Delivery Flow:**
1. `internal/runtime/daemon.go` watches each unread inbox directory and also performs periodic sweeps.
2. The daemon reads the receipt and envelope through `mailbox.Store`, probes the pane through an `adapter.Adapter`, and only injects a short instruction when the pane is ready.
3. Success updates receipt retry metadata; failure records `next_retry_at` and `last_error` for later retry.

**Task State Flow:**
1. `internal/session/task_cmd.go` appends JSON state events under `agents/<agent>/events/`.
2. Task acceptance keeps the receipt active; task completion clears claim info and moves the receipt to `done`.
3. `internal/session/status.go` and `internal/session/pick.go` combine declared state, observed state, and inbox counts into operator-facing summaries.

## State Management

**Authoritative State:**
- The filesystem under the configured session state directory is authoritative.
- Core paths are built by `internal/mailbox/paths.go`.
- `tmux` pane metadata in `@tmuxicate-*` options is auxiliary and used for discovery/reconciliation, not as the primary source of message truth.

**Execution Model:**
- `tmuxicate up` starts the main tmux session and then spawns a detached background process that runs `tmuxicate serve`; see `internal/session/up.go`.
- `tmuxicate serve` runs a long-lived event loop in `internal/runtime/daemon.go` with three concurrent concerns multiplexed in one select loop: fsnotify events, periodic health/heartbeat ticks, and periodic full sweeps.
- Command handlers are otherwise synchronous and short-lived: each CLI subcommand loads config/state, performs one operation, prints output, and exits.

## Key Abstractions

**ResolvedConfig:**
- Purpose: Freeze config defaults and absolute paths before session logic runs.
- Examples: `internal/config/loader.go`, `internal/session/up.go`, `internal/runtime/daemon.go`
- Pattern: Parse once near the edge, then pass a resolved struct through the call chain.

**Envelope and Receipt:**
- Purpose: Separate immutable message content from per-recipient mutable delivery state.
- Examples: `internal/protocol/envelope.go`, `internal/protocol/receipt.go`, `internal/mailbox/store.go`
- Pattern: One message directory plus one receipt file per recipient/folder state.

**tmux.Client:**
- Purpose: Isolate shelling out to `tmux` from application logic.
- Examples: `internal/tmux/client.go`, `internal/tmux/real.go`, `internal/tmux/fake.go`
- Pattern: Interface-driven infrastructure with a real implementation and an in-memory fake for tests.

**Adapter:**
- Purpose: Encapsulate readiness probing and notification phrasing for each agent CLI.
- Examples: `internal/adapter/adapter.go`, `internal/adapter/generic.go`, `internal/adapter/codex.go`, `internal/adapter/claude_code.go`
- Pattern: Generic adapter core with thin vendor-specific wrappers.

## Entry Points

**Executable Entry Point:**
- Location: `cmd/tmuxicate/main.go`
- Triggers: User invokes `tmuxicate`.
- Responsibilities: Build the root command tree and dispatch subcommands.

**Daemon Entry Point:**
- Location: `cmd/tmuxicate/main.go` via `newServeCmd`, implemented by `internal/runtime/daemon.go`
- Triggers: `tmuxicate serve` or the background process spawned by `internal/session/up.go`
- Responsibilities: Delivery retries, observed-state updates, heartbeat emission, and runtime JSONL logging.

**Session Bootstrap Entry Point:**
- Location: `internal/session/up.go`
- Triggers: `tmuxicate up`
- Responsibilities: Prepare state directories, generate agent bootstrap artifacts, create tmux panes, and start the daemon.

**Operator Read/Inspect Entry Points:**
- Location: `internal/session/status.go`, `internal/session/log_view.go`, `internal/session/pick.go`
- Triggers: `tmuxicate status`, `tmuxicate log`, `tmuxicate pick`, `__list-panes`, `__preview-pane`
- Responsibilities: Aggregate runtime state into dashboards, logs, and popup picker data.

## Error Handling

**Strategy:** Return wrapped errors up the stack and keep command handlers thin.

**Patterns:**
- Packages mostly return `fmt.Errorf("context: %w", err)` rather than defining custom error types.
- Validation happens at boundaries: config in `internal/config/loader.go`, protocol schema in `internal/protocol/validation.go`, and filesystem invariants in `internal/mailbox/store.go`.
- Runtime failures in the daemon are logged to `logs/serve.jsonl` and usually converted into retryable receipt metadata rather than crashing the process.

## Cross-Cutting Concerns

**Logging:** Runtime event logging is append-only JSON Lines in `logs/serve.jsonl` from `internal/runtime/daemon.go`; task state logs are append-only JSON Lines in `agents/<agent>/events/state.jsonl` from `internal/session/task_cmd.go`.
**Validation:** Config validation lives in `internal/config/loader.go`; protocol validation lives in `internal/protocol/validation.go`; message body integrity checks live in `internal/mailbox/store.go`.
**Authentication:** Not detected as a networked auth subsystem. Identity is local and process-based through config agent names, aliases, environment variables such as `TMUXICATE_AGENT`, and pane/session metadata.

---

*Architecture analysis: 2026-04-05*
