package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/seb07-cloud/cactl/internal/normalize"
	"github.com/seb07-cloud/cactl/internal/output"
	"github.com/seb07-cloud/cactl/internal/reconcile"
	"github.com/seb07-cloud/cactl/internal/semver"
	"github.com/seb07-cloud/cactl/internal/state"
	"github.com/seb07-cloud/cactl/internal/tui"
	"github.com/seb07-cloud/cactl/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Restore a prior policy version from tag history",
	Long: `Roll back a Conditional Access policy to a previous version.

Use --policy and --version for direct rollback (PATCHes live Entra).
Use --interactive (-i) to browse history and restore to desired state file.

Direct mode reads the historical version from a Git annotated tag, diffs against
the current live state, and applies the change after confirmation.

Interactive mode launches a TUI wizard: select policy, browse versions with diff
summaries, view full diffs, and restore to the on-disk desired state file. After
restore, an auto-commit and auto-plan show what would change in Entra.

A new forward version tag is always created (existing tags are never modified),
preserving the full audit trail.`,
	RunE: runRollback,
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
	rollbackCmd.Flags().String("policy", "", "Policy slug to roll back (required)")
	rollbackCmd.Flags().String("version", "", "Semver version to restore, e.g. 1.0.0 (required)")
	rollbackCmd.Flags().Bool("auto-approve", false, "Skip confirmation prompt (required for CI mode)")
	rollbackCmd.Flags().BoolP("interactive", "i", false, "Launch interactive history browser with restore")
}

func runRollback(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Bootstrap (config + auth + graph + backend + manifest)
	p, err := NewPipeline(ctx)
	if err != nil {
		return err
	}

	// Check for interactive mode BEFORE existing flag validation
	interactive, _ := cmd.Flags().GetBool("interactive")
	if interactive {
		if p.Cfg.CI {
			return &types.ExitError{
				Code:    types.ExitValidationError,
				Message: "--interactive cannot be used with --ci mode; use --policy and --version flags instead",
			}
		}
		return runInteractiveRollback(ctx, p.Cfg)
	}

	// Get flags (direct rollback mode)
	slug, _ := cmd.Flags().GetString("policy")
	version, _ := cmd.Flags().GetString("version")
	autoApprove, _ := cmd.Flags().GetBool("auto-approve")

	if slug == "" {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: "--policy is required",
		}
	}
	if version == "" {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: "--version is required",
		}
	}

	// Validate policy is tracked (ROLL-01)
	entry, tracked := p.Manifest.Policies[slug]
	if !tracked {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("policy '%s' is not tracked in the manifest. Run 'cactl import' first.", slug),
		}
	}

	// Read historical version from tag (ROLL-01)
	tagJSON, err := p.Backend.ReadTagBlob(p.Cfg.Tenant, slug, version)
	if err != nil {
		// List available versions for helpful error
		tags, listErr := p.Backend.ListVersionTags(p.Cfg.Tenant, slug)
		fmt.Fprintf(os.Stderr, "Version %s not found for policy %s.\n", version, slug)
		if listErr == nil && len(tags) > 0 {
			fmt.Fprintln(os.Stderr, "Available versions:")
			for _, t := range tags {
				fmt.Fprintf(os.Stderr, "  %s  %s\n", t.Version, t.Timestamp)
			}
		}
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("version %s not found for policy %s", version, slug),
		}
	}

	// Get current live state
	livePolicy, err := p.GraphClient.GetPolicy(ctx, entry.LiveObjectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Live policy not found (may have been deleted). Rollback would require recreating the policy.\n")
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("live policy %s not found, consider using 'cactl apply' instead: %v", entry.LiveObjectID, err),
		}
	}

	// Compute diff (ROLL-02)
	var tagMap map[string]interface{}
	if err := json.Unmarshal(tagJSON, &tagMap); err != nil {
		return fmt.Errorf("parsing historical version: %w", err)
	}

	normalizedLive, err := normalize.Normalize(livePolicy.RawJSON)
	if err != nil {
		return fmt.Errorf("normalizing live policy: %w", err)
	}
	var liveMap map[string]interface{}
	if err := json.Unmarshal(normalizedLive, &liveMap); err != nil {
		return fmt.Errorf("parsing normalized live policy: %w", err)
	}

	diffs := reconcile.ComputeDiff(tagMap, liveMap)

	// No-diff case
	if len(diffs) == 0 {
		fmt.Fprintf(os.Stdout, "No changes needed. Live policy already matches version %s.\n", version)
		return nil
	}

	// Display diff (ROLL-02)
	fmt.Fprintf(os.Stdout, "Rolling back %s to version %s\n\n", slug, version)

	if p.Cfg.Output == "json" {
		jsonOut := map[string]interface{}{
			"policy":         slug,
			"rollbackTo":     version,
			"currentVersion": entry.Version,
			"diffs":          formatDiffsJSON(diffs),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(jsonOut); err != nil {
			return fmt.Errorf("rendering JSON: %w", err)
		}
		return nil
	}

	for _, d := range diffs {
		switch d.Type {
		case reconcile.DiffAdded:
			fmt.Fprintf(os.Stdout, "  + %s: %v\n", d.Path, d.NewValue)
		case reconcile.DiffRemoved:
			fmt.Fprintf(os.Stdout, "  - %s: %v\n", d.Path, d.OldValue)
		case reconcile.DiffChanged:
			fmt.Fprintf(os.Stdout, "  ~ %s: %v -> %v\n", d.Path, d.OldValue, d.NewValue)
		}
	}
	fmt.Fprintln(os.Stdout)

	// Confirmation
	if p.Cfg.CI && !autoApprove {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: "ci mode requires --auto-approve",
		}
	}

	if !autoApprove {
		if !confirm("Apply rollback? [Y/n]: ") {
			fmt.Fprintln(os.Stdout, "Rollback cancelled.")
			return nil
		}
	}

	// Execute rollback (ROLL-03) - PATCH live policy
	if err := p.GraphClient.UpdatePolicy(ctx, entry.LiveObjectID, tagMap); err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("rollback failed: %v", err),
		}
	}

	// Create new version (ROLL-04 -- never modify existing tags)
	newVersion, verErr := semver.BumpVersion(entry.Version, semver.BumpPatch)
	if verErr != nil {
		newVersion = "1.0.1"
	}

	tagMessage := fmt.Sprintf("cactl rollback: %s %s (rolled back to %s)", slug, newVersion, version)
	if err := p.RecordAppliedAction(slug, entry.LiveObjectID, newVersion, tagMap, tagMessage); err != nil {
		return err
	}

	// Success output
	fmt.Fprintf(os.Stdout, "Rollback complete: %s rolled back to %s (new version: %s)\n", slug, version, newVersion)
	return nil
}

