package testengine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadPolicies reads all .json files from policyDir and returns them as PolicyWithSlug.
func LoadPolicies(policyDir string) ([]PolicyWithSlug, error) {
	entries, err := os.ReadDir(policyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("policy directory not found: %s", policyDir)
		}
		return nil, fmt.Errorf("reading policy directory %s: %w", policyDir, err)
	}

	var policies []PolicyWithSlug
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(policyDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading policy file %s: %w", entry.Name(), err)
		}

		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parsing policy file %s: %w", entry.Name(), err)
		}

		slug := strings.TrimSuffix(entry.Name(), ".json")
		policies = append(policies, PolicyWithSlug{Slug: slug, Data: m})
	}

	if len(policies) == 0 {
		return nil, fmt.Errorf("no policy files found in %s", policyDir)
	}

	return policies, nil
}

// RunTests loads policies from policyDir and runs all test files, returning an aggregated report.
func RunTests(testPaths []string, policyDir string) (*TestReport, error) {
	allPolicies, err := LoadPolicies(policyDir)
	if err != nil {
		return nil, fmt.Errorf("loading policies: %w", err)
	}

	report := &TestReport{}
	for _, path := range testPaths {
		result, err := RunTestFile(path, allPolicies)
		if err != nil {
			// Record file-level error as a single error scenario
			report.Files = append(report.Files, FileResult{
				File:   path,
				Errors: 1,
				Scenarios: []ScenarioResult{
					{ScenarioName: "file-load", Error: err.Error()},
				},
			})
			continue
		}
		report.Files = append(report.Files, *result)
	}

	return report, nil
}

// RunTestFile parses a test file and evaluates each scenario against the given policies.
func RunTestFile(path string, allPolicies []PolicyWithSlug) (*FileResult, error) {
	spec, err := ParseTestFile(path)
	if err != nil {
		return nil, fmt.Errorf("parsing test file: %w", err)
	}

	// Filter policies if spec.Policies is non-empty
	policies := filterPolicies(allPolicies, spec.Policies)

	result := &FileResult{File: path}
	for _, scenario := range spec.Scenarios {
		sr := evaluateScenario(scenario, policies)
		result.Scenarios = append(result.Scenarios, sr)
		if sr.Error != "" {
			result.Errors++
		} else if sr.Passed {
			result.Passed++
		} else {
			result.Failed++
		}
	}

	return result, nil
}

// filterPolicies returns only policies whose slug matches any of the filter prefixes.
// If filters is empty, all policies are returned.
func filterPolicies(all []PolicyWithSlug, filters []string) []PolicyWithSlug {
	if len(filters) == 0 {
		return all
	}

	var filtered []PolicyWithSlug
	for _, p := range all {
		for _, f := range filters {
			if p.Slug == f || strings.HasPrefix(p.Slug, f) {
				filtered = append(filtered, p)
				break
			}
		}
	}
	return filtered
}

// evaluateScenario runs a single scenario against policies and compares the result.
func evaluateScenario(scenario Scenario, policies []PolicyWithSlug) ScenarioResult {
	sr := ScenarioResult{
		ScenarioName: scenario.Name,
		Expected:     scenario.Expect,
	}

	ctx := &scenario.Context
	combined := EvaluateAll(policies, ctx)
	sr.Got = combined
	sr.MatchingPolicies = combined.MatchingPolicies

	// Compare result
	gotResult := combined.Result.String()
	if gotResult != scenario.Expect.Result {
		sr.Passed = false
		return sr
	}

	// Compare grant controls if specified
	if len(scenario.Expect.Controls) > 0 {
		if !containsAllControls(combined.GrantControls, scenario.Expect.Controls) {
			sr.Passed = false
			return sr
		}
	}

	// TODO: SessionControls comparison deferred

	sr.Passed = true
	return sr
}

// containsAllControls checks that all expected controls are present in got.
func containsAllControls(got, expected []string) bool {
	gotSet := make(map[string]bool, len(got))
	for _, c := range got {
		gotSet[c] = true
	}
	for _, c := range expected {
		if !gotSet[c] {
			return false
		}
	}
	return true
}
