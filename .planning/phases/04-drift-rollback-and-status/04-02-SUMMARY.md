---
phase: 04-drift-rollback-and-status
plan: 02
subsystem: cli
tags: [drift-detection, reconcile, cobra, exit-codes, ci]

requires:
  - phase: 03-plan-and-apply
    provides: reconcile engine, output renderers, exit code types
provides:
  - cactl drift command with read-only reconciliation and remediation suggestions
  - filterBySlug helper for policy-level drift filtering
  - filterDriftActionable that includes untracked policies as drift
affects: [04-drift-rollback-and-status, ci-integration]

tech-stack:
  added: []
  patterns: [read-only reconciliation reuse, drift-specific actionable filter]

key-files:
  created: [cmd/drift.go, cmd/drift_test.go]
  modified: []

key-decisions:
  - "Drift keeps Untracked in actionable filter (unlike apply which excludes them) since untracked IS drift"
  - "Remediation footer only shown for human output (not JSON) to keep JSON machine-parseable"
  - "All errors wrapped in ExitError code 2 for consistent fatal error handling"
  - "No semver, validation, or display name resolution in drift (keeps it fast for CI)"

patterns-established:
  - "Read-only command pattern: reuse reconcile engine without writes"
  - "Remediation suggestions footer for drift/status commands"

requirements-completed: [CLI-05, DRIFT-01, DRIFT-02, DRIFT-03, DRIFT-04, VALID-02, DISP-05]

duration: 2min
completed: 2026-03-05
---

# Phase 04 Plan 02: Drift Command Summary

**Read-only drift detection command reusing reconcile engine with sigils, remediation footer, --policy filter, JSON output, and CI-friendly exit codes**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-05T04:59:11Z
- **Completed:** 2026-03-05T05:01:43Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- Drift command performs read-only reconciliation (zero writes to Graph or state)
- Supports --policy slug filter for single-policy drift check
- Supports --output json for CI consumption via RenderPlanJSON
- Exit codes: 0 no drift, 1 drift detected, 2 fatal error
- Remediation footer presents three options: apply, import --force, report-only
- Untracked policies (? sigil) included as drift (unlike apply which excludes them)

## Task Commits

Each task was committed atomically:

1. **Task 1: Drift command with read-only reconciliation** - `c7ff58a` (feat)

## Files Created/Modified
- `cmd/drift.go` - Drift command with read-only reconciliation, --policy filter, remediation footer
- `cmd/drift_test.go` - Tests for command registration, --policy flag, filterBySlug, filterDriftActionable

## Decisions Made
- Drift keeps Untracked in actionable filter (unlike apply which excludes them) since untracked IS drift (DRIFT-02)
- Remediation footer only shown for human output, not JSON, to keep JSON machine-parseable
- All errors wrapped in ExitError code 2 for consistent fatal error handling (VALID-02)
- No semver computation, validation, or display name resolution -- keeps drift fast for CI scheduled checks

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Drift command complete, ready for rollback (04-03) and status (04-04) commands
- Drift reuses same reconcile engine as plan/apply for consistent behavior

---
*Phase: 04-drift-rollback-and-status*
*Completed: 2026-03-05*
