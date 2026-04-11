---
phase: 03
slug: review-handoff-flow
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-05
---

# Phase 03 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — package-local `*_test.go` plus `Makefile` commands |
| **Quick run command** | `go test ./internal/session -count=1` |
| **Full suite command** | `make test` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/session -count=1`
- **After every plan wave:** Run `make test`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 01 | 1 | REVIEW-01 | T-03-01 | `TaskDone` creates or updates one canonical review handoff and routes exactly one linked review task after the durable `done` transition | unit | `go test ./internal/session -run TestTaskDoneCreatesReviewHandoffAndRoutesReview -count=1` | ❌ W0 | ⬜ pending |
| 03-01-02 | 01 | 1 | REVIEW-01 | T-03-03 | repeated completion paths do not create duplicate handoffs; `reviews/<source-task-id>.yaml` remains the uniqueness gate | unit | `go test ./internal/session -run TestTaskDoneReviewHandoffIsIdempotent -count=1` | ❌ W0 | ⬜ pending |
| 03-01-03 | 01 | 1 | REVIEW-01 | T-03-04 | handoff failure leaves source task `done` and records fail-loud review-handoff failure details | unit | `go test ./internal/session -run TestTaskDoneRecordsReviewHandoffFailureWithoutRollback -count=1` | ❌ W0 | ⬜ pending |
| 03-02-01 | 02 | 2 | REVIEW-02 | T-03-02 | only the linked reviewer can record outcome; review response updates `response_message_id`, `outcome`, `responded_at`, and `status` on the canonical handoff | unit | `go test ./internal/session -run TestReviewRespondRecordsOutcome -count=1` | ❌ W0 | ⬜ pending |
| 03-02-02 | 02 | 2 | REVIEW-02 | T-03-01 | `run show` rebuild validates source/review/response links and renders the review handoff block beneath the source task | unit | `go test ./internal/session -run TestRunShowIncludesReviewHandoffBlock -count=1` | ❌ W0 | ⬜ pending |
| 03-02-03 | 02 | 2 | REVIEW-02 | T-03-01 | rebuild fails loudly when handoff artifacts drift from task or message linkage | unit | `go test ./internal/session -run TestLoadRunGraphRejectsBrokenReviewHandoffLinks -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/session/task_cmd_test.go` — post-`done` review handoff creation, idempotency, and fail-loud failure recording
- [ ] `internal/session/review_response_test.go` — dedicated review-response command/outcome recording and `review_response` message semantics
- [ ] `internal/session/run_rebuild_test.go` — review-handoff rendering plus broken-link rejection coverage
- [ ] `cmd/tmuxicate/main_test.go` or equivalent — optional direct Cobra coverage for the dedicated review-response CLI surface

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Review handoff block remains easy to scan in `tmuxicate run show <run-id>` when both source and review tasks are present | REVIEW-02 | Readability of the combined task + handoff output is operator-facing formatting, not just structural correctness | Create a fixture run with one implementation task, one review task, and one review response; inspect `tmuxicate run show <run-id>` and confirm the handoff block appears directly under the source task while the review task still appears in the normal task list |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
