package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/seb07-cloud/cactl/internal/config"
	"github.com/seb07-cloud/cactl/internal/output"
	"github.com/seb07-cloud/cactl/internal/testengine"
	"github.com/seb07-cloud/cactl/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var testCmd = &cobra.Command{
	Use:   "test [test-files...]",
	Short: "Run policy test scenarios",
	Long: `Evaluate YAML test scenarios against local policy files.

Tests are pure local evaluation -- no Azure API calls are made.
Each test file defines scenarios with sign-in contexts and expected outcomes.
The test engine evaluates all matching policies and compares results.

If no test files are specified, discovers tests from tests/<tenantID>/*.yaml.`,
	RunE: runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, args []string) error {
	v := viper.GetViper()

	// Resolve tenant through the standard config chain (flag > env > az CLI context)
	cfg, err := config.LoadFromGlobal()
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("loading config: %v", err),
		}
	}
	tenant := cfg.Tenant
	if tenant == "" {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: "tenant is required: use --tenant, set CACTL_TENANT, or log in with az login",
		}
	}

	// Determine test file paths
	var testPaths []string
	if len(args) > 0 {
		testPaths = args
	} else {
		pattern := filepath.Join("tests", tenant, "*.yaml")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return &types.ExitError{
				Code:    types.ExitFatalError,
				Message: fmt.Sprintf("discovering test files: %v", err),
			}
		}
		testPaths = matches
	}

	if len(testPaths) == 0 {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("no test files found (looked in tests/%s/*.yaml)", tenant),
		}
	}

	// Policy directory
	policyDir := filepath.Join("policies", tenant)

	// Run tests
	report, err := testengine.RunTests(testPaths, policyDir)
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("running tests: %v", err),
		}
	}

	// Render output
	outputFormat := v.GetString("output")
	if outputFormat == "json" {
		if err := testengine.RenderJSON(os.Stdout, report); err != nil {
			return &types.ExitError{
				Code:    types.ExitFatalError,
				Message: fmt.Sprintf("rendering JSON output: %v", err),
			}
		}
	} else {
		useColor := output.ShouldUseColor(v)
		testengine.RenderHuman(os.Stdout, report, useColor)
	}

	// Exit codes
	_, _, failed, errors := testengine.Summary(report)
	if errors > 0 {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("%d test errors", errors),
		}
	}
	if failed > 0 {
		return &types.ExitError{
			Code:    types.ExitChanges,
			Message: fmt.Sprintf("%d tests failed", failed),
		}
	}

	return nil
}
