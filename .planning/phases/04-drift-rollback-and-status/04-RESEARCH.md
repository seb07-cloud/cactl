# Phase 4: Drift, Rollback, and Status - Research

**Researched:** 2026-03-04
**Domain:** Drift detection (backend vs live comparison), version rollback from Git annotated tags, status reporting with sync state
**Confidence:** HIGH

## Summary

Phase 4 builds three CLI commands (`cactl drift`, `cactl rollback`, `cactl status`) on top of the reconciliation engine, state backend, and Graph API client established in Phases 1-3. The core insight is that **drift detection is a read-only reconciliation** -- it reuses the exact same `reconcile.Reconcile()` engine from Phase 3 but never writes to Graph or state. Rollback reads policy JSON from Git annotated tags (already created by Phase 2 import and Phase 3 apply), diffs against live state, and executes a PATCH -- it is a "targeted apply from historical version." Status aggregates manifest entries with live state comparison to produce a dashboard-style table.

All three commands reuse existing infrastructure heavily. No new external dependencies are needed. The primary new code is: (1) a `ListVersionTags` method on `GitBackend` to enumerate tag history for a policy, (2) a `ReadTagBlob` method to read policy JSON from a specific annotated tag, (3) the `cmd/drift.go`, `cmd/rollback.go`, and `cmd/status.go` command files, and (4) a status output formatter.

**Primary recommendation:** Maximize reuse of existing `reconcile.Reconcile()`, `output.RenderPlan`, and `state.GitBackend` -- drift is reconcile-without-write, rollback is apply-from-tag, status is manifest-read-with-live-check.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CLI-05 | User can run `cactl drift` to check for drift without making changes | Drift command reuses `reconcile.Reconcile()` in read-only mode; no Graph writes, no state writes |
| CLI-06 | User can run `cactl rollback` to restore a prior policy version from Git tag history | New `ListVersionTags` + `ReadTagBlob` on GitBackend, then standard plan/confirm/PATCH flow from apply |
| CLI-07 | User can run `cactl status` to see tracked policies with version, timestamp, deployer, and sync status | Reads manifest entries, optionally compares against live for sync status column |
| DRIFT-01 | `cactl drift` outputs diff between backend state and live tenant without making changes | Reconcile engine produces actions; RenderPlan displays them; no write path executed |
| DRIFT-02 | Drift types identified: policy modified (~), policy missing (-/+), untracked policy (?) | Reconcile engine already classifies Update (~), Recreate (-/+), Untracked (?); drift reuses these |
| DRIFT-03 | Exit codes: 0=no drift, 1=drift detected, 2=error | ExitSuccess (0) when all noop, ExitChanges (1) when actionable items exist, ExitFatalError (2) on error |
| DRIFT-04 | Three remediation options presented: remediate (apply backend), import live (update backend), report only | Drift output footer suggests three commands: `cactl apply`, `cactl import --force`, or report-only (current run) |
| ROLL-01 | `cactl rollback --policy <slug> --version <semver>` reads policy JSON from annotated tag | `git cat-file blob <tag>^{}` dereferences annotated tag to blob content; new ReadTagBlob method |
| ROLL-02 | Rollback runs plan diff against current live state and presents for confirmation | Load tag JSON + live JSON, compute diff with `reconcile.ComputeDiff`, render with `output.RenderPlan` |
| ROLL-03 | On confirmation: PATCHes live policy, writes new state manifest entry | Reuses `graph.UpdatePolicy` (PATCH), `backend.WritePolicy`, `backend.CreateVersionTag`, `state.WriteManifest` |
| ROLL-04 | Tag history is never modified -- full audit trail preserved; rollback becomes new deployment event | Rollback creates a NEW tag with bumped version pointing to the old blob content; existing tags untouched |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| encoding/json (stdlib) | go1.24 | JSON marshal/unmarshal for policy comparison | Already used throughout codebase for all JSON operations |
| os/exec (stdlib) | go1.24 | Git plumbing commands for tag listing and blob reading | Decision 02-02: os/exec git plumbing over go-git |
| github.com/spf13/cobra | latest | CLI command registration (drift, rollback, status) | Already the CLI framework; all commands use it |
| github.com/spf13/viper | latest | Config loading (same pattern as all other commands) | Already the config framework |
| github.com/stretchr/testify | latest | Test assertions | Already used in all test files |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| internal/reconcile | existing | Reconcile() and ComputeDiff() for drift and rollback diff | Drift: full Reconcile(); Rollback: ComputeDiff() for targeted diff |
| internal/output | existing | RenderPlan() for drift display, new RenderStatus() for status | Drift reuses plan renderer; status needs new table formatter |
| internal/state | existing | GitBackend, Manifest, ReadManifest, WriteManifest | All three commands read manifest; rollback writes manifest |
| internal/graph | existing | ListPolicies, GetPolicy, UpdatePolicy | Drift/status fetch live state; rollback PATCHes |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| git tag --list + grep for tag enumeration | git for-each-ref refs/tags/cactl/ | for-each-ref is more structured, supports --format, and already used in codebase for ref listing |
| Custom diff for rollback | reconcile.ComputeDiff | ComputeDiff already handles nested JSON maps with dot-path output -- no reason to build another |
| tablewriter library for status | Manual fmt.Fprintf with padding | Zero-dependency approach (consistent with codebase); status table is simple enough for manual formatting |

