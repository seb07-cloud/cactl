# Phase 6: Point-in-Time Restore - Research

**Researched:** 2026-03-06
**Domain:** Interactive TUI, Git version history browsing, desired-state restore flow
**Confidence:** HIGH

## Summary

Phase 6 extends the existing `cactl rollback` command with an interactive history browser (`-i` flag) and adds a standalone `cactl history` command for read-only version browsing. The core infrastructure already exists: `GitBackend.ListVersionTags()` returns semver-sorted version history, `ReadTagBlob()` retrieves historical policy JSON, and `reconcile.ComputeDiff()` produces field-level diffs. The existing `output/diff.go` renders colored sigil-based diffs.

The new work falls into three categories: (1) a TUI layer for interactive policy/version selection using `charmbracelet/huh`, (2) a restore-to-desired-state flow that writes historical JSON to the on-disk policy file and auto-commits, and (3) the standalone `cactl history` command. The diff in the restore flow compares historical version against the *current desired state file* (not live Entra), which is a deliberate design departure from the existing rollback that diffs against live.

**Primary recommendation:** Use `charmbracelet/huh` (v2) for interactive selection. Keep the interactive TUI code isolated in a dedicated `internal/tui` package so commands stay testable and non-interactive paths remain clean.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Interactive flow: list all policies -> user selects one -> show that policy's full version history
- Each history entry shows: version, date, and diff summary (e.g. "3 fields changed: conditions.users, state, displayName")
- No filtering needed -- full list displayed, policies rarely exceed ~20 versions
- Selecting a version immediately shows the full diff (vs current desired state), then offers restore/back
- Single policy restore only -- no bulk/tenant-wide point-in-time restore
- Extends existing `cactl rollback` command rather than creating a new restore command
- Restore goes through the full plan/apply cycle: writes historical version as desired state -> user runs plan -> apply
- Restores always get an automatic patch version bump
- Diff compares historical version against current desired state (local policy file), not live Entra
- Uses the same colored diff format as `cactl plan` (sigils: +, ~, -/+, ?) for consistency
- After viewing diff, user can restore directly from the browser ("Restore this version? [y/N]")
- Confirm overwrite before writing the desired state file
- Warn and require explicit confirmation if the policy file has uncommitted local changes
- Auto-commit the desired state change with message like "restore: policy-name to v1.2.0"
- Auto-run `cactl plan` after commit to show what will change in Entra; user runs apply manually
- `cactl rollback --interactive` (or `-i`) launches the interactive history browser with restore flow
- `cactl history [--policy slug]` as standalone read-only command for viewing version history without restore
- `cactl history` supports --json for machine-readable output (table default for humans)
- CI/non-interactive mode supported: `cactl rollback --policy X --version v1.2.0` works without browser
- Arrow-key selector for interactive terminal navigation

### Claude's Discretion
- Choice of Go TUI library for arrow-key selection
- Exact layout and formatting of the history table
- How auto-plan output integrates with the restore flow
- Error handling for edge cases (deleted policies, corrupted tags)

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| charmbracelet/huh | v2 | Interactive Select, Confirm prompts | Best active Go TUI forms library; MIT licensed; 980+ importers; built on Bubble Tea; has accessible mode for CI fallback |
| spf13/cobra | v1.10.2 | CLI command structure | Already in use throughout project |
| os/exec (git) | stdlib | Git operations (commit, status) | Project convention: os/exec git plumbing over go-git (decision 02-02) |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| charmbracelet/lipgloss | (transitive) | Terminal styling | Pulled in by huh; available if custom styling needed |
| text/tabwriter | stdlib | Table formatting | Already used in output/status.go for history table |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| charmbracelet/huh | charmbracelet/bubbletea directly | Bubbletea is lower-level (Elm architecture); huh provides Select/Confirm out of the box with less code |
| charmbracelet/huh | AlecAivazis/survey | Survey is deprecated and unmaintained; author recommends bubbletea |

**Installation:**
```bash
go get github.com/charmbracelet/huh/v2
```

## Architecture Patterns

