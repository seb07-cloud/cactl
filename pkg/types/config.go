package types

// Config holds the resolved configuration for a cactl invocation.
// All fields use mapstructure tags matching the YAML config keys (snake_case).
type Config struct {
	Tenant   string     `mapstructure:"tenant"`
	Auth     AuthConfig `mapstructure:"auth"`
	Output   string     `mapstructure:"output"`
	LogLevel string     `mapstructure:"log_level"`
	NoColor  bool       `mapstructure:"no_color"`
	CI       bool       `mapstructure:"ci"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Mode         string `mapstructure:"mode"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"` // Resolved from CACTL_CLIENT_SECRET env var only; never from config file
	CertPath     string `mapstructure:"cert_path"`
}
