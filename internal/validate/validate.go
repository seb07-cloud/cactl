package validate

import (
	"fmt"
)

// Severity represents the severity level of a validation result.
type Severity int

const (
	SeverityWarning Severity = iota
	SeverityError
)

// String returns the human-readable name of the severity.
func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "WARNING"
	case SeverityError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ActionType mirrors reconcile.ActionType to avoid circular dependencies.
// The reconcile package defines the canonical type.
type ActionType int

const (
	ActionNoop      ActionType = iota
	ActionCreate
	ActionUpdate
	ActionRecreate
	ActionUntracked
)

// PolicyAction is a local mirror of reconcile.PolicyAction containing the
// fields needed for validation. The reconcile package defines the canonical type.
type PolicyAction struct {
	Slug        string
	Action      ActionType
	BackendJSON map[string]interface{}
}

// ValidationResult represents a single validation finding.
type ValidationResult struct {
	Rule     string
	Severity Severity
	Message  string
	Policy   string
}

// ValidationConfig holds configuration for plan-time validations.
type ValidationConfig struct {
	BreakGlassAccounts []string
	SchemaPath         string // For VALID-02; schema validation deferred
}

// ValidatePlan runs all validation rules against the given actions and returns
// aggregated results. Noop and Untracked actions are skipped.
func ValidatePlan(actions []PolicyAction, cfg ValidationConfig) []ValidationResult {
	var results []ValidationResult

	for _, a := range actions {
		if a.Action == ActionNoop || a.Action == ActionUntracked {
			continue
		}

		results = append(results, checkBreakGlass(a, cfg)...)
		results = append(results, checkConflictingConditions(a)...)
		results = append(results, checkEmptyIncludes(a)...)
		results = append(results, checkOverlyBroad(a)...)
		// TODO (VALID-02): Add checkSchema when schema.json loading is implemented
	}

	return results
}

// checkBreakGlass verifies that break-glass accounts are excluded from the policy (VALID-01).
func checkBreakGlass(action PolicyAction, cfg ValidationConfig) []ValidationResult {
	if len(cfg.BreakGlassAccounts) == 0 {
		return nil
	}

	excludeUsers := getStringSlice(action.BackendJSON, "conditions.users.excludeUsers")
	excludeSet := toSet(excludeUsers)

	var results []ValidationResult
	for _, account := range cfg.BreakGlassAccounts {
		if !excludeSet[account] {
			results = append(results, ValidationResult{
				Rule:     "break-glass",
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("break-glass account %q not excluded from policy", account),
				Policy:   action.Slug,
			})
		}
	}

	return results
}

// checkConflictingConditions detects when the same GUID appears in both include
// and exclude lists for users, groups, or applications (VALID-03).
func checkConflictingConditions(action PolicyAction) []ValidationResult {
	var results []ValidationResult

	pairs := []struct {
		includePath string
		excludePath string
		label       string
	}{
		{"conditions.users.includeUsers", "conditions.users.excludeUsers", "users"},
		{"conditions.users.includeGroups", "conditions.users.excludeGroups", "groups"},
		{"conditions.applications.includeApplications", "conditions.applications.excludeApplications", "applications"},
	}

	for _, pair := range pairs {
		includes := getStringSlice(action.BackendJSON, pair.includePath)
		excludes := getStringSlice(action.BackendJSON, pair.excludePath)

		includeSet := toSet(includes)
		for _, ex := range excludes {
			if includeSet[ex] {
				results = append(results, ValidationResult{
					Rule:     "conflicting-conditions",
					Severity: SeverityError,
					Message:  fmt.Sprintf("%s: %q appears in both include and exclude lists", pair.label, ex),
					Policy:   action.Slug,
				})
			}
		}
	}

	return results
}

// checkEmptyIncludes warns when all user include lists are empty (VALID-04).
// A policy with no included users/groups/roles effectively targets nobody.
func checkEmptyIncludes(action PolicyAction) []ValidationResult {
	includeUsers := getStringSlice(action.BackendJSON, "conditions.users.includeUsers")
	includeGroups := getStringSlice(action.BackendJSON, "conditions.users.includeGroups")
	includeRoles := getStringSlice(action.BackendJSON, "conditions.users.includeRoles")

	if len(includeUsers) == 0 && len(includeGroups) == 0 && len(includeRoles) == 0 {
		// Only warn if conditions.users exists (policy has user conditions)
		users := getNestedValue(action.BackendJSON, "conditions.users")
		if users == nil {
			return nil
		}
		return []ValidationResult{
			{
				Rule:     "empty-includes",
				Severity: SeverityWarning,
				Message:  "all user include lists are empty; policy targets nobody",
				Policy:   action.Slug,
			},
		}
	}

	return nil
}

// checkOverlyBroad warns when a policy includes "All" users with no exclusions
// and is enabled (VALID-05).
func checkOverlyBroad(action PolicyAction) []ValidationResult {
	// Check state - disabled policies are safe
	state, _ := getNestedValue(action.BackendJSON, "state").(string)
	if state == "disabled" || state == "enabledForReportingButNotEnforced" {
		return nil
	}

	includeUsers := getStringSlice(action.BackendJSON, "conditions.users.includeUsers")
	hasAll := false
	for _, u := range includeUsers {
		if u == "All" {
			hasAll = true
			break
		}
	}
	if !hasAll {
		return nil
	}

	// Check for exclusions
	excludeUsers := getStringSlice(action.BackendJSON, "conditions.users.excludeUsers")
	excludeGroups := getStringSlice(action.BackendJSON, "conditions.users.excludeGroups")

	if len(excludeUsers) > 0 || len(excludeGroups) > 0 {
		return nil
	}

	return []ValidationResult{
		{
			Rule:     "overly-broad",
			Severity: SeverityWarning,
			Message:  "policy includes all users with no exclusions and is enabled",
			Policy:   action.Slug,
		},
	}
}

// getNestedValue walks a dot-separated path in a nested map and returns the value.
func getNestedValue(data map[string]interface{}, dotPath string) interface{} {
	parts := splitPath(dotPath)
	var current interface{} = data

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}

	return current
}

// getStringSlice gets a nested value and converts []interface{} to []string.
func getStringSlice(data map[string]interface{}, dotPath string) []string {
	val := getNestedValue(data, dotPath)
	if val == nil {
		return nil
	}

	slice, ok := val.([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}

	return result
}

// toSet converts a string slice to a set (map[string]bool).
func toSet(slice []string) map[string]bool {
	set := make(map[string]bool, len(slice))
	for _, s := range slice {
		set[s] = true
	}
	return set
}

// splitPath splits a dot-separated path into parts.
func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			parts = append(parts, path[start:i])
			start = i + 1
		}
	}
	parts = append(parts, path[start:])
	return parts
}
