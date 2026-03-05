package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/seb07-cloud/cactl/internal/auth"
	"github.com/seb07-cloud/cactl/internal/config"
	"github.com/seb07-cloud/cactl/internal/graph"
	"github.com/seb07-cloud/cactl/internal/normalize"
	"github.com/seb07-cloud/cactl/internal/output"
	"github.com/seb07-cloud/cactl/internal/reconcile"
	"github.com/seb07-cloud/cactl/internal/resolve"
	"github.com/seb07-cloud/cactl/internal/semver"
	"github.com/seb07-cloud/cactl/internal/state"
	"github.com/seb07-cloud/cactl/internal/validate"
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
}

// TODO: SEMV-05 --bump-level flag for user override

func runApply(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Phase A: Generate plan (reuse plan logic)

	// 1. Load config, validate tenant
	cfg, err := config.LoadFromGlobal()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.Tenant == "" {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: "tenant is required: use --tenant or set CACTL_TENANT",
		}
	}

	// 2. Create auth factory and credential
	factory, err := auth.NewClientFactory(cfg.Auth)
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("auth setup failed: %v", err),
		}
	}

	cred, err := factory.Credential(ctx, cfg.Tenant)
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("authentication failed: %v", err),
		}
	}

	// 3. Create graph client and git backend
	graphClient := graph.NewClient(cred, cfg.Tenant)

	backend, err := state.NewGitBackend(".")
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("git backend: %v", err),
		}
	}

	// 4. Load backend policies
	slugs, err := backend.ListPolicies(cfg.Tenant)
	if err != nil {
		return fmt.Errorf("listing backend policies: %w", err)
	}

	backendPolicies := make(map[string]reconcile.BackendPolicy)
	for _, slug := range slugs {
		data, err := backend.ReadPolicy(cfg.Tenant, slug)
		if err != nil {
			return fmt.Errorf("reading backend policy %s: %w", slug, err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("parsing backend policy %s: %w", slug, err)
		}
		backendPolicies[slug] = reconcile.BackendPolicy{Data: m}
	}

	// 5. Load live policies
	livePoliciesGraph, err := graphClient.ListPolicies(ctx)
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("fetching live policies: %v", err),
		}
	}

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

	// 6. Load manifest
	manifest, err := state.ReadManifest(backend, cfg.Tenant)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	// 7. Reconcile
	actions := reconcile.Reconcile(backendPolicies, livePolicies, manifest)

	// 8. Compute semver bumps
	v := viper.GetViper()
	majorFields := v.GetStringSlice("semver.major_fields")
	minorFields := v.GetStringSlice("semver.minor_fields")
	if len(majorFields) == 0 && len(minorFields) == 0 {
		defaults := semver.DefaultSemverConfig()
		majorFields = defaults.MajorFields
		minorFields = defaults.MinorFields
	}

	for i := range actions {
		if actions[i].Action != reconcile.ActionUpdate {
			continue
		}
		semverDiffs := make([]semver.FieldDiff, len(actions[i].Diff))
		for j, d := range actions[i].Diff {
			semverDiffs[j] = semver.FieldDiff{
				Path:     d.Path,
				OldValue: d.OldValue,
				NewValue: d.NewValue,
			}
		}

		bumpLevel := semver.DetermineBump(semverDiffs, majorFields, minorFields)
		actions[i].BumpLevel = bumpLevel.String()

		currentVersion := "1.0.0"
		if entry, ok := manifest.Policies[actions[i].Slug]; ok && entry.Version != "" {
			currentVersion = entry.Version
		}

		newVersion, err := semver.BumpVersion(currentVersion, bumpLevel)
		if err != nil {
			return fmt.Errorf("computing version for %s: %w", actions[i].Slug, err)
		}
		actions[i].VersionFrom = currentVersion
		actions[i].VersionTo = newVersion
	}

	// 9. Run validations -- if any SeverityError, block apply
	breakGlassAccounts := v.GetStringSlice("validation.break_glass_accounts")
	valCfg := validate.ValidationConfig{
		BreakGlassAccounts: breakGlassAccounts,
	}
	valActions := make([]validate.PolicyAction, len(actions))
	for i, a := range actions {
		valActions[i] = validate.PolicyAction{
			Slug:        a.Slug,
			Action:      validate.ActionType(a.Action),
			BackendJSON: a.BackendJSON,
		}
	}
	validations := validate.ValidatePlan(valActions, valCfg)

	// Phase B: Display plan

	// 10. Resolve display names
	policyMaps := make([]map[string]interface{}, 0, len(actions))
	for _, a := range actions {
		if a.BackendJSON != nil {
			policyMaps = append(policyMaps, a.BackendJSON)
		}
	}
	refs := resolve.CollectRefs(policyMaps)

	resolver := resolve.NewResolver(graphClient)
	if len(refs) > 0 {
		if err := resolver.ResolveAll(ctx, refs); err != nil {
			// Non-fatal: continue with raw IDs
			_ = err
		}
	}

	// 11. Render plan
	useColor := output.ShouldUseColor(v)
	if cfg.Output == "json" {
		if err := output.RenderPlanJSON(os.Stdout, actions, validations, resolver); err != nil {
			return fmt.Errorf("rendering JSON: %w", err)
		}
	} else {
		output.RenderPlan(os.Stdout, actions, validations, resolver, useColor)
	}

	// Check for validation errors -- block apply
	hasValidationErrors := false
	for _, val := range validations {
		if val.Severity == validate.SeverityError {
			hasValidationErrors = true
			break
		}
	}
	if hasValidationErrors {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: "validation errors detected, apply blocked",
		}
	}

	// 12. Filter to actionable items
	actionable := filterActionable(actions)
	if len(actionable) == 0 {
		fmt.Fprintln(os.Stdout, "No changes. Infrastructure is up-to-date.")
		return nil
	}

	// Phase C: Dry-run check (PLAN-07)
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		fmt.Fprintln(os.Stdout, "Dry run complete. No changes applied.")
		return nil
	}

	// Phase D: Confirmation (PLAN-05, PLAN-08)
	autoApprove, _ := cmd.Flags().GetBool("auto-approve")

	// CI mode check: must use --auto-approve
	if cfg.CI && !autoApprove {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: "ci mode requires --auto-approve for write operations",
		}
	}

	if !autoApprove {
		// Standard confirmation
		if !confirm("Do you want to apply these changes? [Y/n]: ") {
			fmt.Fprintln(os.Stdout, "Apply cancelled.")
			return nil
		}

		// Escalated confirmation for recreate actions (PLAN-08)
		if hasAction(actionable, reconcile.ActionRecreate) {
			if !confirmExplicit("Recreate actions will DELETE and re-CREATE policies. Type 'yes' to confirm: ") {
				fmt.Fprintln(os.Stdout, "Apply cancelled.")
				return nil
			}
		}
	}

	// Phase E: Execute actions
	// Sort actionable by slug for deterministic order
	sort.Slice(actionable, func(i, j int) bool {
		return actionable[i].Slug < actionable[j].Slug
	})

	var created, updated, recreated int
	for idx, action := range actionable {
		switch action.Action {
		case reconcile.ActionCreate:
			newID, err := graphClient.CreatePolicy(ctx, action.BackendJSON)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed applying policy %q: %v\n", action.Slug, err)
				fmt.Fprintf(os.Stderr, "%d of %d actions succeeded before failure.\n", idx, len(actionable))
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("apply failed on %s: %v", action.Slug, err),
				}
			}

			version := "1.0.0"
			backendJSON, _ := json.Marshal(action.BackendJSON)
			sha, err := backend.WritePolicy(cfg.Tenant, action.Slug, backendJSON)
			if err != nil {
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("writing backend state for %s: %v", action.Slug, err),
				}
			}

			if err := backend.CreateVersionTag(cfg.Tenant, action.Slug, version, sha, fmt.Sprintf("Created %s at %s", action.Slug, version)); err != nil {
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("creating version tag for %s: %v", action.Slug, err),
				}
			}

			manifest.Policies[action.Slug] = state.Entry{
				Slug:         action.Slug,
				Tenant:       cfg.Tenant,
				LiveObjectID: newID,
				Version:      version,
				LastDeployed: time.Now().UTC().Format(time.RFC3339),
				DeployedBy:   deployerIdentity(cfg),
				AuthMode:     cfg.Auth.Mode,
				BackendSHA:   sha,
			}
			if err := state.WriteManifest(backend, cfg.Tenant, manifest); err != nil {
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("writing manifest after %s: %v", action.Slug, err),
				}
			}

			fmt.Fprintf(os.Stdout, "Applied: + %s (%s)\n", action.Slug, version)
			created++

		case reconcile.ActionUpdate:
			if err := graphClient.UpdatePolicy(ctx, action.LiveObjectID, action.BackendJSON); err != nil {
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

			backendJSON, _ := json.Marshal(action.BackendJSON)
			sha, err := backend.WritePolicy(cfg.Tenant, action.Slug, backendJSON)
			if err != nil {
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("writing backend state for %s: %v", action.Slug, err),
				}
			}

			if err := backend.CreateVersionTag(cfg.Tenant, action.Slug, newVersion, sha, fmt.Sprintf("Updated %s to %s", action.Slug, newVersion)); err != nil {
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("creating version tag for %s: %v", action.Slug, err),
				}
			}

			manifest.Policies[action.Slug] = state.Entry{
				Slug:         action.Slug,
				Tenant:       cfg.Tenant,
				LiveObjectID: action.LiveObjectID,
				Version:      newVersion,
				LastDeployed: time.Now().UTC().Format(time.RFC3339),
				DeployedBy:   deployerIdentity(cfg),
				AuthMode:     cfg.Auth.Mode,
				BackendSHA:   sha,
			}
			if err := state.WriteManifest(backend, cfg.Tenant, manifest); err != nil {
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("writing manifest after %s: %v", action.Slug, err),
				}
			}

			fmt.Fprintf(os.Stdout, "Applied: ~ %s (%s -> %s)\n", action.Slug, action.VersionFrom, newVersion)
			updated++

		case reconcile.ActionRecreate:
			newID, err := graphClient.CreatePolicy(ctx, action.BackendJSON)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed applying policy %q: %v\n", action.Slug, err)
				fmt.Fprintf(os.Stderr, "%d of %d actions succeeded before failure.\n", idx, len(actionable))
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("apply failed on %s: %v", action.Slug, err),
				}
			}

			// Compute version from current + bump
			currentVersion := "1.0.0"
			if entry, ok := manifest.Policies[action.Slug]; ok && entry.Version != "" {
				currentVersion = entry.Version
			}
			newVersion, err := semver.BumpVersion(currentVersion, semver.BumpMinor)
			if err != nil {
				newVersion = "1.1.0" // Fallback
			}

			backendJSON, _ := json.Marshal(action.BackendJSON)
			sha, writeErr := backend.WritePolicy(cfg.Tenant, action.Slug, backendJSON)
			if writeErr != nil {
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("writing backend state for %s: %v", action.Slug, writeErr),
				}
			}

			if err := backend.CreateVersionTag(cfg.Tenant, action.Slug, newVersion, sha, fmt.Sprintf("Recreated %s at %s", action.Slug, newVersion)); err != nil {
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("creating version tag for %s: %v", action.Slug, err),
				}
			}

			manifest.Policies[action.Slug] = state.Entry{
				Slug:         action.Slug,
				Tenant:       cfg.Tenant,
				LiveObjectID: newID,
				Version:      newVersion,
				LastDeployed: time.Now().UTC().Format(time.RFC3339),
				DeployedBy:   deployerIdentity(cfg),
				AuthMode:     cfg.Auth.Mode,
				BackendSHA:   sha,
			}
			if err := state.WriteManifest(backend, cfg.Tenant, manifest); err != nil {
				return &types.ExitError{
					Code:    types.ExitFatalError,
					Message: fmt.Sprintf("writing manifest after %s: %v", action.Slug, err),
				}
			}

			fmt.Fprintf(os.Stdout, "Applied: -/+ %s (%s -> %s)\n", action.Slug, currentVersion, newVersion)
			recreated++
		}
	}

	// Phase F: Summary
	fmt.Fprintf(os.Stdout, "Apply complete: %d created, %d updated, %d recreated.\n", created, updated, recreated)
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
	var result []reconcile.PolicyAction
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

// deployerIdentity returns the deployer identity string for manifest entries.
func deployerIdentity(cfg *types.Config) string {
	if cfg.Auth.Mode != "" {
		return "cactl/" + cfg.Auth.Mode
	}
	return "cactl/unknown"
}
