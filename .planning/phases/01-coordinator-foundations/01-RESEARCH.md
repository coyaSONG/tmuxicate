# Phase 01: Coordinator Foundations - Research

**Researched:** 2026-04-05
**Domain:** Coordinator-run persistence, task graph durability, and restart reconstruction in the existing Go CLI/mailbox architecture [VERIFIED: 01-CONTEXT.md] [VERIFIED: codebase grep]
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Coordinator runs should start through a dedicated CLI command rather than overloading the existing generic `send` flow.
- **D-02:** The initial command shape should support an explicit coordinator target, for example `tmuxicate run "goal..." --coordinator pm`, so coordinator-run semantics are distinct from plain mailbox messages.
- **D-03:** Every child task must record `owner`, `goal`, `expected-output`, `depends-on`, `review-required`, and `parent-run-id`.
- **D-04:** Deadlines are intentionally out of scope for Phase 1; the first milestone should establish durable structure before adding time-based workflow policy.
- **D-05:** Phase 1 should use `role + teammate` metadata as the routing baseline rather than role-only assignment or unconstrained model inference.
- **D-06:** Freeform coordinator inference is not a foundation-phase behavior; routing should prefer explicit config relationships so the initial system stays predictable and debuggable.
- **D-07:** Phase 1 visibility should include the coordinator run tree, a compact state summary, and direct links or references back to the underlying messages/tasks.
- **D-08:** The foundation is not complete if operators must inspect raw transcripts to reconstruct who owns what.

### Claude's Discretion
- Exact CLI flag names beyond the dedicated run entrypoint
- Whether run/message references surface as message IDs, task IDs, or both, as long as they are durable and traceable
- Internal file layout for coordinator-run artifacts, provided it preserves mailbox authority and restart reconstruction

### Deferred Ideas (OUT OF SCOPE)
- Review handoff behavior beyond the minimum child-task foundation — covered in Phase 3
- Blocker escalation and retry policy — covered in Phase 4
- Rich run summaries beyond the foundation visibility requirement — covered in Phase 5
- Smarter inference-based routing or adaptive learning — future milestone, not Phase 1
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PLAN-01 | Operator can start a coordinator run from a high-level goal without manually splitting every child task first [VERIFIED: .planning/REQUIREMENTS.md] | `## Architecture Patterns` Pattern 1 and Pattern 2 define a dedicated `run` command and a coordinator-owned decomposition flow that emits child tasks durably. [VERIFIED: 01-CONTEXT.md] [VERIFIED: codebase grep] |
| PLAN-02 | Coordinator creates child tasks that each include an owner, parent linkage, task objective, and expected output [VERIFIED: .planning/REQUIREMENTS.md] | `## Standard Stack`, `## Architecture Patterns`, and `## Don't Hand-Roll` define a structured child-task artifact plus mailbox message linkage rather than freeform Markdown-only task bodies. [VERIFIED: 01-CONTEXT.md] [VERIFIED: codebase grep] |
| PLAN-03 | Coordinator run state and child task linkage survive process restarts and can be reconstructed from durable project artifacts [VERIFIED: .planning/REQUIREMENTS.md] | `## Architecture Patterns`, `## Common Pitfalls`, and `## Validation Architecture` require disk-first artifacts, scan-based reconstruction, and dedicated reload tests in `internal/session`. [VERIFIED: .planning/ROADMAP.md] [VERIFIED: codebase grep] |
</phase_requirements>

## Summary

Phase 1 should be implemented as a new coordinator workflow at the existing CLI/session boundary, not as daemon-native automation. [VERIFIED: 01-CONTEXT.md] [VERIFIED: codebase grep] The current binary already wires dedicated user actions through Cobra in `cmd/tmuxicate/main.go`, and the project decision explicitly requires a dedicated `run` command instead of overloading `send`. [VERIFIED: cmd/tmuxicate/main.go] [VERIFIED: 01-CONTEXT.md]

The safest persistence model is two-layered: keep mailbox messages and receipts as the delivery truth, and add a separate structured coordinator-run artifact set that records run identity, child-task fields, and message/task linkage. [VERIFIED: internal/mailbox/store.go] [VERIFIED: internal/session/send.go] [VERIFIED: internal/session/task_cmd.go] A child task needs more structure than the current `Envelope` and `Receipt` schema exposes, because Phase 1 requires `owner`, `goal`, `expected-output`, `depends-on`, `review-required`, and `parent-run-id`, while `Envelope.Meta` is only a `map[string]string` and cannot represent the full graph cleanly or defensibly. [VERIFIED: 01-CONTEXT.md] [VERIFIED: internal/protocol/envelope.go]