// runInteractiveRollback launches the TUI history browser and restore wizard.
func runInteractiveRollback(ctx context.Context, cfg *types.Config) error {
	backend, err := state.NewGitBackend(".")
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("git backend: %v", err),
		}
	}

	manifest, err := state.ReadManifest(backend, cfg.Tenant)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	restoreCfg := tui.RestoreConfig{
		Backend:         backend,
		Tenant:          cfg.Tenant,
		Manifest:        manifest,
		UseColor:        output.ShouldUseColor(viper.GetViper()),
		RepoDir:         ".",
		WritePolicyFile: WritePolicyFile,
		ReadDesiredPolicies: func(tenant string) (map[string]map[string]interface{}, error) {
			policies, err := ReadDesiredPolicies(tenant)
			if err != nil {
				return nil, err
			}
			result := make(map[string]map[string]interface{}, len(policies))
			for slug, bp := range policies {
				result[slug] = bp.Data
			}
			return result, nil
		},
		RunPlan: func(ctx context.Context) error {
			return runPlan(planCmd, nil)
		},
	}
	return tui.RunInteractiveRestore(ctx, restoreCfg)
}

// formatDiffsJSON converts field diffs to a JSON-friendly representation.
func formatDiffsJSON(diffs []reconcile.FieldDiff) []map[string]interface{} {
	result := make([]map[string]interface{}, len(diffs))
	for i, d := range diffs {
		entry := map[string]interface{}{
			"path": d.Path,
		}
		switch d.Type {
		case reconcile.DiffAdded:
			entry["type"] = "added"
			entry["newValue"] = d.NewValue
		case reconcile.DiffRemoved:
			entry["type"] = "removed"
			entry["oldValue"] = d.OldValue
		case reconcile.DiffChanged:
			entry["type"] = "changed"
			entry["oldValue"] = d.OldValue
			entry["newValue"] = d.NewValue
		}
		result[i] = entry
	}
	return result
}
