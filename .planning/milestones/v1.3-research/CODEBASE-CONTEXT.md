# Code Context

## Files Retrieved
1. `README.md` (lines 1-184), `DESIGN.md` (lines 18-20, 931-941, 1863-1923) - user-facing claims, deferred scope, and explicit risks.
2. `docs/knowledge/INDEX.md` (lines 1-17), `docs/knowledge/decision-2026-03-28-file-based-mailbox.md` (lines 1-33), `docs/knowledge/decision-2026-03-28-generic-adapter-v01.md` (lines 1-30), `docs/knowledge/gotcha-2026-03-28-tmux-send-keys-enter.md` (lines 1-26), `docs/knowledge/gotcha-2026-03-29-fakeclient-race.md` (lines 1-24), `docs/knowledge/pattern-2026-03-29-large-struct-iteration.md` (lines 1-49) - architecture decisions, gotchas, and code conventions.
3. `cmd/tmuxicate/main.go` (lines 27-53, 158-191, 462-498, 998-1368, 1368-1621) - CLI wiring, target/pick/status/log/init/serve wrappers, and stub groups.
4. `cmd/tmuxicate/main_test.go` (lines 22-1045) - CLI coverage is concentrated in `run`, `task done`, and blocker resolve; many other wrappers are untested.
5. `internal/config/config.go` (lines 1-213), `internal/config/loader.go` (lines 1-546), `internal/config/loader_test.go` (lines 13-780) - config schema/defaulting/validation, including execution targets and delivery knobs.
6. `internal/mailbox/store.go` (lines 1-453), `internal/mailbox/coordinator_store.go` (lines 1-475), `internal/mailbox/target_store.go` (lines 1-262), `internal/mailbox/store_test.go` (lines 17-253), `internal/mailbox/coordinator_store_test.go` (lines 11-197) - durable message/receipt storage plus coordinator and target persistence.
7. `internal/session/up.go` (lines 1-505), `internal/session/down.go` (lines 1-96), `internal/session/up_test.go` (lines 15-158) - tmux session lifecycle and bootstrap artifacts.
8. `internal/session/send.go` (lines 1-111), `internal/session/read_msg.go` (lines 1-64), `internal/session/reply.go` (lines 1-90), `internal/session/next.go` (lines 1-28), `internal/session/inbox.go` (lines 1-104) - direct mailbox workflows.
9. `internal/session/run.go` (lines 1-976), `internal/session/run_rebuild.go` (lines 1-815), `internal/session/run_summary.go` (lines 1-310), `internal/session/run_timeline.go` (lines 1-440), `internal/session/run_adaptive.go` (lines 1-329) and representative tests `run_test.go` (lines 19-1331), `run_rebuild_test.go` (lines 16-1050), `run_summary_test.go` (lines 11-500), `run_timeline_test.go` (lines 13-397), `run_adaptive_test.go` (lines 16-285) - coordinator-run routing, reconstruction, summaries, timelines, and adaptive preferences.
10. `internal/session/task_cmd.go` (lines 1-833), `internal/session/blocker_resolve.go` (lines 1-286), `internal/session/review_response.go` (lines 1-108) plus tests `task_cmd_test.go` (lines 14-698), `blocker_resolve_test.go` (lines 11-388), `review_response_test.go` (lines 11-149) - task state machine, blocker escalation, and review handoffs.
11. `internal/session/target.go` (lines 1-437), `internal/session/target_test.go` (lines 13-188) - execution-target status/heartbeat/enable/disable/dispatch; thinly tested.
12. `internal/session/status.go` (lines 1-427), `internal/session/log_view.go` (lines 1-410), `internal/session/pick.go` (lines 1-271), `internal/session/init_cmd.go` (lines 1-228) - operator UX surfaces.
13. `internal/runtime/daemon.go` (lines 1-495), `internal/runtime/daemon_test.go` (lines 19-356), `internal/adapter/adapter.go` (lines 1-29), `internal/adapter/generic.go` (lines 1-154), `internal/adapter/claude_code.go` (lines 1-50), `internal/adapter/codex.go` (lines 1-50), `internal/adapter/factory.go` (lines 1-20), `internal/adapter/generic_test.go` (lines 1-134), `internal/adapter/factory_test.go` (lines 1-173) - notification/probe loop and adapter abstraction.

## Key Code
### Current capabilities
- `mailbox.Store` is the durable source of truth: messages are staged and renamed atomically, sequence allocation is lock-based, receipts move between `unread/active/done/dead`, and reads verify SHA-256 + byte count.
- `send`/`read`/`reply`/`next`/`inbox` are all implemented; `resolveTargetAgent` accepts either name or alias, `ReadMsg` moves unread receipts to active, and `Next` chooses the next unread message by priority/sequence.
- The coordinator flow is mature: `Run` creates a persistent run record and root message, `RouteChildTask`/`AddChildTask` persist child tasks and placements, `run_rebuild.go` reconstructs the graph from disk, and `run_summary.go` / `run_timeline.go` / `run_adaptive.go` derive operator views and adaptive prefs.
- Task state transitions are real, not stubs: `TaskAccept`, `TaskWait`, `TaskBlock`, `TaskDone`, `BlockerResolve`, and `ReviewRespond` all mutate receipts and append JSON state events under `agents/<agent>/events/`.
- `Up` builds the state tree, writes `config.resolved.yaml`, generates `bootstrap.txt` / `run.sh`, starts pane-backed local agents, writes `ready.json`, and launches `serve`; `Down` warns panes, reopens active receipts, and kills the tmux session.
- `runtime.Daemon` only manages pane-backed local agents: it builds adapters, watches unread inboxes, probes readiness, injects notifications, and writes heartbeat/observed-state JSON.
- `session/target.go` introduces a second execution plane for non-pane workers: `target list/status/heartbeat/disable/enable` plus `dispatchNonLocalTask` and `dispatchPendingForTarget`.
- `status.go`, `log_view.go`, `pick.go`, and `init_cmd.go` are operator-facing surfaces built on the same durable files (`ready.json`, `state.current.json`, `observed.current.json`, `serve.jsonl`, transcripts, etc.).

