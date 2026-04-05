# Roadmap: tmuxicate

## Overview

This milestone turns `tmuxicate` from a durable mailbox-and-pane collaboration tool into a coordinator-driven workflow system. The path is deliberately incremental: first make coordinator runs durable and reconstructable, then add deterministic routing, then wire review handoffs, then handle blockers safely, and finally close the loop with operator-facing summaries.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Coordinator Foundations** - Create durable coordinator runs and child task graph primitives
- [ ] **Phase 2: Role-Based Routing** - Assign work to the right agents without duplicate execution
- [ ] **Phase 3: Review Handoff Flow** - Move implementation work into linked review tasks and responses
- [ ] **Phase 4: Blocker Escalation** - Add safe handling for wait, block, reroute, and human escalation
- [ ] **Phase 5: Run Summaries** - Provide trustworthy operator-facing summaries for coordinator runs

## Phase Details

### Phase 1: Coordinator Foundations
**Goal**: A human can start a coordinator run that creates durable, reconstructable child tasks with explicit ownership and expected outputs.
**Depends on**: Nothing (first phase)
**Requirements**: [PLAN-01, PLAN-02, PLAN-03]
**Success Criteria** (what must be TRUE):
  1. Operator can start a coordinator run from a high-level goal and see child tasks created without manual pre-splitting.
  2. Each child task records owner, parent linkage, objective, and expected output in durable project artifacts.
  3. Restarting the process does not lose coordinator-run understanding; the task graph can be reconstructed from disk.
**Plans**: 3 plans

Plans:
- [ ] 01-01-PLAN.md — Define validated coordinator run/task contracts and durable artifact paths
- [ ] 01-02-PLAN.md — Implement `run` and `run add-task` over mailbox-compatible storage
- [ ] 01-03-PLAN.md — Rebuild and inspect coordinator runs from durable artifacts

### Phase 2: Role-Based Routing
**Goal**: Coordinator routes child tasks deterministically to suitable agents using declared roles and teammate relationships.
**Depends on**: Phase 1
**Requirements**: [ROUTE-01, ROUTE-02]
**Success Criteria** (what must be TRUE):
  1. Implementation, research, and review tasks are assigned using configured role metadata rather than freeform guesswork.
  2. The same execution task is not sent to multiple agents unless duplication is an intentional workflow step such as review.
  3. Routing behavior is covered by tests for expected and invalid assignments.
**Plans**: 2 plans

Plans:
- [ ] 02-01: Implement coordinator routing policy over agent config metadata
- [ ] 02-02: Add routing safeguards and duplicate-assignment tests

### Phase 3: Review Handoff Flow
**Goal**: Completed implementation work can transition into a linked review workflow inside the same coordinator run.
**Depends on**: Phase 2
**Requirements**: [REVIEW-01, REVIEW-02]
**Success Criteria** (what must be TRUE):
  1. Coordinator can generate a review task from completed implementation work without losing parent run linkage.
  2. Reviewer responses remain traceable to the originating coordinator run and implementation task.
  3. Operator can inspect the full implementation-to-review chain without transcript spelunking.
**Plans**: 2 plans

Plans:
- [ ] 03-01: Implement review-request generation from completed work
- [ ] 03-02: Link reviewer responses back into coordinator run state and views

### Phase 4: Blocker Escalation
**Goal**: Coordinator handles wait/block states safely through explicit reroute, escalation, and retry limits.
**Depends on**: Phase 3
**Requirements**: [BLOCK-01, BLOCK-02, BLOCK-03]
**Success Criteria** (what must be TRUE):
  1. Wait and block states always lead to an explicit next step instead of silent stalling.
  2. Human escalations include current owner, blocker reason, and recommended action.
  3. Coordinator stops looping after configured limits and surfaces unresolved work clearly.
**Plans**: 3 plans

Plans:
- [ ] 04-01: Classify wait and block states into coordinator actions
- [ ] 04-02: Implement human escalation payloads and retry ceilings
- [ ] 04-03: Add unhappy-path tests for repeated failure and unresolved work

### Phase 5: Run Summaries
**Goal**: Operator can see a trustworthy coordinator-run summary spanning completed, pending, blocked, review, and escalated work.
**Depends on**: Phase 4
**Requirements**: [SUM-01, SUM-02]
**Success Criteria** (what must be TRUE):
  1. End-of-run output lists completed, waiting, blocked, under-review, and escalated items in one place.
  2. Each summary item identifies the responsible agent and related task or message reference.
  3. Summary generation integrates with existing operator workflows without requiring manual transcript review.
**Plans**: 2 plans

Plans:
- [ ] 05-01: Build coordinator-run summary aggregation
- [ ] 05-02: Integrate summary output into operator-visible commands and verification

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Coordinator Foundations | 0/3 | Not started | - |
| 2. Role-Based Routing | 0/2 | Not started | - |
| 3. Review Handoff Flow | 0/2 | Not started | - |
| 4. Blocker Escalation | 0/3 | Not started | - |
| 5. Run Summaries | 0/2 | Not started | - |