Reconstruction should be file-system based and deterministic: rebuild a coordinator run by scanning durable coordinator records, then join them to existing mailbox message directories, receipt folders, and per-agent declared state files. [VERIFIED: internal/session/status.go] [VERIFIED: internal/session/task_cmd.go] This matches the project’s stated filesystem-authoritative architecture and avoids the product failure mode where operators must inspect transcripts to understand ownership or lineage. [VERIFIED: AGENTS.md] [VERIFIED: 01-CONTEXT.md] [VERIFIED: README.md]

**Primary recommendation:** Add `tmuxicate run` plus a new `internal/session/run.go` workflow that persists a canonical run record and canonical child-task records under the session state directory, then emits normal mailbox tasks linked back to those records. [VERIFIED: 01-CONTEXT.md] [VERIFIED: cmd/tmuxicate/main.go] [ASSUMED]

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.26.1 [VERIFIED: go.mod] | Application code, file I/O, CLI runtime, and tests already run on this toolchain. [VERIFIED: go.mod] [VERIFIED: codebase grep] | Phase 1 is a brownfield extension of the current Go CLI, so adding a second runtime or storage layer would violate project constraints. [VERIFIED: AGENTS.md] |
| `github.com/spf13/cobra` | v1.10.2 [VERIFIED: go.mod] | Existing command registration and flag parsing in `cmd/tmuxicate/main.go`. [VERIFIED: cmd/tmuxicate/main.go] | The locked decision is a dedicated `run` entrypoint, and Cobra is already the command boundary used for `send`, `status`, `serve`, and `task`. [VERIFIED: 01-CONTEXT.md] [VERIFIED: cmd/tmuxicate/main.go] |
| `gopkg.in/yaml.v3` | v3.0.1 [VERIFIED: go.mod] | Canonical durable record format for config, envelopes, and receipts. [VERIFIED: internal/config/loader.go] [VERIFIED: internal/mailbox/store.go] | Canonical coordinator records should follow the existing durable YAML pattern, while runtime-only views can remain JSON. [VERIFIED: internal/mailbox/store.go] [VERIFIED: internal/session/up.go] [ASSUMED] |
| `golang.org/x/sys/unix` | v0.42.0 [VERIFIED: go.mod] | File locking for sequence and receipt mutation paths. [VERIFIED: internal/mailbox/store.go] | Reuse the existing `flock` discipline for any mutable coordinator indices or append logs to avoid concurrent write races. [VERIFIED: internal/mailbox/store.go] [ASSUMED] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/json` (stdlib) | bundled with Go 1.26.1 [VERIFIED: go.mod] | Runtime/current-state views such as `ready.json`, `state.current.json`, and heartbeat files. [VERIFIED: internal/session/up.go] [VERIFIED: internal/session/task_cmd.go] [VERIFIED: internal/runtime/daemon.go] | Use JSON for derived or operator-view snapshots, not for the canonical coordinator run/task records. [VERIFIED: internal/session/up.go] [VERIFIED: internal/runtime/daemon.go] [ASSUMED] |
| `github.com/fsnotify/fsnotify` | v1.9.0 [VERIFIED: go.mod] | Existing unread-inbox watching inside the daemon. [VERIFIED: internal/runtime/daemon.go] | Do not make Phase 1 depend on new daemon watchers; coordinator foundations can be command-side persistence plus reconstruction. [VERIFIED: .planning/ROADMAP.md] [VERIFIED: internal/runtime/daemon.go] |
| Go `testing` package | stdlib with Go 1.26.1 [VERIFIED: go.mod] | Existing unit and integration tests across protocol, config, mailbox, tmux, and runtime. [VERIFIED: rg --files -g '*_test.go'] | Phase 1 should add direct `internal/session` tests instead of inventing a new test framework. [VERIFIED: .planning/codebase/CONCERNS.md] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| YAML coordinator run/task records beside the mailbox [VERIFIED: internal/mailbox/store.go] [ASSUMED] | JSON-only coordinator records [VERIFIED: internal/session/up.go] | JSON would match runtime snapshots, but YAML aligns better with the existing canonical persistence model for durable business records. [VERIFIED: internal/mailbox/store.go] [VERIFIED: internal/session/up.go] [ASSUMED] |
| Dedicated coordinator artifacts plus linked mailbox messages [VERIFIED: 01-CONTEXT.md] [VERIFIED: internal/session/send.go] [ASSUMED] | Embedding all coordinator fields in `Envelope.Meta` [VERIFIED: internal/protocol/envelope.go] | `Meta` is flat string data and is a poor fit for dependency lists, booleans, and durable graph reconstruction. [VERIFIED: internal/protocol/envelope.go] |
| Extending the existing mailbox/session model [VERIFIED: AGENTS.md] [VERIFIED: .planning/PROJECT.md] | Separate coordinator database or hidden in-memory graph [VERIFIED: AGENTS.md] | A second authority would undermine the repo’s filesystem-first recovery model and operator visibility promise. [VERIFIED: README.md] [VERIFIED: DESIGN.md] |

**Installation:** No new external module is required for Phase 1 if the implementation stays within the current mailbox/session stack. [VERIFIED: go.mod] [VERIFIED: AGENTS.md]

**Version verification:** Recommended versions above are the versions pinned in `go.mod` on 2026-04-05, and no coordinator-specific dependency is currently necessary. [VERIFIED: go.mod]

## Architecture Patterns

### Recommended Project Structure

```text
cmd/tmuxicate/main.go                  # add the dedicated `run` command entrypoint
internal/session/run.go               # coordinator-run orchestration and decomposition
internal/session/run_rebuild.go       # reconstruction helpers for status/inspection paths
internal/session/run_test.go          # run creation and durable linkage tests
internal/session/run_rebuild_test.go  # restart/reload reconstruction tests
internal/protocol/coordinator.go      # run/task record structs + validation
internal/mailbox/coordinator_store.go # optional narrow helpers if artifact writes need locking
```

This split keeps command wiring in `cmd/`, workflow orchestration in `internal/session/`, and reusable schemas/helpers in lower layers, which matches the existing repo boundary pattern. [VERIFIED: AGENTS.md] [VERIFIED: internal/session/up.go] [VERIFIED: internal/mailbox/store.go] [ASSUMED]

### Pattern 1: Dedicated Coordinator Entry Point

**What:** Add a first-class `tmuxicate run "<goal>" --coordinator <agent>` command that resolves config, validates the coordinator target, creates a new run record, and asks the coordinator to decompose the goal into bounded child tasks. [VERIFIED: 01-CONTEXT.md] [VERIFIED: cmd/tmuxicate/main.go] [ASSUMED]

**When to use:** Use this for any operator-initiated orchestration flow that should create reconstructable coordinator state. Continue to use `send` for plain ad-hoc mailbox messages. [VERIFIED: 01-CONTEXT.md] [VERIFIED: internal/session/send.go]

**Example:**

```go
// Source: /Users/chsong/Developer/Personal/tmuxicate/cmd/tmuxicate/main.go
func newSendCmd() *cobra.Command {
	var configPath string
	var stateDir string

	cmd := &cobra.Command{
		Use:   "send <agent> <message>",
		Short: "Send a message to an agent",
		RunE: func(_ *cobra.Command, args []string) error {
			if stateDir == "" {
				cfg, err := config.LoadResolved(configPath)
				if err != nil {
					return err
				}
				stateDir = cfg.Session.StateDir
			}
			return nil
		},
	}
	return cmd
}
```

Planner guidance: mirror this command shape for `run`, but delegate to a new session-level function rather than stuffing coordinator logic into `main.go`. [VERIFIED: cmd/tmuxicate/main.go] [ASSUMED]

### Pattern 2: Canonical Run Record + Canonical Child-Task Records

**What:** Persist one canonical run record and one canonical child-task record per generated task under the session state directory, then link those records to normal mailbox messages via durable IDs. [VERIFIED: .planning/ROADMAP.md] [VERIFIED: internal/mailbox/store.go] [ASSUMED]

**When to use:** Use this whenever the coordinator creates work that must survive restarts and be reconstructable without transcripts. [VERIFIED: 01-CONTEXT.md]

**Prescriptive shape:** A child-task record should contain the Phase 1 required fields plus message linkage fields such as `message_id`, `thread_id`, `created_at`, and a derived state pointer. [VERIFIED: 01-CONTEXT.md] [VERIFIED: internal/protocol/envelope.go] [ASSUMED]

**Recommended canonical fields:**

| Record | Required fields |
|--------|-----------------|
| Run record | `run_id`, `goal`, `coordinator`, `created_by`, `created_at`, `root_message_id`, `root_thread_id`, `child_task_ids` [VERIFIED: 01-CONTEXT.md] [ASSUMED] |
| Child-task record | `task_id`, `parent_run_id`, `owner`, `goal`, `expected_output`, `depends_on`, `review_required`, `message_id`, `thread_id`, `created_at` [VERIFIED: 01-CONTEXT.md] [ASSUMED] |

### Pattern 3: Mailbox-Compatible Delivery, Not Mailbox Replacement

**What:** After the run/task records are written, emit normal mailbox task messages and receipts using the same sequence, validation, and atomic write flow already used by `session.Send`. [VERIFIED: internal/session/send.go] [VERIFIED: internal/mailbox/store.go]

**When to use:** Use this for every child task so agents still consume work through `tmuxicate inbox`, `read`, `reply`, and `task *`. [VERIFIED: README.md] [VERIFIED: internal/session/read_msg.go] [VERIFIED: internal/session/task_cmd.go]

**Example:**

```go
// Source: /Users/chsong/Developer/Personal/tmuxicate/internal/session/send.go
seq, err := store.AllocateSeq()
if err != nil {
	return "", err
}

