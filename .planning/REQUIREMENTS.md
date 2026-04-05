# Requirements: tmuxicate

**Defined:** 2026-04-05
**Core Value:** A human can coordinate multiple terminal agents through a reliable, observable workflow where the coordinator keeps work moving without hiding what happened.

## v1 Requirements

### Planning

- [ ] **PLAN-01**: Operator can start a coordinator run from a high-level goal without manually splitting every child task first
- [ ] **PLAN-02**: Coordinator creates child tasks that each include an owner, parent linkage, task objective, and expected output
- [ ] **PLAN-03**: Coordinator run state and child task linkage survive process restarts and can be reconstructed from durable project artifacts

### Routing

- [ ] **ROUTE-01**: Coordinator assigns implementation, research, and review tasks using configured agent roles and teammate relationships
- [ ] **ROUTE-02**: Coordinator does not assign the same execution task to multiple agents unless the duplication is an explicit workflow step such as review

### Review

- [ ] **REVIEW-01**: Coordinator can hand completed implementation work to a reviewer as a linked follow-up task
- [ ] **REVIEW-02**: Reviewer response remains linked to the originating coordinator run so the operator can trace implementation and review in one flow

### Blockers

- [ ] **BLOCK-01**: Coordinator reacts to child task `wait` and `block` states with an explicit next step instead of silently stalling
- [ ] **BLOCK-02**: Coordinator can escalate blocked or ambiguous work to the human operator with current owner, blocker reason, and recommended action
- [ ] **BLOCK-03**: Coordinator stops retrying or rerouting after defined limits and surfaces the unresolved task instead of looping indefinitely

### Summaries

- [ ] **SUM-01**: Operator can get an end-of-run summary that lists completed, waiting, blocked, under-review, and escalated work
- [ ] **SUM-02**: Run summaries identify the responsible agent and related message or task references for each reported item

## v2 Requirements

### Smarter Coordination

- **SMART-01**: Coordinator learns routing preferences from prior runs
- **SMART-02**: Coordinator can partially re-plan a run after a blocker without operator help
- **SMART-03**: Coordinator can manage nested teams or multiple coordinators

### Execution Expansion

- **EXEC-01**: Coordinator can target remote or sandboxed worker environments in addition to local tmux panes
- **EXEC-02**: Operator can view a richer coordinator dashboard with per-run timelines and filtering

## Out of Scope

| Feature | Reason |
|---------|--------|
| Unbounded all-agent group chat | Conflicts with the product goal of explicit, observable coordination |
| Fully autonomous long-horizon execution without human escalation | Too risky for the current reliability bar |
| Vendor-locked orchestration behavior | `tmuxicate` must remain multi-vendor |
| Coordinator acting as the primary implementer | This would collapse the specialist role model instead of orchestrating it |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| PLAN-01 | TBD | Pending |
| PLAN-02 | TBD | Pending |
| PLAN-03 | TBD | Pending |
| ROUTE-01 | TBD | Pending |
| ROUTE-02 | TBD | Pending |
| REVIEW-01 | TBD | Pending |
| REVIEW-02 | TBD | Pending |
| BLOCK-01 | TBD | Pending |
| BLOCK-02 | TBD | Pending |
| BLOCK-03 | TBD | Pending |
| SUM-01 | TBD | Pending |
| SUM-02 | TBD | Pending |

**Coverage:**
- v1 requirements: 12 total
- Mapped to phases: 0
- Unmapped: 12 ⚠️

---
*Requirements defined: 2026-04-05*
*Last updated: 2026-04-05 after initial definition*
