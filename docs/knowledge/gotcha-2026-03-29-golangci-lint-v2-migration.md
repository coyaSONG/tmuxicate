---
title: golangci-lint v2 requires action v7 and new config format
category: gotcha
status: active
date: 2026-03-29
tags: [ci, golangci-lint, github-actions, linting]
---

# golangci-lint v2 requires action v7 and new config format

## Symptom
CI lint job failed with: "the Go language version (go1.24) used to build golangci-lint is lower than the targeted Go version (1.26.1)"

## Root Cause
Three layered issues:
1. golangci-lint v1.x is built with Go 1.24, which rejects Go 1.26 targets
2. golangci-lint v2.x requires `golangci-lint-action@v7` (not v6)
3. golangci-lint v2 has a completely different `.golangci.yml` format

## Fix / Workaround
Migration checklist:
1. Update action: `golangci/golangci-lint-action@v7`
2. Pin version to latest v2: `version: v2.11.4` (or later)
3. Migrate `.golangci.yml`:
   - Add `version: "2"` at top
   - Move `linters-settings` under `linters.settings`
   - Move `issues` settings under `linters.exclusions`
   - `enable-all`/`disable-all` replaced by `linters.default: all|standard|none|fast`
   - `max-issues-per-linter` and `max-same-issues` removed from exclusions (not valid in v2.11+)
   - Use `exclusions.presets` instead (e.g., `std-error-handling`)
4. Fix all new lint issues (v2 is stricter about `rangeValCopy`, `hugeParam`)