msgID := protocol.NewMessageID(seq)
threadID := opts.Thread
if threadID == "" {
	threadID = protocol.NewThreadID(seq)
}

if err := store.CreateMessage(&env, payload); err != nil {
	return "", err
}
if err := store.CreateReceipt(&receipt); err != nil {
	return "", err
}
```

Planner guidance: Phase 1 should reuse this exact write ordering discipline for generated child work. [VERIFIED: internal/session/send.go] [ASSUMED]

### Pattern 4: Reconstruction by Scanning Durable Artifacts

**What:** Rebuild a run graph by scanning canonical coordinator records first, then joining to mailbox envelopes, receipts, and per-agent declared state files. [VERIFIED: internal/session/status.go] [VERIFIED: internal/session/task_cmd.go] [ASSUMED]

**When to use:** Use this on process restart, `status` extensions, and any future run-summary/read-model command. [VERIFIED: .planning/ROADMAP.md] [VERIFIED: internal/session/status.go]

**Why this pattern:** The repo already treats the filesystem as authoritative, and `status` already reconstructs flow/thread information by scanning messages and receipts rather than trusting tmux memory. [VERIFIED: AGENTS.md] [VERIFIED: internal/session/status.go]

### Anti-Patterns to Avoid

- **Coordinator state only in Markdown bodies:** This fails Phase 1 because required child-task fields become non-canonical and hard to reconstruct safely. [VERIFIED: 01-CONTEXT.md] [VERIFIED: internal/protocol/envelope.go]
- **Coordinator state only in `Envelope.Meta`:** Flat string metadata is too weak for durable task graphs and dependency lists. [VERIFIED: internal/protocol/envelope.go]
- **A second hidden store that bypasses mailbox conventions:** This contradicts the repo’s filesystem-authoritative design and weakens operator visibility. [VERIFIED: README.md] [VERIFIED: DESIGN.md]
- **New daemon behavior in Phase 1:** The roadmap only requires durable foundations and restart reconstruction, not automated reroute/retry/blocker logic. [VERIFIED: .planning/ROADMAP.md]
- **Shipping without direct `internal/session` tests:** The codebase concern audit explicitly calls the session package the highest-priority coverage gap. [VERIFIED: .planning/codebase/CONCERNS.md]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Durable message persistence | A new bespoke coordinator queue [VERIFIED: AGENTS.md] | `mailbox.Store.CreateMessage`, `CreateReceipt`, `AllocateSeq`, and locking helpers. [VERIFIED: internal/mailbox/store.go] | The mailbox already provides atomic writes, sequence allocation, checksum validation, and receipt locking. [VERIFIED: internal/mailbox/store.go] |
| Task delivery semantics | Pane-text or transcript-only task transport [VERIFIED: README.md] | Normal mailbox messages plus receipts. [VERIFIED: README.md] [VERIFIED: internal/session/read_msg.go] | The product explicitly treats pane injection as notification-only and the filesystem as the source of truth. [VERIFIED: README.md] [VERIFIED: DESIGN.md] |
| Status derivation | Transcript parsing [VERIFIED: 01-CONTEXT.md] | Scan receipts, messages, and state-event files. [VERIFIED: internal/session/status.go] [VERIFIED: internal/session/task_cmd.go] | Current operator views already aggregate from durable artifacts; transcripts are adjunct evidence only. [VERIFIED: internal/session/status.go] [VERIFIED: README.md] |
| Test doubles | Mock-heavy abstractions [VERIFIED: AGENTS.md] | Temp directories plus `tmux.NewFakeClient()`. [VERIFIED: internal/runtime/daemon_test.go] [VERIFIED: AGENTS.md] | The codebase already uses fake infrastructure over mocking frameworks. [VERIFIED: AGENTS.md] |

**Key insight:** Phase 1 should add one new canonical coordinator read/write surface, not invent a second orchestration substrate. [VERIFIED: .planning/PROJECT.md] [VERIFIED: AGENTS.md]

## Common Pitfalls

### Pitfall 1: Treating the Coordinator Run as Just Another Freeform Message

**What goes wrong:** The run can be started, but child ownership and expected outputs are only implicit in prose, so restart reconstruction becomes ambiguous. [VERIFIED: .planning/REQUIREMENTS.md] [VERIFIED: internal/protocol/envelope.go]
**Why it happens:** `send` already writes messages quickly, so it is tempting to stop at a root message and skip canonical child-task records. [VERIFIED: internal/session/send.go]
**How to avoid:** Make the run record and child-task records the canonical planning artifacts, then link mailbox messages back to them. [VERIFIED: 01-CONTEXT.md] [ASSUMED]
**Warning signs:** Operators need transcript review to answer “who owns task X?” or “what output is expected?” [VERIFIED: 01-CONTEXT.md]

### Pitfall 2: Hiding Graph State Inside `Envelope.Meta`

**What goes wrong:** Dependencies, `review_required`, and parent linkage become brittle string conventions with no strong validation boundary. [VERIFIED: 01-CONTEXT.md] [VERIFIED: internal/protocol/envelope.go]
**Why it happens:** `Meta` already exists and looks convenient. [VERIFIED: internal/protocol/envelope.go]
**How to avoid:** Add dedicated coordinator structs with explicit validation methods in `internal/protocol` or a nearby lower layer. [VERIFIED: AGENTS.md] [VERIFIED: internal/protocol/validation.go] [ASSUMED]
**Warning signs:** The planner proposes ad-hoc JSON-in-string blobs or comma-separated dependency lists. [VERIFIED: internal/protocol/envelope.go] [ASSUMED]

### Pitfall 3: Coupling Phase 1 to Daemon Automation

**What goes wrong:** Foundations work becomes entangled with watcher behavior, retries, and readiness probing that are already identified as fragile or under-tested. [VERIFIED: .planning/codebase/CONCERNS.md] [VERIFIED: internal/runtime/daemon.go]
**Why it happens:** The daemon already touches unread receipts and feels like a natural place for “automation.” [VERIFIED: internal/runtime/daemon.go]
**How to avoid:** Keep Phase 1 command-side: persist run/task artifacts and emit ordinary child-task messages; let later phases extend routing/review/blocker behavior deliberately. [VERIFIED: .planning/ROADMAP.md] [VERIFIED: 01-CONTEXT.md]
**Warning signs:** The implementation requires new background loops before a run can even be reconstructed. [VERIFIED: .planning/ROADMAP.md] [ASSUMED]

### Pitfall 4: Adding Coordinator Code Without Session Tests

**What goes wrong:** The highest-coupling workflow code changes, but `go test ./...` still misses regressions because `internal/session` currently has no tests. [VERIFIED: .planning/codebase/CONCERNS.md] [VERIFIED: rg --files -g '*_test.go']
**Why it happens:** Most existing test coverage is in protocol, config, mailbox, tmux, and runtime packages. [VERIFIED: rg --files -g '*_test.go']
**How to avoid:** Add `internal/session` tests in the same phase as the new coordinator run code. [VERIFIED: .planning/ROADMAP.md]
**Warning signs:** Reconstruction logic is validated only manually through `tmuxicate status` output. [VERIFIED: .planning/codebase/CONCERNS.md] [ASSUMED]

## Code Examples

Verified patterns from the current codebase:

### Atomic Durable Message + Receipt Creation

```go
// Source: /Users/chsong/Developer/Personal/tmuxicate/internal/session/send.go
seq, err := store.AllocateSeq()
if err != nil {
	return "", err
}

