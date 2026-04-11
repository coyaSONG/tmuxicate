---
phase: 12-operator-target-control
plan: 02
subsystem: routing
tags: [go, routing, targets, recovery]
requires:
  - phase: 12-operator-target-control
    provides: durable operator-controlled target availability
provides:
  - explainable excluded-target routing decisions
  - reroute away from unavailable targets
  - redispatch of pending unread work on recovery
affects: []
tech-stack:
  added: []
  patterns: [target-aware admission control, redispatch on recovery]
key-files:
  created: []
  modified:
    - internal/session/run.go
    - internal/session/target.go
    - internal/session/target_test.go
requirements-completed: [CTRL-01, CTRL-02]
duration: 14m
completed: 2026-04-11
---

# Phase 12 Plan 02 Summary

Made route selection target-aware, persisted excluded-target evidence, and automatically redispatched unread pending work when a disabled target is re-enabled.
