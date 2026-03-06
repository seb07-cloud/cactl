# Phase 7: Codebase DRY Simplification - Research

**Researched:** 2026-03-06
**Domain:** Go codebase refactoring / DRY consolidation
**Confidence:** HIGH

## Summary

The cactl codebase (~9700 lines across 68 Go files) was built incrementally across 6 phases and 24 plans. This resulted in significant duplication patterns, particularly in the `cmd/` layer where plan, apply, drift, status, and rollback commands share nearly identical bootstrap sequences (config load, auth setup, graph client creation, backend init, policy loading, normalization). The largest file is `apply.go` at 567 lines, with three near-identical action handlers (create/update/recreate) that differ only in the Graph API call and version computation.

The codebase is well-structured at the package level -- internal packages are clean and focused. The duplication is concentrated in two areas: (1) command-level bootstrap/pipeline code in `cmd/*.go`, and (2) duplicate type definitions used to avoid circular imports (e.g., `semver.FieldDiff` mirrors `reconcile.FieldDiff`, `validate.ActionType` mirrors `reconcile.ActionType`). There is also a duplicate `bumpPatchVersion` function (in `cmd/import.go` and `state/backend.go`).

**Primary recommendation:** Extract a shared command pipeline helper that handles config-load/auth/graph-client/backend/manifest bootstrapping, and consolidate the apply action handlers into a loop-driven pattern. Address mirror types by introducing a small shared types package or using interfaces.

## Standard Stack

This phase is pure refactoring -- no new dependencies are needed. The existing stack remains:

### Core
| Library | Purpose | Relevance to DRY |
|---------|---------|-------------------|
| cobra | CLI framework | Command structure stays, RunE functions shrink |
| viper | Configuration | Config loading gets centralized |
| azure-sdk-for-go | Auth/Graph | Client creation gets extracted to shared helper |

### Tools for Refactoring
| Tool | Purpose | Why Use It |
|------|---------|------------|
| `go vet` | Static analysis | Catch issues from refactoring |
| `go test ./...` | Test suite | Regression guard during refactoring |
| `gofmt` / `goimports` | Formatting | Keep code clean post-refactor |

## Architecture Patterns

### Duplication Map (from code analysis)

#### DUP-1: Command Bootstrap Sequence (HIGH impact, ~120 duplicated lines)
The following 5-step sequence appears nearly identically in `plan.go`, `apply.go`, `drift.go`, `rollback.go`:
```
1. config.LoadFromGlobal()
2. validate tenant != ""
3. auth.NewClientFactory() + factory.Credential()
4. graph.NewClient() + state.NewGitBackend()
5. ReadDesiredPolicies() + graphClient.ListPolicies()
```

**Files affected:** `cmd/plan.go` (lines 35-125), `cmd/apply.go` (lines 43-144), `cmd/drift.go` (lines 33-136), `cmd/rollback.go` (lines 51-133)

Each command does this with slight variations in error wrapping but identical structure. The `status.go` command has a variant that does graceful degradation on auth failure.

#### DUP-2: Live Policy Normalization Loop (~15 lines, 3 copies)
This exact loop appears in `plan.go`, `apply.go`, and `drift.go`:
```go
livePolicies := make(map[string]reconcile.LivePolicy)
for _, p := range livePoliciesGraph {
    normalized, err := normalize.Normalize(p.RawJSON)
    // ... json.Unmarshal ...
    livePolicies[p.ID] = reconcile.LivePolicy{...}
}
```

#### DUP-3: Semver Bump Computation (~30 lines, 2 copies)
The semver field config loading + action iteration + FieldDiff conversion appears identically in `plan.go` (lines 128-171) and `apply.go` (lines 150-189).

#### DUP-4: Validation Execution + Type Conversion (~15 lines, 2 copies)
The `validate.PolicyAction` conversion loop + `validate.ValidatePlan` call appears identically in `plan.go` (lines 173-187) and `apply.go` (lines 192-204).

#### DUP-5: Display Name Resolution (~15 lines, 2 copies)
The `resolve.CollectRefs` + `resolve.NewResolver` + `resolver.ResolveAll` sequence appears identically in `plan.go` (lines 189-204) and `apply.go` (lines 209-223).

#### DUP-6: Plan Rendering + Output Format Switch (~10 lines, 3 copies)
The `if cfg.Output == "json" { RenderPlanJSON } else { RenderPlan }` pattern appears in `plan.go`, `apply.go`, and `drift.go`.

#### DUP-7: Apply Action Handlers (~120 lines in apply.go)
The three action cases (Create/Update/Recreate) in `apply.go` lines 298-461 share an identical post-Graph-API pattern:
```
1. json.Marshal + backend.WritePolicy
2. backend.CreateVersionTag
3. manifest.Policies[slug] = state.Entry{...}  (identical struct literal)
4. state.WriteManifest
```
Only the Graph API call and version computation differ.

