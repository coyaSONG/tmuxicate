---
phase: 01
slug: coordinator-foundations
status: ready
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-05
---

# Phase 01 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` |
| **Config file** | none — repository commands are centralized in `Makefile` and `.github/workflows/ci.yml` |
| **Quick run command** | `go test ./internal/session ./internal/mailbox ./internal/protocol -count=1` |
| **Full suite command** | `go test ./... -count=1 -race` |
| **Estimated runtime** | ~90 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/session ./internal/mailbox ./internal/protocol -count=1`
- **After every plan wave:** Run `go test ./... -count=1 -race`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | PLAN-01 | T-01-01 / T-01-02 | Contract tests reject blank run/coordinator fields, malformed task fields, and unsafe generated IDs before writer logic is added | unit | `go test ./internal/session -run 'TestRunRequestValidation|TestChildTaskValidation|TestCoordinatorPathsStayInsideStateDir' -count=1` | ❌ W0 | ⬜ pending |
| 01-01-02 | 01 | 1 | PLAN-02 | T-01-02 / T-01-04 | Canonical schema and path helpers preserve required fields plus deterministic `coordinator/runs/...` layout | unit | `go test ./internal/session -run 'TestRunRequestValidation|TestChildTaskValidation|TestCoordinatorPathsStayInsideStateDir' -count=1` | ❌ W0 | ⬜ pending |
| 01-02-01 | 02 | 2 | PLAN-01 | T-01-05 / T-01-07 | Root run message carries decomposition instructions and durable `run_id` / `root_message_id` / `root_thread_id` references | unit/integration-with-tempdir | `go test ./internal/session -run 'TestRunCreatesCoordinatorArtifactsAndRootMessage|TestAddChildTaskPersistsSchemaAndEmitsMailboxTask|TestAddChildTaskRejectsOwnerOutsideRoutingBaseline' -count=1` | ❌ W0 | ⬜ pending |
| 01-02-02 | 02 | 2 | PLAN-02 | T-01-05 / T-01-08 | Child-task creation persists all required fields and enforces role-plus-teammate routing boundaries before mailbox emission | unit/integration-with-tempdir | `go test ./internal/session -run 'TestRunCreatesCoordinatorArtifactsAndRootMessage|TestAddChildTaskPersistsSchemaAndEmitsMailboxTask|TestAddChildTaskRejectsOwnerOutsideRoutingBaseline' -count=1` | ❌ W0 | ⬜ pending |
| 01-03-01 | 03 | 3 | PLAN-03 | T-01-09 / T-01-10 | Rebuild reconstructs run/task lineage from disk and fails loudly on artifact mismatches | unit/integration-with-tempdir | `go test ./internal/session -run 'TestRebuildRunGraphFromDisk|TestRunShowSummarizesReceiptAndDeclaredState|TestRunShowRejectsMissingOrMismatchedArtifacts' -count=1` | ❌ W0 | ⬜ pending |
| 01-03-02 | 03 | 3 | PLAN-03 | T-01-11 / T-01-12 | `run show` exposes run/task/message references and compact state without transcript dependence | unit/integration-with-tempdir | `go test ./internal/session -run 'TestRebuildRunGraphFromDisk|TestRunShowSummarizesReceiptAndDeclaredState|TestRunShowRejectsMissingOrMismatchedArtifacts' -count=1 && go test ./internal/session ./internal/mailbox ./internal/protocol -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/session/run_test.go` — stubs for PLAN-01 and PLAN-02
- [ ] `internal/session/run_rebuild_test.go` — stubs for PLAN-03
- [ ] `internal/session/test_helpers.go` or equivalent shared fixtures for temp configs and state directories
- [ ] Root-message contract coverage in `internal/session/run_test.go` for decomposition instructions and durable run references

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Operator can inspect run tree plus message/task references from the chosen Phase 1 surface | PLAN-01, PLAN-03 | Final UX surface may land in a command or status view that still needs human judgment for usefulness | Start a coordinator run in a temp session, invoke the chosen inspection surface, confirm run tree and durable references are visible without transcript review |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
