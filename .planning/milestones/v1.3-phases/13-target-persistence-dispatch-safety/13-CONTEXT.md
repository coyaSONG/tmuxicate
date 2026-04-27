# Phase 13: Target Persistence & Dispatch Safety - Context

**Gathered:** 2026-04-27
**Status:** In Progress

## Phase Boundary

Make target health and dispatch persistence trustworthy enough for non-pane execution recovery. This phase hardens the v1.2 target runtime model without adding new remote transports, worker bootstrap, worktree automation, or cloud/vendor integrations.

## Decisions

- Target state and dispatch records should follow the same reliability philosophy as mailbox records: lock, stage, atomic rename, and keep on-disk artifacts human-readable.
- Dispatch must become intent-first: persist a durable pending/running intent for the target/message pair before executing an external command.
- Redispatch must be bounded and idempotent by default; a successful dispatch record should not run the external command again unless a future operator command explicitly forces it.
- Dispatch failure should degrade target state and remain inspectable, but it must not erase canonical run, task, message, or routing artifacts.
- Existing local pane-backed workflows must remain unchanged.

## Specific Ideas

- Add target-scoped locks under the existing state tree, likely near `locks/targets/` or the target runtime directory.
- Add atomic JSON write helpers for target state and target dispatch records, mirroring mailbox store write discipline where practical.
- Add tests for concurrent heartbeat, enable/disable, dispatch writes, list ordering, malformed/partial state behavior where feasible, and redispatch idempotency.
- Extend `TargetDispatchRecord` with stable idempotency/attempt fields only if needed by the implementation plan.
- Ensure `EnableTarget` redispatches only records that are safe to run again.

## Deferred Ideas

- Authenticated remote heartbeat or target identity.
- Non-shell dispatch transports or argv-based command representation.
- Dispatch timeout/process-tree cancellation.
- Worktree-per-task isolation and branch lifecycle.
- Cross-run target rebalancing or attention dashboard.

## Files To Inspect First

- `internal/mailbox/target_store.go`
- `internal/session/target.go`
- `internal/session/target_test.go`
- `internal/session/run.go`
- `internal/session/up.go`
- `internal/mailbox/store.go`
- `internal/mailbox/store_test.go`
- `internal/mailbox/paths.go`

## Verification Strategy

- `go test ./internal/mailbox -count=1 -race`
- `go test ./internal/session -count=1 -race`
- `go test ./... -count=1 -race`
- Manual diff review to confirm no unrelated WIP files were overwritten.
