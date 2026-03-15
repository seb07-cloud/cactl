# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-04)

**Core value:** Reliable, idempotent, state-aware deployment of Entra CA policies with Git-native versioning and plan/apply safety
**Current focus:** Phase 7: Codebase DRY Simplification (In Progress)

## Current Position

Phase: 7 of 7 (Codebase DRY Simplification)
Plan: 1 of 3 in current phase
Status: In Progress
Last activity: 2026-03-15 -- Completed 07-01 (Pipeline Helpers)

Progress: [████████████████████████████████████░░] 96%

## Performance Metrics

**Velocity:**
- Total plans completed: 25
- Average duration: 1.8min
- Total execution time: 0.71 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-foundation | 4/4 | 10min | 2.5min |
| 02-state-and-import | 3/3 | 8min | 2.7min |
| 03-plan-and-apply | 5/5 | 15min | 3.0min |
| 04-drift-rollback-and-status | 5/5 | 8min | 1.6min |
| 05-production-readiness | 5/5 | 10min | 2.0min |
| 06-point-in-time-restore | 2/2 | 2min | 1.0min |
| 07-codebase-dry-simplification | 1/3 | 2min | 2.0min |

**Recent Trend:**
- Last 5 plans: 05-05 (1min), 06-01 (1min), 06-02 (1min), 07-01 (2min)
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
- 04-01: strip=5 in for-each-ref to extract version directly from tag ref path
- 04-01: HashObject wraps private hashObject for public API -- avoids code duplication
- 04-02: Drift keeps Untracked in actionable filter (unlike apply) since untracked IS drift
- 04-02: No semver/validation/resolver in drift -- keeps it fast for CI scheduled checks
- 04-02: Remediation footer only shown for human output (not JSON) to keep JSON machine-parseable
- 04-02: All drift errors wrapped in ExitError code 2 for consistent fatal error handling
- 04-04: Graceful degradation -- auth/network failures show "unknown" sync status instead of erroring
- 04-04: ListPolicies once + index by ID for O(1) per-policy sync lookup (avoids N+1)
- 04-04: Git SHA comparison via HashObject matches backend storage format exactly
- 04-04: Status always exits 0 -- informational command, not a gate
- 04-04: BuildSummary exported for reuse between table and JSON rendering paths
- 04-05: parseBumpLevel helper in cmd package (not semver) since it handles user CLI input
- 04-05: Override read early in runApply, applied inside bump computation loop per action
- 05-01: runForTenants stops immediately on fatal/validation errors but continues through ExitChanges (1)
- 05-01: Backward compat: single CACTL_TENANT env var auto-wrapped into []string slice
- 05-01: Config.Tenant deprecated field kept in sync for gradual migration of other commands
- 05-01: MTNT-04 concurrent pipeline advisory: comment-only in v1, lock file deferred to v1.1
- 05-02: GoReleaser v2 format with version: 2 header
- 05-02: CGO_ENABLED=0 for static binaries across all platforms
- 05-02: Changelog groups by conventional commit prefix (feat, fix, others)
- 05-02: Fixed go.mod module path mismatch (sebdah -> seb07-cloud) to unblock builds
- 05-03: GraphClient interface scoped to ListPolicies/GetPolicy only (write methods deferred)
- 05-03: MockGraphClient uses func fields (not generated mocks) for simplicity
- 05-03: golangci-lint not available locally; config validated by YAML structure only
- 05-04: README structured as project landing page with badges, install, quick start, architecture, and doc links
- 05-04: Docs use relative links between files for portability
- 05-04: CI/CD guide references example pipelines created in 05-02
- 05-05: Kept awk BEGIN block for float comparison (POSIX-portable)
- 06-01: huh v0.8.0 (not v2) as published latest; provides Select/Confirm out of box
- 06-01: Function fields in RestoreConfig avoid circular dep between tui and cmd packages
- 06-01: Diff summaries compare each version to predecessor (not current desired state)
- 06-01: Auto-plan errors treated as non-fatal since exit code 1 is expected when changes exist
- 06-02: Diff summaries show top-level field names only (deduped from dot-path diffs)
- 06-02: Graceful degradation: tag listing failure shows 0 versions instead of erroring
- 06-02: No restore capability in history command (per user decision: read-only only)
- 07-01: Manifest loaded in NewPipeline (not separate step) since all consumers need it
- 07-01: NormalizeLivePolicies and HasValidationErrors as standalone functions (no pipeline state needed)
- 07-01: ComputeSemverBumps accepts *semver.BumpLevel pointer for apply's --bump-level override reuse

### Roadmap Evolution

- Phase 6 added: Point-in-Time Restore - git history timeline, point-in-time policy restore with full diffs
- Phase 7 added: Codebase DRY Simplification - aggressive multi-pass deduplication across entire codebase

### Pending Todos

1. Add point-in-time restore for policies (2026-03-06)
2. Run aggressive DRY code-simplifier over entire codebase (2026-03-06)

### Blockers/Concerns

None yet.

## Session Continuity

Last session: 2026-03-15
Stopped at: Completed 07-01-PLAN.md (Pipeline Helpers)
Resume file: None
