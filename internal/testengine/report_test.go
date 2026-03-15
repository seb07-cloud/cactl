package testengine

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleReport() *TestReport {
	return &TestReport{
		Files: []FileResult{
			{
				File:   "tests/block-legacy-auth.yaml",
				Passed: 1,
				Failed: 1,
				Scenarios: []ScenarioResult{
					{
						ScenarioName: "Legacy client should be blocked",
						Passed:       true,
						Expected:     ExpectedOutcome{Result: "block"},
						Got: CombinedDecision{
							Result:           ResultBlock,
							MatchingPolicies: []string{"cap001-block-legacy"},
						},
						MatchingPolicies: []string{"cap001-block-legacy"},
					},
					{
						ScenarioName: "Modern browser should not be blocked",
						Passed:       false,
						Expected:     ExpectedOutcome{Result: "notApplicable"},
						Got: CombinedDecision{
							Result:           ResultGrant,
							GrantControls:    []string{"mfa"},
							MatchingPolicies: []string{"cap100-admin-mfa"},
						},
						MatchingPolicies: []string{"cap100-admin-mfa"},
					},
				},
			},
		},
	}
}

func TestRenderHuman_Format(t *testing.T) {
	var buf bytes.Buffer
	report := sampleReport()

	RenderHuman(&buf, report, false)
	output := buf.String()

	assert.Contains(t, output, "=== cactl test ===")
	assert.Contains(t, output, "tests/block-legacy-auth.yaml")
	assert.Contains(t, output, "PASS  Legacy client should be blocked")
	assert.Contains(t, output, "FAIL  Modern browser should not be blocked")
	assert.Contains(t, output, "expected: notApplicable")
	assert.Contains(t, output, "got:      grant [mfa]")
	assert.Contains(t, output, "matching policies: cap100-admin-mfa")
	assert.Contains(t, output, "Results: 1 passed, 1 failed, 0 errors")
}

func TestRenderHuman_WithColor(t *testing.T) {
	var buf bytes.Buffer
	report := sampleReport()

	RenderHuman(&buf, report, true)
	output := buf.String()

	assert.Contains(t, output, greenCode+"PASS"+resetCode)
	assert.Contains(t, output, redCode+"FAIL"+resetCode)
}

func TestRenderJSON_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	report := sampleReport()

	err := RenderJSON(&buf, report)
	require.NoError(t, err)

	var jr jsonReport
	err = json.Unmarshal(buf.Bytes(), &jr)
	require.NoError(t, err)

	assert.Equal(t, 2, jr.Summary.Total)
	assert.Equal(t, 1, jr.Summary.Passed)
	assert.Equal(t, 1, jr.Summary.Failed)
	assert.Equal(t, 0, jr.Summary.Errors)
	assert.Len(t, jr.Files, 1)
	assert.Len(t, jr.Files[0].Scenarios, 2)
}

func TestRenderJSON_ScenarioDetails(t *testing.T) {
	var buf bytes.Buffer
	report := sampleReport()

	err := RenderJSON(&buf, report)
	require.NoError(t, err)

	var jr jsonReport
	require.NoError(t, json.Unmarshal(buf.Bytes(), &jr))

	s := jr.Files[0].Scenarios[1]
	assert.Equal(t, "Modern browser should not be blocked", s.Name)
	assert.False(t, s.Passed)
	assert.Equal(t, "notApplicable", s.ExpectedResult)
	assert.Equal(t, "grant", s.GotResult)
	assert.Contains(t, s.GotControls, "mfa")
}

func TestSummary(t *testing.T) {
	report := &TestReport{
		Files: []FileResult{
			{Passed: 3, Failed: 1, Errors: 0},
			{Passed: 2, Failed: 0, Errors: 1},
		},
	}

	total, passed, failed, errors := Summary(report)
	assert.Equal(t, 7, total)
	assert.Equal(t, 5, passed)
	assert.Equal(t, 1, failed)
	assert.Equal(t, 1, errors)
}

func TestRenderHuman_ErrorScenario(t *testing.T) {
	var buf bytes.Buffer
	report := &TestReport{
		Files: []FileResult{
			{
				File:   "tests/broken.yaml",
				Errors: 1,
				Scenarios: []ScenarioResult{
					{
						ScenarioName: "file-load",
						Error:        "parsing error: invalid YAML",
					},
				},
			},
		},
	}

	RenderHuman(&buf, report, false)
	output := buf.String()

	assert.Contains(t, output, "FAIL")
	assert.Contains(t, output, "error: parsing error: invalid YAML")
	assert.Contains(t, output, "Results: 0 passed, 0 failed, 1 errors")
}
