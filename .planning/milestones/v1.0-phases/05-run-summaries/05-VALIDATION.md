---
phase: 05
slug: run-summaries
status: ready
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-06
---

# Phase 05 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` |
| **Config file** | none — repository commands are centralized in `Makefile` and `.github/workflows/ci.yml` |
| **Quick run command** | `Run the task-specific focused test command from the verification map entry for the task you just changed` |
| **Full suite command** | `go test ./... -count=1 -race` |
| **Estimated runtime** | ~30 seconds for task-level sampling, ~120 seconds for the full suite |

---

## Sampling Rate

- **After every task commit:** Run the exact command from the verification-map row for the task you just changed
- **After every plan wave:** Run `go test ./... -count=1 -race`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 05-01-01 | 01 | 1 | SUM-01, SUM-02 | T-05-01-01 / T-05-01-03 | Summary contracts expose one logical source-task row with durable task/message references and no second persistence model | unit | `rg -n "type RunSummary|type RunSummaryItem|type RunSummaryStatus|func BuildRunSummary|func FormatRunSummary|TestBuildRunSummaryDerivesStatusBucketsAndReferences|TestBuildRunSummaryCollapsesReviewAndRerouteArtifactsIntoSourceRows|TestFormatRunSummaryGroupsItemsWithoutTaskDetailSprawl" internal/session/run_summary.go internal/session/run_summary_test.go` | ❌ W0 | ⬜ pending |
| 05-01-02 | 01 | 1 | SUM-01, SUM-02 | T-05-01-01 / T-05-01-02 | Aggregation uses `RunGraph`, excludes review/rerouted descendant rows, applies locked precedence, and derives `pending` from ordinary unread/active work only | unit | `go test ./internal/session -run 'TestBuildRunSummaryDerivesStatusBucketsAndReferences|TestBuildRunSummaryCollapsesReviewAndRerouteArtifactsIntoSourceRows' -count=1` | ❌ W0 | ⬜ pending |
| 05-01-03 | 01 | 1 | SUM-01, SUM-02 | T-05-01-03 | Formatter groups items by summary status with medium-density rows and no task-detail sprawl | unit | `go test ./internal/session -run 'TestFormatRunSummaryGroupsItemsWithoutTaskDetailSprawl' -count=1` | ❌ W0 | ⬜ pending |
| 05-02-01 | 02 | 2 | SUM-01 | T-05-02-02 / T-05-02-03 | `run show` renders the summary immediately below the run header while preserving task-local detail below it | unit + cmd | `rg -n "TestFormatRunGraphIncludesSummaryBeforeTaskDetails|TestRunShowCommandPrintsSummaryUnderHeader|TestTaskDoneCommandPrintsSummaryOnlyForRootRunCompletion" internal/session/run_rebuild_test.go cmd/tmuxicate/main_test.go` | ❌ W0 | ⬜ pending |
| 05-02-02 | 02 | 2 | SUM-01, SUM-02 | T-05-02-01 / T-05-02-03 | Root-task completion prints the shared summary once using canonical root-message metadata, while non-root `task done` output stays unchanged | cmd | `go test ./cmd/tmuxicate -run 'TestTaskDoneCommandPrintsSummaryOnlyForRootRunCompletion' -count=1 && go test ./internal/session ./cmd/tmuxicate -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/session/run_summary_test.go` — red tests for summary contracts, precedence, descendant folding, and grouped formatting
- [ ] `internal/session/run_rebuild_test.go` — summary-ordering coverage for `FormatRunGraph`
- [ ] `cmd/tmuxicate/main_test.go` — Cobra command tests for `run show` summary visibility and root-only completion printing

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Summary rows remain noticeably shorter than the full task-detail blocks while still exposing owner and reference context | SUM-01, SUM-02 | Scan-friendliness is a product-facing presentation judgment, not only a string-presence check | Run `tmuxicate run show <run-id>` against a fixture or real temp session, confirm `Summary:` appears before the first `Task:` block, and verify the detailed review/blocker blocks remain intact below it |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
