# External Integrations

**Analysis Date:** 2026-04-05

## APIs & External Services

**Terminal / Local Process Integrations:**
- `tmux` - primary control-plane integration for pane lifecycle, metadata, transcript piping, popup UI, and key injection.
  - SDK/Client: internal wrapper in `internal/tmux/real.go` using `os/exec` to invoke the `tmux` binary.
  - Auth: none; access is through the local shell environment and the active `tmux` server.
- Agent CLIs (`codex`, `claude`, `gemini`, `aider`, or any configured command) - launched as child processes per agent from `internal/session/up.go` and discovered during config generation in `internal/session/init_cmd.go`.
  - SDK/Client: shell execution via generated `run.sh` in `internal/session/up.go`.
  - Auth: inherited from the user environment and each CLI's own local configuration; this repo does not manage those credentials.
- `fzf` - optional picker dependency used by `tmuxicate pick` in `internal/session/pick.go`.
  - SDK/Client: `exec.LookPath("fzf")` plus shell pipeline execution in `internal/session/pick.go`.
  - Auth: none.

**Filesystem Integrations:**
- Local filesystem mailbox - durable message, receipt, lock, transcript, and runtime state store under the configured session state dir.
  - SDK/Client: internal store in `internal/mailbox/store.go`.
  - Auth: filesystem permissions only.
- Filesystem watch events - unread inbox delivery and log follow mode depend on inotify/FSEvents-style notifications through `github.com/fsnotify/fsnotify` in `internal/runtime/daemon.go` and `internal/session/log_view.go`.

## Data Storage

**Databases:**
- Not detected.
  - Connection: not applicable.
  - Client: not applicable.

**File Storage:**
- Local filesystem only.
  - Session root is configured by `session.state_dir` in `tmuxicate.yaml` and resolved by `internal/config/loader.go`.
  - Messages, receipts, transcripts, logs, and runtime files are created by `internal/session/up.go` and `internal/mailbox/store.go`.

**Caching:**
- None detected.

## Authentication & Identity

**Auth Provider:**
- No external auth provider is integrated.
  - Implementation: operator identity is implicit in the local shell and configured agent commands; message sender identity falls back to `TMUXICATE_AGENT` or `human` in `internal/session/send.go`.

## Monitoring & Observability

**Error Tracking:**
- No third-party error tracking service is integrated.

**Logs:**
- Daemon structured JSONL logs are appended to `logs/serve.jsonl` by `internal/runtime/daemon.go`.
- Runtime heartbeat is written to `runtime/daemon.heartbeat.json` by `internal/runtime/daemon.go`.
- Pane-to-agent mapping is written to `runtime/ready.json` by `internal/session/up.go`.
- Per-agent observed readiness is written to `agents/<agent>/events/observed.current.json` by `internal/runtime/daemon.go`.
- Raw pane transcripts are captured via `tmux pipe-pane` into `agents/<agent>/transcripts/raw.ansi.log` by `internal/session/up.go`.

## CI/CD & Deployment

**Hosting:**
- Not applicable; the repo produces a local CLI binary rather than a deployed service.

**CI Pipeline:**
- GitHub Actions workflow in `.github/workflows/ci.yml`.
  - Build job runs `go build ./...`.
  - Test job runs `go test ./... -count=1 -race`.
  - Lint job runs `golangci/golangci-lint-action@v7` with version `v2.11.4`.

## Environment Configuration

**Required env vars:**
- `TMUXICATE_SESSION` - exported into agent panes by `internal/session/up.go` and represented in `tmuxicate.yaml`.
- `TMUXICATE_AGENT` - exported into agent panes by `internal/session/up.go`; also used by `internal/session/send.go` and `cmd/tmuxicate/main.go`.
- `TMUXICATE_ALIAS` - exported into agent panes by `internal/session/up.go`.
- `TMUXICATE_STATE_DIR` - exported into agent panes by `internal/session/up.go`; used as CLI fallback in `cmd/tmuxicate/main.go`.
- `TMUXICATE_PICK_TARGET` - optional picker target override in `internal/session/pick.go`.
- `TMUX_PANE` - consumed by `internal/session/pick.go` when picker target is not explicitly provided.

**Secrets location:**
- No repo-managed secret store is present.
- Credentials for agent CLIs are expected to live in each CLI's standard local config outside this repository.
- No `.env` files were detected at the repo root during this pass.

## Webhooks & Callbacks

**Incoming:**
- None. The system is driven by CLI commands, local filesystem writes, and `fsnotify` events in `internal/runtime/daemon.go`.

**Outgoing:**
- None over HTTP.
- Outgoing control signals are local `tmux send-keys`, `tmux pipe-pane`, popup invocations, and child-process launches from `internal/tmux/real.go`, `internal/session/up.go`, and `internal/session/pick.go`.

## Integration-Specific Config Touchpoints

- Agent adapter type and executable command are configured per agent in `tmuxicate.yaml` and validated in `internal/config/loader.go`.
- `tmuxicate init` detects supported local CLIs with `exec.LookPath` in `internal/session/init_cmd.go`.
- Adapter-specific invocation flags are embedded in generated `run.sh` scripts in `internal/session/up.go`:
  - `codex` uses `--no-alt-screen` and passes bootstrap text as an argument.
  - `claude-code` uses `--append-system-prompt` and `-n <agent@session>`.
  - `generic` executes the configured command without extra flags.
- Delivery readiness heuristics for `codex` and `claude-code` are configured in `internal/runtime/daemon.go` using adapter-specific prompt regexes.

---

*Integration audit: 2026-04-05*
