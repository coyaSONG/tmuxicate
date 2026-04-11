---
phase: 02
slug: role-based-routing
status: ready
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-05
---

# Phase 02 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` |
| **Config file** | none — repository commands are centralized in `Makefile` and `.github/workflows/ci.yml` |
| **Quick run command** | `go test ./internal/config ./internal/session ./internal/protocol -count=1` |
| **Full suite command** | `go test ./... -count=1 -race` |
| **Estimated runtime** | ~90 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/config ./internal/session ./internal/protocol -count=1`
- **After every plan wave:** Run `go test ./... -count=1 -race`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | ROUTE-01 | T-02-01 / T-02-02 | Structured config and route-task contract parse deterministic routing inputs instead of freeform role strings | unit | `go test ./internal/config ./internal/session -run 'TestLoadValidConfigWithStructuredRoles|TestRunRootMessageContractUsesRouteTaskCommand|TestRouteChildTaskSelectsDeterministicOwner|TestRouteChildTaskRejectsNoMatchWithStructuredReason' -count=1` | ❌ W0 | ⬜ pending |
| 02-01-02 | 01 | 1 | ROUTE-01 | T-02-02 / T-02-03 | Routing chooses one owner by `route_priority` then config order and preserves teammate boundaries | unit/integration-with-tempdir | `go test ./internal/config ./internal/session -run 'TestLoadValidConfigWithStructuredRoles|TestRunRootMessageContractUsesRouteTaskCommand|TestRouteChildTaskSelectsDeterministicOwner|TestRouteChildTaskRejectsNoMatchWithStructuredReason' -count=1` | ❌ W0 | ⬜ pending |
| 02-02-01 | 02 | 2 | ROUTE-02 | T-02-04 / T-02-05 | Duplicate keys block accidental re-routing for exclusive task classes while allowed fanout remains explicit and testable | unit/integration-with-tempdir | `go test ./internal/session -run 'TestRouteChildTaskBlocksExclusiveDuplicate|TestRouteChildTaskAllowsFanoutReviewClass|TestRouteChildTaskRequiresOverrideReason|TestAddChildTaskRejectsDuplicateWithoutRouteDecision' -count=1` | ❌ W0 | ⬜ pending |
| 02-02-02 | 02 | 2 | ROUTE-01, ROUTE-02 | T-02-06 / T-02-07 | Routed tasks persist candidate set, duplicate key, tie-break evidence, and override reason so `run show` explains routing from disk | unit/integration-with-tempdir | `go test ./internal/session -run 'TestRunShowIncludesRoutingDecisionEvidence' -count=1 && go test ./internal/config ./internal/session ./internal/protocol -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/config/loader_test.go` — structured `RoleSpec` config parsing and validation coverage
- [ ] `internal/session/run_test.go` — route-task, no-match, duplicate, and override coverage
- [ ] `internal/session/run_rebuild_test.go` — routing-evidence rendering coverage for `run show`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Coordinator prompt is understandable when it tells the coordinator to use `tmuxicate run route-task ...` instead of guessing owners | ROUTE-01 | Automated tests can pin exact strings but not whether the command reads naturally to the operator/coordinator pair | Start a temp run, inspect the root message body, confirm the route-task command and routing metadata instructions are explicit and actionable |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
