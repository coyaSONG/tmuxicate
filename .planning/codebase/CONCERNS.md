# Codebase Concerns

**Analysis Date:** 2026-04-05

## Tech Debt

**Monolithic command and orchestration modules:**
- Issue: CLI composition, lifecycle orchestration, mailbox persistence, and status/log rendering are concentrated in a few large files instead of smaller units with narrow responsibilities.
- Files: `cmd/tmuxicate/main.go`, `internal/session/up.go`, `internal/session/status.go`, `internal/session/log_view.go`, `internal/runtime/daemon.go`, `internal/mailbox/store.go`
- Impact: Small changes require editing large files with mixed concerns, which raises review cost and makes regressions harder to localize.
- Fix approach: Split command wiring from business logic, extract daemon lifecycle and notification policy from `internal/runtime/daemon.go`, and isolate status/log scanners into dedicated packages.

**Dead configuration surface:**
- Issue: Several config knobs are defined, defaulted, and validated, but runtime code does not consume them.
- Files: `internal/config/config.go`, `internal/config/loader.go`, `internal/session/init_cmd.go`, `internal/runtime/daemon.go`
- Impact: Operators can set `delivery.mode`, `delivery.ack_timeout`, `delivery.max_retries`, `delivery.safe_notify_only_when_ready`, `delivery.auto_notify`, `session.attach`, `defaults.bootstrap_template`, `defaults.notify.enabled`, and `transcript.dir` and reasonably expect behavior changes that never occur.
- Fix approach: Either implement each knob in runtime/session code or remove it from the schema and generated config to keep the product surface honest.

## Known Bugs

**Daemon process survives session shutdown:**
- Symptoms: `tmuxicate down` kills the tmux session, but the background daemon has no direct shutdown path and can continue running until manually terminated.
- Files: `internal/session/up.go`, `internal/session/down.go`, `internal/runtime/daemon.go`
- Trigger: Start a session with `tmuxicate up`, then stop it with `tmuxicate down`.
- Workaround: Manually kill the PID recorded in `runtime/daemon.pid` if it still exists.

**Notification retries never exhaust:**
- Symptoms: A message can stay in `unread` forever with repeated `next_retry_at` updates when an adapter is missing or not ready.
- Files: `internal/config/config.go`, `internal/config/loader.go`, `internal/runtime/daemon.go`
- Trigger: Create an unread receipt for an agent whose pane is not ready or whose adapter is absent.
- Workaround: Manually inspect and move the receipt out of `unread`; there is no built-in retry ceiling.

**Manual delivery mode is not enforced:**
- Symptoms: `delivery.mode` accepts `"manual"` in config validation, but the daemon still watches unread folders and injects notifications.
- Files: `internal/config/loader.go`, `internal/runtime/daemon.go`
- Trigger: Configure `delivery.mode: manual` and start the daemon through `tmuxicate up`.
- Workaround: Disable the daemon operationally; there is no runtime branch for manual mode.

## Security Considerations

**Secrets are materialized into plaintext session artifacts:**
- Risk: Values from `defaults.env` are written into `config.resolved.yaml` and exported into per-agent `run.sh` scripts, while transcripts and logs are stored as readable files under the state directory.
- Files: `internal/session/up.go`
- Current mitigation: The generated state lives under `.tmuxicate/` and `init` adds that directory to `.gitignore` in `internal/session/init_cmd.go`.
- Recommendations: Use `0600` permissions for secret-bearing files, stop serializing raw env values to disk, and prefer process-level env inheritance or secret references over embedded exports.

**Shell execution path trusts raw config commands:**
- Risk: `renderRunScript` builds `exec` lines from raw `agent.Command` strings, so quoting and argument safety depend entirely on shell parsing.
- Files: `internal/session/up.go`
- Current mitigation: `run.sh` is generated locally rather than sourced from remote input.
- Recommendations: Change command representation from a shell string to an argv array, escape arguments explicitly, and validate allowed executables before script generation.

## Performance Bottlenecks

**Receipt lookup scales with filesystem scans:**
- Problem: Every `ReadReceipt`, `UpdateReceipt`, and `MoveReceipt` call searches four inbox directories with `filepath.Glob`.
- Files: `internal/mailbox/store.go`
- Cause: The store does not maintain a deterministic receipt index or direct path map.
- Improvement path: Store receipts at stable paths or maintain a lightweight index keyed by `(agent, message_id)` to avoid repeated directory scans.

