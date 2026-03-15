package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/seb07-cloud/cactl/internal/output"
	"github.com/seb07-cloud/cactl/internal/reconcile"
	"github.com/seb07-cloud/cactl/internal/semver"
	"github.com/seb07-cloud/cactl/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Deploy Conditional Access policy changes",
	Long:  "Generate a plan, prompt for confirmation, then apply changes to the tenant.\nUse --dry-run to preview without making changes, or --auto-approve for CI pipelines.",
	RunE:  runApply,
}

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().Bool("auto-approve", false, "Skip confirmation prompt (required for CI mode)")
	applyCmd.Flags().Bool("dry-run", false, "Generate plan but make no Graph API writes")
	applyCmd.Flags().String("bump-level", "", "Override computed bump level (major|minor|patch)")
}

func runApply(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Read --bump-level override before pipeline (fast fail on bad input)
	bumpLevelOverride, _ := cmd.Flags().GetString("bump-level")
	var overrideBump *semver.BumpLevel
	if bumpLevelOverride != "" {
		bl, err := parseBumpLevel(bumpLevelOverride)
		if err != nil {
			return &types.ExitError{
				Code:    types.ExitFatalError,
				Message: fmt.Sprintf("invalid --bump-level %q: must be major, minor, or patch", bumpLevelOverride),
			}
		}
		overrideBump = &bl
	}

	// Bootstrap
	p, err := NewPipeline(ctx)
	if err != nil {
		return err
	}

	// Load desired + live state
	backendPolicies, err := ReadDesiredPolicies(p.Cfg.Tenant)
	if err != nil {
		return &types.ExitError{Code: types.ExitFatalError, Message: fmt.Sprintf("loading desired state: %v", err)}
	}

	livePoliciesGraph, err := p.GraphClient.ListPolicies(ctx)
	if err != nil {
		return &types.ExitError{Code: types.ExitFatalError, Message: fmt.Sprintf("fetching live policies: %v", err)}
	}

	livePolicies, err := NormalizeLivePolicies(livePoliciesGraph)
	if err != nil {
		return err
	}

	// Reconcile
	actions := reconcile.Reconcile(backendPolicies, livePolicies, p.Manifest)

	// Semver, validate, resolve, render
	if err := p.ComputeSemverBumps(actions, overrideBump); err != nil {
		return err
	}
	validations := p.RunValidations(actions)
	resolver := p.ResolveDisplayNames(ctx, actions)

	if err := p.RenderPlan(os.Stdout, actions, validations, resolver); err != nil {
		return err
	}

	// Check for validation errors -- block apply
	if HasValidationErrors(validations) {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: "validation errors detected, apply blocked",
		}
	}

	// Filter to actionable items
	actionable := filterActionable(actions)
	if len(actionable) == 0 {
		return nil
	}

	// Dry-run check (PLAN-07)
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		fmt.Fprintln(os.Stdout, "Dry run complete. No changes applied.")
		return nil
	}

	// Confirmation (PLAN-05, PLAN-08)
	autoApprove, _ := cmd.Flags().GetBool("auto-approve")

	// CI mode check: must use --auto-approve
	if p.Cfg.CI && !autoApprove {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: "ci mode requires --auto-approve for write operations",
		}
	}

	if !autoApprove {
		if hasAction(actionable, reconcile.ActionRecreate) {
			// Recreate requires explicit "yes" — single prompt covers both approval and destructive acknowledgement
			if !confirmExplicit("Plan includes recreate (DELETE + CREATE). Type 'yes' to apply: ") {
				fmt.Fprintln(os.Stdout, "Apply cancelled.")
				return nil
			}

			// Prompt for recreate bump level (interactive only)
			if overrideBump == nil {
				bl := promptBumpLevel("Recreate version bump level (major|minor|patch) [minor]: ", os.Stdin)
				overrideBump = &bl
				// Recompute bumps for recreates with chosen level
				for i := range actionable {
					if actionable[i].Action == reconcile.ActionRecreate {
						currentVersion := "1.0.0"
						if entry, ok := p.Manifest.Policies[actionable[i].Slug]; ok && entry.Version != "" {
							currentVersion = entry.Version
						}
						newVersion, err := semver.BumpVersion(currentVersion, *overrideBump)
						if err != nil {
							newVersion = currentVersion
						}
						actionable[i].VersionFrom = currentVersion
						actionable[i].VersionTo = newVersion
						actionable[i].BumpLevel = overrideBump.String()
					}
				}
			}
		} else if !confirm("Do you want to apply these changes? [Y/n]: ") {
			fmt.Fprintln(os.Stdout, "Apply cancelled.")
			return nil
		}
	}

	useColor := output.ShouldUseColor(viper.GetViper())

	// Execute actions -- sort by slug for deterministic order
	sort.Slice(actionable, func(i, j int) bool {
		return actionable[i].Slug < actionable[j].Slug
	})

	var created, updated, recreated int
	for idx, action := range actionable {
		switch action.Action {
		case reconcile.ActionCreate:
			newID, err := p.GraphClient.CreatePolicy(ctx, action.BackendJSON)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed applying policy %q: %v\n", action.Slug, err)
				fmt.Fprintf(os.Stderr, "%d of %d actions succeeded before failure.\n", idx, len(actionable))
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("apply failed on %s: %v", action.Slug, err),
				}
			}

			version := "1.0.0"
			if err := p.RecordAppliedAction(action.Slug, newID, version, action.BackendJSON, fmt.Sprintf("Created %s at %s", action.Slug, version)); err != nil {
				return err
			}

			fmt.Fprintln(os.Stdout, output.FormatApplied(reconcile.ActionCreate, fmt.Sprintf("%s (%s)", action.Slug, version), useColor))
			created++

		case reconcile.ActionUpdate:
			patchBody := buildPatchBody(action.BackendJSON, action.Diff)
			if err := p.GraphClient.UpdatePolicy(ctx, action.LiveObjectID, patchBody); err != nil {
				fmt.Fprintf(os.Stderr, "Failed applying policy %q: %v\n", action.Slug, err)
				fmt.Fprintf(os.Stderr, "%d of %d actions succeeded before failure.\n", idx, len(actionable))
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("apply failed on %s: %v", action.Slug, err),
				}
			}

			newVersion := action.VersionTo
			if newVersion == "" {
				newVersion = "1.0.1"
			}

			if err := p.RecordAppliedAction(action.Slug, action.LiveObjectID, newVersion, action.BackendJSON, fmt.Sprintf("Updated %s to %s", action.Slug, newVersion)); err != nil {
				return err
			}

			fmt.Fprintln(os.Stdout, output.FormatApplied(reconcile.ActionUpdate, fmt.Sprintf("%s (%s -> %s)", action.Slug, action.VersionFrom, newVersion), useColor))
			updated++

		case reconcile.ActionRecreate:
			newID, err := p.GraphClient.CreatePolicy(ctx, action.BackendJSON)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed applying policy %q: %v\n", action.Slug, err)
				fmt.Fprintf(os.Stderr, "%d of %d actions succeeded before failure.\n", idx, len(actionable))
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("apply failed on %s: %v", action.Slug, err),
				}
			}

			currentVersion := action.VersionFrom
			newVersion := action.VersionTo
			if currentVersion == "" {
				currentVersion = "1.0.0"
			}
			if newVersion == "" {
				// Fallback: bump minor if ComputeSemverBumps didn't populate
				v, err := semver.BumpVersion(currentVersion, semver.BumpMinor)
				if err != nil {
					v = "1.1.0"
				}
				newVersion = v
			}

			if err := p.RecordAppliedAction(action.Slug, newID, newVersion, action.BackendJSON, fmt.Sprintf("Recreated %s at %s", action.Slug, newVersion)); err != nil {
				return err
			}

			fmt.Fprintln(os.Stdout, output.FormatApplied(reconcile.ActionRecreate, fmt.Sprintf("%s (%s -> %s)", action.Slug, currentVersion, newVersion), useColor))
			recreated++

		case reconcile.ActionNoop, reconcile.ActionUntracked, reconcile.ActionDuplicate:
			continue
		}
	}

	// Summary
	fmt.Fprintln(os.Stdout, output.FormatApplySummary(created, updated, recreated, useColor))
	return nil
}

