---
phase: 02-state-and-import
plan: 03
subsystem: graph-import
tags: [graph-api, http-client, azcore, pagination, import-pipeline, cobra, cli]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: CLI skeleton, config loading, auth provider interface, output renderer
  - phase: 02-state-and-import
    plan: 01
    provides: Normalize() and Slugify() for JSON canonicalization and slug derivation
  - phase: 02-state-and-import
    plan: 02
    provides: GitBackend, Manifest, ConfigureRefspec for state storage
provides:
  - Graph HTTP client with azcore token auth and CA policy list/get with pagination
  - "cactl import command with --all, --policy, --force flags and interactive selection"
affects: [03-reconciliation, 04-plan-apply]

# Tech tracking
tech-stack:
  added: []
  patterns: [httptest-mock-server, pagination-loop, interactive-cli-selection, semver-patch-bump]

key-files:
  created:
    - internal/graph/client.go
    - internal/graph/policies.go
    - internal/graph/client_test.go
    - cmd/import.go
    - cmd/import_test.go
  modified: []

key-decisions:
  - "Graph client uses baseURL field allowing httptest override in tests"
  - "RawJSON preserved on Policy struct for downstream normalization (avoids double-marshal)"
  - "Slug collision detection prevents two different Entra policies from mapping to same slug"
  - "Interactive selection uses numbered list with comma-separated input parsing"

patterns-established:
  - "httptest.NewServer for Graph API integration testing without live Azure"
  - "Pipeline orchestration pattern: fetch -> normalize -> state write -> tag -> manifest"
  - "bumpPatchVersion for --force re-import version incrementing"

requirements-completed: [CLI-04, IMPORT-01, IMPORT-02, IMPORT-07, IMPORT-08]

# Metrics
duration: 3min
completed: 2026-03-04
---

# Phase 02 Plan 03: Graph Client and Import Command Summary

**Graph API client with azcore auth and pagination, wired into cactl import command with --all, --policy, --force, and interactive selection modes**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-04T20:58:09Z
- **Completed:** 2026-03-04T21:01:08Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Graph HTTP client authenticates via azcore.TokenCredential and follows @odata.nextLink pagination to retrieve all CA policies
- `cactl import` command orchestrates full pipeline: Graph fetch -> normalize JSON -> write to Git state -> create version tag -> update manifest
- Three import modes: --all (bulk), --policy (single by slug/name), interactive (untracked list with ? sigil)
- --force overwrites tracked policies with automatic patch version bump (1.0.0 -> 1.0.1)
- CI mode validation prevents interactive selection, slug collision detection prevents ambiguous imports
- 9 new tests (5 graph + 4 import) all passing alongside existing 33+ tests

## Task Commits

Each task was committed atomically:

1. **Task 1: Graph API client with azcore auth and CA policy pagination** - `6c19b17` (feat)
2. **Task 2: cactl import command wiring Graph, normalize, and state pipeline** - `cd62948` (feat)

## Files Created/Modified
- `internal/graph/client.go` - Graph HTTP client with azcore TokenCredential auth, Bearer header, 30s timeout
- `internal/graph/policies.go` - ListPolicies with @odata.nextLink pagination, GetPolicy, Policy struct with RawJSON
- `internal/graph/client_test.go` - 5 tests: list, pagination, auth header, get single, HTTP error handling
- `cmd/import.go` - Import command: --all/--policy/--force flags, interactive selection, full pipeline orchestration
- `cmd/import_test.go` - 4 tests: flag validation, CI mode constraint, command registration, version bumping

## Decisions Made
- Graph client baseURL is a struct field (not const) enabling httptest override in tests without build tags
- RawJSON kept as json.RawMessage on Policy struct to avoid re-marshalling before normalization
- Slug collision detection: if existing manifest entry has different LiveObjectID than fetched policy, error prevents silent overwrite
- Interactive selection parses comma-separated numbers, "all", or "none" for flexible user input

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 2 is now complete: normalize, state backend, and import command are all wired together
- `cactl import --all --tenant <id>` is ready for UAT with real Entra tenant
- Graph client and state backend APIs are ready for Phase 3 reconciliation engine

## Self-Check: PASSED

All 5 created files verified on disk. Both commit hashes verified in git log.

---
*Phase: 02-state-and-import*
*Completed: 2026-03-04*
