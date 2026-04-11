# Roadmap: tmuxicate

## Milestones

- ✅ **v1.0 Coordinator Automation** — Phases 1-5 shipped 2026-04-11. Archive: `.planning/milestones/v1.0-ROADMAP.md`
- 🚧 **v1.1 Adaptive Coordination** — Phases 6-9 planned

## Overview

This milestone builds on the shipped coordinator workflow foundation by making coordinator decisions adaptive, extending execution beyond local `tmux` panes, and improving operator visibility into how work moves through a run. The intent is to add smarter behavior without replacing the durable mailbox/runtime model or making coordinator behavior harder to inspect.

## Phases

- [ ] **Phase 6: Adaptive Routing Signals** - Persist adaptive routing inputs and explain why routed work went to a specific owner
- [ ] **Phase 7: Partial Replanning Flow** - Add bounded task replacement and lineage-preserving partial replans for blocked work
- [ ] **Phase 8: Remote Execution Targets** - Extend dispatch and operator controls to remote or sandboxed worker targets
- [ ] **Phase 9: Run Timeline Views** - Add per-run timelines and filtering for operator inspection

## Phase Details

### Phase 6: Adaptive Routing Signals
**Goal**: Coordinator can reuse prior run outcomes as explicit routing signals and explain adaptive decisions to the operator.
**Depends on**: Phase 5
**Requirements**: [ADAPT-01, ADAPT-02]
**Success Criteria** (what must be TRUE):
  1. Coordinator persists routing preference evidence from prior runs in durable artifacts tied to existing run/task lineage.
  2. Adaptive routing still produces deterministic, inspectable owner selection rather than opaque heuristic output.
  3. Operators can inspect why a routed task preferred one owner over others without transcript review.
**Plans**: 2 plans

Plans:
- [ ] 06-01-PLAN.md — Persist adaptive routing signals and operator-tunable preference inputs
- [ ] 06-02-PLAN.md — Apply adaptive routing signals in `route-task` and expose decision evidence in operator views

### Phase 7: Partial Replanning Flow
**Goal**: Coordinator can replace blocked work with a bounded partial replan while preserving durable lineage and explicit operator control.
**Depends on**: Phase 6
**Requirements**: [REPLAN-01, REPLAN-02]
**Success Criteria** (what must be TRUE):
  1. Blocked work can create replacement tasks or mini-plans without losing the relationship to the original task and blocker case.
  2. Operators can see which work was superseded, replaced, or resumed from the same durable run graph.
  3. Partial replanning remains bounded and escalates clearly when the coordinator cannot recover safely.
**Plans**: 2 plans

Plans:
- [ ] 07-01-PLAN.md — Define partial replan artifacts, replacement semantics, and bounded recovery rules
- [ ] 07-02-PLAN.md — Implement partial replan execution and render lineage in existing run inspection surfaces

### Phase 8: Remote Execution Targets
**Goal**: Coordinator can dispatch work to remote or sandboxed execution targets without breaking current mailbox and adapter expectations.
**Depends on**: Phase 7
**Requirements**: [EXEC-01, EXEC-02]
**Success Criteria** (what must be TRUE):
  1. Coordinator can declare and select non-local execution targets using explicit, durable target metadata.
  2. Operators can inspect task placement and target capabilities before dispatching or rerouting work.
  3. Existing local `tmux` workflows continue to work unchanged when remote targets are not configured.
**Plans**: 2 plans

Plans:
- [ ] 08-01-PLAN.md — Define execution-target contracts and target-aware dispatch boundaries
- [ ] 08-02-PLAN.md — Implement remote target placement and operator-visible target inspection

### Phase 9: Run Timeline Views
**Goal**: Operators can inspect the full shape of a run through timelines and filters instead of transcript spelunking.
**Depends on**: Phase 8
**Requirements**: [OBS-01, OBS-02]
**Success Criteria** (what must be TRUE):
  1. Operators can see a chronological run timeline covering routing, review, blockers, replans, and resolution events.
  2. Operators can filter run inspection by owner, task class, state, and execution target.
  3. Timeline views are derived from the existing durable event and artifact model rather than a second reporting backend.
**Plans**: 2 plans

Plans:
- [ ] 09-01-PLAN.md — Build the derived timeline read model over run and workflow artifacts
- [ ] 09-02-PLAN.md — Add filtered operator-facing timeline views to existing run inspection workflows

## Progress

**Execution Order:**
Phases execute in numeric order: 6 → 7 → 8 → 9

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 6. Adaptive Routing Signals | v1.1 | 0/2 | Not started | - |
| 7. Partial Replanning Flow | v1.1 | 0/2 | Not started | - |
| 8. Remote Execution Targets | v1.1 | 0/2 | Not started | - |
| 9. Run Timeline Views | v1.1 | 0/2 | Not started | - |