#### DUP-8: Validation Error Check (~8 lines, 2 copies)
The `hasValidationErrors` loop + ExitError return appears in `plan.go` (lines 217-229) and `apply.go` (lines 236-248).

#### DUP-9: Mirror Type Definitions (structural duplication)
- `semver.FieldDiff` mirrors `reconcile.FieldDiff` (3 fields: Path, OldValue, NewValue)
- `validate.ActionType` mirrors `reconcile.ActionType` (5 constants)
- `validate.PolicyAction` mirrors a subset of `reconcile.PolicyAction`

These exist to avoid circular imports but create maintenance burden.

#### DUP-10: bumpPatchVersion (exact duplicate)
- `cmd/import.go` line 323: `bumpPatchVersion(version string) string`
- `internal/state/backend.go` line 94: `bumpPatch(version string) string`

Both do the same thing with slightly different implementations.

#### DUP-11: Diff Summary Logic (~20 lines, 2 copies)
- `output/diff.go` `DiffSummary()` -- collects top-level field paths, deduplicates, formats summary
- `cmd/history.go` `computeDiffSummaries()` -- has an inline version of the same field-collection logic

#### DUP-12: History JSON Output Structure (~20 lines, 2 copies)
- `cmd/history.go` `runHistorySinglePolicy()` -- defines `historyEntry` struct and renders JSON
- `cmd/status.go` `runStatusHistory()` -- defines nearly identical `historyEntry` struct and renders JSON

### Recommended Refactoring Structure

```
cmd/
  pipeline.go        # NEW: shared bootstrap + pipeline helpers
  apply.go           # Shrinks from 567 to ~200 lines
  plan.go            # Shrinks from 246 to ~60 lines
  drift.go           # Shrinks from 205 to ~60 lines
  ...
internal/
  reconcile/
    types.go         # Move shared types here (FieldDiff, ActionType)
  pipeline/          # ALTERNATIVE: new package for command pipeline
    pipeline.go      # Bootstrap, normalization, semver, validation, rendering
```

### Pattern 1: Command Pipeline Helper
**What:** Extract repeated command bootstrap into a reusable struct/function
**When to use:** Any command that needs config + auth + graph + backend + manifest
**Example:**
```go
// cmd/pipeline.go
type CommandPipeline struct {
    Cfg         *types.Config
    GraphClient *graph.Client
    Backend     *state.GitBackend
    Manifest    *state.Manifest
    UseColor    bool
}

func NewPipeline(ctx context.Context) (*CommandPipeline, error) {
    cfg, err := config.LoadFromGlobal()
    if err != nil {
        return nil, fmt.Errorf("loading config: %w", err)
    }
    if cfg.Tenant == "" {
        return nil, &types.ExitError{
            Code:    types.ExitFatalError,
            Message: "tenant is required: ...",
        }
    }
    factory, err := auth.NewClientFactory(cfg.Auth)
    // ... auth, graph client, backend, manifest ...
    return &CommandPipeline{...}, nil
}

// LoadLivePolicies fetches and normalizes live policies
func (p *CommandPipeline) LoadLivePolicies(ctx context.Context) (map[string]reconcile.LivePolicy, error) { ... }

// ComputeSemverBumps adds version info to update actions
func (p *CommandPipeline) ComputeSemverBumps(actions []reconcile.PolicyAction) error { ... }

// RunValidations converts and validates actions
func (p *CommandPipeline) RunValidations(actions []reconcile.PolicyAction) []validate.ValidationResult { ... }

// RenderPlan outputs the plan in the configured format
func (p *CommandPipeline) RenderPlan(w io.Writer, actions []reconcile.PolicyAction, ...) { ... }
```

### Pattern 2: Apply Action Consolidation
**What:** Replace the 3 nearly-identical switch cases with a unified post-action handler
**Example:**
```go
// After Graph API call succeeds for any action type:
func (p *CommandPipeline) RecordAppliedAction(ctx context.Context, slug string, objectID string, version string, data map[string]interface{}) error {
    backendJSON, _ := json.Marshal(data)
    sha, err := p.Backend.WritePolicy(p.Cfg.Tenant, slug, backendJSON)
    if err != nil { return ... }
    actualVersion, err := p.Backend.CreateVersionTag(p.Cfg.Tenant, slug, version, sha, ...)
    if err != nil { return ... }
    p.Manifest.Policies[slug] = state.Entry{
        Slug: slug, Tenant: p.Cfg.Tenant, LiveObjectID: objectID,
        Version: actualVersion, LastDeployed: time.Now().UTC().Format(time.RFC3339),
        DeployedBy: deployerIdentity(p.Cfg), AuthMode: p.Cfg.Auth.Mode,
        BackendSHA: sha,
    }
    return state.WriteManifest(p.Backend, p.Cfg.Tenant, p.Manifest)
}
```

