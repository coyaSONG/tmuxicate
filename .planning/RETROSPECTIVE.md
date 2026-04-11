# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — Coordinator Automation

**Shipped:** 2026-04-11
**Phases:** 5 | **Plans:** 12 | **Sessions:** 5

### What Was Built
- Durable coordinator run and child-task contracts with restart-safe reconstruction from disk
- Deterministic role-based routing with duplicate-safe persistence and operator-visible routing evidence
- Linked review handoff, blocker escalation, and shared run summaries layered on top of the existing mailbox workflow

### What Worked
- The milestone stayed inside the existing Go CLI, mailbox, and tmux boundaries instead of growing a second orchestration system.
- Phase-by-phase execution with direct tests in `internal/session` and related runtime surfaces kept new coordinator behavior concrete and inspectable.

### What Was Inefficient
- Milestone archival happened without a formal `v1.0` audit artifact, which weakens the final readiness signal even though the work itself is complete.
- Planning and summary artifacts were strong at the phase level, but end-of-milestone reporting still needed manual curation to become release-ready.

### Patterns Established
- Coordinator state should live in dedicated durable artifacts and be rebuilt from disk rather than cached in runtime-only structures.
- Operator-facing summary views should be derived from `RunGraph` and added to existing workflows rather than introduced as separate state machines.

### Key Lessons
1. Durable coordination features stay trustworthy when they extend the mailbox protocol rather than bypass it with hidden runtime channels.
2. Session- and runtime-level automation needs direct regression coverage before expanding autonomy, because those seams amplify subtle workflow bugs quickly.

### Cost Observations
- Model mix: not tracked in-repo
- Sessions: 5 phase cycles
- Notable: most implementation work landed quickly once protocol contracts and rebuild rules were pinned first

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | 5 | 5 | Added a full coordinator workflow layer on top of the durable mailbox runtime |

### Cumulative Quality

| Milestone | Tests | Coverage | Zero-Dep Additions |
|-----------|-------|----------|-------------------|
| v1.0 | Direct session/runtime regression tests added across all shipped workflow slices | Not tracked | Preserved the existing zero-service local runtime model |

### Top Lessons (Verified Across Milestones)

1. Keep durable artifacts authoritative and make operator views derived, not separately persisted.
2. Expand automation only where the operator can still reconstruct what happened from disk and CLI output.
