# Phase 9: Run Timeline Views - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Add operator-facing run timelines and filtering to existing run inspection workflows so a human can reconstruct routing, review, blocker, replan, and completion flow chronologically without transcript spelunking or a second reporting backend.

</domain>

<decisions>
## Implementation Decisions

### Timeline source of truth
- **D-01:** Timeline data must be derived from the current durable artifact set: coordinator run/task/review/blocker/replan YAML plus per-agent state events.
- **D-02:** Timeline views are projections over existing persisted data, not a new persisted timeline store or background summarizer.
- **D-03:** When artifacts disagree, timeline rebuild should fail loudly in the same spirit as existing run-graph reconstruction rather than silently guessing.

### Operator workflow
- **D-04:** Timeline output should extend existing operator surfaces, especially `tmuxicate run show`, instead of becoming a separate reporting subsystem.
- **D-05:** Filtering must stay explicit and deterministic: owner, task class, state/status, and execution target should all derive from durable task and event metadata.
- **D-06:** The timeline must remain compact enough for terminal use while still showing key workflow transitions in order.

### Phase boundaries
- **D-07:** Phase 9 may consume execution-target placement metadata introduced by Phase 8, but it should not redefine target contracts or runtime behavior.
- **D-08:** Transcript mining and fuzzy reconstruction are out of scope; if a key event is not represented in durable artifacts or state-event logs, this phase should add a durable projection rule rather than a heuristic.

### the agent's Discretion
- Exact CLI shape for invoking filtered timeline views, as long as it fits current `run show` operator workflows and does not fragment inspection into multiple unrelated commands.
- Exact event record type names and sort keys, as long as the resulting projection stays deterministic and testable.

</decisions>

<specifics>
## Specific Ideas

- Operators should be able to answer “what happened to this run, in order?” and “show only events for owner X / blocked items / target Y” from one terminal-friendly flow.
- Existing `state.jsonl` task events already cover accept/wait/block/done; coordinator artifacts cover route, review, blocker, and partial replan edges. The missing piece is a unified read model.
- Phase 9 should reuse the placement metadata from Phase 8 so execution-target filters are first-class instead of implied by owner names.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope
- `.planning/PROJECT.md` — milestone goal and operator-visibility constraints
- `.planning/REQUIREMENTS.md` — `OBS-01` and `OBS-02`
- `.planning/ROADMAP.md` — Phase 9 goal, dependency ordering, and success criteria
- `.planning/STATE.md` — current milestone progression

### Existing workflow and projection patterns
- `.planning/phases/07-partial-replanning-flow/07-02-SUMMARY.md` — durable lineage and run-show rendering for bounded replans
- `.planning/phases/06-adaptive-routing-signals/06-02-SUMMARY.md` — additive operator evidence rendered from durable task YAML
- `.planning/milestones/v1.0-phases/05-run-summaries/05-01-SUMMARY.md` — derived run-summary projection over existing run graph
- `.planning/milestones/v1.0-phases/05-run-summaries/05-02-SUMMARY.md` — shared summary rendering contract and operator formatting expectations

### Existing code seams
- `internal/session/task_cmd.go` — durable per-agent `TaskEvent` emission into `state.jsonl`
- `internal/session/review_response.go` — review response events
- `internal/session/run.go` — task creation/routing artifacts that can anchor timeline event refs
- `internal/session/run_rebuild.go` — canonical run graph reconstruction and current `run show` formatting
- `internal/session/run_summary.go` — existing derived summary projection patterns
- `internal/session/log_view.go` — current event/transcript viewing seam
- `cmd/tmuxicate/main.go` — CLI surface for `run show` and log/event flags
- `internal/session/status.go` — current event-derived status aggregation

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `TaskEvent` already captures timestamped task lifecycle changes (`accept`, `wait`, `block`, `done`) in JSONL per agent.
- `LoadRunGraph()` already correlates run, task, review, blocker, and partial-replan artifacts with strict validation.
- `BuildRunSummary()` proves the repo already accepts derived, non-persisted read models built from the same canonical run graph.

### Current Gaps
- `FormatRunGraph()` is organized by task detail blocks, not by chronological event order.
- Existing operator views can show logs or events per agent, but not a run-scoped merged timeline across all participants and coordinator artifacts.
- No current filtering layer lets operators narrow inspection by owner, task class, state, or execution target from the run view itself.

### Integration Points
- Build a run-scoped timeline projection that merges coordinator artifact timestamps with task-state JSONL events
- Extend `run show` output or adjacent run subcommands with terminal-friendly timeline rendering
- Add filter parsing in CLI/session boundaries using durable task metadata and timeline event fields
- Reuse placement metadata from Phase 8 so execution-target filters stay artifact-driven

</code_context>

<deferred>
## Deferred Ideas

- Cross-run analytics or recommendations
- Transcript-derived semantic event extraction
- Multi-coordinator or team-of-teams timeline aggregation

</deferred>

---

*Phase: 09-run-timeline-views*
*Context gathered: 2026-04-11*
