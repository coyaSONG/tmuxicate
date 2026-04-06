---
phase: 04
slug: blocker-escalation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-06
---

# Phase 04 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package |
| **Config file** | none — repo-standard commands live in `Makefile` |
| **Quick run command** | `go test ./internal/session -count=1` |
| **Full suite command** | `go test ./... -count=1 -race` |
| **Estimated runtime** | ~20 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/session -count=1`
- **After every plan wave:** Run `go test ./... -count=1 -race`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 20 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 04-01-01 | 01 | 1 | BLOCK-01 | T-04-01 | `task wait` / `task block` validate structured kinds and persist explicit next-action state instead of freeform-only blocker handling | unit | `go test ./internal/session -run 'TestTask(Wait|Block).*Blocker' -count=1` | ✅ | ⬜ pending |
| 04-02-01 | 02 | 1 | BLOCK-02 | T-04-02 | Escalated blocker cases include current owner, reason, and recommended action, and operator resolution is recorded canonically | unit + CLI | `go test ./internal/session -run 'TestBlocker(Resolve|Escalation|RunShow)' -count=1` | ✅ / ❌ W0 | ⬜ pending |
| 04-03-01 | 03 | 2 | BLOCK-03 | T-04-03 | Reroute ceilings stop loops, `watch` / `clarification_request` do not consume reroute budget, and unresolved work stays visible under the source task | unit | `go test ./internal/session -run 'TestBlocker(Reroute|Ceiling|Unresolved)' -count=1` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/tmuxicate/main_test.go` — add Cobra wiring and flag validation for `tmuxicate blocker resolve`
- [ ] `internal/config/loader_test.go` — add validation coverage for `blockers.max_reroutes_default` and task-class overrides
- [ ] `internal/session/task_cmd_test.go` — add unhappy-path coverage for ambiguous blocker escalation, watch/clarify budget semantics, and reroute ceiling exhaustion
- [ ] `internal/session/run_rebuild_test.go` — add blocker artifact linkage and render validation under `run show`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Operator readability of a task-local blocker block under `tmuxicate run show <run-id>` when both blocker and review-derived blocks are present | BLOCK-02 / BLOCK-03 | Formatting quality and scanability are operator-facing concerns beyond pure structural correctness | Create a fixture run with one implementation task, one escalated blocker case, and one review handoff; run `tmuxicate run show <run-id>` and confirm the blocker block appears directly under the source task without introducing a separate blockers section |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 20s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