### Recommended Project Structure
```
cmd/
  rollback.go          # Extend with --interactive/-i flag; dispatch to interactive or direct flow
  history.go           # NEW: standalone cactl history command
internal/
  tui/
    selector.go        # Wraps huh.Select for policy and version selection
    restore.go         # Interactive restore wizard flow (select -> diff -> confirm -> write)
  output/
    diff.go            # Already exists: reuse renderFieldDiff for historical diffs
    status.go          # Already exists: extend RenderHistory for diff summary column
```

### Pattern 1: Interactive vs Non-Interactive Dispatch
**What:** The rollback command checks for `--interactive`/`-i` flag and dispatches to either the TUI flow or the existing direct rollback flow.
**When to use:** Always -- this preserves backward compatibility and CI support.
**Example:**
```go
// In cmd/rollback.go init()
rollbackCmd.Flags().BoolP("interactive", "i", false, "Launch interactive history browser")

// In runRollback()
interactive, _ := cmd.Flags().GetBool("interactive")
if interactive {
    if cfg.CI {
        return &types.ExitError{
            Code:    types.ExitValidationError,
            Message: "--interactive cannot be used with --ci mode; use --policy and --version instead",
        }
    }
    return runInteractiveRestore(ctx, cfg, backend, manifest)
}
// ... existing direct rollback flow unchanged
```

### Pattern 2: Desired-State Restore (Not Live PATCH)
**What:** The interactive restore writes the historical version to the on-disk policy file (`policies/<tenant>/<slug>.json`), auto-commits, then runs `cactl plan` to show what would change in Entra. This is fundamentally different from the existing rollback which PATCHes live directly.
**When to use:** Always for the interactive restore flow.
**Key difference from existing rollback:**
- Existing `cactl rollback --policy X --version Y`: reads tag blob, diffs against LIVE Entra, PATCHes Graph API directly
- New `cactl rollback -i`: reads tag blob, diffs against DESIRED STATE file, writes file, auto-commits, runs plan

```go
// Restore flow pseudocode:
// 1. Read historical JSON from tag: backend.ReadTagBlob(tenant, slug, version)
// 2. Read current desired state: ReadDesiredPolicies(tenant) -> policies[slug]
// 3. Diff historical vs desired: reconcile.ComputeDiff(historicalMap, desiredMap)
// 4. Display diff with output.renderFieldDiff (reuse existing)
// 5. Confirm restore
// 6. Check for uncommitted changes: git status policies/<tenant>/<slug>.json
// 7. Write file: cmd.WritePolicyFile(tenant, slug, historicalJSON)
// 8. Auto-commit: git add + git commit -m "restore: <slug> to v<version>"
// 9. Auto-run: exec cactl plan (or call runPlan directly)
```

### Pattern 3: Diff Summary for History Entries
**What:** Each history entry needs a diff summary showing what changed. Compute by diffing each version against its predecessor (or against current desired state).
**When to use:** History display with diff summary column.
**Implementation approach:** For each version tag, read both the tag blob and the previous version's tag blob, then run ComputeDiff. Summarize as "N fields changed: field1, field2, ...".
```go
// Diff summary computation:
func diffSummary(diffs []reconcile.FieldDiff) string {
    if len(diffs) == 0 {
        return "no changes"
    }
    paths := make([]string, len(diffs))
    for i, d := range diffs {
        paths[i] = d.Path
    }
    return fmt.Sprintf("%d fields changed: %s", len(diffs), strings.Join(paths, ", "))
}
```

### Pattern 4: Huh Select for Policy and Version Picking
**What:** Use huh.NewSelect for arrow-key navigation through policies and versions.
**Example:**
```go
import "github.com/charmbracelet/huh/v2"

func selectPolicy(slugs []string) (string, error) {
    var selected string
    options := make([]huh.Option[string], len(slugs))
    for i, s := range slugs {
        options[i] = huh.NewOption(s, s)
    }
    err := huh.NewSelect[string]().
        Title("Select a policy").
        Options(options...).
        Value(&selected).
        Run()
    return selected, err
}

func selectVersion(tags []state.VersionTag, summaries []string) (string, error) {
    var selected string
    options := make([]huh.Option[string], len(tags))
    for i, t := range tags {
        label := fmt.Sprintf("%-8s  %s  %s", t.Version, t.Timestamp[:10], summaries[i])
        options[i] = huh.NewOption(label, t.Version)
    }
    err := huh.NewSelect[string]().
        Title("Select a version to inspect").
        Options(options...).
        Value(&selected).
        Run()
    return selected, err
}
```

