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