msgID := protocol.NewMessageID(seq)

if err := store.CreateMessage(&env, payload); err != nil {
	return "", err
}

receipt := protocol.Receipt{
	Schema:         protocol.ReceiptSchemaV1,
	MessageID:      msgID,
	Seq:            seq,
	Recipient:      protocol.AgentName(targetName),
	FolderState:    protocol.FolderStateUnread,
	Revision:       0,
	NotifyAttempts: 0,
}
if err := store.CreateReceipt(&receipt); err != nil {
	return "", err
}
```

Why it matters: generated child tasks should follow this persistence order so coordinator flows remain mailbox-compatible and restart-safe. [VERIFIED: internal/session/send.go] [VERIFIED: internal/mailbox/store.go]

### Durable Task State Transition

```go
// Source: /Users/chsong/Developer/Personal/tmuxicate/internal/session/task_cmd.go
if err := store.UpdateReceipt(agentName, msgID, func(r *protocol.Receipt) {
	r.ClaimedBy = nil
	r.ClaimedAt = nil
	r.DoneAt = &now
	r.Revision++
}); err != nil {
	return err
}
if err := store.MoveReceipt(agentName, msgID, protocol.FolderStateActive, protocol.FolderStateDone); err != nil {
	return err
}
```

Why it matters: run reconstruction should derive task progress from the same durable receipt/state event model rather than inventing a parallel task-status machine. [VERIFIED: internal/session/task_cmd.go]

### Read-Model Reconstruction by Filesystem Scan

```go
// Source: /Users/chsong/Developer/Personal/tmuxicate/internal/session/status.go
if err := scanMessages(stateDir, func(path string) error {
	data, err := os.ReadFile(filepath.Join(path, "envelope.yaml"))
	if err != nil {
		return err
	}
	// status joins message threads and receipts from disk
	return nil
}); err != nil {
	return nil, fmt.Errorf("scan messages: %w", err)
}
```

Why it matters: Phase 1 reconstruction should follow this scan-and-join style over durable artifacts. [VERIFIED: internal/session/status.go]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Human manually pre-splits work and sends individual tasks with `tmuxicate send`. [VERIFIED: README.md] [VERIFIED: internal/session/send.go] | Phase 1 should introduce a coordinator-owned `run` entrypoint that decomposes a high-level goal into child tasks. [VERIFIED: 01-CONTEXT.md] [ASSUMED] | Planned for Phase 1 on 2026-04-05. [VERIFIED: .planning/ROADMAP.md] | Work decomposition becomes explicit, durable, and reconstructable. [VERIFIED: .planning/REQUIREMENTS.md] [ASSUMED] |
| Mailbox threads exist, but there is no canonical coordinator run aggregate. [VERIFIED: internal/protocol/envelope.go] [VERIFIED: internal/session/status.go] | Phase 1 should add canonical run/task artifacts linked to mailbox messages. [VERIFIED: .planning/ROADMAP.md] [ASSUMED] | Planned for Phase 1 on 2026-04-05. [VERIFIED: .planning/ROADMAP.md] | Operators can recover ownership and expected outputs after restart without transcript review. [VERIFIED: .planning/REQUIREMENTS.md] [VERIFIED: 01-CONTEXT.md] |

**Deprecated/outdated:**
- Using transcripts as the fallback source for task graph reconstruction is outdated for this milestone because D-08 explicitly rejects transcript spelunking as the visibility mechanism. [VERIFIED: 01-CONTEXT.md]

## Assumptions Log

> List all claims tagged `[ASSUMED]` in this research. The planner and discuss-phase use this
> section to identify decisions that need user confirmation before execution.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Canonical coordinator artifacts should live in a dedicated subtree under the session state directory rather than only inside mailbox message metadata. | `## Summary`, `## Architecture Patterns` | Low to medium. The exact path is discretionary, but choosing a bad boundary could create a second authority or make reconstruction awkward. |
| A2 | Durable coordinator records should use YAML for canonical storage and JSON only for derived/current views. | `## Standard Stack`, `## Architecture Patterns` | Low. JSON would still work, but it would be less aligned with the existing canonical persistence model. |
| A3 | Any mutable coordinator index or append log should reuse the mailbox locking discipline. | `## Standard Stack` | Medium. If the implementation uses immutable-only files and full scans, extra locking may be unnecessary. |
| A4 | New workflow code should be split into `internal/session/run.go`, `internal/session/run_rebuild.go`, and optional lower-layer helpers rather than appended to existing large files. | `## Architecture Patterns` | Low. The repo can still work if files are arranged differently, but overly large files will worsen current maintainability concerns. |
| A5 | The recommended `run` command will emit mailbox child-task messages immediately after canonical run/task records are written. | `## Summary`, `## Architecture Patterns`, `## State of the Art` | Medium. A different execution order could work, but writing mailbox records first would weaken canonical coordinator-state guarantees. |

