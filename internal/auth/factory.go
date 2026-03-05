package auth

import (
	"context"
	"fmt"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/seb07-cloud/cactl/pkg/types"
)

// ClientFactory creates and caches credential instances per tenant.
//
// CRITICAL (AUTH-05): Each tenant gets its own credential instance. The cache
// is keyed by tenantID. Credentials are never shared across tenants. This
// prevents the azidentity token cache bug (Azure/azure-sdk-for-go#19726)
// where multi-tenant token acquisition can silently use the wrong tenant's
// cached token.
type ClientFactory struct {
	provider    AuthProvider
	credentials map[string]azcore.TokenCredential
	mu          sync.RWMutex
}

// NewClientFactory creates a ClientFactory from the given auth configuration.
// It resolves the auth mode and creates the appropriate credential provider.
// Returns an error if required configuration fields are missing or the mode
// is unknown.
func NewClientFactory(cfg types.AuthConfig) (*ClientFactory, error) {
	mode := ResolveAuthMode(cfg)

	var provider AuthProvider

	switch mode {
	case AuthModeAzCLI:
		provider = &AzureCLIProvider{}

	case AuthModeClientSecret:
		if cfg.ClientID == "" {
			return nil, fmt.Errorf("client-secret auth mode requires client_id")
		}
		if cfg.ClientSecret == "" {
			return nil, fmt.Errorf("client-secret auth mode requires client_secret")
		}
		provider = NewClientSecretProvider(cfg.ClientID, cfg.ClientSecret)

	case AuthModeClientCertificate:
		if cfg.ClientID == "" {
			return nil, fmt.Errorf("client-certificate auth mode requires client_id")
		}
		if cfg.CertPath == "" {
			return nil, fmt.Errorf("client-certificate auth mode requires cert_path")
		}
		provider = NewClientCertificateProvider(cfg.ClientID, cfg.CertPath)

	default:
		return nil, fmt.Errorf("unknown auth mode: %s", mode)
	}

	return &ClientFactory{
		provider:    provider,
		credentials: make(map[string]azcore.TokenCredential),
	}, nil
}

// Credential returns a TokenCredential for the given tenant, creating one if
// it does not already exist in the cache. Thread-safe via RWMutex with
// double-check locking pattern.
func (f *ClientFactory) Credential(ctx context.Context, tenantID string) (azcore.TokenCredential, error) {
	// Fast path: read lock to check cache
	f.mu.RLock()
	if cred, ok := f.credentials[tenantID]; ok {
		f.mu.RUnlock()
		return cred, nil
	}
	f.mu.RUnlock()

	// Slow path: write lock, double-check, create
	f.mu.Lock()
	defer f.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have created it)
	if cred, ok := f.credentials[tenantID]; ok {
		return cred, nil
	}

	cred, err := f.provider.Credential(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	f.credentials[tenantID] = cred
	return cred, nil
}

// Mode returns the resolved auth mode name.
func (f *ClientFactory) Mode() string {
	return f.provider.Mode()
}