### Pattern 5: Uncommitted Changes Detection
**What:** Before overwriting a policy file, check if it has uncommitted local changes via `git status --porcelain`.
**Example:**
```go
func hasUncommittedChanges(repoDir, filePath string) (bool, error) {
    cmd := exec.Command("git", "status", "--porcelain", filePath)
    cmd.Dir = repoDir
    out, err := cmd.Output()
    if err != nil {
        return false, err
    }
    return strings.TrimSpace(string(out)) != "", nil
}
```

### Pattern 6: Auto-Commit via Git Plumbing
**What:** After writing the desired state file, auto-commit using git add + git commit.
**Consistent with project convention:** os/exec git plumbing.
```go
func autoCommit(repoDir, filePath, message string) error {
    add := exec.Command("git", "add", filePath)
    add.Dir = repoDir
    if out, err := add.CombinedOutput(); err != nil {
        return fmt.Errorf("git add: %s: %w", strings.TrimSpace(string(out)), err)
    }
    commit := exec.Command("git", "commit", "-m", message)
    commit.Dir = repoDir
    if out, err := commit.CombinedOutput(); err != nil {
        return fmt.Errorf("git commit: %s: %w", strings.TrimSpace(string(out)), err)
    }
    return nil
}
```

### Anti-Patterns to Avoid
- **Mixing interactive TUI code into cmd/ directly:** Keep huh interactions in `internal/tui/` so commands remain unit-testable with injected I/O.
- **Diffing against live Entra in restore flow:** The user explicitly decided diffs compare historical vs desired state (local file), not live. The plan step after restore handles the live comparison.
- **Modifying existing rollback behavior:** The direct `--policy --version` rollback flow must remain unchanged. Interactive mode is additive only.
- **Running huh in CI mode:** huh has no built-in non-interactive mode. Guard with `cfg.CI` check and reject `--interactive` in CI.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Arrow-key terminal selection | Custom ANSI escape handling | `huh.NewSelect` | Handles terminal raw mode, key events, scrolling, accessibility |
| Confirmation prompts | Custom scanner (for TUI flow) | `huh.NewConfirm` | Consistent with Select styling; accessible mode support |
| Diff computation | New diff logic | `reconcile.ComputeDiff()` | Already handles nested maps, sorted output, all diff types |
| Colored diff rendering | New rendering | `output.renderFieldDiff()` | Already implements sigil format (+, ~, -, -/+) with ANSI colors |
| Version tag listing | New git parsing | `backend.ListVersionTags()` | Already parses for-each-ref with strip=5, semver-sorted descending |
| Tag blob reading | New git cat-file wrapper | `backend.ReadTagBlob()` | Already handles annotated tag dereferencing with ^{} |
| Policy file writing | Custom file I/O | `cmd.WritePolicyFile()` | Already handles directory creation and path construction |
| Semver patch bump | New version parsing | `bumpPatchVersion()` in cmd/rollback.go | Already handles edge cases and invalid input fallback |

**Key insight:** Almost all the infrastructure exists. Phase 6 is primarily a UX/integration layer that wires existing capabilities into interactive flows.

## Common Pitfalls

### Pitfall 1: huh Blocks on Non-TTY Stdin
**What goes wrong:** `huh.Run()` hangs or panics when stdin is not a terminal (piped input, CI runners).
**Why it happens:** huh uses terminal raw mode which requires a real TTY.
**How to avoid:** Always guard interactive paths with a TTY check and `cfg.CI` rejection. Use `huh.WithAccessible(true)` as fallback for unusual terminals if needed.
**Warning signs:** Tests hanging, CI timeouts on interactive commands.

### Pitfall 2: Diff Direction Confusion
**What goes wrong:** Diff shows changes "backwards" because desired and historical arguments are swapped.
**Why it happens:** `ComputeDiff(desired, actual)` convention. For restore preview, "desired" is the historical version (what we want to restore TO) and "actual" is the current desired state file (what we have now).
**How to avoid:** Document clearly: `ComputeDiff(historicalMap, currentDesiredMap)` -- historical is what we WANT, current desired is what we HAVE.
**Warning signs:** + and - sigils feel inverted to the user.

