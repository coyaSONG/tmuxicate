# Requirements: tmuxicate

**Defined:** 2026-04-11
**Core Value:** A human can coordinate multiple terminal agents through a reliable, observable workflow where the coordinator keeps work moving without hiding what happened.

## v1.1 Requirements

### Adaptive Routing

- [x] **ADAPT-01**: Operator can configure and persist coordinator routing preferences derived from prior run outcomes without changing the mailbox protocol
- [x] **ADAPT-02**: Operator can inspect the evidence and rationale behind an adaptive routing decision for each routed child task

### Partial Replanning

- [x] **REPLAN-01**: Coordinator can create a bounded partial replan for blocked work instead of only rerouting or escalating the current task
- [x] **REPLAN-02**: Partial replans preserve lineage to the original task, blocker case, and any follow-up decisions or replacements

### Execution Targets

- [x] **EXEC-01**: Coordinator can dispatch child tasks to remote or sandboxed worker targets in addition to local `tmux` panes
- [x] **EXEC-02**: Operator can inspect execution target capabilities and task placement before coordinator dispatch commits work

### Operator Visibility

- [ ] **OBS-01**: Operator can view a per-run timeline of task creation, routing, review, blocker, and resolution events
- [ ] **OBS-02**: Operator can filter run views by owner, state, task class, and execution target without transcript review

## v2 Requirements

### Team Topology

- **TEAM-01**: Coordinator can manage nested teams or multiple coordinators within one durable workflow graph
- **TEAM-02**: Operators can compare or rebalance work across multiple coordinators without manual artifact reconstruction

### Adaptive Automation

- **AUTO-01**: Coordinator can tune routing preferences automatically from prior runs while remaining fully inspectable
- **AUTO-02**: Coordinator can recommend cross-run workflow improvements based on timeline and blocker history

## Out of Scope

| Feature | Reason |
|---------|--------|
| Fully autonomous long-horizon replanning | Violates the current reliability and operator-visibility bar |
| Replacing the mailbox protocol with a separate orchestration backend | The new milestone must extend the existing durable workflow model |
| Hiding adaptive routing signals behind opaque scoring | Operators need explicit evidence for every coordinator choice |
| Cross-coordinator swarm behavior | Too much topology expansion for the next milestone; defer until single-coordinator adaptive flows are stable |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| ADAPT-01 | Phase 6 | Complete |
| ADAPT-02 | Phase 6 | Complete |
| REPLAN-01 | Phase 7 | Complete |
| REPLAN-02 | Phase 7 | Complete |
| EXEC-01 | Phase 8 | Complete |
| EXEC-02 | Phase 8 | Complete |
| OBS-01 | Phase 9 | Pending |
| OBS-02 | Phase 9 | Pending |

**Coverage:**
- v1.1 requirements: 8 total
- Mapped to phases: 8
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-11*
*Last updated: 2026-04-11 after initial definition for v1.1 Adaptive Coordination*
