---
phase: 04-drift-rollback-and-status
plan: 03
subsystem: cmd
tags: [rollback, git-tags, graph-api, patch, confirmation]

# Dependency graph
requires:
  - phase: 04-drift-rollback-and-status
    plan: 01
    provides: ReadTagBlob, ListVersionTags for historical version access
provides:
  - cactl rollback command for restoring prior policy versions
  - Forward version tagging on rollback (never modifies existing tags)
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: [tag-based rollback with forward versioning, diff-before-apply confirmation]

key-files:
  created:
    - cmd/rollback.go
    - cmd/rollback_test.go
  modified: []

key-decisions:
  - "PATCH bump for rollback (always minor version increment avoided -- rollback is a deployment event)"
  - "New forward version tag created on rollback -- existing tags never modified (ROLL-04)"
  - "Deleted live policy returns error suggesting cactl apply instead of rollback"

patterns-established:
  - "Tag read → diff → confirm → PATCH → new tag pipeline for rollback"
  - "bumpPatchVersion helper for semver increment"

requirements-completed: [CLI-06, ROLL-01, ROLL-02, ROLL-03, ROLL-04, SEMV-05]

# Metrics
duration: 2min
completed: 2026-03-05
---

# Phase 04 Plan 03: Rollback Command Summary

**`cactl rollback --policy <slug> --version <semver>` restores historical policy versions from Git annotated tags with diff preview, confirmation, and forward version tagging**

## Performance

- **Duration:** 2 min
- **Tasks:** 1
- **Files created:** 2

## Accomplishments
- Rollback reads historical JSON from annotated tags via ReadTagBlob
- Computes field-level diff between historical and current live state
- Confirmation prompt before applying (--auto-approve for CI)
- PATCHes live policy via Graph API
- Creates new forward version tag (never modifies existing tags)
- Updates manifest with new version, timestamp, deployer
- Non-existent version shows available versions from tag history
- CI mode requires --auto-approve

## Task Commits

1. **Task 1: Rollback command** - `4f4c08e` (feat)

## Files Created/Modified
- `cmd/rollback.go` - Full rollback pipeline: tag read, diff, confirm, PATCH, state update
- `cmd/rollback_test.go` - Tests for command registration, flags, bumpPatchVersion

## Decisions Made
- Always PATCH bump for rollback (SEMV-05)
- Forward version tag preserves audit trail (ROLL-04)
- Deleted live policy handled gracefully with suggestion to use apply instead

## Deviations from Plan

None.

## Issues Encountered

None.

## User Setup Required

None.

---
*Phase: 04-drift-rollback-and-status*
*Completed: 2026-03-05*