// confirm prompts the user for a Y/n confirmation.
// Empty input, "y", and "yes" (case-insensitive) return true.
func confirm(prompt string) bool {
	return confirmFromReader(prompt, os.Stdin)
}

// confirmFromReader is the testable version of confirm that reads from a provided reader.
func confirmFromReader(prompt string, r io.Reader) bool {
	fmt.Fprint(os.Stdout, prompt)
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return false
	}
	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return input == "" || input == "y" || input == "yes"
}

// confirmExplicit prompts the user for explicit "yes" confirmation.
// Only the exact string "yes" (case-insensitive) returns true.
func confirmExplicit(prompt string) bool {
	return confirmExplicitFromReader(prompt, os.Stdin)
}

// confirmExplicitFromReader is the testable version of confirmExplicit.
func confirmExplicitFromReader(prompt string, r io.Reader) bool {
	fmt.Fprint(os.Stdout, prompt)
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return false
	}
	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return input == "yes"
}

// filterActionable returns only actions that require Graph API writes.
func filterActionable(actions []reconcile.PolicyAction) []reconcile.PolicyAction {
	result := make([]reconcile.PolicyAction, 0, len(actions))
	for _, a := range actions {
		if a.Action == reconcile.ActionCreate || a.Action == reconcile.ActionUpdate || a.Action == reconcile.ActionRecreate {
			result = append(result, a)
		}
	}
	return result
}

