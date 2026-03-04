// Package auth provides the authentication layer for cactl.
//
// It implements three credential types (Azure CLI, client secret, client certificate),
// auth mode resolution with priority chain (explicit > auto-detect > fallback),
// and per-tenant credential isolation via ClientFactory.
package auth

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/sebdah/cactl/pkg/types"
)

// Auth mode constants used throughout the application.
const (
	AuthModeAzCLI             = "az-cli"
	AuthModeClientSecret      = "client-secret"
	AuthModeClientCertificate = "client-certificate"
)

// AuthProvider is the interface that all credential providers implement.
// Each provider creates azcore.TokenCredential instances for a given tenant.
type AuthProvider interface {
	// Credential returns a TokenCredential for the given tenant.
	// Each call with a different tenantID must return an isolated credential instance.
	Credential(ctx context.Context, tenantID string) (azcore.TokenCredential, error)

	// Mode returns the resolved auth mode name for display and logging.
	Mode() string
}

// ResolveAuthMode determines which authentication mode to use based on the
// provided configuration. The priority chain is:
//
//  1. Explicit mode (cfg.Mode) - already resolved from flag > env > config by viper
//  2. Auto-detect: ClientID + ClientSecret -> client-secret
//  3. Auto-detect: ClientID + CertPath -> client-certificate
//  4. Fallback: az-cli (AUTH-01)
func ResolveAuthMode(cfg types.AuthConfig) string {
	// Priority 1: explicit mode set via flag/env/config
	if cfg.Mode != "" {
		return cfg.Mode
	}

	// Priority 2: auto-detect from available credentials
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		return AuthModeClientSecret
	}
	if cfg.ClientID != "" && cfg.CertPath != "" {
		return AuthModeClientCertificate
	}

	// Priority 3: fallback to Azure CLI (AUTH-01)
	return AuthModeAzCLI
}
