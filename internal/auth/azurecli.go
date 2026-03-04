package auth

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// AzureCLIProvider wraps azidentity.AzureCLICredential.
// It uses the active az login session to acquire tokens.
type AzureCLIProvider struct{}

// Credential creates a new AzureCLICredential for the given tenant.
// IMPORTANT: TenantID is passed in options to force token acquisition for
// the correct tenant, avoiding Pitfall 4 (wrong tenant from active subscription).
func (p *AzureCLIProvider) Credential(_ context.Context, tenantID string) (azcore.TokenCredential, error) {
	opts := &azidentity.AzureCLICredentialOptions{
		TenantID: tenantID,
	}

	cred, err := azidentity.NewAzureCLICredential(opts)
	if err != nil {
		return nil, fmt.Errorf("creating az-cli credential for tenant %s: %w", tenantID, err)
	}

	return cred, nil
}

// Mode returns the auth mode identifier for Azure CLI authentication.
func (p *AzureCLIProvider) Mode() string {
	return AuthModeAzCLI
}
