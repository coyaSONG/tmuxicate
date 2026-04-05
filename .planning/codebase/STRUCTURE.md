# Codebase Structure

**Analysis Date:** 2026-04-05

## Directory Layout

```text
tmuxicate/
├── cmd/tmuxicate/        # Go CLI entrypoint
├── internal/             # Non-exported application packages
├── docs/knowledge/       # Architecture decisions, discoveries, and gotchas
├── .github/workflows/    # CI definitions
├── test-agents/          # Test harness scripts for agent simulation
├── bin/                  # Built development binary output
├── .tmuxicate/           # Local runtime session state in working tree
├── tmuxicate.yaml        # Sample/local session config
├── README.md             # User-facing overview and CLI reference
├── DESIGN.md             # Detailed design specification
├── Makefile              # Common build/test/lint targets
└── go.mod                # Module definition
```

## Directory Purposes

**`cmd/tmuxicate`:**
- Purpose: Hold the binary entrypoint only.
- Contains: `main.go` with Cobra command registration and text rendering helpers.
- Key files: `cmd/tmuxicate/main.go`

**`internal/session`:**
- Purpose: Host application use cases and user-visible behaviors.
- Contains: One file per command or workflow such as `up.go`, `send.go`, `read_msg.go`, `reply.go`, `status.go`, `log_view.go`, `pick.go`, and `init_cmd.go`.
- Key files: `internal/session/up.go`, `internal/session/task_cmd.go`, `internal/session/status.go`

**`internal/config`:**
- Purpose: Define config schema and path/default resolution.
- Contains: Config structs and loader/validator logic.
- Key files: `internal/config/config.go`, `internal/config/loader.go`

**`internal/protocol`:**
- Purpose: Define stable mailbox data shapes.
- Contains: Message IDs, thread IDs, envelope schema, receipt schema, and validators.
- Key files: `internal/protocol/envelope.go`, `internal/protocol/receipt.go`, `internal/protocol/validation.go`

**`internal/mailbox`:**
- Purpose: Implement durable filesystem storage.
- Contains: Path constructors and the mailbox store with atomic writes and `flock` locking.
- Key files: `internal/mailbox/store.go`, `internal/mailbox/paths.go`

**`internal/runtime`:**
- Purpose: Run the background daemon.
- Contains: The event loop, fsnotify watch handling, retry scheduling, heartbeat, and observed-state persistence.
- Key files: `internal/runtime/daemon.go`

**`internal/adapter`:**
- Purpose: Translate generic notification events into agent-specific pane interactions.
- Contains: The adapter interface, generic readiness detection, and thin `codex` and `claude-code` specializations.
- Key files: `internal/adapter/adapter.go`, `internal/adapter/generic.go`, `internal/adapter/codex.go`, `internal/adapter/claude_code.go`

**`internal/tmux`:**
- Purpose: Encapsulate all direct `tmux` process interaction.
- Contains: `Client` interface, real shell-backed implementation, and fake implementation for tests.
- Key files: `internal/tmux/client.go`, `internal/tmux/real.go`, `internal/tmux/fake.go`

**`docs/knowledge`:**
- Purpose: Preserve architecture decisions and implementation discoveries that explain current design choices.
- Contains: Decision logs such as `docs/knowledge/decision-2026-03-28-file-based-mailbox.md` and `docs/knowledge/decision-2026-03-28-generic-adapter-v01.md`.
- Key files: `docs/knowledge/INDEX.md`

**Empty internal directories:**
- Purpose: Reserved extension points only in current state.
- Contains: No Go files under `internal/app`, `internal/lock`, `internal/logx`, `internal/pane`, `internal/state`, `internal/testutil`, `internal/transcript`, and `internal/ui`.
- Key files: Not applicable

## Key File Locations

**Entry Points:**
- `cmd/tmuxicate/main.go`: Cobra root command, subcommand wiring, output formatting, and environment fallback helpers.

**Configuration:**
- `go.mod`: Module and dependency declarations.
- `tmuxicate.yaml`: Session configuration example/local config.
- `Makefile`: Build, test, lint, and install commands.
- `.github/workflows/ci.yml`: CI build, test, and lint workflow.

