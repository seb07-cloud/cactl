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
	"github.com/seb07-cloud/cactl/internal/state"
	"github.com/seb07-cloud/cactl/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var driftCmd = &cobra.Command{
	Use:   "drift",
	Short: "Detect configuration drift between backend and live tenant",
	Long:  "Perform read-only reconciliation to detect drift between Git backend state\nand the live Entra tenant. No changes are made.\nUse in CI for scheduled checks or pre-deploy validation.",
	RunE:  runDrift,
}

func init() {
	rootCmd.AddCommand(driftCmd)
	driftCmd.Flags().String("policy", "", "Filter drift output to a single policy slug")
}

func runDrift(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. Load config
	cfg, err := config.LoadFromGlobal()
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("loading config: %v", err),
		}
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
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("listing backend policies: %v", err),
		}
	}

	backendPolicies := make(map[string]reconcile.BackendPolicy)
	for _, slug := range slugs {
		data, err := backend.ReadPolicy(cfg.Tenant, slug)
		if err != nil {
			return &types.ExitError{
				Code:    types.ExitFatalError,
				Message: fmt.Sprintf("reading backend policy %s: %v", slug, err),
			}
		}
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			return &types.ExitError{
				Code:    types.ExitFatalError,
				Message: fmt.Sprintf("parsing backend policy %s: %v", slug, err),
			}
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
			return &types.ExitError{
				Code:    types.ExitFatalError,
				Message: fmt.Sprintf("normalizing live policy %s: %v", p.ID, err),
			}
		}
		var m map[string]interface{}
		if err := json.Unmarshal(normalized, &m); err != nil {
			return &types.ExitError{
				Code:    types.ExitFatalError,
				Message: fmt.Sprintf("parsing normalized policy %s: %v", p.ID, err),
			}
		}
		livePolicies[p.ID] = reconcile.LivePolicy{
			NormalizedData: m,
			Slug:           p.DisplayName,
		}
	}

	// 8. Load manifest
	manifest, err := state.ReadManifest(backend, cfg.Tenant)
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("reading manifest: %v", err),
		}
	}

	// 9. Reconcile
	actions := reconcile.Reconcile(backendPolicies, livePolicies, manifest)

	// 10. Filter to actionable (non-noop). For drift, keep Untracked since untracked IS drift.
	actionable := filterDriftActionable(actions)

	// Apply --policy filter if set
	policyFilter, _ := cmd.Flags().GetString("policy")
	if policyFilter != "" {
		actionable = filterBySlug(actionable, policyFilter)
	}

	// 11. No drift
	if len(actionable) == 0 {
		fmt.Fprintln(os.Stdout, "No drift detected. Backend and live state are in sync.")
		return nil
	}

	// 12. Render drift output (reuse plan renderer)
	v := viper.GetViper()
	useColor := output.ShouldUseColor(v)
	if cfg.Output == "json" {
		if err := output.RenderPlanJSON(os.Stdout, actionable, nil, nil); err != nil {
			return &types.ExitError{
				Code:    types.ExitFatalError,
				Message: fmt.Sprintf("rendering JSON: %v", err),
			}
		}
	} else {
		output.RenderPlan(os.Stdout, actionable, nil, nil, useColor)
	}

	// 13. Print remediation suggestions (DRIFT-04)
	if cfg.Output != "json" {
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintln(os.Stdout, "Remediation options:")
		fmt.Fprintln(os.Stdout, "  cactl apply            apply backend state to live tenant")
		fmt.Fprintln(os.Stdout, "  cactl import --force   update backend from live tenant state")
		fmt.Fprintln(os.Stdout, "  (no action)            this was a report-only check")
	}

	// 14. Exit 1 for drift detected (DRIFT-03)
	return &types.ExitError{Code: types.ExitChanges, Message: "drift detected"}
}

// filterDriftActionable returns actions that represent drift (non-noop).
// Unlike apply's filterActionable, drift keeps Untracked actions since
// untracked policies represent drift (DRIFT-02).
func filterDriftActionable(actions []reconcile.PolicyAction) []reconcile.PolicyAction {
	var result []reconcile.PolicyAction
	for _, a := range actions {
		if a.Action != reconcile.ActionNoop {
			result = append(result, a)
		}
	}
	return result
}

// filterBySlug returns only actions matching the given slug.
// If slug is empty, all actions are returned.
func filterBySlug(actions []reconcile.PolicyAction, slug string) []reconcile.PolicyAction {
	if slug == "" {
		return actions
	}
	var result []reconcile.PolicyAction
	for _, a := range actions {
		if a.Slug == slug {
			result = append(result, a)
		}
	}
	return result
}
