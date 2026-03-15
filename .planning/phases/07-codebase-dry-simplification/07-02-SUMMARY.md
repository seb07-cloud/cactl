---
phase: 07-codebase-dry-simplification
plan: 02
subsystem: cli
tags: [refactoring, dry, pipeline, cobra, go]

# Dependency graph
requires:
  - phase: 07-codebase-dry-simplification
    provides: CommandPipeline with NewPipeline, NormalizeLivePolicies, RenderPlan helpers
provides:
  - apply.go refactored to use pipeline + RecordAppliedAction
  - drift.go refactored to use pipeline helpers
  - rollback.go refactored to use pipeline + RecordAppliedAction
  - bumpPatchVersion eliminated from import.go (DUP-10)
  - RecordAppliedAction consolidates post-action bookkeeping (DUP-7)
affects: [07-03]

# Tech tracking
tech-stack:
  added: []
  patterns: [RecordAppliedAction post-action consolidation]

key-files:
  created: []
  modified: [cmd/apply.go, cmd/drift.go, cmd/rollback.go, cmd/import.go, cmd/pipeline.go]

key-decisions:
  - "RecordAppliedAction as pipeline method since it needs Cfg, Backend, and Manifest state"
  - "drift.go error wrapping changed from ExitError to fmt.Errorf via NewPipeline (compatible behavior)"
  - "rollback interactive mode keeps separate backend/manifest init (TUI has own lifecycle)"

patterns-established:
  - "RecordAppliedAction: single method for write-state, create-tag, update-manifest sequence"

requirements-completed: [DUP-1, DUP-2, DUP-3, DUP-4, DUP-5, DUP-6, DUP-7, DUP-8, DUP-10]

# Metrics
duration: 5min
completed: 2026-03-15
---

# Phase 7 Plan 2: Apply/Drift/Rollback Pipeline Refactoring Summary

**Apply, drift, and rollback commands refactored to use CommandPipeline with RecordAppliedAction consolidating 120+ lines of post-action bookkeeping, bumpPatchVersion eliminated**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-15T05:54:46Z
- **Completed:** 2026-03-15T05:59:49Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Added RecordAppliedAction to CommandPipeline, consolidating identical post-Graph-API state writes from create/update/recreate handlers
- Refactored apply.go from 568 to 316 lines using pipeline bootstrap + RecordAppliedAction
- Refactored drift.go from 206 to 121 lines using pipeline bootstrap + NormalizeLivePolicies + RenderPlan
- Refactored rollback.go from 355 to 283 lines using pipeline bootstrap + RecordAppliedAction
- Eliminated bumpPatchVersion from import.go, replaced with semver.BumpVersion (DUP-10)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add RecordAppliedAction and refactor apply.go** - `035ff33` (refactor)
2. **Task 2: Refactor drift.go, rollback.go, eliminate bumpPatchVersion** - `adb5949` (refactor)

## Files Created/Modified
- `cmd/pipeline.go` - Added RecordAppliedAction method and time import
- `cmd/apply.go` - Simplified to use pipeline + RecordAppliedAction (568 -> 316 lines)
- `cmd/drift.go` - Simplified to use pipeline helpers (206 -> 121 lines)
- `cmd/rollback.go` - Simplified to use pipeline + RecordAppliedAction (355 -> 283 lines)
- `cmd/import.go` - Replaced bumpPatchVersion with semver.BumpVersion, removed function

## Decisions Made
- RecordAppliedAction implemented as pipeline method (not standalone function) since it needs Cfg, Backend, and Manifest pipeline state
- drift.go config error wrapping changed from ExitError(Sprintf) to fmt.Errorf via NewPipeline -- compatible since tests don't assert on config error format
- rollback interactive mode keeps separate backend/manifest initialization since TUI has its own lifecycle independent of the pipeline

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated tests referencing removed bumpPatchVersion**
- **Found during:** Task 2
- **Issue:** rollback_test.go and import_test.go had tests for the deleted bumpPatchVersion function
- **Fix:** Removed bumpPatchVersion test cases (equivalent behavior now covered by semver.BumpVersion tests in internal/semver/version_test.go)
- **Files modified:** cmd/rollback_test.go, cmd/import_test.go
- **Verification:** go test ./cmd/... passes
- **Committed in:** adb5949

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Necessary test cleanup for removed function. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All three remaining commands now use CommandPipeline for bootstrap
- RecordAppliedAction available for any future commands needing post-action state writes
- Ready for 07-03 final cleanup pass

## Self-Check: PASSED

All files and commits verified.

---
*Phase: 07-codebase-dry-simplification*
*Completed: 2026-03-15*