**Core Logic:**
- `internal/session/up.go`: Session bootstrap and tmux pane startup.
- `internal/session/send.go`: Message creation and initial receipt generation.
- `internal/session/read_msg.go`: Ack and unread-to-active transition.
- `internal/session/task_cmd.go`: Declared state events and active-to-done transitions.
- `internal/session/status.go`: Runtime aggregation across receipts, ready files, and tmux panes.
- `internal/runtime/daemon.go`: Background notification delivery.
- `internal/mailbox/store.go`: Filesystem persistence and locking.

**Testing:**
- `internal/config/loader_test.go`: Config resolution tests.
- `internal/mailbox/store_test.go`: Atomicity, integrity, and concurrency tests.
- `internal/runtime/daemon_test.go`: Delivery behavior tests.
- `internal/tmux/fake_test.go`, `internal/tmux/real_test.go`: tmux abstraction tests.
- `test-agents/fake-agent.sh`: Agent test harness script.

## Naming Conventions

**Files:**
- Command/use-case files use lower snake case with verb-oriented names, for example `read_msg.go`, `init_cmd.go`, and `task_cmd.go`.
- Package-internal specializations also use lower snake case, for example `claude_code.go`.
- Tests are colocated with `_test.go` suffix, for example `internal/mailbox/store_test.go`.

**Directories:**
- Go packages use short singular nouns such as `session`, `config`, `protocol`, `mailbox`, `runtime`, `adapter`, and `tmux`.
- The only nested directory under `cmd/` matches the binary name: `cmd/tmuxicate`.

## Organization Rules

**CLI wiring:**
- Add new top-level or nested CLI commands in `cmd/tmuxicate/main.go`.
- Keep argument parsing and output formatting there, and move business logic into `internal/session/`.

**Application logic:**
- Add a new workflow as a new file in `internal/session/` when it represents a distinct user action.
- Keep cross-package orchestration in `internal/session/`, not in `cmd/tmuxicate/main.go` and not in `internal/tmux/`.

**Protocol changes:**
- Add new message or receipt fields in `internal/protocol/` first.
- Update validation in `internal/protocol/validation.go` before changing storage or command behavior.

**Filesystem persistence:**
- Add new session-tree paths in `internal/mailbox/paths.go`.
- Put atomic write, lock, and rename behavior in `internal/mailbox/store.go`, not scattered across session handlers.

**tmux-specific behavior:**
- Add new tmux operations to `internal/tmux/client.go` and implement them in both `internal/tmux/real.go` and `internal/tmux/fake.go`.
- Keep shelling out to `tmux` out of higher-level packages.

**Agent-specific behavior:**
- Add a new agent CLI integration under `internal/adapter/`.
- Prefer extending `internal/adapter/generic.go` via a thin wrapper file rather than embedding agent-specific conditionals across the daemon.

## Where to Add New Code

**New Feature:**
- Primary code: `internal/session/<feature>.go`
- Tests: `internal/session/<feature>_test.go` if added, or adjacent package tests when the feature primarily affects `mailbox`, `runtime`, or `adapter`

**New Component/Module:**
- Mailbox/data model changes: `internal/protocol/` and `internal/mailbox/`
- Runtime delivery changes: `internal/runtime/daemon.go`
- tmux execution changes: `internal/tmux/`
- Agent CLI adapter changes: `internal/adapter/`

**Utilities:**
- Shared helpers should stay inside the most specific existing package that owns the behavior.
- Create a new top-level `internal/<package>` only when the helper set forms a coherent boundary; several placeholder directories already exist but are not yet populated.

## Special Directories

**`.tmuxicate`:**
- Purpose: Session runtime state, message store, receipts, logs, and generated agent artifacts.
- Generated: Yes
- Committed: No, intended to be ignored; `internal/session/init_cmd.go` ensures `.tmuxicate/` is added to `.gitignore`.

**`bin`:**
- Purpose: Local development build output from `make build`.
- Generated: Yes
- Committed: No in normal development flow

**`.planning/codebase`:**
- Purpose: Generated codebase reference documents for higher-level planning workflows.
- Generated: Yes
- Committed: Depends on the planning workflow

---

*Structure analysis: 2026-04-05*
