# Roadmap: tmuxicate

## Milestones

- ✅ **v1.0 Coordinator Automation** — Phases 1-5 shipped 2026-04-11. Archive: `.planning/milestones/v1.0-ROADMAP.md`
- ✅ **v1.1 Adaptive Coordination** — Phases 6-9 shipped 2026-04-11. Archive: `.planning/milestones/v1.1-ROADMAP.md`
- ✅ **v1.2 Remote Execution Foundations** — Phases 10-12 shipped 2026-04-11. Archive: `.planning/milestones/v1.2-ROADMAP.md`
- 🟡 **v1.3 Runtime Trust & Honest Controls** — Phases 13-15 active. Plan: `.planning/milestones/v1.3-ROADMAP.md`

## Current Status

`v1.3 Runtime Trust & Honest Controls` is the active milestone. It focuses on hardening the target/runtime foundations before broader remote transport, worktree automation, or multi-coordinator topology:

- mailbox-grade target state and dispatch durability
- intent-first, idempotent non-pane dispatch recovery
- truthful delivery policy behavior for manual mode, notification disablement, readiness checks, retry ceilings, and timeout visibility
- owned daemon lifecycle across `up`, `serve`, `status`, and `down`
- safer local artifacts plus documentation and command UX alignment

## Next Step

Execute Phase 13 Plan 02 with:

```bash
/gsd-execute-phase 13
```
