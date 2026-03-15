package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

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
	"github.com/spf13/viper"
)

// CommandPipeline holds shared state for plan/apply/drift commands.
type CommandPipeline struct {
	Cfg         *types.Config
	GraphClient *graph.Client
	Backend     *state.GitBackend
	Manifest    *state.Manifest
}

// NewPipeline performs the 5-step bootstrap sequence shared across commands:
// config load, tenant validation, auth, graph client, backend + manifest.
func NewPipeline(ctx context.Context) (*CommandPipeline, error) {
	// 1. Load config
	cfg, err := config.LoadFromGlobal()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	// 2. Validate tenant is set
	if cfg.Tenant == "" {
		return nil, &types.ExitError{
			Code:    types.ExitFatalError,
			Message: "tenant is required: use --tenant, set CACTL_TENANT, or log in with az login",
		}
	}

	// 3. Create auth factory and credential
	factory, err := auth.NewClientFactory(cfg.Auth)
	if err != nil {
		return nil, &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("auth setup failed: %v", err),
		}
	}

	cred, err := factory.Credential(ctx, cfg.Tenant)
	if err != nil {
		return nil, &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("authentication failed: %v", err),
		}
	}

	// 4. Create graph client
	graphClient := graph.NewClient(cred, cfg.Tenant)

	// 5. Create git backend and load manifest
	backend, err := state.NewGitBackend(".")
	if err != nil {
		return nil, &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("git backend: %v", err),
		}
	}

	manifest, err := state.ReadManifest(backend, cfg.Tenant)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	return &CommandPipeline{
		Cfg:         cfg,
		GraphClient: graphClient,
		Backend:     backend,
		Manifest:    manifest,
	}, nil
}

// NormalizeLivePolicies converts raw graph policies into the normalized
// LivePolicy map used by the reconciler.
func NormalizeLivePolicies(policies []graph.Policy) (map[string]reconcile.LivePolicy, error) {
	livePolicies := make(map[string]reconcile.LivePolicy)
	for _, p := range policies {
		normalized, err := normalize.Normalize(p.RawJSON)
		if err != nil {
			return nil, fmt.Errorf("normalizing live policy %s: %w", p.ID, err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(normalized, &m); err != nil {
			return nil, fmt.Errorf("parsing normalized policy %s: %w", p.ID, err)
		}
		livePolicies[p.ID] = reconcile.LivePolicy{
			NormalizedData: m,
			Slug:           p.DisplayName,
		}
	}
	return livePolicies, nil
}

// ComputeSemverBumps computes version bumps for update actions based on
// semver field configuration. If overrideBump is non-nil, it overrides
// the computed bump level for all actions.
func (p *CommandPipeline) ComputeSemverBumps(actions []reconcile.PolicyAction, overrideBump *semver.BumpLevel) error {
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
		if overrideBump != nil {
			bumpLevel = *overrideBump
		}
		actions[i].BumpLevel = bumpLevel.String()

		// Look up current version from manifest
		currentVersion := "1.0.0"
		if entry, ok := p.Manifest.Policies[actions[i].Slug]; ok && entry.Version != "" {
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
	return nil
}

// RunValidations runs the validation suite against the given actions.
func (p *CommandPipeline) RunValidations(actions []reconcile.PolicyAction) []validate.ValidationResult {
	breakGlassAccounts := viper.GetStringSlice("validation.break_glass_accounts")
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
	return validate.ValidatePlan(valActions, valCfg)
}

// ResolveDisplayNames resolves Azure AD object GUIDs to display names
// for human-readable output. Errors are non-fatal.
func (p *CommandPipeline) ResolveDisplayNames(ctx context.Context, actions []reconcile.PolicyAction) *resolve.Resolver {
	policyMaps := make([]map[string]interface{}, 0, len(actions))
	for _, a := range actions {
		if a.BackendJSON != nil {
			policyMaps = append(policyMaps, a.BackendJSON)
		}
	}
	refs := resolve.CollectRefs(policyMaps)

	resolver := resolve.NewResolver(p.GraphClient)
	if len(refs) > 0 {
		if err := resolver.ResolveAll(ctx, refs); err != nil {
			// Non-fatal: continue with raw IDs
			_ = err
		}
	}
	return resolver
}

// RenderPlan renders the plan output in the configured format (human or JSON).
func (p *CommandPipeline) RenderPlan(w io.Writer, actions []reconcile.PolicyAction, validations []validate.ValidationResult, resolver *resolve.Resolver) error {
	if p.Cfg.Output == "json" {
		if err := output.RenderPlanJSON(w, actions, validations, resolver); err != nil {
			return fmt.Errorf("rendering JSON: %w", err)
		}
	} else {
		useColor := output.ShouldUseColor(viper.GetViper())
		output.RenderPlan(w, actions, validations, resolver, useColor)
	}
	return nil
}

// HasValidationErrors checks if any validation result has error severity.
func HasValidationErrors(validations []validate.ValidationResult) bool {
	for _, val := range validations {
		if val.Severity == validate.SeverityError {
			return true
		}
	}
	return false
}
