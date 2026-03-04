package cmd

import (
	"errors"
	"testing"

	"github.com/seb07-cloud/cactl/pkg/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvalidOutputFormatReturnsExitError(t *testing.T) {
	// Reset viper state to avoid cross-test pollution
	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
	})

	rootCmd.SetArgs([]string{"--output", "invalid"})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
	})

	err := rootCmd.Execute()
	require.Error(t, err)

	var exitErr *types.ExitError
	require.True(t, errors.As(err, &exitErr), "expected ExitError, got: %v", err)
	assert.Equal(t, types.ExitValidationError, exitErr.Code)
	assert.Contains(t, exitErr.Message, "invalid output format")
}

func TestValidOutputFormatPasses(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
	})

	// Use "json" which is valid -- run init in a clean dir so it succeeds
	rootCmd.SetArgs([]string{"--output", "json", "init"})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
	})

	err := rootCmd.Execute()
	assert.NoError(t, err, "valid output format should not cause an error")
}