### Underdeveloped areas / obvious gaps
- Several config knobs are parsed/defaulted but not consumed anywhere: `delivery.mode=manual`, `delivery.safe_notify_only_when_ready`, `delivery.auto_notify`, `defaults.notify.enabled`, and `defaults.bootstrap_template` are present in config but not wired into runtime/startup behavior.
- `ThreadStats.Closed` is printed in `status` but never incremented; the design explicitly defers first-class thread lifecycle metadata, so this is currently a placeholder.
- Bare `task`, `blocker`, and `review` group commands still use `stubRun`:

```go
func stubRun(_ *cobra.Command, _ []string) {
	fmt.Println("not implemented yet")
}
```

- `README.md` still says `pick` is planned/stubbed, but the code now ships `session.Pick` and the hidden `__list-panes` / `__preview-pane` helpers; `run` and `target` are also missing from the docs.
- CLI coverage is heavily skewed toward run/task/blocker/review flows; `status`, `log`, `init`, `pick`, `target`, `down`, and `serve` have little or no direct command-level coverage.
- `internal/mailbox/target_store.go` is the weakest persistence layer in the repo: it has no tests and uses plain writes instead of the mailbox store’s lock + atomic-rename discipline.

### Risky / valuable opportunities
- Wire the delivery/notification config knobs into `runtime.Daemon.tryNotify` and startup behavior so manual delivery and safe-notify flags actually matter.
- Consolidate adapter behavior: `internal/adapter/*` has a factory/wrapper layer, but production runtime/startup still hardcode vendor-specific readiness/startup logic separately.
- Harden the execution-target subsystem: `TargetHeartbeat` / `EnableTarget` / `dispatchNonLocalTask` execute shell commands from config and persist target state with plain writes; high leverage, but the durability story is the weakest part of the system.
- Decide whether to implement or remove the placeholder thread-closed notion; right now it is a dashboard field with no producer.

### Functions to inspect next
- `internal/session/target.go`: `ListTargetStatuses`, `TargetHeartbeat`, `DisableTarget`, `EnableTarget`, `dispatchPendingForTarget`, `dispatchNonLocalTask`, `effectiveTargetAvailability`.
- `internal/mailbox/target_store.go`: `UpsertTargetState`, `RecordTargetHeartbeat`, `WriteTargetDispatch`, `ListTargetDispatches`.
- `internal/runtime/daemon.go`: `buildAdapters`, `tryNotify`, `runHealthCheck`, `writeObservedState`, `logEvent`.
- `internal/config/loader.go` + `internal/session/up.go`: where the unused delivery/bootstrap config knobs would need to be wired.
- `cmd/tmuxicate/main.go`: `newTargetCmd`, `newPickCmd`, `stubRun`, and the `serve` / `status` / `log` / `init` wrappers.

## Architecture
- `config.LoadResolved` is the edge of truth for config: it applies defaults, resolves relative paths, validates the session/agents/targets, and produces `config.resolved.yaml` for all later commands.
- `tmuxicate up` and `tmuxicate serve` form the lifecycle pair. `up` creates the durable state tree and pane-backed local agents; `serve` consumes the resolved config from the state dir and runs the unread-notification daemon.
- The mailbox remains authoritative. `send`, `reply`, `task`, `review`, and `blocker` workflows all mutate message/receipt files on disk, while `status`, `log`, and `run show` are derived views over those files plus state-event logs.
- There are now two execution planes:
  - pane-backed local agents, managed by tmux + daemon notifications;
  - non-pane execution targets, managed by `session/target.go` and `mailbox/target_store.go`.
- `run`/`run show` are the coordinator-oriented orchestration layer on top of the mailbox. They persist run/task/review/blocker/replan artifacts, then reconstruct them for operator output.
- The docs are partly ahead of and partly behind the code: the design still emphasizes v0.1 deferrals (reconcile, runtime add/remove agent, thread lifecycle, picker popup), while the CLI already exposes newer `run` and `target` surfaces that are not described in the README.
- Working tree note: there are pre-existing uncommitted edits in `cmd/tmuxicate/main.go`, `cmd/tmuxicate/main_test.go`, `internal/mailbox/target_store.go`, `internal/session/target.go`, `internal/session/target_test.go`, and `internal/session/up.go`; inspect diffs before touching them.

## Start Here
`internal/session/target.go` — it is the best first stop for the under-documented execution-target subsystem, which is currently the highest-value / least-tested area and the clearest next development surface.