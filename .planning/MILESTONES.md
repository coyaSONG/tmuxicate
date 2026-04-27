# Project Milestones: tmuxicate

## v1.0 Coordinator Automation (Shipped: 2026-04-11)

**Delivered:** The first shipped coordinator workflow layer for `tmuxicate`, covering durable run decomposition, deterministic routing, linked review handoff, blocker escalation, and operator-visible run summaries.

**Phases completed:** 1-5 (12 plans total)

**Key accomplishments:**
- Added durable coordinator run and child-task contracts with restart-safe reconstruction from disk.
- Shipped deterministic role-based routing with duplicate-safe task persistence and operator-visible routing evidence.
- Added linked implementation-to-review handoff and reviewer response flow without transcript-only state.
- Added blocker cases, reroute ceilings, and explicit `tmuxicate blocker resolve` operator actions.
- Added shared run summaries at the top of `run show` and on root-task completion.

**Stats:**
- 92 files changed
- 17,605 insertions and 100 deletions across the milestone range
- 5 phases, 12 plans, 28 tasks
- 3 calendar days from milestone definition to final phase completion (2026-04-05 → 2026-04-07)

**Git range:** `docs: define v1 requirements` → `docs(phase-05): evolve PROJECT.md after phase completion`

**What's next:** Define the next milestone around smarter coordination, richer operator visibility, and expansion beyond local tmux-only execution.

---

## v1.1 Adaptive Coordination (Shipped: 2026-04-11)

**Delivered:** Adaptive coordination on top of the shipped coordinator foundation, covering inspectable adaptive routing, bounded partial replans, explicit execution-target placement, and per-run timelines with filtering.

**Phases completed:** 6-9 (8 plans total)

**Key accomplishments:**
- Added coordinator-scoped adaptive routing preferences and durable adaptive decision evidence in both route output and `run show`.
- Added bounded partial replan artifacts and blocker-driven replacement-task lineage without introducing autonomous long-horizon replanning.
- Added execution target catalogs, dry-run placement previews, and mixed local/non-local runtime boundaries that preserve current mailbox semantics.
- Added strict run timeline projection from canonical artifacts plus `state.jsonl` and filtered timeline rendering in the existing `run show` workflow.
- Verified milestone integration with a passed `v1.1` milestone audit and a passing `go test ./...` suite.

**Stats:**
- 50 files changed
- 9,475 insertions and 207 deletions across the milestone range
- 4 phases, 8 plans, 19 tasks
- 1 calendar day from milestone definition to shipment (2026-04-11 → 2026-04-11)

**Git range:** `docs: define milestone v1.1 requirements` → `docs(09-02): complete run timeline views plan`

**What's next:** Turn non-local target metadata into concrete remote execution integration, expand to multi-coordinator topology, and evolve adaptive signals into richer inspectable automation.

---

## v1.2 Remote Execution Foundations (Shipped: 2026-04-11)

**Delivered:** The first concrete remote execution foundation for `tmuxicate`, turning non-local target metadata into command-based dispatch, durable target health, and explicit operator control while preserving mailbox-backed coordinator artifacts.

**Phases completed:** 10-12 (6 plans total)

**Key accomplishments:**
- Added target transport contracts with dispatch commands, per-target runtime state, and durable dispatch records for non-pane execution targets.
- Shipped concrete non-local dispatch on routed task creation with a stable environment contract and non-fatal dispatch failure handling.
- Added target heartbeat, timeout-based availability derivation, and target visibility in `status` plus a dedicated `tmuxicate target` command family.
- Made routing target-aware with durable excluded-target evidence and operator-visible rejection reasons.
- Added explicit disable, enable, heartbeat, and bounded redispatch flows so operators can recover targets without mutating run history.
- Verified milestone integration with a passed `v1.2` milestone audit and a passing `go test ./...` suite.

**Stats:**
- 15 files changed
- 1,553 insertions and 38 deletions across the milestone range
- 3 phases, 6 plans, 18 tasks
- 1 calendar day from milestone definition to shipment (2026-04-11 → 2026-04-11)

**Git range:** `docs: define milestone v1.2 requirements` → `feat: add remote target dispatch and control`

**What's next:** Define the next milestone around richer authenticated transport, multi-coordinator topology, and cross-run operator control on top of the shipped target runtime model.

---

## v1.3 Runtime Trust & Honest Controls (Active: 2026-04-27)

**Goal:** Harden the v1.2 target/runtime foundation before expanding remote transport, worktree isolation, or multi-coordinator topology.

**Phases planned:** 13-15 (6 plans total)

**Planned outcomes:**
- Target health and dispatch artifacts use mailbox-grade durability under concurrent updates.
- Non-pane dispatch records durable intent before execution and supports idempotent recovery for each target/message pair.
- Delivery policy settings such as manual mode, auto-notify, readiness checks, retry ceilings, and timeouts are enforced or surfaced clearly.
- Daemon lifecycle is owned across `up`, `serve`, `status`, and `down`, including stale PID and duplicate-daemon handling.
- Local artifact permissions, env validation, dispatch output handling, README, and command UX align with the reliability-first product contract.

**Plan:** `.planning/milestones/v1.3-ROADMAP.md`

**Requirements:** `.planning/milestones/v1.3-REQUIREMENTS.md`

**What's next:** Execute Phase 13 Plan 02: Intent-First Dispatch Recovery.

---