**Status and log commands rescan the full session state:**
- Problem: `status` walks all message directories and all receipt folders on every invocation, and `log` loads entire transcript/event files before trimming to `--tail`.
- Files: `internal/session/status.go`, `internal/session/log_view.go`
- Cause: Both commands are implemented as full filesystem aggregations without cached summaries or reverse-tail reads.
- Improvement path: Persist compact counters in runtime state, and use seek-from-end logic for transcript tails instead of full-file reads.

## Fragile Areas

**Adapter readiness depends on brittle prompt regexes:**
- Files: `internal/runtime/daemon.go`, `internal/adapter/generic.go`, `internal/adapter/codex.go`, `internal/adapter/claude_code.go`
- Why fragile: Readiness is inferred from hard-coded prompt regexes and quiet periods. Any upstream CLI prompt change can silently stop notifications.
- Safe modification: Treat adapter probing as versioned adapter-specific logic, with captured transcript fixtures for each supported CLI.
- Test coverage: `internal/runtime/daemon_test.go` covers only the happy-path notification case; it does not exercise prompt drift, manual mode, or retry exhaustion.

**Shutdown path is timing-based instead of state-based:**
- Files: `internal/session/down.go`, `internal/runtime/daemon_test.go`, `internal/tmux/real_test.go`
- Why fragile: Graceful shutdown waits a fixed 10 seconds and several tests rely on `time.Sleep`, which makes behavior sensitive to machine speed and timing variance.
- Safe modification: Replace fixed sleeps with explicit acknowledgements or polling on runtime state, then migrate tests to event-driven waits.
- Test coverage: No `*_test.go` files exist under `internal/session`, so `Up`, `Down`, `Status`, `LogView`, `Pick`, and task state transitions have no direct automated coverage.

## Scaling Limits

**Runtime design assumes small agent counts and modest mailbox volume:**
- Current capacity: One daemon process watches one unread directory per agent and performs a full unread sweep every 15 seconds.
- Limit: Large sessions or high message churn increase fsnotify traffic, heartbeat churn, receipt scans, and status/log latency.
- Scaling path: Consolidate runtime state into indexed metadata, batch notification work, and replace directory-wide sweeps with explicit work queues.

## Dependencies at Risk

**`tmux` CLI contract:**
- Risk: Pane listing, pane capture, send-keys, and option lookups rely on specific `tmux` command output and behavior.
- Impact: A tmux version change can break pane parsing or notification injection without a compile-time signal.
- Migration plan: Add compatibility tests against supported tmux versions and isolate format strings/parsing behind narrower helpers in `internal/tmux/real.go`.

## Missing Critical Features

**Ack timeout and retry ceiling enforcement:**
- Problem: `delivery.ack_timeout` and `delivery.max_retries` exist in config but no runtime code enforces them.
- Blocks: Reliable escalation of stuck work, automatic dead-lettering, and truthful operator expectations around delivery guarantees.

**Daemon lifecycle control:**
- Problem: There is no explicit stop/restart/reconcile mechanism for the daemon even though it persists its PID and heartbeat files.
- Blocks: Safe recovery after crashed tmux sessions, deterministic shutdown, and cleanup of orphaned processes.

## Test Coverage Gaps

**Session package is untested:**
- What's not tested: Session startup, shutdown, status aggregation, log viewing, pane picking, message read/reply flows, and task state transitions.
- Files: `internal/session/up.go`, `internal/session/down.go`, `internal/session/status.go`, `internal/session/log_view.go`, `internal/session/pick.go`, `internal/session/read_msg.go`, `internal/session/reply.go`, `internal/session/task_cmd.go`
- Risk: The highest-coupling lifecycle code can regress while `go test ./...` still reports success because `internal/session` has no test files.
- Priority: High

**CLI wiring is untested:**
- What's not tested: Cobra command registration, flag handling, and root command behavior in `cmd/tmuxicate`.
- Files: `cmd/tmuxicate/main.go`
- Risk: User-facing command regressions can slip through because `go test ./...` reports `[no test files]` for `cmd/tmuxicate`.
- Priority: Medium

**Daemon failure scenarios are untested:**
- What's not tested: Retry exhaustion, manual delivery mode, orphaned daemon shutdown, adapter absence, and receipt read failures inside `tryNotify`.
- Files: `internal/runtime/daemon.go`, `internal/runtime/daemon_test.go`
- Risk: Notification failure handling can spin indefinitely or hide errors without a failing test.
- Priority: High

---

*Concerns audit: 2026-04-05*