### Pattern 3: Eliminate Mirror Types
**What:** Move shared types to a common location to avoid mirroring
**Option A:** Move `FieldDiff` and `ActionType` to `pkg/types` (already exists)
**Option B:** Have `validate` and `semver` accept interfaces instead of concrete types
**Recommended:** Option A -- simplest, and `pkg/types` already serves this role

### Anti-Patterns to Avoid
- **Over-abstracting small commands:** `init.go` and `desired.go` are small and unique; don't force them into the pipeline pattern
- **Breaking test contracts:** Many tests mock specific behaviors; ensure extracted helpers are testable
- **Creating God objects:** The pipeline helper should be a simple struct, not a do-everything framework
- **Changing public APIs:** Internal package APIs visible to tests should maintain compatibility

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Patch version bumping | Multiple `bumpPatchVersion` implementations | `semver.BumpVersion(v, semver.BumpPatch)` | Already exists in semver package |
| Diff summary formatting | Inline field-collection logic | `output.DiffSummary()` | Already exists in output package |

## Common Pitfalls

### Pitfall 1: Breaking Tests During Refactoring
**What goes wrong:** Moving functions between files/packages breaks imports in test files
**Why it happens:** Tests import specific packages and reference specific function signatures
**How to avoid:** Run `go test ./...` after every extraction. Keep a running test pass throughout
**Warning signs:** Test files importing moved packages; function signature changes

### Pitfall 2: Circular Import Introduction
**What goes wrong:** Moving types to resolve mirror-type duplication creates circular imports
**Why it happens:** `reconcile` imports `state`, so `state` cannot import `reconcile`
**How to avoid:** Use `pkg/types` as the shared type home (it has no internal imports). Map concrete analysis: reconcile->state (one-way), validate->nothing, semver->nothing, output->reconcile+validate+resolve
**Warning signs:** `import cycle not allowed` errors

### Pitfall 3: Status Command's Graceful Degradation
**What goes wrong:** Extracting the pipeline helper breaks status.go's special auth-failure handling
**Why it happens:** Status degrades gracefully when auth fails; other commands fail hard
**How to avoid:** Make the pipeline helper accept options (e.g., `WithGracefulAuth()`) or keep status.go partially separate
**Warning signs:** Status command failing when offline instead of showing "unknown" sync status

### Pitfall 4: Import Command's Multi-Tenant Pattern
**What goes wrong:** Import uses `runForTenants` which creates its own auth/cred per tenant
**Why it happens:** Import was built with multi-tenant support; other commands use single-tenant
**How to avoid:** Don't force import into the single-tenant pipeline pattern; leave `runForTenants` as-is
**Warning signs:** Import breaking for multi-tenant configurations

### Pitfall 5: Scope Creep
**What goes wrong:** DRY refactoring turns into feature changes or architecture overhaul
**Why it happens:** While reading code to consolidate, developer spots "better" ways to do things
**How to avoid:** Strict rule: behavior-preserving changes only. No new features, no API changes, no dependency upgrades
**Warning signs:** Changing logic, adding new capabilities, modifying output formats

## Code Examples

### Example: Extracting the Normalization Loop
```go
// Before (in plan.go, apply.go, drift.go):
livePolicies := make(map[string]reconcile.LivePolicy)
for _, p := range livePoliciesGraph {
    normalized, err := normalize.Normalize(p.RawJSON)
    if err != nil {
        return fmt.Errorf("normalizing live policy %s: %w", p.ID, err)
    }
    var m map[string]interface{}
    if err := json.Unmarshal(normalized, &m); err != nil {
        return fmt.Errorf("parsing normalized policy %s: %w", p.ID, err)
    }
    livePolicies[p.ID] = reconcile.LivePolicy{
        NormalizedData: m,
        Slug:           p.DisplayName,
    }
}

// After (shared helper):
func NormalizeLivePolicies(policies []graph.Policy) (map[string]reconcile.LivePolicy, error) {
    result := make(map[string]reconcile.LivePolicy, len(policies))
    for _, p := range policies {
        normalized, err := normalize.Normalize(p.RawJSON)
        if err != nil {
            return nil, fmt.Errorf("normalizing live policy %s: %w", p.ID, err)
        }
        var m map[string]interface{}
        if err := json.Unmarshal(normalized, &m); err != nil {
            return nil, fmt.Errorf("parsing normalized policy %s: %w", p.ID, err)
        }
        result[p.ID] = reconcile.LivePolicy{
            NormalizedData: m,
            Slug:           p.DisplayName,
        }
    }
    return result, nil
}
```

