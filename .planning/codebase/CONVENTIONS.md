# Coding Conventions

**Analysis Date:** 2026-04-05

## Naming Patterns

**Files:**
- Use lowercase package directories with short domain names under `internal/`, for example `internal/config`, `internal/mailbox`, `internal/runtime`, and `internal/tmux`.
- Use snake_case filenames for multiword Go files, especially command-oriented session files such as `internal/session/read_msg.go`, `internal/session/init_cmd.go`, and `internal/session/log_view.go`.
- Keep test files co-located and named `*_test.go`, for example `internal/mailbox/store_test.go` and `internal/runtime/daemon_test.go`.

**Functions:**
- Exported entry points use PascalCase and map to package responsibilities, such as `config.LoadResolved` in `internal/config/loader.go`, `session.Up` in `internal/session/up.go`, and `runtime.NewDaemon` in `internal/runtime/daemon.go`.
- Internal helpers use lowerCamelCase and are usually narrow, for example `resolveTargetAgent` in `internal/session/send.go`, `createStateTree` in `internal/session/up.go`, and `validateBody` in `internal/mailbox/store.go`.
- Constructors follow `NewX` naming, for example `tmux.NewRealClient` in `internal/tmux/real.go`, `tmux.NewFakeClient` in `internal/tmux/fake.go`, and `adapter.NewGenericAdapter` in `internal/adapter/generic.go`.

**Variables:**
- Use explicit domain names over abbreviations. Common examples are `stateDir`, `agentName`, `paneID`, `receipt`, `resolvedStateDir`, and `bootstrapPath` across `internal/session/*.go` and `internal/runtime/daemon.go`.
- Use `cfg` for configuration values and `ctx` for `context.Context`, consistently in `cmd/tmuxicate/main.go`, `internal/session/up.go`, and `internal/tmux/real.go`.

**Types:**
- Data structs are noun-based and package-scoped to the domain, for example `Config` in `internal/config/config.go`, `Envelope` and `Receipt` in `internal/protocol/*.go`, `Daemon` in `internal/runtime/daemon.go`, and `FakeClient` in `internal/tmux/fake.go`.
- Interfaces stay small and capability-oriented. `tmux.Client` in `internal/tmux/client.go` and `adapter.Adapter` in `internal/adapter/adapter.go` are the main examples.

## Code Style

**Formatting:**
- Format code with `gofumpt` and `goimports` via `make fmt` in `Makefile`.
- Keep imports grouped by standard library first, then internal/external packages as produced by `goimports`; representative files are `cmd/tmuxicate/main.go` and `internal/session/up.go`.
- Favor early returns and guard clauses instead of deep nesting. This is the dominant shape in `internal/config/loader.go`, `internal/tmux/real.go`, and `internal/session/reply.go`.

**Linting:**
- Lint with `golangci-lint run ./...` from `Makefile`.
- The enabled rules in `.golangci.yml` enforce practical correctness over stylistic churn: `govet`, `staticcheck`, `errcheck`, `ineffassign`, `unused`, `gocritic`, `misspell`, `revive`, `unconvert`, and `prealloc`.
- `revive` explicitly enforces `context-as-argument`, `error-return`, and `error-naming` in `.golangci.yml`. Follow that pattern when adding new APIs.

## Import Organization

**Order:**
1. Standard library imports.
2. Internal module imports under `github.com/coyaSONG/tmuxicate/internal/...`.
3. External libraries such as `github.com/spf13/cobra`, `github.com/fsnotify/fsnotify`, and `gopkg.in/yaml.v3`.

**Path Aliases:**
- There are no custom path aliases. Import packages by full Go module path, as in `cmd/tmuxicate/main.go`.
- When package names would collide with common identifiers, use a local alias only where necessary, such as `tmuxruntime` for `internal/runtime` in `cmd/tmuxicate/main.go`.

## Error Handling

**Patterns:**
- Validate inputs first and return direct errors for missing requirements. Examples: `internal/tmux/real.go`, `internal/session/send.go`, and `internal/session/reply.go`.
- Wrap downstream failures with context using `fmt.Errorf("context: %w", err)`. This is the standard pattern across `internal/config/loader.go`, `internal/mailbox/store.go`, `internal/runtime/daemon.go`, and `internal/session/up.go`.
- Use `errors.New(...)` for package-level sentinel or simple invariant failures, for example `ErrNoUnreadMessages` in `internal/session/next.go`.
- Keep domain validation close to the struct being validated. `(*Envelope).Validate` and `(*Receipt).Validate` in `internal/protocol/validation.go` are the canonical examples.
- Do not silently coerce invalid values except for defaulting in config resolution. Validation failures are explicit and specific.

## Logging

**Framework:** `fmt` for user-facing CLI output plus JSON line/event files on disk

**Patterns:**
- CLI commands print human-readable output directly with `fmt.Println`, `fmt.Printf`, and tabwriters in `cmd/tmuxicate/main.go`.
- Runtime diagnostics are persisted as JSON/JSONL files instead of going through a shared logger. See `logEvent` in `internal/runtime/daemon.go`, `appendStateEvent` in `internal/session/task_cmd.go`, and status/heartbeat writers in `internal/session/up.go` and `internal/session/down.go`.
- No active shared logging package is used. `internal/logx/` exists as a directory but currently contains no files.

## Comments

**When to Comment:**
- Comments are sparse and used only when the code needs behavioral justification, not narration.
- The main example is the invariant note in `internal/protocol/validation.go` explaining why active receipts may temporarily hold `done_at` before the folder move completes.

**JSDoc/TSDoc:**
- Not applicable. This codebase is Go-only.
- Go doc comments are not broadly used for internal functions. Follow the current style unless a new exported package API needs package-level documentation.

## Function Design

**Size:**
- Keep low-level helpers small and focused, for example `replyKind` in `internal/session/reply.go` and `priorityRank` in `internal/session/inbox.go`.
- Larger orchestration functions are acceptable in boundary packages when they sequence multiple side effects. Examples include `Up` in `internal/session/up.go`, `Status` in `internal/session/status.go`, and `Run` in `internal/runtime/daemon.go`.

**Parameters:**
- Pass infrastructure dependencies explicitly instead of relying on globals. Examples: `session.Up(cfg, tmuxClient)` in `internal/session/up.go` and `NewDaemon(stateDir, tmuxClient, cfg)` in `internal/runtime/daemon.go`.
- Keep config/state paths explicit. Many session functions take `stateDir` and derive additional dependencies locally, such as `ReadMsg` in `internal/session/read_msg.go` and `TaskDone` in `internal/session/task_cmd.go`.

**Return Values:**
- Return domain results plus `error` when state is being queried, such as `(*ResolvedConfig, error)` in `internal/config/loader.go`, `(*ReadResult, error)` in `internal/session/read_msg.go`, and `(*StatusReport, error)` in `internal/session/status.go`.
- Return only `error` for command-like mutations unless a stable identifier is produced, such as `Send` and `Reply` returning `protocol.MessageID`.

## Module Design

**Exports:**
- Keep most implementation details behind package-local helpers. Export only the package surface needed by the CLI and neighboring layers.
- Boundary split is consistent:
- `cmd/tmuxicate/main.go` owns Cobra command wiring and console formatting.
- `internal/session/*.go` owns user-visible workflows.
- `internal/runtime/daemon.go` owns background delivery behavior.
- `internal/mailbox/*.go` owns durable filesystem state.
- `internal/tmux/*.go` owns tmux process interaction and fakes.
- `internal/protocol/*.go` owns message/receipt schemas and validation.

**Barrel Files:**
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

---

*Convention analysis: 2026-04-05*
