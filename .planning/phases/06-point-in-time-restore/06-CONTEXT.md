# Phase 6: Point-in-Time Restore - Context

**Gathered:** 2026-03-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Extend cactl with interactive history browsing and point-in-time restore for individual policies. Users can browse version history, view diffs against current desired state, and restore a policy to any previous version through the plan/apply cycle. A standalone `cactl history` command provides read-only version browsing.

</domain>

<decisions>
## Implementation Decisions

### History browsing
- Interactive flow: list all policies -> user selects one -> show that policy's full version history
- Each history entry shows: version, date, and diff summary (e.g. "3 fields changed: conditions.users, state, displayName")
- No filtering needed -- full list displayed, policies rarely exceed ~20 versions
- Selecting a version immediately shows the full diff (vs current desired state), then offers restore/back

### Restore target
- Single policy restore only -- no bulk/tenant-wide point-in-time restore
- Extends existing `cactl rollback` command rather than creating a new restore command
- Restore goes through the full plan/apply cycle: writes historical version as desired state -> user runs plan -> apply
- Restores always get an automatic patch version bump

### Diff & confirmation
- Diff compares historical version against current desired state (local policy file), not live Entra
- Uses the same colored diff format as `cactl plan` (sigils: +, ~, -/+, ?) for consistency
- After viewing diff, user can restore directly from the browser ("Restore this version? [y/N]")
- Confirm overwrite before writing the desired state file
- Warn and require explicit confirmation if the policy file has uncommitted local changes
- Auto-commit the desired state change with message like "restore: policy-name to v1.2.0"
- Auto-run `cactl plan` after commit to show what will change in Entra; user runs apply manually

### Command design
- `cactl rollback --interactive` (or `-i`) launches the interactive history browser with restore flow
- `cactl history [--policy slug]` as standalone read-only command for viewing version history without restore
- `cactl history` supports --json for machine-readable output (table default for humans)
- CI/non-interactive mode supported: `cactl rollback --policy X --version v1.2.0` works without browser
- Arrow-key selector for interactive terminal navigation (like charmbracelet/huh or similar Go TUI library)

### Claude's Discretion
- Choice of Go TUI library for arrow-key selection
- Exact layout and formatting of the history table
- How auto-plan output integrates with the restore flow
- Error handling for edge cases (deleted policies, corrupted tags)

</decisions>

<specifics>
## Specific Ideas

- Interactive flow should feel like a guided wizard: pick policy -> pick version -> see diff -> confirm restore
- The flow from diff view to restore should be seamless -- no need to exit and re-run commands
- History browsing is a natural extension of rollback, not a separate concept

</specifics>

<deferred>
## Deferred Ideas

None -- discussion stayed within phase scope

</deferred>

---

*Phase: 06-point-in-time-restore*
*Context gathered: 2026-03-06*
