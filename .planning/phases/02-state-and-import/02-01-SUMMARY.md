---
phase: 02-state-and-import
plan: 01
subsystem: normalize
tags: [json, normalization, slug, kebab-case, tdd, graph-api]

requires:
  - phase: 01-foundation
    provides: "Go project structure, go.mod with testify dependency"
provides:
  - "Normalize() function for deterministic canonical JSON from Graph API responses"
  - "Slugify() function for kebab-case slug derivation from display names"
affects: [02-state-and-import, 03-reconciliation]

tech-stack:
  added: []
  patterns: [table-driven-tdd, recursive-map-walking, compiled-regex-singletons]

key-files:
  created:
    - internal/normalize/normalize.go
    - internal/normalize/normalize_test.go
    - internal/normalize/slug.go
    - internal/normalize/slug_test.go
  modified: []

key-decisions:
  - "Used strings.Contains for @odata key matching (not HasPrefix) to catch embedded patterns like authenticationStrength@odata.context"
  - "Preserved empty arrays as semantically meaningful in CA policies (excludeUsers:[] differs from absent field)"
  - "Package-level compiled regexes for Slugify to avoid recompilation per call"

patterns-established:
  - "TDD with table-driven tests for pure functions in internal packages"
  - "Recursive map walking for JSON transformation (stripODataFields, removeNulls)"

requirements-completed: [IMPORT-03, IMPORT-04, IMPORT-05, IMPORT-06]

duration: 2min
completed: 2026-03-04
---

# Phase 02 Plan 01: Normalize and Slugify Summary

**Deterministic JSON normalization pipeline and kebab-case slug derivation for CA policy import using TDD**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-04T20:53:00Z
- **Completed:** 2026-03-04T20:54:44Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Normalize() strips server-managed fields, @odata metadata recursively, null values recursively, sorts keys, and pretty-prints with 2-space indent and trailing newline
- Slugify() converts display names to kebab-case slugs handling colons, spaces, underscores, consecutive special chars, and edge cases
- Full pipeline test verifies byte-for-byte output against realistic Graph API response
- 16 total test cases across both functions, all passing

## Task Commits

Each task was committed atomically:

1. **Task 1: TDD normalize package** (RED) - `5127f59` (test)
2. **Task 1: TDD normalize package** (GREEN) - `85962f2` (feat)
3. **Task 2: TDD slug derivation** (RED) - `9ff93da` (test)
4. **Task 2: TDD slug derivation** (GREEN) - `6ea2d79` (feat)

## Files Created/Modified
- `internal/normalize/normalize.go` - Normalize() function: strip server fields, strip @odata recursively, remove nulls recursively, sort keys, pretty-print
- `internal/normalize/normalize_test.go` - 8 table-driven tests for normalization pipeline including full Graph API response
- `internal/normalize/slug.go` - Slugify() function: display name to kebab-case slug via compiled regex
- `internal/normalize/slug_test.go` - 8 table-driven tests for slug derivation edge cases

## Decisions Made
- Used `strings.Contains(k, "@odata.")` instead of `strings.HasPrefix` to catch embedded OData keys like `authenticationStrength@odata.context`
- Preserved empty arrays (semantically meaningful in CA policies -- `excludeUsers:[]` means "no exclusions")
- Package-level compiled regexes for Slugify to avoid per-call recompilation

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Normalize() and Slugify() ready for import pipeline (02-02, 02-03)
- No blockers for downstream plans

## Self-Check: PASSED

All 4 created files verified on disk. All 4 commit hashes verified in git log.

---
*Phase: 02-state-and-import*
*Completed: 2026-03-04*
