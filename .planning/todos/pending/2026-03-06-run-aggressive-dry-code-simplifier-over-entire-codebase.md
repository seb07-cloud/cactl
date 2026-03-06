---
created: 2026-03-06T16:15:01.480Z
title: Run aggressive DRY code-simplifier over entire codebase
area: general
files:
  - cmd/*.go
  - internal/**/*.go
---

## Problem

The codebase has accumulated duplication and similar patterns across 6 phases of rapid development. Common patterns like error handling, adapter conversions, CLI flag setup, and Graph API interactions likely have repeated code that could be consolidated. A structured, multi-pass simplification focusing on DRY principles is needed to reduce maintenance burden and improve consistency.

## Solution

Run the code-simplifier skill over the entire codebase in 3-5 structured iterations:

1. **Pass 1 — Cross-package patterns:** Identify duplicated helper functions, error handling patterns, and shared types across `cmd/`, `internal/` packages
2. **Pass 2 — Within-package dedup:** Consolidate repeated logic within each package (e.g., similar Graph API call patterns, config loading, output formatting)
3. **Pass 3 — Adapter/conversion cleanup:** Review the adapter patterns between packages (noted in decisions as workarounds for circular deps) and simplify where possible
4. **Pass 4 — Test helpers:** Deduplicate test setup, mock patterns, and assertion helpers
5. **Pass 5 — Final review:** Verify no regressions, run full test suite

Be aggressive — favor shared helpers and consolidated interfaces over repeated inline code. DRY is the primary principle.