## Architecture Patterns

### Recommended Project Structure

```
cmd/
├── drift.go          # cactl drift command
├── rollback.go       # cactl rollback command
└── status.go         # cactl status command
internal/
├── state/
│   └── backend.go    # Add ListVersionTags, ReadTagBlob methods
└── output/
    └── status.go     # Status table renderer (human + JSON)
pkg/
└── types/
    └── status.go     # StatusOutput, PolicyStatus types for JSON output
```

### Pattern 1: Drift as Read-Only Reconcile

**What:** `cactl drift` executes the same reconciliation pipeline as `cactl plan` but never enters the write phase. It compares backend state (Git refs) against live tenant (Graph API) and renders the result.

**When to use:** Drift detection, CI scheduled checks, pre-deploy validation.

**Implementation:**
```go
// cmd/drift.go -- runDrift follows same flow as runPlan but:
// 1. No semver computation needed (drift reports what IS, not what WILL BE)
// 2. No validation needed (drift is observational)
// 3. Adds remediation suggestions footer
// 4. Exit code: 0=no drift, 1=drift detected

func runDrift(cmd *cobra.Command, args []string) error {
    // Load config, validate tenant, create clients (identical to plan)
    // Load backend policies, live policies, manifest (identical to plan)
    // Reconcile to get actions (identical to plan)

    // Filter to drift-relevant actions (exclude noop)
    actionable := filterActionable(actions)
    if len(actionable) == 0 {
        r.Success("No drift detected. Backend and live state are in sync.")
        return nil // exit 0
    }

    // Render diff (reuse RenderPlan or a DriftPlan variant)
    output.RenderDrift(os.Stdout, actions, useColor)

    // Print remediation suggestions (DRIFT-04)
    r.Print("\nRemediation options:")
    r.Print("  cactl apply          -- apply backend state to live tenant")
    r.Print("  cactl import --force -- update backend from live tenant")
    r.Print("  (no action)          -- report only (this run)")

    return &types.ExitError{Code: types.ExitChanges, Message: "drift detected"}
}
```

**Key insight:** The reconcile engine direction is backend-as-desired, live-as-actual. This naturally produces drift: if live differs from backend, that is drift. The sigils already map correctly: ~ = modified in live, -/+ = missing from live (deleted out-of-band), ? = exists in live but not tracked.

### Pattern 2: Rollback as Apply-from-Tag

**What:** `cactl rollback` reads a historical policy version from a Git annotated tag, diffs it against the current live state, and applies it like a normal `cactl apply` for a single policy.

**When to use:** Reverting a bad deployment, restoring a known-good configuration.

**Implementation:**
```go
// cmd/rollback.go
func runRollback(cmd *cobra.Command, args []string) error {
    slug, _ := cmd.Flags().GetString("policy")
    version, _ := cmd.Flags().GetString("version")

    // Step 1: Read policy JSON from annotated tag
    tagJSON, err := backend.ReadTagBlob(tenantID, slug, version)

    // Step 2: Get current live state for this policy
    entry := manifest.Policies[slug]
    livePolicy, err := graphClient.GetPolicy(ctx, entry.LiveObjectID)
    liveNormalized, _ := normalize.Normalize(livePolicy.RawJSON)

    // Step 3: Compute diff between tag version and live
    var tagMap, liveMap map[string]interface{}
    json.Unmarshal(tagJSON, &tagMap)
    json.Unmarshal(liveNormalized, &liveMap)
    diffs := reconcile.ComputeDiff(tagMap, liveMap)

    // Step 4: Display diff and confirm
    // Step 5: PATCH live policy
    // Step 6: Create NEW version tag (bumped), update manifest
    // ROLL-04: Never modify existing tags
}
```