### Example: Consolidating bumpPatchVersion
```go
// Remove cmd/import.go bumpPatchVersion and state/backend.go bumpPatch
// Replace all call sites with:
newVersion, err := semver.BumpVersion(currentVersion, semver.BumpPatch)
if err != nil {
    newVersion = "1.0.1" // fallback for malformed versions
}
```

## Quantitative Impact Estimate

| Duplication | Current Lines | After DRY | Lines Saved |
|-------------|--------------|-----------|-------------|
| DUP-1: Bootstrap sequence | ~120 x 4 = 480 | ~120 + 4x5 = 140 | ~340 |
| DUP-2: Normalization loop | ~15 x 3 = 45 | 15 + 3x1 = 18 | ~27 |
| DUP-3: Semver computation | ~30 x 2 = 60 | 30 + 2x1 = 32 | ~28 |
| DUP-4: Validation execution | ~15 x 2 = 30 | 15 + 2x1 = 17 | ~13 |
| DUP-5: Display name resolution | ~15 x 2 = 30 | 15 + 2x1 = 17 | ~13 |
| DUP-6: Render format switch | ~10 x 3 = 30 | 10 + 3x1 = 13 | ~17 |
| DUP-7: Apply action handlers | ~120 | ~50 | ~70 |
| DUP-8: Validation error check | ~8 x 2 = 16 | 8 + 2x1 = 10 | ~6 |
| DUP-9: Mirror types | ~30 | 0 | ~30 |
| DUP-10: bumpPatchVersion | ~24 | 0 | ~24 |
| DUP-11: Diff summary logic | ~20 | 0 | ~20 |
| DUP-12: History JSON output | ~20 | ~10 | ~10 |
| **Total** | **~885** | **~285** | **~598** |

Estimated net reduction: ~600 lines (~6% of total codebase), concentrated in `cmd/` layer.

## Recommended Plan Structure

### Plan 07-01: Extract Command Pipeline Helper
- Create `cmd/pipeline.go` with `CommandPipeline` struct
- Extract bootstrap sequence (config, auth, graph, backend, manifest)
- Extract `NormalizeLivePolicies` helper
- Refactor `plan.go` to use pipeline (first consumer, validates the pattern)
- Run full test suite

### Plan 07-02: Consolidate Plan/Apply/Drift Shared Logic
- Refactor `drift.go` and `apply.go` to use pipeline
- Extract semver computation helper
- Extract validation execution helper
- Extract display name resolution helper
- Extract render format switch helper
- Handle status.go graceful degradation variant

### Plan 07-03: Consolidate Apply Action Handlers + Eliminate Duplicates
- Extract `RecordAppliedAction` to consolidate create/update/recreate post-processing
- Replace `bumpPatchVersion` with `semver.BumpVersion`
- Consolidate history JSON output structure
- Use `output.DiffSummary` in history command

### Plan 07-04: Eliminate Mirror Types
- Move `FieldDiff` to `pkg/types` (or create interface)
- Move `ActionType` to `pkg/types`
- Remove `validate.ActionType`, `validate.PolicyAction`, `semver.FieldDiff`
- Update all import paths
- Run full test suite

## Open Questions

1. **Pipeline helper placement: `cmd/pipeline.go` vs `internal/pipeline/`**
   - What we know: Keeping it in `cmd/` is simpler (same package, no new imports needed)
   - What's unclear: Whether future phases might need pipeline logic outside `cmd/`
   - Recommendation: Start in `cmd/pipeline.go`; move to `internal/` only if needed later

2. **Status command integration depth**
   - What we know: Status has unique graceful-degradation auth handling
   - What's unclear: How much of the pipeline it can share vs. must keep separate
   - Recommendation: Use pipeline for config/backend/manifest; keep auth logic separate in status

## Sources

### Primary (HIGH confidence)
- Direct codebase analysis of all 68 Go files (line-by-line review)
- All duplication claims verified by reading both occurrences in full

### Notes
- No external research needed -- this is a codebase-internal analysis phase
- All line counts and duplication patterns verified against current working tree
- Import dependency graph verified: reconcile->state (one-way), no circular risk for proposed changes

## Metadata

**Confidence breakdown:**
- Duplication identification: HIGH - verified by reading every source file
- Refactoring patterns: HIGH - standard Go patterns, no novel techniques
- Line count estimates: MEDIUM - rough estimates based on typical extraction overhead
- Plan ordering: HIGH - dependency chain is clear (pipeline first, consumers second, types last)

**Research date:** 2026-03-06
**Valid until:** No expiry (codebase-specific analysis, valid until code changes)