### Pitfall 3: Auto-Commit Fails When Git Has No User Config
**What goes wrong:** `git commit` fails with "please tell me who you are" in fresh environments.
**Why it happens:** No git user.name/user.email configured.
**How to avoid:** Catch the error and provide a helpful message. Don't try to auto-configure git user settings.
**Warning signs:** Error message about git config in CI or container environments.

### Pitfall 4: Stale Desired State File After Restore
**What goes wrong:** User restores, auto-commit succeeds, but `cactl plan` shows unexpected diffs because the desired state file has extra fields the historical version lacks.
**Why it happens:** Historical versions may have been normalized differently or may lack fields added in later schema versions.
**How to avoid:** Write the historical JSON exactly as stored in the tag, without re-normalizing. The plan step will handle normalization against live.

### Pitfall 5: Race Between Uncommitted Changes Check and File Write
**What goes wrong:** File changes between the check and the write.
**Why it happens:** User edits file while the interactive prompt is waiting for input.
**How to avoid:** Accept this as a minor race; the git commit step will include whatever is on disk. The uncommitted-changes warning is advisory, not a hard lock.

### Pitfall 6: History Performance with Many Versions
**What goes wrong:** Computing diff summaries for every version tag is slow because each requires two ReadTagBlob + ComputeDiff calls.
**Why it happens:** O(n) git cat-file calls for n versions.
**How to avoid:** For the history table, compute diff summaries lazily or use tag messages (which already contain descriptive text). Only compute full diffs when the user selects a specific version.
**Warning signs:** Noticeable delay (>2s) when listing history for a policy with many versions.

## Code Examples

### Existing Infrastructure to Reuse

#### List All Tracked Policies (from manifest)
```go
// Source: cmd/rollback.go lines 107-118, cmd/status.go lines 69-87
manifest, err := state.ReadManifest(backend, cfg.Tenant)
// manifest.Policies is map[string]state.Entry
// Keys are slugs, iterate for policy list
```

#### List Version History for a Policy
```go
// Source: internal/state/backend.go lines 188-233
tags, err := backend.ListVersionTags(cfg.Tenant, slug)
// Returns []VersionTag sorted by semver descending
// Each has Version, Timestamp, Message fields
```

#### Read Historical Policy JSON from Tag
```go
// Source: internal/state/backend.go lines 237-246
tagJSON, err := backend.ReadTagBlob(cfg.Tenant, slug, version)
// Returns raw []byte of policy JSON stored in annotated tag
```

#### Compute Field-Level Diff
```go
// Source: internal/reconcile/diff.go lines 31-43
diffs := reconcile.ComputeDiff(desired, actual)
// Returns []FieldDiff with Path, Type (Added/Removed/Changed), OldValue, NewValue
```

#### Write Desired State File
```go
// Source: cmd/desired.go lines 53-63
err := WritePolicyFile(tenantID, slug, data)
// Writes to policies/<tenantID>/<slug>.json with directory creation
```

#### Bump Patch Version
```go
// Source: cmd/rollback.go (referenced but defined in helpers)
newVersion := bumpPatchVersion(entry.Version)
// "1.2.3" -> "1.2.4"
```

### New Code Patterns Needed

#### Interactive Restore Wizard Flow
```go
func runInteractiveRestore(ctx context.Context, cfg *types.Config, backend *state.GitBackend, manifest *state.Manifest) error {
    // Step 1: Build policy list from manifest
    slugs := make([]string, 0, len(manifest.Policies))
    for slug := range manifest.Policies {
        slugs = append(slugs, slug)
    }
    sort.Strings(slugs)

    // Step 2: Select policy via huh
    selectedSlug, err := tui.SelectPolicy(slugs)
    if err != nil { return err }

    // Step 3: Load version history
    tags, err := backend.ListVersionTags(cfg.Tenant, selectedSlug)
    if err != nil { return err }
    if len(tags) == 0 {
        fmt.Fprintf(os.Stdout, "No version history for '%s'.\n", selectedSlug)
        return nil
    }

    // Step 4: Select version via huh (with summary info in labels)
    selectedVersion, err := tui.SelectVersion(tags)
    if err != nil { return err }

    // Step 5: Read historical JSON and current desired state
    historicalJSON, err := backend.ReadTagBlob(cfg.Tenant, selectedSlug, selectedVersion)
    if err != nil { return err }

    currentPolicies, err := ReadDesiredPolicies(cfg.Tenant)
    if err != nil { return err }

    // Step 6: Compute and display diff (historical vs current desired)
    var historicalMap, currentMap map[string]interface{}
    json.Unmarshal(historicalJSON, &historicalMap)
    // currentPolicies[selectedSlug].Data is already map[string]interface{}
    currentMap = currentPolicies[selectedSlug].Data
    diffs := reconcile.ComputeDiff(historicalMap, currentMap)

    // Step 7: Render diff using existing output functions
    // ... render diffs ...

    // Step 8: Confirm restore via huh.NewConfirm
    // Step 9: Check uncommitted changes, warn if needed
    // Step 10: Write file, auto-commit, auto-plan
}
```