### Pattern 3: Git Tag Operations for Version History

**What:** Reading and listing annotated Git tags to retrieve historical policy versions.

**When to use:** Rollback (read specific version), status (show current version and history).

**Implementation -- new methods on GitBackend:**
```go
// ListVersionTags returns all version tags for a policy, sorted by semver.
// Tag format: cactl/<tenant>/<slug>/<version>
func (b *GitBackend) ListVersionTags(tenantID, slug string) ([]VersionTag, error) {
    prefix := fmt.Sprintf("cactl/%s/%s/", tenantID, slug)
    // git tag --list '<prefix>*' --sort=-version:refname
    cmd := exec.Command("git", "tag", "--list", prefix+"*",
        "--sort=-version:refname",
        "--format=%(refname:strip=0)%09%(creatordate:iso)%09%(contents:lines=1)")
    cmd.Dir = b.repoDir
    // Parse output into VersionTag structs
}

// ReadTagBlob reads the policy JSON blob content from an annotated tag.
// Uses git cat-file to dereference the tag to its blob.
func (b *GitBackend) ReadTagBlob(tenantID, slug, version string) ([]byte, error) {
    tagName := fmt.Sprintf("cactl/%s/%s/%s", tenantID, slug, version)
    // Annotated tags point to tag objects, not directly to blobs.
    // The tag object contains a pointer to the blob.
    // git cat-file blob <tagname>^{} dereferences to the underlying object.
    // BUT: our tags point to blobs (not commits), so ^{} dereferences
    // the tag object to the blob directly.
    cmd := exec.Command("git", "cat-file", "blob", tagName+"^{}")
    cmd.Dir = b.repoDir
    return cmd.Output()
}
```