## Open Questions

1. **What operator-facing identifier should the UX emphasize: run/task IDs, message IDs, or both?**
   What we know: The context explicitly leaves this discretionary as long as references are durable and traceable. [VERIFIED: 01-CONTEXT.md]
   What's unclear: Whether `status` and future views should default to message IDs, task IDs, or a paired display. [VERIFIED: 01-CONTEXT.md]
   Recommendation: Persist both and display task ID first with message ID as the drill-down reference. [ASSUMED]

2. **Should reconstruction source declared task state from receipts only, or from receipts plus `state.current.json`?**
   What we know: Receipt folders already express delivery/progress buckets, and `task_cmd.go` writes per-agent declared state events. [VERIFIED: internal/session/task_cmd.go] [VERIFIED: internal/mailbox/store.go]
   What's unclear: Whether run views need the richer “awaiting_reply” and “blocked” semantics immediately in Phase 1 or can stay with receipt-state summaries only. [VERIFIED: .planning/ROADMAP.md] [ASSUMED]
   Recommendation: Join both; use receipts for durable folder state and state events for the compact run-tree summary. [VERIFIED: 01-CONTEXT.md] [ASSUMED]

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | build and test commands [VERIFIED: Makefile] | ✓ [VERIFIED: local command] | `go1.26.1 darwin/arm64` [VERIFIED: `go version`] | — |
| `tmux` | runtime behavior and integration-style tests [VERIFIED: AGENTS.md] [VERIFIED: internal/tmux/real_test.go] | ✓ [VERIFIED: local command] | `tmux 3.6a` [VERIFIED: `tmux -V`] | — |
| `bash` | generated agent `run.sh` scripts [VERIFIED: internal/session/up.go] | ✓ [VERIFIED: local command] | `GNU bash 3.2.57` [VERIFIED: `bash --version`] | — |
| `golangci-lint` | local lint parity with `make lint` and CI conventions [VERIFIED: Makefile] [VERIFIED: .github/workflows/ci.yml] | ✗ [VERIFIED: local command] | — | CI lint job on GitHub Actions. [VERIFIED: .github/workflows/ci.yml] |
| `gofumpt` | local formatting parity with `make fmt` [VERIFIED: Makefile] | ✗ [VERIFIED: local command] | — | `gofmt` can format Go syntax, but it will not match project formatting rules completely. [VERIFIED: Makefile] [ASSUMED] |
| `goimports` | import organization parity with `make fmt` [VERIFIED: Makefile] | ✗ [VERIFIED: local command] | — | Manual import cleanup or `gofmt` only, with reduced parity. [VERIFIED: Makefile] [ASSUMED] |

