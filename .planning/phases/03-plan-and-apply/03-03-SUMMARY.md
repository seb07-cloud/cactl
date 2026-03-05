---
phase: 03-plan-and-apply
plan: 03
subsystem: api
tags: [graph-api, batch, resolver, guid, caching, conditional-access]

requires:
  - phase: 02-state-and-import
    provides: "Graph API client with authenticated HTTP and httptest pattern"
provides:
  - "Graph API write operations (CreatePolicy, UpdatePolicy, DeletePolicy)"
  - "Graph API batch endpoint (ExecuteBatch for /$batch)"
  - "Display name resolver with batch resolution and caching"
  - "GUID extraction from policy JSON (CollectRefs)"
affects: [03-plan-and-apply, 04-drift-rollback-and-status]

tech-stack:
  added: []
  patterns: ["Batch API requests chunked to 20 items", "Interface-based dependency injection for testability (BatchClient)", "Graceful degradation on resolution failure"]

key-files:
  created:
    - internal/graph/batch.go
    - internal/resolve/resolver.go
    - internal/resolve/resolver_test.go
  modified:
    - internal/graph/policies.go
    - internal/graph/client_test.go

key-decisions:
  - "CollectRefs accepts []map[string]interface{} instead of reconcile.PolicyAction to avoid circular dependency on unbuilt package"
  - "BatchClient interface defined in resolve package for mock injection (not concrete *graph.Client)"
  - "isGUID filter uses UUID format check (36 chars, dash positions) to exclude sentinel values like All/None"

patterns-established:
  - "BatchClient interface: packages needing batch resolution depend on interface, not concrete client"
  - "Graceful degradation: 404 -> '(deleted)' suffix, other errors -> raw ID fallback"

requirements-completed: [DISP-03, DISP-04]

duration: 3min
completed: 2026-03-05
---

# Phase 3 Plan 3: Graph Write Operations and Display Name Resolver Summary

**Graph API write methods (create/update/delete) and batched GUID-to-display-name resolver with caching and graceful 404 handling**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-05T04:32:45Z
- **Completed:** 2026-03-05T04:35:58Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Graph client extended with CreatePolicy (POST, returns server-assigned ID), UpdatePolicy (PATCH), and DeletePolicy (DELETE)
- ExecuteBatch method for Graph API /$batch endpoint supporting up to 20 requests per batch
- Display name resolver with caching, deduplication, and batch chunking
- CollectRefs extracts GUIDs from policy JSON with correct object type mapping
- Graceful degradation: 404s cached as "(deleted)", errors fall back to raw ID

## Task Commits

Each task was committed atomically:

1. **Task 1: Graph API write operations and batch endpoint** - `2d219f7` (feat)
2. **Task 2: Display name resolver with caching and batch resolution** - `f14479a` (feat)

## Files Created/Modified
- `internal/graph/policies.go` - Added CreatePolicy, UpdatePolicy, DeletePolicy methods
- `internal/graph/batch.go` - BatchRequestItem, BatchResponseItem types and ExecuteBatch method
- `internal/graph/client_test.go` - 5 new tests for write operations and batch
- `internal/resolve/resolver.go` - Resolver with BatchClient interface, caching, CollectRefs
- `internal/resolve/resolver_test.go` - 6 tests covering resolution, caching, batching, GUID extraction

## Decisions Made
- CollectRefs accepts `[]map[string]interface{}` instead of `reconcile.PolicyAction` since the reconcile package does not exist yet (plan 03-01 creates it). Integration will pass `action.BackendJSON` fields when available.
- BatchClient interface defined in the resolve package rather than graph package, enabling clean mock injection without importing the full graph client.
- isGUID uses structural UUID format check (length 36, dashes at positions 8/13/18/23) to filter out sentinel values like "All", "None", "GuestsOrExternalUsers".

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] CollectRefs signature adapted for missing reconcile package**
- **Found during:** Task 2 (Display name resolver)
- **Issue:** Plan specified `CollectRefs(actions []reconcile.PolicyAction)` but the reconcile package (plan 03-01) has not been built yet
- **Fix:** Changed signature to `CollectRefs(policyMaps []map[string]interface{})` accepting raw JSON maps directly
- **Files modified:** internal/resolve/resolver.go
- **Verification:** Tests pass with raw map input; integration with reconcile.PolicyAction is straightforward (pass action.BackendJSON)
- **Committed in:** f14479a (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Signature change is API-compatible with future reconcile integration. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Graph write operations ready for `cactl apply` command (plan 03-05)
- Resolver ready for `cactl plan` display name enrichment (plan 03-04)
- Both packages have comprehensive test coverage

## Self-Check: PASSED

All 5 files verified. Both commit hashes (2d219f7, f14479a) confirmed in git log.

---
*Phase: 03-plan-and-apply*
*Completed: 2026-03-05*
