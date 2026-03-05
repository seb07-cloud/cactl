---
phase: 03-plan-and-apply
plan: 04
subsystem: cli, output
tags: [diff-renderer, terraform-style, sigils, json-output, plan-command, semver, validation, resolver]

requires:
  - phase: 03-plan-and-apply
    provides: "Reconcile engine, semver, validate, resolve packages from plans 01-03"
provides:
  - "Terraform-style diff renderer with sigils (+, ~, -/+, ?) and ANSI color support"
  - "JSON plan output with stable schema (schema_version=1)"
  - "cactl plan command orchestrating reconcile -> semver -> validate -> resolve -> render"
  - "PlanOutput, ActionOutput, DiffOutput, SummaryOutput JSON types"
affects: [03-plan-and-apply, 04-drift-rollback-and-status]

tech-stack:
  added: []
  patterns: [terraform-style-diff-sigils, adapter-pattern-for-local-mirror-types, exit-code-semantics]

key-files:
  created:
    - internal/output/diff.go
    - internal/output/diff_test.go
    - pkg/types/plan.go
    - cmd/plan.go
    - cmd/plan_test.go
  modified:
    - internal/reconcile/action.go

key-decisions:
  - "VersionFrom/VersionTo/BumpLevel added to PolicyAction struct for semver enrichment by plan command"
  - "Adapter pattern for local mirror types: reconcile.FieldDiff -> semver.FieldDiff, reconcile.PolicyAction -> validate.PolicyAction"
  - "Non-fatal resolver errors: plan continues with raw GUIDs if display name resolution fails"
  - "Exit code 1 for any actionable changes (create/update/recreate), validation errors override to exit 3"

patterns-established:
  - "Terraform-style sigils: + create (green), ~ update (yellow), -/+ recreate (red), ? untracked (cyan)"
  - "Type adapter pattern: cmd/plan.go converts between package-local mirror types to avoid circular deps"

requirements-completed: [CLI-02, PLAN-01, PLAN-03, PLAN-04, DISP-01, DISP-02, SEMV-06]

duration: 4min
completed: 2026-03-05
---

# Phase 3 Plan 4: Diff Output and Plan Command Summary

**Terraform-style colored diff renderer with sigils and cactl plan command wiring reconcile, semver, validate, resolve, and output pipelines**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-05T04:38:41Z
- **Completed:** 2026-03-05T04:43:19Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Diff renderer with terraform-style sigils (+, ~, -/+, ?), field-level diffs, MAJOR bump warnings, and summary line
- JSON output with stable schema (schema_version=1) for tooling integration
- Full cactl plan command orchestrating config -> auth -> graph -> backend -> reconcile -> semver -> validate -> resolve -> render pipeline
- Correct exit codes: 0 no changes, 1 changes detected, 3 validation errors

## Task Commits

Each task was committed atomically:

1. **Task 1: Diff output renderer and JSON plan types** - `75e6d9e` (feat)
2. **Task 2: Wire cactl plan command** - `7a34091` (feat)

## Files Created/Modified
- `pkg/types/plan.go` - PlanOutput, ActionOutput, DiffOutput, SummaryOutput JSON types
- `internal/output/diff.go` - RenderPlan (terraform-style) and RenderPlanJSON with sigils, colors, field diffs
- `internal/output/diff_test.go` - 9 tests covering all action types, JSON schema, color modes, MAJOR warnings
- `internal/reconcile/action.go` - Added VersionFrom, VersionTo, BumpLevel fields for semver enrichment
- `cmd/plan.go` - Full plan command: config, auth, graph, backend, reconcile, semver, validate, resolve, render
- `cmd/plan_test.go` - 3 tests: command registration, name, RunE presence

## Decisions Made
- Added VersionFrom, VersionTo, BumpLevel to reconcile.PolicyAction struct -- plan command enriches after semver computation
- Adapter pattern converts between package-local mirror types (semver.FieldDiff, validate.PolicyAction) in cmd/plan.go
- Resolver errors are non-fatal: plan continues with raw GUIDs if display name resolution fails
- Exit code 1 for any actionable changes; validation errors (SeverityError) override to exit 3

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added VersionFrom/VersionTo/BumpLevel fields to PolicyAction**
- **Found during:** Task 1 (diff renderer creation)
- **Issue:** RenderPlan needs version info on PolicyAction but fields did not exist
- **Fix:** Added VersionFrom, VersionTo, BumpLevel string fields to reconcile.PolicyAction struct
- **Files modified:** internal/reconcile/action.go
- **Verification:** All existing reconcile tests still pass; diff renderer tests use new fields
- **Committed in:** 75e6d9e (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Fields are required for the renderer to display version bumps. No scope creep -- purely additive struct fields.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- cactl plan command fully wired and ready for end-to-end testing with real tenant
- Diff renderer ready for apply command output (plan 03-05)
- JSON output schema stable for CI tooling consumption
- Resolver integration ready for display name enrichment in plan output

## Self-Check: PASSED

All 6 files verified. Both commits (75e6d9e, 7a34091) confirmed in git log.

---
*Phase: 03-plan-and-apply*
*Completed: 2026-03-05*
