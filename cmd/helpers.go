package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/seb07-cloud/cactl/internal/auth"
	"github.com/seb07-cloud/cactl/pkg/types"
)

// MTNT-04: Concurrent pipeline applies are not safe in v1. Advisory only -- defer lock file to v1.1.

// runForTenants executes fn sequentially for each tenant, using the ClientFactory
// to obtain per-tenant credentials. The overall exit code is the highest severity
// across all tenant executions: ExitFatalError (2) > ExitChanges (1) > ExitSuccess (0).
func runForTenants(
	ctx context.Context,
	tenants []string,
	authCfg types.AuthConfig,
	fn func(ctx context.Context, tenantID string, cred azcore.TokenCredential) error,
) error {
	factory, err := auth.NewClientFactory(authCfg)
	if err != nil {
		return fmt.Errorf("creating auth factory: %w", err)
	}

	multi := len(tenants) > 1
	maxCode := types.ExitSuccess

	for _, tenantID := range tenants {
		if multi {
			log.Printf("=== Tenant: %s ===", tenantID)
		}

		cred, err := factory.Credential(ctx, tenantID)
		if err != nil {
			return fmt.Errorf("acquiring credential for tenant %s: %w", tenantID, err)
		}

		if err := fn(ctx, tenantID, cred); err != nil {
			exitErr, ok := err.(*types.ExitError)
			if !ok {
				// Non-ExitError is fatal
				return err
			}
			if exitErr.Code > maxCode {
				maxCode = exitErr.Code
			}
			// For ExitChanges (1), continue to next tenant
			// For ExitFatalError (2) or ExitValidationError (3), stop immediately
			if exitErr.Code >= types.ExitFatalError {
				return exitErr
			}
		}
	}

	if maxCode > types.ExitSuccess {
		return &types.ExitError{
			Code:    maxCode,
			Message: fmt.Sprintf("completed with exit code %d across %d tenant(s)", maxCode, len(tenants)),
		}
	}

	return nil
}

// requireApproveInCI returns a validation error if CI mode is active but
// --auto-approve was not provided. This guards write operations (apply, rollback)
// from running unattended without explicit approval.
func requireApproveInCI(ciMode, autoApprove bool) error {
	if ciMode && !autoApprove {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: "--ci mode requires --auto-approve for write operations",
		}
	}
	return nil
}
