# Roadmap: tmuxicate

## Milestones

- ✅ **v1.0 Coordinator Automation** — Phases 1-5 shipped 2026-04-11. Archive: `.planning/milestones/v1.0-ROADMAP.md`
- ✅ **v1.1 Adaptive Coordination** — Phases 6-9 shipped 2026-04-11. Archive: `.planning/milestones/v1.1-ROADMAP.md`
- 🚧 **v1.2 Remote Execution Foundations** — Phases 10-12 planned. Goal: turn non-local execution-target metadata into concrete remote execution flows without losing inspectability.

## v1.2 Overview

`v1.2` narrows scope to the first real remote execution path. `v1.1` already introduced explicit target metadata and non-local placement boundaries, but execution still assumes operator-managed environments and local runtime semantics. This milestone turns that metadata into a concrete transport, durable health model, and operator control surface while preserving the mailbox-backed run graph and timeline model.

## Planned Phases

- **Phase 10: Remote Transport Contracts**  
  Define the transport/config contract for remote execution targets and route eligible work through a concrete non-local execution path.

- **Phase 11: Target Health & Lifecycle Parity**  
  Make remote targets observable through durable heartbeat/capability state and keep lifecycle events compatible with summaries and timelines.

- **Phase 12: Operator Target Control**  
  Give operators explicit tools to disable, recover, and reroute around unhealthy targets while keeping coordinator choices explainable.

## Phase Details

### Phase 10: Remote Transport Contracts

**Goal:** Convert execution-target metadata into a concrete remote dispatch path that preserves canonical run and task artifacts.

**Plans:**

- **10.1 Target Transport Model**  
  Define target configuration, validation, and coordinator-side transport assumptions for concrete remote execution.
- **10.2 Remote Dispatch Path**  
  Execute non-local work through the new transport boundary while keeping local pane-backed behavior intact and tested.

**Exit Criteria:**

- Coordinator can distinguish local and remote execution through validated target contracts
- Routed remote work persists canonical task and placement artifacts without special-case drift
- Local execution behavior remains backward compatible

### Phase 11: Target Health & Lifecycle Parity

**Goal:** Make remote targets durably observable and preserve operator inspection parity for non-local task progress.

**Plans:**

- **11.1 Durable Target Health State**  
  Persist readiness, heartbeat, and capability state for remote targets in an operator-inspectable form.
- **11.2 Remote Lifecycle Event Parity**  
  Ensure remote execution emits the durable events needed by `run show`, summaries, and timeline rebuilds.

**Exit Criteria:**

- Operators can inspect remote target health before dispatching work
- Timeline and summary projections remain consistent for remote and local tasks
- Missing or invalid remote lifecycle data fails loudly instead of being guessed

### Phase 12: Operator Target Control

**Goal:** Keep remote execution operator-steerable through explicit availability, recovery, and reroute workflows.

**Plans:**

- **12.1 Target Availability Controls**  
  Add explicit operator controls to disable, quarantine, or recover remote targets.
- **12.2 Explainable Reroute & Recovery**  
  Surface target rejection, fallback, and reroute reasoning in operator-facing coordinator workflows.

**Exit Criteria:**

- Operators can take unhealthy targets out of service without rewriting history
- Coordinator explains why a target was used, skipped, or replaced
- Recovery flows preserve inspectable run and routing artifacts

## Progress

| Phase | Name | Plans | Status |
|-------|------|-------|--------|
| 10 | Remote Transport Contracts | 0/2 | Not started |
| 11 | Target Health & Lifecycle Parity | 0/2 | Not started |
| 12 | Operator Target Control | 0/2 | Not started |

**Overall:** 0/6 plans complete (0%)

## Next Step

Start planning with:

```bash
/gsd-discuss-phase 10
```
