package types

// Config holds the resolved configuration for a cactl invocation.
// All fields use mapstructure tags matching the YAML config keys (snake_case).
type Config struct {
	Tenants  []string   `mapstructure:"tenant"`
	Auth     AuthConfig `mapstructure:"auth"`
	Output   string     `mapstructure:"output"`
	LogLevel string     `mapstructure:"log_level"`
	NoColor  bool       `mapstructure:"no_color"`
	CI       bool       `mapstructure:"ci"`

	// Tenant is kept for backward compatibility with code that reads a single tenant.
	//
	// Deprecated: Use Tenants or FirstTenant() instead.
	Tenant string `mapstructure:"-"`
}

// FirstTenant returns the first tenant ID or empty string if none configured.
func (c *Config) FirstTenant() string {
	if len(c.Tenants) > 0 {
		return c.Tenants[0]
	}
	return ""
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Mode         string `mapstructure:"mode"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"` // Resolved from CACTL_CLIENT_SECRET env var only; never from config file //nolint:gosec // G117 - field name matches pattern but is not a hardcoded secret
	CertPath     string `mapstructure:"cert_path"`
}
