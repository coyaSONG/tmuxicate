# Requirements: v1.2 Remote Execution Foundations

## Milestone Goal

Turn non-local execution-target metadata into concrete remote execution flows while preserving durable, inspectable coordination.

## Current Milestone Requirements

### Remote Dispatch

- [ ] `REMOTE-01` Coordinator can persist enough target transport configuration to distinguish concrete remote execution from local pane-backed execution without breaking existing local targets.
- [ ] `REMOTE-02` Coordinator can dispatch eligible work through a concrete remote execution path while preserving canonical run, task, and routing artifacts.

### Target Health

- [ ] `HEALTH-01` Operator can inspect durable readiness, heartbeat, and capability state for remote targets before routing work.
- [ ] `HEALTH-02` Remote execution emits durable lifecycle state that keeps `run show`, summaries, and timeline projections consistent with local execution.

### Operator Control

- [ ] `CTRL-01` Operator can explicitly disable, quarantine, or recover a remote target without mutating historical run artifacts.
- [ ] `CTRL-02` Coordinator explains target selection, rejection, or reroute decisions when remote targets are unavailable or unhealthy.

## Deferred Requirements

- [ ] `DEFER-01` Coordinator can manage nested teams or multiple coordinators within one durable workflow graph.
- [ ] `DEFER-02` Operators can compare, rebalance, and inspect work across multiple coordinator runs or teams.
- [ ] `DEFER-03` Coordinator can evolve adaptive signals into richer inspectable recommendations or auto-tuning without hiding control boundaries.

## Out of Scope

- Fully managed infrastructure provisioning for remote workers
- Replacing mailbox-backed artifacts with a separate remote orchestration backend
- Opaque failover that reroutes work without explicit durable operator-visible state
- Multi-coordinator topology work beyond the transport assumptions needed for one remote target

## Traceability

| Requirement | Planned Phase | Notes |
|-------------|---------------|-------|
| `REMOTE-01` | Phase 10 | Define remote transport contract, config shape, and routing boundary |
| `REMOTE-02` | Phase 10 | Execute non-local work through the new remote path with test coverage |
| `HEALTH-01` | Phase 11 | Add durable remote target status, heartbeat, and capability views |
| `HEALTH-02` | Phase 11 | Preserve event parity for summaries and timeline rebuilds |
| `CTRL-01` | Phase 12 | Add explicit operator controls for target availability and recovery |
| `CTRL-02` | Phase 12 | Surface target choice and fallback reasoning in operator workflows |

## Coverage Expectation

Every requirement in this milestone must land with direct test coverage in the affected session, runtime, or coordinator surfaces. Remote execution expands failure modes, so operator-visible state and artifact rebuild paths need explicit verification rather than inferred confidence.

---
*Last updated: 2026-04-11 for milestone v1.2 Remote Execution Foundations*
