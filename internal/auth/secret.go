package auth

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// ClientSecretProvider wraps azidentity.ClientSecretCredential.
// It authenticates using a service principal's client ID and secret.
//
// CRITICAL (AUTH-06): This type deliberately has no String(), Format(), or
// GoString() method. The clientSecret field is unexported and must never
// appear in error messages, logs, or output.
type ClientSecretProvider struct {
	clientID     string
	clientSecret string
}

// NewClientSecretProvider creates a new ClientSecretProvider.
func NewClientSecretProvider(clientID, clientSecret string) *ClientSecretProvider {
	return &ClientSecretProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// Credential creates a new ClientSecretCredential for the given tenant.
// Error messages include the tenant ID but never the client secret value.
func (p *ClientSecretProvider) Credential(_ context.Context, tenantID string) (azcore.TokenCredential, error) {
	cred, err := azidentity.NewClientSecretCredential(tenantID, p.clientID, p.clientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("creating client secret credential for tenant %s: %w", tenantID, err)
	}

	return cred, nil
}

// Mode returns the auth mode identifier for client secret authentication.
func (p *ClientSecretProvider) Mode() string {
	return AuthModeClientSecret
}
