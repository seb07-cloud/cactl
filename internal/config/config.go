package config

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/seb07-cloud/cactl/pkg/types"
	"github.com/spf13/viper"
)

// Load reads the resolved configuration from a viper instance and returns
// a populated Config struct. Auth secrets are explicitly overridden from
// viper (which resolves env vars) rather than from config file unmarshalling.
func Load(v *viper.Viper) (*types.Config, error) {
	var cfg types.Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	// Resolve tenants from StringSlice flag binding
	cfg.Tenants = v.GetStringSlice("tenant")

	// Backward compatibility: if --tenant slice is empty but CACTL_TENANT env
	// var provides a single string value, wrap it in a slice.
	if len(cfg.Tenants) == 0 {
		if single := v.GetString("tenant"); single != "" {
			cfg.Tenants = []string{single}
		}
	}

	// Fallback: resolve tenant from current az CLI context if still unset.
	if len(cfg.Tenants) == 0 {
		if tenantID := resolveAzCLITenant(); tenantID != "" {
			cfg.Tenants = []string{tenantID}
		}
	}

	// Keep deprecated Tenant field in sync for backward compatibility
	if len(cfg.Tenants) > 0 {
		cfg.Tenant = cfg.Tenants[0]
	}

	// Override auth secrets from viper (env vars via CACTL_ prefix).
	// These are intentionally read from viper.GetString rather than
	// unmarshalling, to ensure they come from env vars, not config file.
	cfg.Auth.ClientID = v.GetString("client_id")
	cfg.Auth.ClientSecret = v.GetString("client_secret")
	cfg.Auth.CertPath = v.GetString("cert_path")

	return &cfg, nil
}

// LoadFromGlobal is a convenience wrapper that loads config from the
// global viper instance.
func LoadFromGlobal() (*types.Config, error) {
	return Load(viper.GetViper())
}

// resolveAzCLITenant shells out to `az account show` and extracts the tenantId
// from the current subscription context. Returns empty string on any failure
// (az CLI not installed, not logged in, etc.) — callers treat this as a
// best-effort fallback.
func resolveAzCLITenant() string {
	out, err := exec.Command("az", "account", "show", "--query", "tenantId", "-o", "tsv").Output()
	if err != nil {
		return ""
	}
	tid := strings.TrimSpace(string(out))
	// Sanity-check: tenant IDs are UUIDs, reject obviously wrong output.
	if len(tid) != 36 {
		return ""
	}
	return tid
}