// hasAction checks if any action in the slice has the given type.
func hasAction(actions []reconcile.PolicyAction, t reconcile.ActionType) bool {
	for _, a := range actions {
		if a.Action == t {
			return true
		}
	}
	return false
}

// promptBumpLevel prompts the user to choose a bump level for recreate actions.
// Empty input defaults to minor. Invalid input retries.
func promptBumpLevel(prompt string, r io.Reader) semver.BumpLevel {
	scanner := bufio.NewScanner(r)
	for {
		fmt.Fprint(os.Stdout, prompt)
		if !scanner.Scan() {
			return semver.BumpMinor
		}
		input := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if input == "" {
			return semver.BumpMinor
		}
		bl, err := parseBumpLevel(input)
		if err != nil {
			fmt.Fprintf(os.Stdout, "Invalid bump level %q. Use major, minor, or patch.\n", input) //nolint:gosec // G705 - CLI output, not web
			continue
		}
		return bl
	}
}

// parseBumpLevel converts a user-provided string to a semver.BumpLevel.
func parseBumpLevel(s string) (semver.BumpLevel, error) {
	switch strings.ToLower(s) {
	case "major":
		return semver.BumpMajor, nil
	case "minor":
		return semver.BumpMinor, nil
	case "patch":
		return semver.BumpPatch, nil
	default:
		return 0, fmt.Errorf("unknown bump level: %s", s)
	}
}

// buildPatchBody creates a minimal PATCH body containing only the top-level
// keys from the desired state that have changed fields. Graph API recommends
// sending only changed values in PATCH requests.
func buildPatchBody(desired map[string]interface{}, diffs []reconcile.FieldDiff) map[string]interface{} {
	// Collect top-level keys that contain changes
	changedKeys := make(map[string]bool)
	for _, d := range diffs {
		// Extract top-level key from dot-separated path
		key := d.Path
		if idx := strings.Index(key, "."); idx >= 0 {
			key = key[:idx]
		}
		changedKeys[key] = true
	}

	patch := make(map[string]interface{})
	for key := range changedKeys {
		if val, ok := desired[key]; ok {
			patch[key] = val
		}
	}
	return patch
}

// deployerIdentity returns the deployer identity string for manifest entries.
func deployerIdentity(cfg *types.Config) string {
	if cfg.Auth.Mode != "" {
		return "cactl/" + cfg.Auth.Mode
	}
	return "cactl/unknown"
}
