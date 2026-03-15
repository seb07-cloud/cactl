package testengine

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func blockPolicy(slug string) PolicyWithSlug {
	return PolicyWithSlug{
		Slug: slug,
		Data: map[string]interface{}{
			"state": "enabled",
			"conditions": map[string]interface{}{
				"users": map[string]interface{}{
					"includeUsers": []interface{}{"All"},
				},
				"applications": map[string]interface{}{
					"includeApplications": []interface{}{"All"},
				},
				"clientAppTypes": []interface{}{"exchangeActiveSync", "other"},
			},
			"grantControls": map[string]interface{}{
				"builtInControls": []interface{}{"block"},
				"operator":        "OR",
			},
		},
	}
}

func grantMFAPolicy(slug string) PolicyWithSlug {
	return PolicyWithSlug{
		Slug: slug,
		Data: map[string]interface{}{
			"state": "enabled",
			"conditions": map[string]interface{}{
				"users": map[string]interface{}{
					"includeUsers": []interface{}{"All"},
				},
				"applications": map[string]interface{}{
					"includeApplications": []interface{}{"All"},
				},
			},
			"grantControls": map[string]interface{}{
				"builtInControls": []interface{}{"mfa"},
				"operator":        "OR",
			},
		},
	}
}

func TestRunTestFile_AllPass(t *testing.T) {
	policies := []PolicyWithSlug{blockPolicy("cap001-block-legacy")}

	// Write a temp test file
	dir := t.TempDir()
	testFile := dir + "/test.yaml"
	writeTestYAML(t, testFile, `
name: Block legacy auth
scenarios:
  - name: Legacy client blocked
    context:
      user: any
      application: All
      clientAppType: exchangeActiveSync
    expect:
      result: block
`)

	result, err := RunTestFile(testFile, policies)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 0, result.Failed)
	assert.True(t, result.Scenarios[0].Passed)
}

func TestRunTestFile_OneFail(t *testing.T) {
	policies := []PolicyWithSlug{blockPolicy("cap001-block-legacy")}

	dir := t.TempDir()
	testFile := dir + "/test.yaml"
	writeTestYAML(t, testFile, `
name: Incorrect expectation
scenarios:
  - name: Expects grant but gets block
    context:
      user: any
      application: All
      clientAppType: exchangeActiveSync
    expect:
      result: grant
`)

	result, err := RunTestFile(testFile, policies)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 1, result.Failed)
	assert.False(t, result.Scenarios[0].Passed)
}

func TestRunTestFile_PolicyFilter(t *testing.T) {
	policies := []PolicyWithSlug{
		blockPolicy("cap001-block-legacy"),
		grantMFAPolicy("cap100-admin-mfa"),
	}

	dir := t.TempDir()
	testFile := dir + "/test.yaml"
	// Filter to only cap100 -- legacy block policy excluded
	writeTestYAML(t, testFile, `
name: Filtered test
policies:
  - cap100
scenarios:
  - name: MFA grant when filtered
    context:
      user: any
      application: All
    expect:
      result: grant
      controls:
        - mfa
`)

	result, err := RunTestFile(testFile, policies)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed, "should pass with filtered policies")
	assert.True(t, result.Scenarios[0].Passed)
}

func TestRunTestFile_GrantControls(t *testing.T) {
	policies := []PolicyWithSlug{grantMFAPolicy("cap100-admin-mfa")}

	dir := t.TempDir()
	testFile := dir + "/test.yaml"
	writeTestYAML(t, testFile, `
name: Control check
scenarios:
  - name: MFA required
    context:
      user: any
      application: All
    expect:
      result: grant
      controls:
        - mfa
  - name: Missing control
    context:
      user: any
      application: All
    expect:
      result: grant
      controls:
        - compliantDevice
`)

	result, err := RunTestFile(testFile, policies)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Passed, "mfa control should match")
	assert.Equal(t, 1, result.Failed, "compliantDevice not present should fail")
}

func TestFilterPolicies(t *testing.T) {
	all := []PolicyWithSlug{
		{Slug: "cap001-block-legacy"},
		{Slug: "cap100-admin-mfa"},
		{Slug: "cap100-admin-risk"},
	}

	// Empty filter returns all
	filtered := filterPolicies(all, nil)
	assert.Len(t, filtered, 3)

	// Exact match
	filtered = filterPolicies(all, []string{"cap001-block-legacy"})
	assert.Len(t, filtered, 1)

	// Prefix match
	filtered = filterPolicies(all, []string{"cap100"})
	assert.Len(t, filtered, 2)

	// No match
	filtered = filterPolicies(all, []string{"cap999"})
	assert.Empty(t, filtered)
}

func TestContainsAllControls(t *testing.T) {
	assert.True(t, containsAllControls([]string{"mfa", "compliantDevice"}, []string{"mfa"}))
	assert.True(t, containsAllControls([]string{"mfa"}, []string{"mfa"}))
	assert.False(t, containsAllControls([]string{"mfa"}, []string{"compliantDevice"}))
	assert.True(t, containsAllControls([]string{"mfa"}, nil))
}

func writeTestYAML(t *testing.T, path, content string) {
	t.Helper()
	err := writeFile(path, []byte(content))
	require.NoError(t, err)
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
