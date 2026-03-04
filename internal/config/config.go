package config

import (
	"fmt"

	"github.com/sebdah/cactl/pkg/types"
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
