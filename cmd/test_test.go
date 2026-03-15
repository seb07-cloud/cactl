package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestCmd_Registered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "test" {
			found = true
			break
		}
	}
	assert.True(t, found, "testCmd should be registered on rootCmd")
}

func TestTestCmd_MissingTenant(t *testing.T) {
	// Reset viper for test isolation
	rootCmd.SetArgs([]string{"test"})
	err := rootCmd.Execute()
	// Should fail with tenant required error (exit code 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tenant")
}

func TestTestCmd_WithTestFiles(t *testing.T) {
	dir := t.TempDir()

	// Create policy directory with a simple policy
	tenantID := "test-tenant-123"
	policyDir := filepath.Join(dir, "policies", tenantID)
	require.NoError(t, os.MkdirAll(policyDir, 0755))

	policy := map[string]interface{}{
		"state": "enabled",
		"conditions": map[string]interface{}{
			"users": map[string]interface{}{
				"includeUsers": []string{"All"},
			},
			"applications": map[string]interface{}{
				"includeApplications": []string{"All"},
			},
			"clientAppTypes": []string{"exchangeActiveSync", "other"},
		},
		"grantControls": map[string]interface{}{
			"builtInControls": []string{"block"},
			"operator":        "OR",
		},
	}
	policyJSON, _ := json.Marshal(policy)
	require.NoError(t, os.WriteFile(filepath.Join(policyDir, "cap001-block-legacy.json"), policyJSON, 0644))

	// Create test file
	testDir := filepath.Join(dir, "tests", tenantID)
	require.NoError(t, os.MkdirAll(testDir, 0755))
	testYAML := `name: Block legacy auth
scenarios:
  - name: Legacy client blocked
    context:
      user: any
      application: All
      clientAppType: exchangeActiveSync
    expect:
      result: block
`
	testFile := filepath.Join(testDir, "block-legacy.yaml")
	require.NoError(t, os.WriteFile(testFile, []byte(testYAML), 0644))

	// Run from the temp directory
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(origDir) }()

	rootCmd.SetArgs([]string{"test", "--tenant", tenantID, testFile})
	err := rootCmd.Execute()
	assert.NoError(t, err, "test should pass with matching policy and expectation")
}
