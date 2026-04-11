# Phase 2: Role-Based Routing - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-05
**Phase:** 02-Role-Based Routing
**Areas discussed:** Role matching, Duplicate safeguards, Tie-breaking, No-match behavior

---

## Role matching

### Role shape

| Option | Description | Selected |
|--------|-------------|----------|
| Structured enum | `kind: implementer|reviewer|researcher|coordinator` plus structured domains only | |
| Prose only | Keep role as natural-language description and let models infer matching | |
| Hybrid | `RoleSpec{Kind, Domains, Description}` with structured routing inputs plus descriptive prose | ✓ |

**User's choice:** Hybrid `RoleSpec{Kind, Domains, Description}`.
**Notes:** `Kind` and `Domains` are authoritative routing inputs. `Description` remains useful operator-facing context but should not drive final matching.

### Matching authority

| Option | Description | Selected |
|--------|-------------|----------|
| Code-driven | `RouteChildTask` filters candidates using structured metadata, then coordinator works from that result | ✓ |
| LLM-driven | Coordinator reads the team snapshot and directly chooses an owner | |

**User's choice:** Code-driven matching.
**Notes:** The routing layer must remain predictable, inspectable, and multi-vendor. `AddChildTask` stays an explicit-owner persistence writer, and `TaskClass` is introduced as a separate routing-intent field.

---

## Duplicate safeguards

### Policy surface

| Option | Description | Selected |
|--------|-------------|----------|
| `protocol.Kind` policy | Keep exclusive/fanout attached to message kind | |
| `TaskClass` policy | Move exclusive/fanout to routing-intent class | ✓ |
| Support both | Allow both task-class and message-kind policy knobs | |

**User's choice:** `TaskClass` policy.
**Notes:** Message kind is transport/workflow shape, not routing intent. Supporting both would blur operator understanding.

### Duplicate identity

| Option | Description | Selected |
|--------|-------------|----------|
| `TaskClass + Domains` within a run | Same run, same class, same normalized domains | ✓ |
| Goal similarity | Compare goal text semantically | |
| Owner-scoped duplicate | Only block repeats for the same owner | |

**User's choice:** Duplicate key is `(RunID, TaskClass, normalized Domains)`.
**Notes:** `Owner` is intentionally excluded so the same work cannot be sent to multiple agents accidentally.

### Enforcement point

| Option | Description | Selected |
|--------|-------------|----------|
| `RouteChildTask` only | Block before owner selection | |
| `AddChildTask` only | Block at persistence boundary | |
| Both | Pre-check in routing, final check at persistence | ✓ |

**User's choice:** Both.
**Notes:** `RouteChildTask` should reject early, while `AddChildTask` protects direct CLI calls and race windows.

### Intentional exceptions

| Option | Description | Selected |
|--------|-------------|----------|
| Fanout classes only | Only configured fanout classes may run in parallel | |
| Override only | Always require an explicit override | |
| Fanout classes plus reasoned override | Normal policy-based fanout plus explicit override escape hatch | ✓ |

**User's choice:** Fanout classes plus reasoned override.
**Notes:** `fanout_task_classes` are the normal parallel path. Override remains available but requires a reason. The default policy for `research` was left intentionally undecided.

---

## Tie-breaking

| Option | Description | Selected |
|--------|-------------|----------|
| Config declaration order only | First eligible agent in config wins | |
| Active-task load balancing | Lowest current active task count wins | |
| Explicit priority then declaration order | Higher priority wins, declaration order breaks ties | ✓ |
| Round-robin | Rotate across candidates using mutable routing state | |

**User's choice:** Explicit priority then declaration order.
**Notes:** Tie-breaking must be strictly deterministic. `AgentConfig.RoutePriority` is added with default `0`, higher values win, and config order is the stable fallback. Load balancing and round-robin are outside Phase 2 scope.

---

## No-match behavior

### Default behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Hard error | Fail loudly and return structured rejection data | ✓ |
| Domain fallback | Retry by ignoring domains but keeping kind | |
| Broad fallback | Ignore role constraints and route to any teammate | |

**User's choice:** Hard error.
**Notes:** Automatic fallback would hide misrouting. The system should fail loudly instead of silently widening the search.

### Owner override

| Option | Description | Selected |
|--------|-------------|----------|
| No override | No-match can never be bypassed | |
| Narrow override | `OwnerOverride` may bypass role/domain no-match with a reason, but not teammate or duplicate checks | ✓ |
| Full override | Owner override bypasses all routing guards | |

**User's choice:** Narrow override.
**Notes:** `OwnerOverride` requires a reason and can bypass role/domain mismatch only. Teammate boundaries and duplicate guards still apply.

### Rejection shape

| Option | Description | Selected |
|--------|-------------|----------|
| Plain error string | Return text only | |
| Structured rejection | Return code, requested class/domains, kind-level candidates, allowed owners, and suggestions | ✓ |

**User's choice:** Structured rejection.
**Notes:** The coordinator should receive actionable retry context: `RouteRejection{Code, RequestedTaskClass, RequestedDomains, EligibleByKind, AllowedOwners, Suggestions}`.

---

## the agent's Discretion

- Exact naming of routing decision and rejection helper structs.
- Exact domain-normalization implementation details, as long as behavior is deterministic and tested.
- Exact package location for routing helpers, as long as the current Go CLI architecture remains intact.

## Deferred Ideas

- Load-balanced routing by active task count.
- Round-robin routing state.
- Automatic no-match fallback.
- Goal-text similarity duplicate detection.

