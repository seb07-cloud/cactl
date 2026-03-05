package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Preview changes between backend and live policies",
	Long:  "Show a terraform-style diff of Conditional Access policy changes.\nCompares Git backend state against the live tenant and displays actions needed.",
	RunE:  runPlan,
}

func init() {
	rootCmd.AddCommand(planCmd)
}

func runPlan(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. Load config
	cfg, err := config.LoadFromGlobal()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// 2. Validate tenant is set
	if cfg.Tenant == "" {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: "tenant is required: use --tenant or set CACTL_TENANT",
		}
	}

	// 3. Create auth factory and credential
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

	// 4. Create graph client
	graphClient := graph.NewClient(cred, cfg.Tenant)

	// 5. Create git backend
	backend, err := state.NewGitBackend(".")
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("git backend: %v", err),
		}
	}

	// 6. Load backend policies
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

	// 7. Load live policies
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

	// 8. Load manifest
	manifest, err := state.ReadManifest(backend, cfg.Tenant)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	// 9. Reconcile
	actions := reconcile.Reconcile(backendPolicies, livePolicies, manifest)

	// 10. Compute semver bumps
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
		// Convert reconcile.FieldDiff to semver.FieldDiff
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

		// Look up current version from manifest
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

		// SEMV-06: Add warning for MAJOR bumps
		if bumpLevel == semver.BumpMajor {
			actions[i].Warnings = append(actions[i].Warnings, "MAJOR version bump: scope-affecting change detected")
		}
	}

	// 11. Run validations
	breakGlassAccounts := v.GetStringSlice("validation.break_glass_accounts")
	valCfg := validate.ValidationConfig{
		BreakGlassAccounts: breakGlassAccounts,
	}
	// Convert to validate.PolicyAction (local mirror types)
	valActions := make([]validate.PolicyAction, len(actions))
	for i, a := range actions {
		valActions[i] = validate.PolicyAction{
			Slug:        a.Slug,
			Action:      validate.ActionType(a.Action),
			BackendJSON: a.BackendJSON,
		}
	}
	validations := validate.ValidatePlan(valActions, valCfg)

	// 12. Resolve display names
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

	// 13. Render
	useColor := output.ShouldUseColor(v)
	if cfg.Output == "json" {
		if err := output.RenderPlanJSON(os.Stdout, actions, validations, resolver); err != nil {
			return fmt.Errorf("rendering JSON: %w", err)
		}
	} else {
		output.RenderPlan(os.Stdout, actions, validations, resolver, useColor)
	}

	// 14. Exit code
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
			Message: "validation errors detected",
		}
	}

	hasChanges := false
	for _, a := range actions {
		if a.Action == reconcile.ActionCreate || a.Action == reconcile.ActionUpdate || a.Action == reconcile.ActionRecreate {
			hasChanges = true
			break
		}
	}
	if hasChanges {
		return &types.ExitError{
			Code:    types.ExitChanges,
			Message: "changes detected",
		}
	}

	return nil
}
