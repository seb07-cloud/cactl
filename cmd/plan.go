package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/seb07-cloud/cactl/internal/reconcile"
	"github.com/seb07-cloud/cactl/pkg/types"
	"github.com/spf13/cobra"
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
	if err := p.ComputeSemverBumps(actions, nil); err != nil {
		return err
	}
	validations := p.RunValidations(actions)
	resolver := p.ResolveDisplayNames(ctx, actions)

	if err := p.RenderPlan(os.Stdout, actions, validations, resolver); err != nil {
		return err
	}

	// Exit codes
	if HasValidationErrors(validations) {
		return &types.ExitError{Code: types.ExitValidationError, Message: "validation errors detected"}
	}

	hasChanges := false
	for _, a := range actions {
		if a.Action == reconcile.ActionCreate || a.Action == reconcile.ActionUpdate || a.Action == reconcile.ActionRecreate {
			hasChanges = true
			break
		}
	}
	if hasChanges {
		return &types.ExitError{Code: types.ExitChanges, Message: "changes detected"}
	}
	return nil
}
