# Phase 10: Remote Transport Contracts - Context

**Gathered:** 2026-04-11
**Status:** Complete

## Phase Boundary

Turn non-local execution-target metadata into a concrete dispatch path without changing the mailbox-backed coordination model.

## Decisions

- Remote execution remains mailbox-first: task, receipt, and coordinator artifacts stay canonical.
- The first concrete transport is command-based dispatch configured on execution targets.
- Dispatch failure must not destroy the task artifact; it must degrade target health and remain inspectable.

## Specific Ideas

- Add dispatch contract fields to execution target config.
- Persist target dispatch records under the session state tree.
- Invoke non-pane target dispatch when a routed task is created.

## Deferred Ideas

- Built-in SSH/session provisioning.
- File sync, artifact upload, or remote shell orchestration beyond command dispatch.
