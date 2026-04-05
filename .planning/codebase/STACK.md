# Technology Stack

**Analysis Date:** 2026-04-05

## Languages

**Primary:**
- Go 1.26.1 - application code, CLI entrypoint, runtime daemon, adapters, mailbox store, and tests in `cmd/tmuxicate/main.go`, `internal/runtime/daemon.go`, `internal/session/*.go`, and `internal/*/*_test.go`.

**Secondary:**
- Bash - generated agent launcher scripts and test helper shell automation in `internal/session/up.go` and `test-agents/fake-agent.sh`.
- YAML - operator configuration and persisted mailbox/config artifacts in `tmuxicate.yaml`, `internal/config/config.go`, `internal/config/loader.go`, and `internal/mailbox/store.go`.
- JSON - runtime heartbeat, ready-state, observed-state, and log/event payloads in `internal/runtime/daemon.go` and `internal/session/up.go`.
- Markdown - operator docs and message bodies via `README.md`, `DESIGN.md`, and markdown mailbox payloads referenced by `internal/session/send.go`.

## Runtime

**Environment:**
- Native Go CLI binary built from `cmd/tmuxicate/main.go`.
- Go toolchain requirement is declared as `go 1.26.1` in `go.mod`.
- Shell execution assumes `bash` for generated `run.sh` scripts in `internal/session/up.go`.

**Package Manager:**
- Go modules via `go.mod` and `go.sum`.
- Lockfile: present in `go.sum`.

## Frameworks

**Core:**
- `github.com/spf13/cobra` v1.10.2 - CLI command tree, flags, and argument parsing in `cmd/tmuxicate/main.go`.
- `github.com/knadh/koanf/v2` v2.3.4 - declared dependency for configuration layering support in `go.mod`; current loader code in `internal/config/loader.go` reads YAML directly and does not invoke Koanf.
- `gopkg.in/yaml.v3` v3.0.1 - config, envelope, and receipt serialization in `internal/config/loader.go`, `internal/mailbox/store.go`, and `internal/session/up.go`.
- `github.com/fsnotify/fsnotify` v1.9.0 - inbox and log file watching in `internal/runtime/daemon.go` and `internal/session/log_view.go`.

**Testing:**
- Go `testing` package - unit and integration tests throughout `internal/*/*_test.go`.
- Race detector enabled in `Makefile` and `.github/workflows/ci.yml` via `go test ./... -count=1 -race`.

**Build/Dev:**
- `go build` / `go install` - local build and install flows in `Makefile`, `README.md`, and `.github/workflows/ci.yml`.
- `golangci-lint` - lint runner configured in `.golangci.yml` and executed in `Makefile` plus `.github/workflows/ci.yml`.
- `gofumpt` and `goimports` - formatting tools invoked from `Makefile`.

## Key Dependencies

**Critical:**
- `github.com/spf13/cobra` v1.10.2 - all user-facing commands are registered in `cmd/tmuxicate/main.go`.
- `github.com/fsnotify/fsnotify` v1.9.0 - delivery daemon and log follower depend on filesystem notifications in `internal/runtime/daemon.go` and `internal/session/log_view.go`.
- `gopkg.in/yaml.v3` v3.0.1 - configuration and mailbox persistence format in `internal/config/loader.go` and `internal/mailbox/store.go`.
- `golang.org/x/sys` v0.42.0 - filesystem locking for receipt/message sequencing in `internal/mailbox/store.go`.

**Infrastructure:**
- `github.com/go-viper/mapstructure/v2` v2.4.0 - indirect dependency via `go.mod`.
- `github.com/knadh/koanf/maps` v0.1.2 - indirect dependency via `go.mod`.
- `github.com/mitchellh/copystructure` v1.2.0 and `github.com/mitchellh/reflectwalk` v1.0.2 - indirect dependencies via `go.mod`.
- `github.com/inconshreveable/mousetrap` v1.1.0 and `github.com/spf13/pflag` v1.0.9 - Cobra support dependencies via `go.mod`.

## Configuration

**Environment:**
- Primary operator config file is `tmuxicate.yaml`, parsed in `internal/config/loader.go`.
- Resolved session config is written to `config.resolved.yaml` under the session state dir by `internal/session/up.go`.
- Runtime environment variables injected into agent panes are `TMUXICATE_SESSION`, `TMUXICATE_AGENT`, `TMUXICATE_ALIAS`, and `TMUXICATE_STATE_DIR` from `internal/session/up.go`.
- CLI fallback resolution also reads `TMUXICATE_STATE_DIR` and `TMUXICATE_AGENT` in `cmd/tmuxicate/main.go`.
- Picker behavior reads `TMUXICATE_PICK_TARGET` and `TMUX_PANE` in `internal/session/pick.go`.

**Build:**
- Build/test/lint/format tasks live in `Makefile`.
- CI automation lives in `.github/workflows/ci.yml`.
- Linter configuration lives in `.golangci.yml`.

## Notable Tooling

- `tmux` is a hard runtime dependency used through the process-backed client in `internal/tmux/real.go`.
- `fzf` is an optional local dependency for `tmuxicate pick`, validated in `internal/session/pick.go`.
- Agent CLIs are user-supplied commands configured per agent in `tmuxicate.yaml` and auto-detected during `tmuxicate init` in `internal/session/init_cmd.go`.
- Transcript capture relies on `tmux pipe-pane` writing raw ANSI logs to per-agent files created in `internal/session/up.go`.

## Platform Requirements

**Development:**
- Go toolchain compatible with `go.mod`.
- `tmux` available on `PATH` for runtime and integration tests in `internal/tmux/real_test.go`.
- `bash` available for generated launcher scripts in `internal/session/up.go`.
- `golangci-lint`, `gofumpt`, and `goimports` are expected by `Makefile` but their versions are not pinned in-repo.

**Production:**
- No hosted deployment target is defined.
- Runtime target is a local or remote POSIX-like machine with filesystem access, `tmux`, configured agent CLIs, and permission to create the session state tree under `.tmuxicate/` or another configured state dir.

---

*Stack analysis: 2026-04-05*