**Missing dependencies with no fallback:**
- None for Phase 1 research and planning. [VERIFIED: local command]

**Missing dependencies with fallback:**
- `golangci-lint`, `gofumpt`, and `goimports` are missing locally, but CI linting and basic `go test` validation still exist. [VERIFIED: Makefile] [VERIFIED: .github/workflows/ci.yml] [VERIFIED: local command]

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` package with race-enabled repository test command. [VERIFIED: Makefile] [VERIFIED: .github/workflows/ci.yml] |
| Config file | none; test/lint commands are centralized in `Makefile` and CI workflow. [VERIFIED: Makefile] [VERIFIED: .github/workflows/ci.yml] |
| Quick run command | `go test ./internal/session ./internal/mailbox ./internal/protocol -count=1` [VERIFIED: repo structure] [ASSUMED] |
| Full suite command | `go test ./... -count=1 -race` [VERIFIED: Makefile] |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PLAN-01 | Starting a coordinator run from a high-level goal creates a canonical run plus generated child tasks. [VERIFIED: .planning/REQUIREMENTS.md] | unit/integration-with-tempdir [VERIFIED: existing test style] | `go test ./internal/session -run TestRunCreatesChildTasks -count=1` [ASSUMED] | ❌ Wave 0 |
| PLAN-02 | Each child task persists owner, parent linkage, objective, expected output, and required Phase 1 fields. [VERIFIED: .planning/REQUIREMENTS.md] [VERIFIED: 01-CONTEXT.md] | unit [VERIFIED: existing test style] | `go test ./internal/session -run TestRunPersistsChildTaskFields -count=1` [ASSUMED] | ❌ Wave 0 |
| PLAN-03 | Restart/reload can reconstruct run state and task graph from disk artifacts. [VERIFIED: .planning/REQUIREMENTS.md] | unit/integration-with-tempdir [VERIFIED: existing test style] | `go test ./internal/session -run TestRebuildRunGraphFromDisk -count=1` [ASSUMED] | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/session ./internal/mailbox ./internal/protocol -count=1` [ASSUMED]
- **Per wave merge:** `go test ./... -count=1 -race` [VERIFIED: Makefile]
- **Phase gate:** Full suite green before `/gsd-verify-work`. [VERIFIED: .planning/config.json]

