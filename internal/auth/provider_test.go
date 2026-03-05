package auth

import (
	"testing"

	"github.com/seb07-cloud/cactl/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestResolveAuthMode(t *testing.T) {
	tests := []struct {
		name     string
		cfg      types.AuthConfig
		expected string
	}{
		{
			name:     "empty config falls back to az-cli",
			cfg:      types.AuthConfig{},
			expected: AuthModeAzCLI,
		},
		{
			name: "explicit mode is returned directly",
			cfg: types.AuthConfig{
				Mode: AuthModeClientSecret,
			},
			expected: AuthModeClientSecret,
		},
		{
			name: "auto-detect client-secret from ClientID and ClientSecret",
			cfg: types.AuthConfig{
				ClientID:     "some-client-id",
				ClientSecret: "some-secret",
			},
			expected: AuthModeClientSecret,
		},
		{
			name: "auto-detect client-certificate from ClientID and CertPath",
			cfg: types.AuthConfig{
				ClientID: "some-client-id",
				CertPath: "/path/to/cert.pem",
			},
			expected: AuthModeClientCertificate,
		},
		{
			name: "client-secret wins when both secret and cert are present",
			cfg: types.AuthConfig{
				ClientID:     "some-client-id",
				ClientSecret: "some-secret",
				CertPath:     "/path/to/cert.pem",
			},
			expected: AuthModeClientSecret,
		},
		{
			name: "explicit mode overrides auto-detect",
			cfg: types.AuthConfig{
				Mode:         AuthModeAzCLI,
				ClientID:     "some-client-id",
				ClientSecret: "some-secret",
			},
			expected: AuthModeAzCLI,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ResolveAuthMode(tc.cfg)
			assert.Equal(t, tc.expected, result)
		})
	}
}
