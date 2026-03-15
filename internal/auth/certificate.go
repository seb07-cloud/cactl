package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// ClientCertificateProvider wraps azidentity.ClientCertificateCredential.
// It authenticates using a service principal's client ID and certificate.
//
// CRITICAL (AUTH-06): Error messages include the certificate path but never
// the certificate file contents.
type ClientCertificateProvider struct {
	clientID string
	certPath string
}

// NewClientCertificateProvider creates a new ClientCertificateProvider.
func NewClientCertificateProvider(clientID, certPath string) *ClientCertificateProvider {
	return &ClientCertificateProvider{
		clientID: clientID,
		certPath: certPath,
	}
}

// Credential creates a new ClientCertificateCredential for the given tenant.
// It reads and parses the certificate file using azidentity.ParseCertificates,
// which handles both PEM and PKCS#12 formats.
func (p *ClientCertificateProvider) Credential(_ context.Context, tenantID string) (azcore.TokenCredential, error) {
	certData, err := os.ReadFile(p.certPath) //nolint:gosec // G304 - path from config/traversal
	if err != nil {
		return nil, fmt.Errorf("reading certificate file %s: %w", p.certPath, err)
	}

	certs, key, err := azidentity.ParseCertificates(certData, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing certificate from %s: %w", p.certPath, err)
	}

	cred, err := azidentity.NewClientCertificateCredential(tenantID, p.clientID, certs, key, nil)
	if err != nil {
		return nil, fmt.Errorf("creating client certificate credential for tenant %s: %w", tenantID, err)
	}

	return cred, nil
}

// Mode returns the auth mode identifier for client certificate authentication.
func (p *ClientCertificateProvider) Mode() string {
	return AuthModeClientCertificate
}
