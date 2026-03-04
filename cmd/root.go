package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "cactl",
	Short:         "Conditional Access policy deploy framework",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command and returns any error.
func Execute() error {
	return rootCmd.Execute()
}
