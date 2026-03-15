package testengine

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// validResults is the set of valid expected result values.
var validResults = map[string]bool{
	"block":         true,
	"grant":         true,
	"notApplicable": true,
}

// ParseTestFile reads a YAML test spec file from disk and returns a validated TestSpec.
func ParseTestFile(path string) (*TestSpec, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304 - path from config/traversal
	if err != nil {
		return nil, fmt.Errorf("reading test file %s: %w", path, err)
	}
	return ParseTestBytes(data)
}

// ParseTestBytes parses YAML bytes into a validated TestSpec.
func ParseTestBytes(data []byte) (*TestSpec, error) {
	var spec TestSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	if err := validateSpec(&spec); err != nil {
		return nil, err
	}

	return &spec, nil
}

// validateSpec checks that required fields are present and valid.
func validateSpec(spec *TestSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("test spec missing required field: name")
	}

	if len(spec.Scenarios) == 0 {
		return fmt.Errorf("test spec %q has no scenarios", spec.Name)
	}

	for i, s := range spec.Scenarios {
		if s.Name == "" {
			return fmt.Errorf("scenario %d in %q missing required field: name", i+1, spec.Name)
		}

		if s.Expect.Result == "" {
			return fmt.Errorf("scenario %q in %q missing required field: expect.result", s.Name, spec.Name)
		}

		if !validResults[s.Expect.Result] {
			return fmt.Errorf("scenario %q in %q has invalid expect.result %q; must be one of: block, grant, notApplicable", s.Name, spec.Name, s.Expect.Result)
		}
	}

	return nil
}
