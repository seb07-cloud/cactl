package auth

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/sebdah/cactl/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTokenCredential implements azcore.TokenCredential for testing.
type mockTokenCredential struct {
	tenantID string
}

func (m *mockTokenCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{}, nil
}

// mockProvider implements AuthProvider for testing credential isolation.
type mockProvider struct {
	mode        string
	callCount   int
	mu          sync.Mutex
	tenantsSeen []string
}

func (p *mockProvider) Credential(_ context.Context, tenantID string) (azcore.TokenCredential, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callCount++
	p.tenantsSeen = append(p.tenantsSeen, tenantID)
	return &mockTokenCredential{tenantID: tenantID}, nil
}

func (p *mockProvider) Mode() string {
	return p.mode
}

func TestNewClientFactory_AzCLI(t *testing.T) {
	factory, err := NewClientFactory(types.AuthConfig{})
	require.NoError(t, err)
	assert.Equal(t, AuthModeAzCLI, factory.Mode())
}

func TestNewClientFactory_ClientSecret_MissingClientID(t *testing.T) {
	_, err := NewClientFactory(types.AuthConfig{
		Mode:         AuthModeClientSecret,
		ClientSecret: "some-secret",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_id")
}

func TestNewClientFactory_ClientSecret_MissingSecret(t *testing.T) {
	_, err := NewClientFactory(types.AuthConfig{
		Mode:     AuthModeClientSecret,
		ClientID: "some-client-id",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_secret")
}

func TestNewClientFactory_ClientCertificate_MissingCertPath(t *testing.T) {
	_, err := NewClientFactory(types.AuthConfig{
		Mode:     AuthModeClientCertificate,
		ClientID: "some-client-id",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cert_path")
}

func TestNewClientFactory_UnknownMode(t *testing.T) {
	_, err := NewClientFactory(types.AuthConfig{
		Mode: "unknown-mode",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown auth mode")
}

func TestClientFactory_PerTenantIsolation(t *testing.T) {
	mp := &mockProvider{mode: AuthModeAzCLI}
	factory := &ClientFactory{
		provider:    mp,
		credentials: make(map[string]azcore.TokenCredential),
	}

	ctx := context.Background()

	// Get credential for tenant-a
	credA, err := factory.Credential(ctx, "tenant-a")
	require.NoError(t, err)
	require.NotNil(t, credA)

	// Get credential for tenant-b
	credB, err := factory.Credential(ctx, "tenant-b")
	require.NoError(t, err)
	require.NotNil(t, credB)

	// Verify provider was called twice (once per tenant)
	assert.Equal(t, 2, mp.callCount)
	assert.ElementsMatch(t, []string{"tenant-a", "tenant-b"}, mp.tenantsSeen)

	// Verify different credential instances returned
	assert.NotSame(t, credA, credB)
}

func TestClientFactory_CachesCredentials(t *testing.T) {
	mp := &mockProvider{mode: AuthModeAzCLI}
	factory := &ClientFactory{
		provider:    mp,
		credentials: make(map[string]azcore.TokenCredential),
	}

	ctx := context.Background()

	// Get credential for same tenant twice
	cred1, err := factory.Credential(ctx, "tenant-a")
	require.NoError(t, err)

	cred2, err := factory.Credential(ctx, "tenant-a")
	require.NoError(t, err)

	// Verify provider was called only once (cached on second call)
	assert.Equal(t, 1, mp.callCount)

	// Verify same instance returned
	assert.Same(t, cred1, cred2)
}

func TestClientFactory_ConcurrentAccess(t *testing.T) {
	mp := &mockProvider{mode: AuthModeAzCLI}
	factory := &ClientFactory{
		provider:    mp,
		credentials: make(map[string]azcore.TokenCredential),
	}

	ctx := context.Background()
	var wg sync.WaitGroup

	// Spawn multiple goroutines requesting credentials for different tenants
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(tenantID string) {
			defer wg.Done()
			cred, err := factory.Credential(ctx, tenantID)
			assert.NoError(t, err)
			assert.NotNil(t, cred)
		}(fmt.Sprintf("tenant-%d", i))
	}

	wg.Wait()

	// Verify each tenant got exactly one credential
	assert.Equal(t, 10, mp.callCount)
}
