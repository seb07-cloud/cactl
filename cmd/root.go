package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/seb07-cloud/cactl/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "cactl",
	Short: "Conditional Access policy deploy framework",
	Long:  "cactl is a CLI deploy framework for Microsoft Entra Conditional Access policies.\nIt provides plan/apply safety with Git-native versioning and multi-tenant support.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initConfig(cmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	// Global flags (CLI-08)
	rootCmd.PersistentFlags().StringSlice("tenant", nil, "Entra tenant ID(s) -- supports multiple values")
	rootCmd.PersistentFlags().String("output", "human", "Output format: human|json")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable ANSI color output")
	rootCmd.PersistentFlags().Bool("ci", false, "Non-interactive CI mode")
	rootCmd.PersistentFlags().Bool("auto-approve", false, "Skip confirmation prompts (required with --ci for write operations)")
	rootCmd.PersistentFlags().String("config", "", "Config file path (default: .cactl/config.yaml)")
	rootCmd.PersistentFlags().String("log-level", "info", "Log level: debug|info|warn|error")
	rootCmd.PersistentFlags().String("auth-mode", "", "Auth mode: az-cli|client-secret|client-certificate")
}

func initConfig(cmd *cobra.Command) error {
	v := viper.GetViper()

	// 1. Config file
	cfgFile, _ := cmd.Flags().GetString("config")
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".cactl")
	}

	// 2. Env vars: CACTL_TENANT, CACTL_OUTPUT, CACTL_NO_COLOR, etc.
	v.SetEnvPrefix("CACTL")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	// 3. Read config file (ignore if not found -- init hasn't been run yet)
	if err := v.ReadInConfig(); err != nil {
		var configNotFound viper.ConfigFileNotFoundError
		if !errors.As(err, &configNotFound) {
			return fmt.Errorf("reading config: %w", err)
		}
	}

	// 4. Bind flags to viper AFTER config is read (critical ordering per Pitfall 3)
	if err := v.BindPFlags(cmd.Flags()); err != nil {
		return fmt.Errorf("binding flags: %w", err)
	}

	// 5. Load and validate resolved config
	cfg, err := config.Load(v)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if err := config.Validate(cfg); err != nil {
		return err
	}

	return nil
}

// SetVersionInfo configures the root command version string from build-time ldflags.
func SetVersionInfo(v, c, d string) {
	rootCmd.Version = fmt.Sprintf("%s (commit: %s, built: %s)", v, c, d)
}

// Execute runs the root command and returns any error.
func Execute() error {
	return rootCmd.Execute()
}
