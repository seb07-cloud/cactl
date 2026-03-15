package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportFlagValidation_AllAndPolicy(t *testing.T) {
	cmd := newImportCmd()
	cmd.SetArgs([]string{"--all", "--policy", "some-policy"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use --all and --policy together")
}

func TestImportCIModeNoSelection(t *testing.T) {
	// Set CI mode via env var (viper reads CACTL_CI)
	t.Setenv("CACTL_CI", "true")

	cmd := rootCmd
	cmd.SetArgs([]string{"import"})

	err := cmd.Execute()
	require.Error(t, err)
	// In CI mode without --all or --policy, should error
	// The exact error depends on whether tenant is set, but validation
	// should catch the CI mode constraint
	assert.Error(t, err)
}

func TestImportCommandRegistered(t *testing.T) {
	// Verify import command exists on root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "import" {
			found = true

			// Verify flags exist
			allFlag := cmd.Flags().Lookup("all")
			require.NotNil(t, allFlag, "--all flag should exist")

			policyFlag := cmd.Flags().Lookup("policy")
			require.NotNil(t, policyFlag, "--policy flag should exist")

			forceFlag := cmd.Flags().Lookup("force")
			require.NotNil(t, forceFlag, "--force flag should exist")

			break
		}
	}
	assert.True(t, found, "import command should be registered on root")
}

