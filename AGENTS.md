<!-- GSD:project-start source:PROJECT.md -->
## Project

**tmuxicate**

`tmuxicate` is a Go CLI for running multiple AI coding agents side by side in `tmux` with a durable, file-backed coordination layer. It gives each agent a pane, mailbox, and task workflow so a human operator can watch work happen, intervene when needed, and keep coordination reliable rather than implicit. The next project scope extends that foundation with coordinator-driven automation for task decomposition, routing, review flow, and blocker handling.

**Core Value:** A human can coordinate multiple terminal agents through a reliable, observable workflow where the coordinator keeps work moving without hiding what happened.

### Constraints

- **Tech stack**: Stay within the existing Go CLI architecture and current tmux/mailbox runtime — the new work should extend current packages rather than introduce a second orchestration system
- **Product philosophy**: Reliability and operator visibility come before autonomy — automated behavior must remain inspectable and explicit
- **Compatibility**: Preserve the existing mailbox protocol and multi-vendor adapter model — current Codex/Claude/generic flows must not be broken by coordinator features
- **Operational model**: Human operator remains the final escalation point — coordinator automation should surface blocked or risky situations instead of hiding them
- **Quality**: New orchestration flows need direct test coverage in the currently under-tested session/runtime areas — otherwise automation will amplify regressions
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- Go 1.26.1 - application code, CLI entrypoint, runtime daemon, adapters, mailbox store, and tests in `cmd/tmuxicate/main.go`, `internal/runtime/daemon.go`, `internal/session/*.go`, and `internal/*/*_test.go`.
- Bash - generated agent launcher scripts and test helper shell automation in `internal/session/up.go` and `test-agents/fake-agent.sh`.
- YAML - operator configuration and persisted mailbox/config artifacts in `tmuxicate.yaml`, `internal/config/config.go`, `internal/config/loader.go`, and `internal/mailbox/store.go`.
- JSON - runtime heartbeat, ready-state, observed-state, and log/event payloads in `internal/runtime/daemon.go` and `internal/session/up.go`.
- Markdown - operator docs and message bodies via `README.md`, `DESIGN.md`, and markdown mailbox payloads referenced by `internal/session/send.go`.
## Runtime
- Native Go CLI binary built from `cmd/tmuxicate/main.go`.
- Go toolchain requirement is declared as `go 1.26.1` in `go.mod`.
- Shell execution assumes `bash` for generated `run.sh` scripts in `internal/session/up.go`.
- Go modules via `go.mod` and `go.sum`.
- Lockfile: present in `go.sum`.
## Frameworks
- `github.com/spf13/cobra` v1.10.2 - CLI command tree, flags, and argument parsing in `cmd/tmuxicate/main.go`.
- `github.com/knadh/koanf/v2` v2.3.4 - declared dependency for configuration layering support in `go.mod`; current loader code in `internal/config/loader.go` reads YAML directly and does not invoke Koanf.
- `gopkg.in/yaml.v3` v3.0.1 - config, envelope, and receipt serialization in `internal/config/loader.go`, `internal/mailbox/store.go`, and `internal/session/up.go`.
- `github.com/fsnotify/fsnotify` v1.9.0 - inbox and log file watching in `internal/runtime/daemon.go` and `internal/session/log_view.go`.
- Go `testing` package - unit and integration tests throughout `internal/*/*_test.go`.
- Race detector enabled in `Makefile` and `.github/workflows/ci.yml` via `go test ./... -count=1 -race`.
- `go build` / `go install` - local build and install flows in `Makefile`, `README.md`, and `.github/workflows/ci.yml`.
- `golangci-lint` - lint runner configured in `.golangci.yml` and executed in `Makefile` plus `.github/workflows/ci.yml`.
- `gofumpt` and `goimports` - formatting tools invoked from `Makefile`.
## Key Dependencies
- `github.com/spf13/cobra` v1.10.2 - all user-facing commands are registered in `cmd/tmuxicate/main.go`.
- `github.com/fsnotify/fsnotify` v1.9.0 - delivery daemon and log follower depend on filesystem notifications in `internal/runtime/daemon.go` and `internal/session/log_view.go`.
- `gopkg.in/yaml.v3` v3.0.1 - configuration and mailbox persistence format in `internal/config/loader.go` and `internal/mailbox/store.go`.
- `golang.org/x/sys` v0.42.0 - filesystem locking for receipt/message sequencing in `internal/mailbox/store.go`.
- `github.com/go-viper/mapstructure/v2` v2.4.0 - indirect dependency via `go.mod`.
- `github.com/knadh/koanf/maps` v0.1.2 - indirect dependency via `go.mod`.
- `github.com/mitchellh/copystructure` v1.2.0 and `github.com/mitchellh/reflectwalk` v1.0.2 - indirect dependencies via `go.mod`.
- `github.com/inconshreveable/mousetrap` v1.1.0 and `github.com/spf13/pflag` v1.0.9 - Cobra support dependencies via `go.mod`.
## Configuration
- Primary operator config file is `tmuxicate.yaml`, parsed in `internal/config/loader.go`.
- Resolved session config is written to `config.resolved.yaml` under the session state dir by `internal/session/up.go`.
- Runtime environment variables injected into agent panes are `TMUXICATE_SESSION`, `TMUXICATE_AGENT`, `TMUXICATE_ALIAS`, and `TMUXICATE_STATE_DIR` from `internal/session/up.go`.
- CLI fallback resolution also reads `TMUXICATE_STATE_DIR` and `TMUXICATE_AGENT` in `cmd/tmuxicate/main.go`.
- Picker behavior reads `TMUXICATE_PICK_TARGET` and `TMUX_PANE` in `internal/session/pick.go`.
- Build/test/lint/format tasks live in `Makefile`.
- CI automation lives in `.github/workflows/ci.yml`.
- Linter configuration lives in `.golangci.yml`.
## Notable Tooling
- `tmux` is a hard runtime dependency used through the process-backed client in `internal/tmux/real.go`.
- `fzf` is an optional local dependency for `tmuxicate pick`, validated in `internal/session/pick.go`.
- Agent CLIs are user-supplied commands configured per agent in `tmuxicate.yaml` and auto-detected during `tmuxicate init` in `internal/session/init_cmd.go`.
- Transcript capture relies on `tmux pipe-pane` writing raw ANSI logs to per-agent files created in `internal/session/up.go`.
## Platform Requirements
- Go toolchain compatible with `go.mod`.
- `tmux` available on `PATH` for runtime and integration tests in `internal/tmux/real_test.go`.
- `bash` available for generated launcher scripts in `internal/session/up.go`.
- `golangci-lint`, `gofumpt`, and `goimports` are expected by `Makefile` but their versions are not pinned in-repo.
- No hosted deployment target is defined.
- Runtime target is a local or remote POSIX-like machine with filesystem access, `tmux`, configured agent CLIs, and permission to create the session state tree under `.tmuxicate/` or another configured state dir.
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Naming Patterns
- Use lowercase package directories with short domain names under `internal/`, for example `internal/config`, `internal/mailbox`, `internal/runtime`, and `internal/tmux`.
- Use snake_case filenames for multiword Go files, especially command-oriented session files such as `internal/session/read_msg.go`, `internal/session/init_cmd.go`, and `internal/session/log_view.go`.
- Keep test files co-located and named `*_test.go`, for example `internal/mailbox/store_test.go` and `internal/runtime/daemon_test.go`.
- Exported entry points use PascalCase and map to package responsibilities, such as `config.LoadResolved` in `internal/config/loader.go`, `session.Up` in `internal/session/up.go`, and `runtime.NewDaemon` in `internal/runtime/daemon.go`.
- Internal helpers use lowerCamelCase and are usually narrow, for example `resolveTargetAgent` in `internal/session/send.go`, `createStateTree` in `internal/session/up.go`, and `validateBody` in `internal/mailbox/store.go`.
- Constructors follow `NewX` naming, for example `tmux.NewRealClient` in `internal/tmux/real.go`, `tmux.NewFakeClient` in `internal/tmux/fake.go`, and `adapter.NewGenericAdapter` in `internal/adapter/generic.go`.
- Use explicit domain names over abbreviations. Common examples are `stateDir`, `agentName`, `paneID`, `receipt`, `resolvedStateDir`, and `bootstrapPath` across `internal/session/*.go` and `internal/runtime/daemon.go`.
- Use `cfg` for configuration values and `ctx` for `context.Context`, consistently in `cmd/tmuxicate/main.go`, `internal/session/up.go`, and `internal/tmux/real.go`.
- Data structs are noun-based and package-scoped to the domain, for example `Config` in `internal/config/config.go`, `Envelope` and `Receipt` in `internal/protocol/*.go`, `Daemon` in `internal/runtime/daemon.go`, and `FakeClient` in `internal/tmux/fake.go`.
- Interfaces stay small and capability-oriented. `tmux.Client` in `internal/tmux/client.go` and `adapter.Adapter` in `internal/adapter/adapter.go` are the main examples.
## Code Style
- Format code with `gofumpt` and `goimports` via `make fmt` in `Makefile`.
- Keep imports grouped by standard library first, then internal/external packages as produced by `goimports`; representative files are `cmd/tmuxicate/main.go` and `internal/session/up.go`.
- Favor early returns and guard clauses instead of deep nesting. This is the dominant shape in `internal/config/loader.go`, `internal/tmux/real.go`, and `internal/session/reply.go`.
- Lint with `golangci-lint run ./...` from `Makefile`.
- The enabled rules in `.golangci.yml` enforce practical correctness over stylistic churn: `govet`, `staticcheck`, `errcheck`, `ineffassign`, `unused`, `gocritic`, `misspell`, `revive`, `unconvert`, and `prealloc`.
- `revive` explicitly enforces `context-as-argument`, `error-return`, and `error-naming` in `.golangci.yml`. Follow that pattern when adding new APIs.
## Import Organization
- There are no custom path aliases. Import packages by full Go module path, as in `cmd/tmuxicate/main.go`.
- When package names would collide with common identifiers, use a local alias only where necessary, such as `tmuxruntime` for `internal/runtime` in `cmd/tmuxicate/main.go`.
## Error Handling
- Validate inputs first and return direct errors for missing requirements. Examples: `internal/tmux/real.go`, `internal/session/send.go`, and `internal/session/reply.go`.
- Wrap downstream failures with context using `fmt.Errorf("context: %w", err)`. This is the standard pattern across `internal/config/loader.go`, `internal/mailbox/store.go`, `internal/runtime/daemon.go`, and `internal/session/up.go`.
- Use `errors.New(...)` for package-level sentinel or simple invariant failures, for example `ErrNoUnreadMessages` in `internal/session/next.go`.
- Keep domain validation close to the struct being validated. `(*Envelope).Validate` and `(*Receipt).Validate` in `internal/protocol/validation.go` are the canonical examples.
- Do not silently coerce invalid values except for defaulting in config resolution. Validation failures are explicit and specific.
## Logging
- CLI commands print human-readable output directly with `fmt.Println`, `fmt.Printf`, and tabwriters in `cmd/tmuxicate/main.go`.
- Runtime diagnostics are persisted as JSON/JSONL files instead of going through a shared logger. See `logEvent` in `internal/runtime/daemon.go`, `appendStateEvent` in `internal/session/task_cmd.go`, and status/heartbeat writers in `internal/session/up.go` and `internal/session/down.go`.
- No active shared logging package is used. `internal/logx/` exists as a directory but currently contains no files.
## Comments
- Comments are sparse and used only when the code needs behavioral justification, not narration.
- The main example is the invariant note in `internal/protocol/validation.go` explaining why active receipts may temporarily hold `done_at` before the folder move completes.
- Not applicable. This codebase is Go-only.
- Go doc comments are not broadly used for internal functions. Follow the current style unless a new exported package API needs package-level documentation.
## Function Design
- Keep low-level helpers small and focused, for example `replyKind` in `internal/session/reply.go` and `priorityRank` in `internal/session/inbox.go`.
- Larger orchestration functions are acceptable in boundary packages when they sequence multiple side effects. Examples include `Up` in `internal/session/up.go`, `Status` in `internal/session/status.go`, and `Run` in `internal/runtime/daemon.go`.
- Pass infrastructure dependencies explicitly instead of relying on globals. Examples: `session.Up(cfg, tmuxClient)` in `internal/session/up.go` and `NewDaemon(stateDir, tmuxClient, cfg)` in `internal/runtime/daemon.go`.
- Keep config/state paths explicit. Many session functions take `stateDir` and derive additional dependencies locally, such as `ReadMsg` in `internal/session/read_msg.go` and `TaskDone` in `internal/session/task_cmd.go`.
- Return domain results plus `error` when state is being queried, such as `(*ResolvedConfig, error)` in `internal/config/loader.go`, `(*ReadResult, error)` in `internal/session/read_msg.go`, and `(*StatusReport, error)` in `internal/session/status.go`.
- Return only `error` for command-like mutations unless a stable identifier is produced, such as `Send` and `Reply` returning `protocol.MessageID`.
## Module Design
- Keep most implementation details behind package-local helpers. Export only the package surface needed by the CLI and neighboring layers.
- Boundary split is consistent:
- `cmd/tmuxicate/main.go` owns Cobra command wiring and console formatting.
- `internal/session/*.go` owns user-visible workflows.
- `internal/runtime/daemon.go` owns background delivery behavior.
- `internal/mailbox/*.go` owns durable filesystem state.
- `internal/tmux/*.go` owns tmux process interaction and fakes.
- `internal/protocol/*.go` owns message/receipt schemas and validation.
- Not used. Packages are composed through normal Go files, not re-export aggregators.
## Boundaries And Config Handling
- Treat `internal/config/loader.go` as the single place for config parsing, defaulting, path resolution, and structural validation. New config fields should be added there and to `internal/config/config.go`.
- Persist operational state under `cfg.Session.StateDir` and not the repo root. Session writers consistently use `mailbox.*Dir(...)` helpers from `internal/mailbox/paths.go`.
- Environment reads are narrow and explicit: `TMUXICATE_AGENT` in `internal/session/send.go` and `cmd/tmuxicate/main.go`, `TMUXICATE_STATE_DIR` in `cmd/tmuxicate/main.go`, and picker-related tmux vars in `internal/session/pick.go`.
- External process access is isolated to `internal/tmux/real.go`, CLI detection in `internal/session/init_cmd.go`, and daemon spawning in `internal/session/up.go`. Keep new shelling-out logic behind those boundaries.
## Recurring Patterns
- Filesystem writes are usually followed by validation or atomic-move semantics. `internal/mailbox/store.go` is the model: stage, sync, rename, then sync parent directories.
- JSON files are written pretty-printed with a trailing newline for operator readability in `internal/session/up.go`, `internal/session/down.go`, `internal/session/task_cmd.go`, and `internal/runtime/daemon.go`.
- YAML is the persistence format for durable mailbox/config records. Follow `yaml.Marshal` and `yaml.Unmarshal` usage in `internal/config/loader.go` and `internal/mailbox/store.go`.
- Fake implementations are preferred over mocking frameworks for boundary tests. `internal/tmux/fake.go` is the reference fake.
- No `TODO`, `FIXME`, `HACK`, or `XXX` markers were detected under `cmd/` or `internal/`; new work should either be implemented or filed externally instead of leaving inline debt markers.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## Pattern Overview
- `cmd/tmuxicate/main.go` is the single executable entrypoint and wires every subcommand with `cobra`.
- `internal/session/*.go` is the application layer: each file maps closely to one user action such as `up`, `send`, `read`, `reply`, `status`, or `pick`.
- Durable state lives on disk under the session state directory, while `tmux` is used as the operator-facing pane/process layer rather than the message bus.
## Layers
- Purpose: Parse flags, resolve defaults, print user-facing output, and delegate to internal services.
- Location: `cmd/tmuxicate/main.go`
- Contains: Cobra command constructors such as `newUpCmd`, `newSendCmd`, `newServeCmd`, `newStatusCmd`, and hidden picker helpers.
- Depends on: `internal/config`, `internal/session`, `internal/runtime`, `internal/mailbox`, `internal/protocol`, `internal/tmux`.
- Used by: The compiled binary launched from `./cmd/tmuxicate`.
- Purpose: Implement session lifecycle and mailbox workflows as plain functions.
- Location: `internal/session/up.go`, `internal/session/down.go`, `internal/session/send.go`, `internal/session/read_msg.go`, `internal/session/reply.go`, `internal/session/task_cmd.go`, `internal/session/status.go`, `internal/session/log_view.go`, `internal/session/pick.go`, `internal/session/init_cmd.go`
- Contains: Orchestration logic, file writes for runtime artifacts, state transitions, dashboard aggregation, and picker UX.
- Depends on: `internal/config`, `internal/mailbox`, `internal/protocol`, `internal/tmux`, `internal/runtime`.
- Used by: `cmd/tmuxicate/main.go`.
- Purpose: Load YAML config, apply defaults, resolve relative paths, and validate agent/session definitions.
- Location: `internal/config/config.go`, `internal/config/loader.go`
- Contains: `Config`, `ResolvedConfig`, duration parsing, and validation helpers for layouts, adapters, and task kinds.
- Depends on: `internal/protocol` and `gopkg.in/yaml.v3`.
- Used by: CLI commands, session functions, and daemon startup.
- Purpose: Define canonical message and receipt schemas that all other packages exchange.
- Location: `internal/protocol/envelope.go`, `internal/protocol/receipt.go`, `internal/protocol/ids.go`, `internal/protocol/validation.go`
- Contains: `Envelope`, `Receipt`, message/thread IDs, folder states, kinds, priorities, and validation rules.
- Depends on: Standard library only.
- Used by: `internal/session`, `internal/mailbox`, and `internal/runtime`.
- Purpose: Persist immutable messages and mutable per-agent receipts using atomic filesystem operations.
- Location: `internal/mailbox/store.go`, `internal/mailbox/paths.go`
- Contains: Sequence allocation, receipt moves, receipt locking, body hash verification, and path helpers.
- Depends on: `internal/protocol` and `golang.org/x/sys/unix` for `flock`.
- Used by: `internal/session` and `internal/runtime`.
- Purpose: Watch unread inboxes, probe panes, inject notifications, and publish heartbeat/observed state.
- Location: `internal/runtime/daemon.go`
- Contains: `Daemon`, fsnotify watcher loop, periodic sweep, retry bookkeeping, and JSON event logging.
- Depends on: `internal/adapter`, `internal/config`, `internal/mailbox`, `internal/protocol`, `internal/tmux`, `github.com/fsnotify/fsnotify`.
- Used by: `tmuxicate serve` and background daemon startup in `internal/session/up.go`.
- Purpose: Hide agent-specific notification behavior and tmux command execution behind interfaces.
- Location: `internal/adapter/*.go`, `internal/tmux/*.go`
- Contains: `adapter.Adapter`, `tmux.Client`, `GenericAdapter`, `CodexAdapter`, `ClaudeCodeAdapter`, `RealClient`, and `FakeClient`.
- Depends on: Standard library plus internal protocol/session state where needed.
- Used by: `internal/runtime/daemon.go`, `internal/session/up.go`, `internal/session/down.go`, `internal/session/status.go`, `internal/session/pick.go`.
## Data Flow
## State Management
- The filesystem under the configured session state directory is authoritative.
- Core paths are built by `internal/mailbox/paths.go`.
- `tmux` pane metadata in `@tmuxicate-*` options is auxiliary and used for discovery/reconciliation, not as the primary source of message truth.
- `tmuxicate up` starts the main tmux session and then spawns a detached background process that runs `tmuxicate serve`; see `internal/session/up.go`.
- `tmuxicate serve` runs a long-lived event loop in `internal/runtime/daemon.go` with three concurrent concerns multiplexed in one select loop: fsnotify events, periodic health/heartbeat ticks, and periodic full sweeps.
- Command handlers are otherwise synchronous and short-lived: each CLI subcommand loads config/state, performs one operation, prints output, and exits.
## Key Abstractions
- Purpose: Freeze config defaults and absolute paths before session logic runs.
- Examples: `internal/config/loader.go`, `internal/session/up.go`, `internal/runtime/daemon.go`
- Pattern: Parse once near the edge, then pass a resolved struct through the call chain.
- Purpose: Separate immutable message content from per-recipient mutable delivery state.
- Examples: `internal/protocol/envelope.go`, `internal/protocol/receipt.go`, `internal/mailbox/store.go`
- Pattern: One message directory plus one receipt file per recipient/folder state.
- Purpose: Isolate shelling out to `tmux` from application logic.
- Examples: `internal/tmux/client.go`, `internal/tmux/real.go`, `internal/tmux/fake.go`
- Pattern: Interface-driven infrastructure with a real implementation and an in-memory fake for tests.
- Purpose: Encapsulate readiness probing and notification phrasing for each agent CLI.
- Examples: `internal/adapter/adapter.go`, `internal/adapter/generic.go`, `internal/adapter/codex.go`, `internal/adapter/claude_code.go`
- Pattern: Generic adapter core with thin vendor-specific wrappers.
## Entry Points
- Location: `cmd/tmuxicate/main.go`
- Triggers: User invokes `tmuxicate`.
- Responsibilities: Build the root command tree and dispatch subcommands.
- Location: `cmd/tmuxicate/main.go` via `newServeCmd`, implemented by `internal/runtime/daemon.go`
- Triggers: `tmuxicate serve` or the background process spawned by `internal/session/up.go`
- Responsibilities: Delivery retries, observed-state updates, heartbeat emission, and runtime JSONL logging.
- Location: `internal/session/up.go`
- Triggers: `tmuxicate up`
- Responsibilities: Prepare state directories, generate agent bootstrap artifacts, create tmux panes, and start the daemon.
- Location: `internal/session/status.go`, `internal/session/log_view.go`, `internal/session/pick.go`
- Triggers: `tmuxicate status`, `tmuxicate log`, `tmuxicate pick`, `__list-panes`, `__preview-pane`
- Responsibilities: Aggregate runtime state into dashboards, logs, and popup picker data.
## Error Handling
- Packages mostly return `fmt.Errorf("context: %w", err)` rather than defining custom error types.
- Validation happens at boundaries: config in `internal/config/loader.go`, protocol schema in `internal/protocol/validation.go`, and filesystem invariants in `internal/mailbox/store.go`.
- Runtime failures in the daemon are logged to `logs/serve.jsonl` and usually converted into retryable receipt metadata rather than crashing the process.
## Cross-Cutting Concerns
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, or `.github/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
