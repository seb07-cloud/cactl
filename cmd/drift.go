package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/seb07-cloud/cactl/internal/reconcile"
	"github.com/seb07-cloud/cactl/pkg/types"
	"github.com/spf13/cobra"
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

	// Bootstrap
	p, err := NewPipeline(ctx)
	if err != nil {
		return err
	}

	// Load desired + live state
	backendPolicies, err := ReadDesiredPolicies(p.Cfg.Tenant)
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("loading desired state: %v", err),
		}
	}

	livePoliciesGraph, err := p.GraphClient.ListPolicies(ctx)
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("fetching live policies: %v", err),
		}
	}

	livePolicies, err := NormalizeLivePolicies(livePoliciesGraph)
	if err != nil {
		return err
	}

	// Reconcile
	actions := reconcile.Reconcile(backendPolicies, livePolicies, p.Manifest)

	// Filter to actionable (non-noop). For drift, keep Untracked since untracked IS drift.
	actionable := filterDriftActionable(actions)

	// Apply --policy filter if set
	policyFilter, _ := cmd.Flags().GetString("policy")
	if policyFilter != "" {
		actionable = filterBySlug(actionable, policyFilter)
	}

	// No drift
	if len(actionable) == 0 {
		fmt.Fprintln(os.Stdout, "No drift detected. Backend and live state are in sync.")
		return nil
	}

	// Render drift output (reuse plan renderer)
	if err := p.RenderPlan(os.Stdout, actionable, nil, nil); err != nil {
		return err
	}

	// Print remediation suggestions (DRIFT-04)
	if p.Cfg.Output != "json" {
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintln(os.Stdout, "Remediation options:")
		fmt.Fprintln(os.Stdout, "  cactl apply            apply backend state to live tenant")
		fmt.Fprintln(os.Stdout, "  cactl import --force   update backend from live tenant state")
		fmt.Fprintln(os.Stdout, "  (no action)            this was a report-only check")
	}

	// Exit 1 for drift detected (DRIFT-03)
	return &types.ExitError{Code: types.ExitChanges, Message: "drift detected"}
}

// filterDriftActionable returns actions that represent drift (non-noop).
// Unlike apply's filterActionable, drift keeps Untracked actions since
// untracked policies represent drift (DRIFT-02).
func filterDriftActionable(actions []reconcile.PolicyAction) []reconcile.PolicyAction {
	result := make([]reconcile.PolicyAction, 0, len(actions))
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
	result := make([]reconcile.PolicyAction, 0, len(actions))
	for _, a := range actions {
		if a.Slug == slug {
			result = append(result, a)
		}
	}
	return result
}
