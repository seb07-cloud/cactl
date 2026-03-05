---
phase: 03-plan-and-apply
plan: 05
subsystem: cli
tags: [apply-command, confirmation-flow, dry-run, auto-approve, graph-api-writes, state-updates, manifest, version-tags]

requires:
  - phase: 03-plan-and-apply
    provides: "Reconcile engine, semver, validate, resolve, output packages from plans 01-04"
provides:
  - "cactl apply command with full plan-then-execute pipeline"
  - "Confirmation flow with standard Y/n and escalated 'yes' for recreate"
  - "Dry-run mode generating plan without Graph API writes"
  - "Per-action state updates: manifest + version tags after each successful action"
  - "CI mode gating: --auto-approve required for write operations"
affects: [04-drift-rollback-and-status, 05-production-readiness]

tech-stack:
  added: []
  patterns: [plan-then-apply-pipeline, per-action-state-consistency, escalated-confirmation-for-destructive-ops]

key-files:
  created:
    - cmd/apply.go
    - cmd/apply_test.go
  modified: []

key-decisions:
  - "Reader-based confirm/confirmExplicit helpers for testability without stdin mocking"
  - "Per-action manifest+tag writes ensure state consistency even on mid-apply failure"
  - "Recreate uses BumpMinor (not BumpPatch) since policy identity changes"
  - "CI mode returns exit 2 when --auto-approve missing (not prompt, not silent)"

patterns-established:
  - "Plan-then-apply: same reconcile pipeline for both plan and apply commands"
  - "Escalated confirmation: destructive operations (recreate) require explicit 'yes'"
  - "Per-action state updates: manifest written after each successful Graph API call"

requirements-completed: [CLI-03, PLAN-05, PLAN-06, PLAN-07, PLAN-08, SEMV-01, DISP-02]

duration: 3min
completed: 2026-03-05
---

# Phase 3 Plan 5: Apply Command Summary

**cactl apply command with confirmation flow, dry-run, --auto-approve for CI, recreate escalation, and per-action manifest + version tag state updates**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-05T04:46:50Z
- **Completed:** 2026-03-05T04:50:00Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- Full apply pipeline reusing plan logic: config, auth, graph, backend, reconcile, semver, validate, resolve, render, then execute
- Confirmation flow with standard Y/n prompt and escalated "yes" for recreate actions
- --dry-run generates full plan but makes no Graph API writes
- --auto-approve skips prompts (required in CI mode)
- Per-action state consistency: manifest + version tags updated after each successful Graph API call
- Error recovery: reports which policy failed and how many succeeded before failure

## Task Commits

Each task was committed atomically:

1. **Task 1: Apply command with confirmation, dry-run, and state updates** - `6fdf7db` (feat)

## Files Created/Modified
- `cmd/apply.go` - Full apply command: plan generation, display, confirmation, Graph API writes (create/update/recreate), per-action state updates
- `cmd/apply_test.go` - 13 tests: command registration, flags, confirm helpers, filterActionable, hasAction, deployerIdentity

## Decisions Made
- Reader-based confirm/confirmExplicit helpers (`confirmFromReader`, `confirmExplicitFromReader`) for testability without stdin mocking
- Per-action manifest + version tag writes ensure state consistency even on mid-apply failure
- Recreate actions use BumpMinor since policy identity changes (new LiveObjectID)
- CI mode without --auto-approve returns ExitFatalError (code 2) with clear message
- SEMV-05 (--bump-level user override) deferred with TODO comment as specified in plan

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 3 (Plan and Apply) is fully complete: plan + apply commands both wired
- Ready for Phase 4 (Drift, Rollback, and Status) which builds on the apply pipeline
- Apply command ready for end-to-end testing with real tenant

## Self-Check: PASSED

All 2 files verified. Commit 6fdf7db confirmed in git log.

---
*Phase: 03-plan-and-apply*
*Completed: 2026-03-05*