**Critical detail about tag dereferencing:** In the current codebase, `CreateVersionTag` does `git tag -a tagName blobHash -m message`. This creates an annotated tag object that points to a blob (not a commit). To read the blob content back:
- `git cat-file -t <tagname>` returns "tag" (it's an annotated tag)
- `git cat-file -t <tagname>^{}` returns "blob" (the underlying object)
- `git cat-file blob <tagname>^{}` returns the blob content (the policy JSON)

This is verified by the existing test `TestCreateVersionTag` in `backend_test.go` which confirms the tag points to a blob.

### Pattern 4: Status Table Output

**What:** `cactl status` reads the manifest and optionally compares against live to show sync status.

**When to use:** Dashboard view of all tracked policies.

**Implementation:**
```go
// Status table columns:
// POLICY          VERSION  LAST DEPLOYED         DEPLOYED BY    SYNC
// ca-mfa-admins   1.2.0    2026-03-04T10:30:00Z  az-cli         in-sync
// ca-block-legacy 1.0.0    2026-03-03T14:15:00Z  client-secret  drifted
// ca-require-mfa  2.0.0    2026-03-04T09:00:00Z  az-cli         unknown

// Sync status is determined by:
// - "in-sync": backend SHA matches live normalized JSON
// - "drifted": backend SHA differs from live normalized JSON
// - "missing": live policy not found (deleted out-of-band)
// - "unknown": could not check (--offline flag or Graph error)
```

### Pattern 5: Rollback Version Bumping (ROLL-04)

**What:** Rollback never modifies existing tags. Instead, it creates a new deployment event with a bumped version.

**Implementation:**
```go
// Example rollback flow:
// Current state: ca-mfa-admins is at v2.1.0 (tag exists)
// User runs: cactl rollback --policy ca-mfa-admins --version 1.0.0
//
// 1. Read v1.0.0 tag blob -> get the old policy JSON
// 2. Diff v1.0.0 JSON against current live state
// 3. Confirm with user
// 4. PATCH live policy with v1.0.0 JSON
// 5. Write policy blob to backend (same content as v1.0.0)
// 6. Create NEW tag: cactl/<tenant>/ca-mfa-admins/2.2.0 (bumped from current 2.1.0)
//    Tag message: "cactl rollback: ca-mfa-admins 2.2.0 (rolled back to 1.0.0)"
// 7. Update manifest: version=2.2.0, last_deployed=now
//
// Result: Full audit trail preserved. Tags 1.0.0, 2.0.0, 2.1.0, 2.2.0 all exist.
// Tag 2.2.0 happens to contain the same blob content as 1.0.0.
```

The version bump level for rollback should be PATCH (it is a restoration, not a feature change). The tag message should indicate it was a rollback for traceability.

### Anti-Patterns to Avoid

- **Deleting or force-updating Git tags for rollback:** ROLL-04 explicitly requires tag history never be modified. Rollback creates a new forward version, not a rewrite.
- **Building a separate diff engine for drift:** The reconcile engine already does exactly what drift needs. Duplicating this logic creates maintenance burden and inconsistency.
- **Fetching all live policies for status:** Status should use `graphClient.GetPolicy(ctx, entry.LiveObjectID)` per policy for sync check, not `ListPolicies()`, because status is per-tracked-policy. However, for performance with many policies, `ListPolicies()` once and indexing by ID is better. Use `ListPolicies()` with index-by-ID.
- **Making status sync check mandatory:** Status should work offline (manifest-only) with a `--check-sync` flag or default to checking sync but gracefully degrading on auth failure.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Backend-vs-live comparison | Custom comparison logic | `reconcile.Reconcile()` | Already handles all 5 action types with full idempotency truth table |
| Diff rendering for drift | Separate drift renderer | `output.RenderPlan()` or thin wrapper | Same sigils, same coloring, same format as plan |
| Tag content reading | Custom Git object parsing | `git cat-file blob <tag>^{}` via os/exec | Git plumbing handles tag dereferencing correctly |
| Version sorting for tag listing | Manual semver sort | `git tag --sort=-version:refname` | Git has built-in semver-aware sorting since v2.12 |
| Status table alignment | tablewriter dependency | `fmt.Fprintf` with `text/tabwriter` (stdlib) | tabwriter is stdlib, handles column alignment |

**Key insight:** Phase 4 is primarily a composition phase. The building blocks (reconcile, graph client, state backend, output renderer) are all done. The new code is command wiring and a few Git plumbing extensions.

## Common Pitfalls

### Pitfall 1: Tag Dereference Syntax

**What goes wrong:** Using `git cat-file blob <tagname>` on an annotated tag returns an error because the tag ref points to a tag object, not directly to a blob.
**Why it happens:** Annotated tags are two-level: ref -> tag object -> target object. Lightweight tags are one-level: ref -> target object.
**How to avoid:** Always use `<tagname>^{}` suffix to dereference annotated tags to their target object. For cactl, the target is a blob.
**Warning signs:** `git cat-file -t <tagname>` returns "tag" instead of "blob". If you see "tag", you need `^{}`.

### Pitfall 2: Drift Direction Confusion

**What goes wrong:** Drift could be interpreted as "what changed in live since last apply" or "what needs to change in live to match backend." These produce different output directions.
**Why it happens:** The reconciliation engine uses backend-as-desired, live-as-actual. This means drift output shows "what would change if you applied backend to live." This is the correct direction for DRIFT-01.
**How to avoid:** Drift reuses the reconcile engine as-is. The sigils mean: ~ = "live differs from backend (would be updated)", -/+ = "live policy missing (would be recreated)", ? = "live has policy not in backend (untracked)."
**Warning signs:** If drift shows "+" (create), that means a policy exists in backend but was never applied -- this is technically not drift but rather an unapplied policy. Consider filtering or annotating differently.

### Pitfall 3: Rollback to Non-Existent Version

**What goes wrong:** User specifies `--version 3.0.0` but tag `cactl/<tenant>/<slug>/3.0.0` does not exist.
**Why it happens:** Typo, or the version was never deployed.
**How to avoid:** Validate tag existence before proceeding. On failure, list available versions with `ListVersionTags` and suggest the closest match.
**Warning signs:** `git cat-file` returns error for non-existent tag.

### Pitfall 4: Status Sync Check Performance

**What goes wrong:** Checking sync status for 50+ policies by calling `GetPolicy` individually results in slow execution (50 HTTP calls).
**Why it happens:** Graph API does not support batch GET for individual policy IDs.
**How to avoid:** Call `ListPolicies()` once to get all live policies, then index by ID for O(1) lookup per tracked policy. The list endpoint returns all policies in a single paginated call.
**Warning signs:** Status command taking >10 seconds for moderate policy counts.

### Pitfall 5: Rollback Without Live Object ID

**What goes wrong:** Manifest entry has a LiveObjectID, but the policy was deleted out-of-band. Rollback tries to PATCH a non-existent policy.
**Why it happens:** Between the last apply/import and the rollback, someone deleted the policy from Entra.
**How to avoid:** Before PATCHing, verify the policy exists by calling `GetPolicy`. If 404, offer to CREATE instead (like a recreate action). The rollback message should indicate this was a recreate-from-rollback.
**Warning signs:** Graph API returns 404 on PATCH.

## Code Examples

Verified patterns from the existing codebase:

### Reading Policy from Annotated Tag

```go
// Source: internal/state/backend.go pattern + git tag documentation
// ReadTagBlob reads the blob content pointed to by an annotated version tag.
func (b *GitBackend) ReadTagBlob(tenantID, slug, version string) ([]byte, error) {
    tagName := fmt.Sprintf("cactl/%s/%s/%s", tenantID, slug, version)
    cmd := exec.Command("git", "cat-file", "blob", tagName+"^{}")
    cmd.Dir = b.repoDir
    out, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("reading tag %s: %w", tagName, err)
    }
    return out, nil
}
```

### Listing Version Tags with Metadata

```go
// Source: git-for-each-ref documentation + existing forEachRef pattern
// VersionTag holds metadata for a single version tag.
type VersionTag struct {
    Version   string
    Timestamp string
    Message   string
}

func (b *GitBackend) ListVersionTags(tenantID, slug string) ([]VersionTag, error) {
    prefix := fmt.Sprintf("refs/tags/cactl/%s/%s/", tenantID, slug)
    cmd := exec.Command("git", "for-each-ref",
        "--format=%(refname:strip=4)\t%(creatordate:iso)\t%(contents:lines=1)",
        "--sort=-version:refname",
        prefix)
    cmd.Dir = b.repoDir
    out, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("listing tags: %w", err)
    }
    // Parse tab-separated output into VersionTag structs
    // strip=4 removes "refs/tags/cactl/<tenant>/" leaving "<slug>/<version>"
    // Then extract version from last path component
}
```

### Drift Command Skeleton (reusing plan pipeline)

```go
// Source: cmd/plan.go pattern (from 03-04-PLAN.md)
func runDrift(cmd *cobra.Command, args []string) error {
    // Steps 1-8 are identical to runPlan (load config, auth, graph, backend, reconcile)
    // ... shared setup code ...

    actions := reconcile.Reconcile(backendPolicies, livePolicies, manifest)

    // Drift-specific: no semver, no validation, just display + exit code
    actionable := filterActionable(actions)
    if len(actionable) == 0 {
        r.Success("No drift detected.")
        return nil // exit 0
    }

    // Reuse plan renderer for consistent output
    output.RenderPlan(os.Stdout, actions, nil, nil, useColor)

    // DRIFT-04: remediation suggestions
    fmt.Fprintln(os.Stdout, "\nRemediation options:")
    fmt.Fprintln(os.Stdout, "  cactl apply            apply backend state to live tenant")
    fmt.Fprintln(os.Stdout, "  cactl import --force   update backend from live tenant state")
    fmt.Fprintln(os.Stdout, "  (no action)            this was a report-only check")

    return &types.ExitError{Code: types.ExitChanges, Message: "drift detected"}
}
```

### Status Table Rendering

```go
// Source: text/tabwriter stdlib documentation
func RenderStatus(w io.Writer, entries []PolicyStatus, useColor bool) {
    tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
    fmt.Fprintln(tw, "POLICY\tVERSION\tLAST DEPLOYED\tDEPLOYED BY\tSYNC")
    for _, e := range entries {
        syncLabel := e.SyncStatus
        if useColor {
            switch e.SyncStatus {
            case "in-sync":
                syncLabel = colorGreen + "in-sync" + colorReset
            case "drifted":
                syncLabel = colorYellow + "drifted" + colorReset
            case "missing":
                syncLabel = colorRed + "missing" + colorReset
            }
        }
        fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
            e.Slug, e.Version, e.LastDeployed, e.DeployedBy, syncLabel)
    }
    tw.Flush()
}
```

### Status JSON Output Types

```go
// Source: pkg/types/plan.go pattern (from 03-04-PLAN.md)
type StatusOutput struct {
    SchemaVersion int            `json:"schema_version"`
    Tenant        string         `json:"tenant"`
    Policies      []PolicyStatus `json:"policies"`
    Summary       StatusSummary  `json:"summary"`
}

type PolicyStatus struct {
    Slug         string `json:"slug"`
    Version      string `json:"version"`
    LastDeployed string `json:"last_deployed"`
    DeployedBy   string `json:"deployed_by"`
    SyncStatus   string `json:"sync_status"` // "in-sync", "drifted", "missing", "unknown"
    LiveObjectID string `json:"live_object_id"`
}

type StatusSummary struct {
    Total   int `json:"total"`
    InSync  int `json:"in_sync"`
    Drifted int `json:"drifted"`
    Missing int `json:"missing"`
    Unknown int `json:"unknown"`
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Separate diff engine per command | Single reconcile engine shared across plan/apply/drift | Phase 3 design | Phase 4 drift reuses reconcile without modification |
| Store rollback targets in separate state | Annotated Git tags with immutable history | Phase 2 design | Rollback reads from tags directly; no extra state needed |
| Interactive-only rollback | Rollback with --auto-approve for CI pipelines | Phase 4 | Enables scheduled rollback scripts and CI recovery |

**Deprecated/outdated:**
- None for Phase 4. All dependencies are from Phases 1-3 which were built fresh.

## Open Questions

1. **Should `cactl drift` support `--policy <slug>` for single-policy drift check?**
   - What we know: The reconcile engine processes all policies at once. Filtering to a single policy is a post-reconcile filter.
   - What's unclear: Whether this is a v1 requirement or can be deferred.
   - Recommendation: Implement `--policy <slug>` filter as a simple post-reconcile filter on drift output. Low effort, high utility for CI per-policy checks.

2. **Should `cactl status` sync check be opt-in or default?**
   - What we know: Sync check requires Graph API access (authentication). Status without sync check is purely manifest-based and works offline.
   - What's unclear: User expectation -- do they expect status to always show sync state?
   - Recommendation: Default to sync check when tenant is configured and auth succeeds. Fall back to "unknown" sync status on auth failure with a warning. No separate flag needed -- graceful degradation.

3. **What version bump level should rollback use?**
   - What we know: ROLL-04 says rollback becomes a new deployment event. The content is old but the version must be new.
   - What's unclear: Whether PATCH bump is always appropriate, or if rolling back to a significantly older version should trigger MINOR/MAJOR.
   - Recommendation: Always PATCH bump for rollback. The content change may be large, but the intent is "restore known-good" not "introduce new functionality." The tag message records the rollback source for context.

4. **Should `cactl status --history <slug>` show full version history?**
   - What we know: `ListVersionTags` can enumerate all versions. This is useful for choosing a rollback target.
   - What's unclear: Whether this belongs in status or in a separate `cactl history` command.
   - Recommendation: Add `--history <slug>` to status command for v1. If the flag is present, show version history table for that policy instead of the normal status table. This avoids a new top-level command.

## Sources

### Primary (HIGH confidence)
- Existing codebase: `internal/state/backend.go` -- GitBackend pattern, WritePolicy, ReadPolicy, CreateVersionTag, catFile, forEachRef
- Existing codebase: `internal/reconcile/engine.go` (planned Phase 3) -- Reconcile() function signature and action types
- Existing codebase: `internal/graph/policies.go` -- ListPolicies, GetPolicy, UpdatePolicy patterns
- Existing codebase: `internal/state/manifest.go` -- Manifest and Entry struct, ReadManifest, WriteManifest
- Existing codebase: `pkg/types/exitcodes.go` -- Exit code constants (ExitSuccess=0, ExitChanges=1, ExitFatalError=2)
- Existing codebase: `cmd/import.go` -- Command registration pattern, config loading, auth factory, graph client creation

### Secondary (MEDIUM confidence)
- Git documentation: `git-cat-file(1)` -- `^{}` dereference syntax for annotated tags to their target objects
- Git documentation: `git-for-each-ref(1)` -- `--format`, `--sort=-version:refname` for semver-sorted tag listing
- Git documentation: `git-tag(1)` -- Annotated tags store tagger, date, message + pointer to target object
- Go stdlib: `text/tabwriter` -- Column-aligned text output for status table

### Tertiary (LOW confidence)
- None. All Phase 4 patterns are direct extensions of verified Phase 1-3 code.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - No new dependencies; all reuse from existing codebase
- Architecture: HIGH - Drift=read-only-reconcile, Rollback=apply-from-tag, Status=manifest-read are well-understood compositions
- Pitfalls: HIGH - Git tag dereferencing syntax verified against codebase test (TestCreateVersionTag); performance pitfall for status is standard Graph API consideration

**Research date:** 2026-03-04
**Valid until:** 2026-04-04 (stable -- no external dependency changes expected)