#### Standalone History Command
```go
var historyCmd = &cobra.Command{
    Use:   "history",
    Short: "View version history for tracked policies",
    RunE:  runHistory,
}

func init() {
    rootCmd.AddCommand(historyCmd)
    historyCmd.Flags().String("policy", "", "Policy slug to show history for")
    historyCmd.Flags().Bool("json", false, "Output in JSON format")
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| AlecAivazis/survey | charmbracelet/huh | 2023-2024 | survey deprecated; huh is the modern replacement |
| go-git library | os/exec git plumbing | Project convention | Lighter dependency, more predictable behavior |

**Deprecated/outdated:**
- `AlecAivazis/survey`: Deprecated by its own author; do not use.

## Open Questions

1. **Auto-plan integration after restore**
   - What we know: User wants `cactl plan` to run automatically after the auto-commit
   - What's unclear: Should it call `runPlan()` directly (shares process, needs careful flag handling) or shell out to `cactl plan` (simpler but starts new process)?
   - Recommendation: Call `runPlan()` directly by extracting plan logic into a shared function. Shelling out adds process overhead and flag-passing complexity.

2. **Diff summary computation strategy for history table**
   - What we know: Each history entry should show "3 fields changed: field1, field2"
   - What's unclear: Should diff summaries compare each version to its predecessor, or each version to current desired state?
   - Recommendation: Compare each version to its *predecessor* (more meaningful for understanding what changed in each version). For the first version, show "initial import" or similar.

3. **Back navigation from diff view**
   - What we know: User wants ability to go "back" from diff view to version list
   - What's unclear: huh forms are one-directional (forward flow). Supporting "back" requires either a loop or custom Bubble Tea.
   - Recommendation: Use a loop pattern: after showing diff, offer "Restore / Back to versions / Quit" via another huh.Select. This avoids needing raw Bubble Tea.

## Sources

### Primary (HIGH confidence)
- Codebase analysis: cmd/rollback.go, cmd/status.go, cmd/desired.go, cmd/apply.go -- existing flows for rollback, history display, desired state management
- Codebase analysis: internal/state/backend.go -- ListVersionTags, ReadTagBlob, CreateVersionTag, WritePolicy APIs
- Codebase analysis: internal/reconcile/diff.go -- ComputeDiff field-level diff engine
- Codebase analysis: internal/output/diff.go, status.go -- existing diff rendering and history table formatting

### Secondary (MEDIUM confidence)
- [charmbracelet/huh GitHub](https://github.com/charmbracelet/huh) -- v2 API, Select/Confirm fields, dynamic forms, accessible mode
- [charmbracelet/huh Go Packages](https://pkg.go.dev/github.com/charmbracelet/huh/v2) -- v2 published Oct 2025
- [charmbracelet/bubbletea GitHub](https://github.com/charmbracelet/bubbletea) -- underlying framework, 11K importers

### Tertiary (LOW confidence)
- huh CI/non-interactive behavior: Not explicitly documented; inferred from accessible mode documentation. Needs validation during implementation.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - huh is clearly the right choice; well-maintained, active, MIT licensed, only viable option since survey is deprecated
- Architecture: HIGH - almost all infrastructure already exists in the codebase; architecture follows established project patterns
- Pitfalls: MEDIUM - TTY handling and diff direction are well-understood, but auto-commit edge cases and huh behavior in edge terminal environments need validation during implementation

**Research date:** 2026-03-06
**Valid until:** 2026-04-06 (stable domain, 30 days)
