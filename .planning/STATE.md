# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-04)

**Core value:** Reliable, idempotent, state-aware deployment of Entra CA policies with Git-native versioning and plan/apply safety
**Current focus:** Phase 3: Plan and Apply

## Current Position

Phase: 3 of 5 (Plan and Apply) -- COMPLETE
Plan: 5 of 5 in current phase
Status: Phase Complete
Last activity: 2026-03-05 -- Completed 03-05 Apply Command

Progress: [████████████████████] 60%

## Performance Metrics

**Velocity:**
- Total plans completed: 12
- Average duration: 2.5min
- Total execution time: 0.47 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-foundation | 4/4 | 10min | 2.5min |
| 02-state-and-import | 3/3 | 8min | 2.7min |
| 03-plan-and-apply | 5/5 | 15min | 3.0min |

**Recent Trend:**
- Last 5 plans: 03-01 (2min), 03-02 (3min), 03-03 (3min), 03-04 (4min), 03-05 (3min)
- Trend: Stable

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Roadmap: 5 phases derived from 87 v1 requirements across 15 categories
- Roadmap: Auth and Graph client in Phase 1 per research -- pitfalls cannot be retrofitted
- Roadmap: State storage before reconciliation -- engine needs all three inputs defined
- 01-01: Used global viper singleton for Phase 1 (standard Cobra pattern)
- 01-01: Root command RunE shows help when invoked without subcommands
- 01-01: HumanRenderer uses text prefixes when color disabled for accessibility
- 01-02: ClientFactory uses RWMutex double-check locking for thread-safe per-tenant credential caching
- 01-02: Auth providers are separate types implementing AuthProvider interface (not a single switch-case function)
- 01-02: Mock provider used in factory tests instead of real azidentity calls to avoid Azure dependency in CI
- 01-03: Phase 1 schema fetch always falls back to embedded (30MB OpenAPI YAML extraction deferred)
- 01-03: FetchOrFallback convenience function encapsulates fetch-then-fallback pattern
- 01-03: Git tracking check skipped silently when git unavailable (non-git workspaces supported)
- 01-04: Error output via fmt.Fprintln(os.Stderr) in main.go -- single error display point, SilenceErrors stays true
- 02-01: Used strings.Contains for @odata key matching (not HasPrefix) to catch embedded patterns like authenticationStrength@odata.context
- 02-01: Preserved empty arrays as semantically meaningful in CA policies
- 02-01: Package-level compiled regexes for Slugify to avoid per-call recompilation
- 02-02: os/exec git plumbing over go-git -- avoids blob-ref ErrUnsupportedObject issues
- 02-02: Empty manifest returned (not error) when ref missing -- enables first-use without init
- 02-02: Refspec silently skips when no remote origin -- supports local-only workflows
- 02-03: Graph client baseURL is struct field (not const) enabling httptest override without build tags
- 02-03: RawJSON preserved on Policy struct for downstream normalization (avoids double-marshal)
- 02-03: Slug collision detection prevents two Entra policies from mapping to same slug
- 02-03: Interactive selection parses comma-separated numbers, "all", or "none"
- 03-01: reflect.DeepEqual for leaf comparison -- consistent handling of slices and nested types
- 03-01: Noop actions suppressed (not emitted) -- plan output only shows actionable changes
- 03-01: Actions sorted by slug for deterministic output across runs
- 03-01: nil returned instead of empty slice for zero-action cases (idiomatic Go)
- 03-03: CollectRefs accepts []map[string]interface{} instead of reconcile.PolicyAction (avoids circular dep on unbuilt package)
- 03-03: BatchClient interface in resolve package for mock injection (not concrete *graph.Client)
- 03-03: isGUID uses structural UUID format check to exclude sentinel values like All/None
- [Phase 03]: 03-02: Local FieldDiff type in semver package -- reconcile package not yet available, avoids circular deps
- [Phase 03]: 03-02: Local PolicyAction/ActionType in validate package -- mirrors reconcile types for wave-1 independence
- [Phase 03]: 03-02: VALID-02 schema validation stubbed with TODO -- requires schema.json loading
- [Phase 03]: 03-02: checkEmptyIncludes only warns when conditions.users node exists -- avoids false positives
- 03-04: VersionFrom/VersionTo/BumpLevel added to PolicyAction -- plan command enriches after semver computation
- 03-04: Adapter pattern converts between package-local mirror types in cmd/plan.go (avoids circular deps)
- 03-04: Resolver errors non-fatal -- plan continues with raw GUIDs if display name resolution fails
- 03-04: Exit code 1 for actionable changes; validation errors override to exit 3
- 03-05: Reader-based confirm/confirmExplicit helpers for testability without stdin mocking
- 03-05: Per-action manifest+tag writes ensure state consistency even on mid-apply failure
- 03-05: Recreate uses BumpMinor (not BumpPatch) since policy identity changes
- 03-05: CI mode returns exit 2 when --auto-approve missing

### Pending Todos

None yet.

### Blockers/Concerns

None yet.

## Session Continuity

Last session: 2026-03-05
Stopped at: Completed 03-05-PLAN.md (Apply Command) -- Phase 03 complete
Resume file: None