### Wave 0 Gaps

- [ ] `internal/session/run_test.go` — covers run creation, coordinator targeting, and durable child-task emission for `PLAN-01` and `PLAN-02`. [ASSUMED]
- [ ] `internal/session/run_rebuild_test.go` — covers reload/reconstruction for `PLAN-03`. [ASSUMED]
- [ ] Shared test helpers for building temp resolved configs and seeded state dirs in `internal/session`. [VERIFIED: internal/config/loader_test.go] [ASSUMED]
- [ ] Optional thin lower-layer tests if a dedicated coordinator store/helper package is added. [ASSUMED]

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no [VERIFIED: project is a local CLI without auth flow in current architecture] | — |
| V3 Session Management | no [VERIFIED: current tmux session lifecycle is local process orchestration, not authenticated web sessions] | — |
| V4 Access Control | no [VERIFIED: current repo exposes no multi-user access-control layer] | — |
| V5 Input Validation | yes [VERIFIED: coordinator run goals and task fields are operator/agent inputs] | Struct validation near the schema, following `Envelope.Validate` and `Receipt.Validate`. [VERIFIED: internal/protocol/validation.go] |
| V6 Cryptography | no for new Phase 1 features; existing SHA-256 body integrity remains relevant. [VERIFIED: internal/mailbox/store.go] | Reuse existing SHA-256 verification only where payload integrity is already part of mailbox semantics. [VERIFIED: internal/mailbox/store.go] |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Malformed or partial coordinator artifacts leading to bad reconstruction | Tampering | Canonical struct validation plus atomic write/rename patterns reused from `mailbox.Store`. [VERIFIED: internal/mailbox/store.go] [VERIFIED: internal/protocol/validation.go] |
| Goal/task fields containing unsafe path or identifier content | Tampering | Treat IDs and artifact paths as generated values, not raw user input; validate any referenced IDs before joining records. [VERIFIED: internal/protocol/ids.go] [VERIFIED: internal/protocol/validation.go] [ASSUMED] |
| Hidden coordinator state only in transcripts or pane memory | Repudiation | Persist run/task graph and link IDs on disk so operators can audit what happened after restart. [VERIFIED: README.md] [VERIFIED: 01-CONTEXT.md] |
| Coordinator records accidentally exposing secrets in durable artifacts | Information Disclosure | Keep coordinator records limited to goal/task metadata and avoid serializing process env or secret-bearing runtime details. [VERIFIED: .planning/codebase/CONCERNS.md] [ASSUMED] |

