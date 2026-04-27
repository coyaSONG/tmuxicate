# Requirements: tmuxicate

**Defined:** 2026-04-27
**Milestone:** v1.3 Runtime Trust & Honest Controls
**Status:** ACTIVE
**Core Value:** A human can coordinate multiple terminal agents through a reliable, observable workflow where the coordinator keeps work moving without hiding what happened.

## v1.3 Requirements

### Target Durability

- [x] **TRUST-01**: Target health and dispatch artifacts are persisted with mailbox-grade durability so concurrent heartbeat, enable/disable, and dispatch updates cannot leave truncated or lost state.
- [ ] **TRUST-02**: Non-pane dispatch records a durable intent before external command execution and uses stable idempotency keys so target recovery cannot silently duplicate work.

### Delivery Policy

- [ ] **DELIVERY-01**: Runtime notification behavior honors configured delivery policy, including manual mode, auto-notify disablement, and safe-notify readiness checks.
- [ ] **DELIVERY-02**: Notification retry ceilings and timeout semantics are explicit, tested, and operator-visible instead of allowing unread receipts to retry forever without escalation.

### Daemon Lifecycle

- [ ] **DAEMON-01**: Session startup, serving, and shutdown own the daemon lifecycle end to end, including stale PID detection, duplicate-daemon prevention, and deterministic stop on `tmuxicate down`.
- [ ] **DAEMON-02**: Operator status surfaces distinguish healthy, stopped, stale, and duplicate daemon states with actionable recovery guidance.

### Artifact Safety and Product Surface

- [ ] **SURFACE-01**: Secret-bearing and operator-sensitive artifacts use intentional permissions, validated environment keys, and redacted command output where dispatch or startup artifacts could expose credentials.
- [ ] **SURFACE-02**: User-facing documentation and command UX match shipped behavior, including `run`, `target`, implemented `pick`, and helpful parent command output for `task`, `blocker`, and `review`.

## Deferred Requirements

- **DEFER-01**: Per-task or per-agent git worktree isolation with branch lifecycle, dirty-state checks, and diff summaries.
- **DEFER-02**: Authenticated remote worker bootstrap or non-shell transport beyond local command dispatch.
- **DEFER-03**: Cross-run attention and rebalancing dashboards across multiple coordinator runs or teams.
- **DEFER-04**: Nested teams or multiple coordinators within one durable workflow graph.

## Scope Boundaries

- Do not introduce a second orchestration backend; preserve the mailbox and coordinator artifacts as the source of truth.
- Do not add cloud/vendor-specific remote runners in this milestone.
- Do not implement automatic git cleanup, push, merge, or destructive workspace operations.
- Do not weaken current local pane-backed workflows while hardening remote-target foundations.
- Treat existing uncommitted code changes as user work in progress and preserve them unless explicitly asked otherwise.

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| TRUST-01 | Phase 13 | Complete |
| TRUST-02 | Phase 13 | Planned |
| DELIVERY-01 | Phase 14 | Planned |
| DELIVERY-02 | Phase 14 | Planned |
| DAEMON-01 | Phase 14 | Planned |
| DAEMON-02 | Phase 14 | Planned |
| SURFACE-01 | Phase 15 | Planned |
| SURFACE-02 | Phase 15 | Planned |

**Coverage:**
- v1.3 requirements: 8 total
- Completed: 1
- Planned: 7
- Deferred to later milestones: 4

---
*Requirements defined: 2026-04-27 after next-development research and risk review.*