## Sources

### Primary (HIGH confidence)
- `.planning/phases/01-coordinator-foundations/01-CONTEXT.md` — locked decisions, discretion, and out-of-scope boundaries. [VERIFIED: codebase grep]
- `.planning/REQUIREMENTS.md` — `PLAN-01`, `PLAN-02`, and `PLAN-03`. [VERIFIED: codebase grep]
- `.planning/ROADMAP.md` — Phase 1 success criteria and plan boundaries. [VERIFIED: codebase grep]
- `.planning/STATE.md` — current project state and known concerns for this phase. [VERIFIED: codebase grep]
- `AGENTS.md` — project constraints, architecture rules, conventions, and workflow requirements. [VERIFIED: codebase grep]
- `README.md` and `DESIGN.md` — product philosophy and mailbox-first coordination model. [VERIFIED: codebase grep]
- `go.mod`, `Makefile`, `.github/workflows/ci.yml` — pinned module versions and validation commands. [VERIFIED: codebase grep]
- `internal/protocol/envelope.go`, `internal/protocol/receipt.go`, `internal/protocol/ids.go`, `internal/protocol/validation.go` — current schema limits, ID patterns, and validation style. [VERIFIED: codebase grep]
- `internal/mailbox/store.go`, `internal/mailbox/paths.go` — atomic persistence, locking, and path conventions. [VERIFIED: codebase grep]
- `internal/session/send.go`, `internal/session/read_msg.go`, `internal/session/reply.go`, `internal/session/task_cmd.go`, `internal/session/status.go`, `internal/session/up.go`, `internal/session/down.go` — current workflow, state files, and reconstruction style. [VERIFIED: codebase grep]
- `internal/mailbox/store_test.go`, `internal/config/loader_test.go`, `internal/runtime/daemon_test.go`, `internal/tmux/real_test.go`, `internal/protocol/protocol_test.go` — current test patterns and coverage gaps. [VERIFIED: codebase grep]
- Local environment probes: `go version`, `tmux -V`, `bash --version`, and missing local formatter/linter binaries. [VERIFIED: local command]

### Secondary (MEDIUM confidence)
- None. [VERIFIED: research session]

### Tertiary (LOW confidence)
- None. [VERIFIED: research session]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Phase 1 can stay inside the already pinned Go/Cobra/YAML/mailbox stack with no new external dependency required. [VERIFIED: go.mod] [VERIFIED: AGENTS.md]
- Architecture: HIGH - The repo’s existing boundaries and the phase context strongly constrain the design toward a dedicated session workflow plus durable artifacts. [VERIFIED: 01-CONTEXT.md] [VERIFIED: AGENTS.md] [VERIFIED: codebase grep]
- Pitfalls: HIGH - The codebase concern audit and current schema/store code make the main failure modes explicit. [VERIFIED: .planning/codebase/CONCERNS.md] [VERIFIED: internal/protocol/envelope.go] [VERIFIED: internal/mailbox/store.go]

**Research date:** 2026-04-05
**Valid until:** 2026-05-05 for repo-local architecture findings, assuming no major refactor lands first. [VERIFIED: research session] [ASSUMED]
