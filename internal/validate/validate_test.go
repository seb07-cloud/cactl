package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// helper to build a PolicyAction with the given backend JSON and action type.
func makeAction(slug string, action ActionType, backendJSON map[string]interface{}) PolicyAction {
	return PolicyAction{
		Slug:        slug,
		Action:      action,
		BackendJSON: backendJSON,
	}
}

func TestCheckBreakGlass(t *testing.T) {
	tests := []struct {
		name    string
		action  PolicyAction
		cfg     ValidationConfig
		wantLen int
		wantSev Severity
	}{
		{
			name: "break-glass accounts configured and excluded - no result",
			action: makeAction("ca-mfa", ActionUpdate, map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"excludeUsers": []interface{}{"bg-account-1", "bg-account-2"},
					},
				},
			}),
			cfg:     ValidationConfig{BreakGlassAccounts: []string{"bg-account-1", "bg-account-2"}},
			wantLen: 0,
		},
		{
			name: "break-glass accounts configured but one missing - warning",
			action: makeAction("ca-mfa", ActionUpdate, map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"excludeUsers": []interface{}{"bg-account-1"},
					},
				},
			}),
			cfg:     ValidationConfig{BreakGlassAccounts: []string{"bg-account-1", "bg-account-2"}},
			wantLen: 1,
			wantSev: SeverityWarning,
		},
		{
			name:    "no break-glass accounts configured - no result",
			action:  makeAction("ca-mfa", ActionUpdate, map[string]interface{}{}),
			cfg:     ValidationConfig{BreakGlassAccounts: []string{}},
			wantLen: 0,
		},
		{
			name: "policy has no excludeUsers field - warning for each account",
			action: makeAction("ca-mfa", ActionUpdate, map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{},
				},
			}),
			cfg:     ValidationConfig{BreakGlassAccounts: []string{"bg-account-1", "bg-account-2"}},
			wantLen: 2,
			wantSev: SeverityWarning,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := checkBreakGlass(tc.action, tc.cfg)
			assert.Len(t, results, tc.wantLen)
			if tc.wantLen > 0 {
				assert.Equal(t, tc.wantSev, results[0].Severity)
			}
		})
	}
}

func TestCheckConflictingConditions(t *testing.T) {
	tests := []struct {
		name    string
		action  PolicyAction
		wantLen int
		wantSev Severity
	}{
		{
			name: "same GUID in includeUsers and excludeUsers - error",
			action: makeAction("ca-mfa", ActionUpdate, map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers": []interface{}{"user-1", "user-2"},
						"excludeUsers": []interface{}{"user-2", "user-3"},
					},
				},
			}),
			wantLen: 1,
			wantSev: SeverityError,
		},
		{
			name: "same GUID in includeGroups and excludeGroups - error",
			action: makeAction("ca-mfa", ActionUpdate, map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeGroups": []interface{}{"group-1"},
						"excludeGroups": []interface{}{"group-1"},
					},
				},
			}),
			wantLen: 1,
			wantSev: SeverityError,
		},
		{
			name: "no overlap - no result",
			action: makeAction("ca-mfa", ActionUpdate, map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers": []interface{}{"user-1"},
						"excludeUsers": []interface{}{"user-2"},
					},
				},
			}),
			wantLen: 0,
		},
		{
			name: "same app in includeApplications and excludeApplications - error",
			action: makeAction("ca-mfa", ActionUpdate, map[string]interface{}{
				"conditions": map[string]interface{}{
					"applications": map[string]interface{}{
						"includeApplications": []interface{}{"app-1"},
						"excludeApplications": []interface{}{"app-1"},
					},
				},
			}),
			wantLen: 1,
			wantSev: SeverityError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := checkConflictingConditions(tc.action)
			assert.Len(t, results, tc.wantLen)
			if tc.wantLen > 0 {
				assert.Equal(t, tc.wantSev, results[0].Severity)
			}
		})
	}
}

func TestCheckEmptyIncludes(t *testing.T) {
	tests := []struct {
		name    string
		action  PolicyAction
		wantLen int
	}{
		{
			name: "all include lists empty - warning",
			action: makeAction("ca-mfa", ActionUpdate, map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers":  []interface{}{},
						"includeGroups": []interface{}{},
						"includeRoles":  []interface{}{},
					},
				},
			}),
			wantLen: 1,
		},
		{
			name: "includeUsers has All - no result",
			action: makeAction("ca-mfa", ActionUpdate, map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers": []interface{}{"All"},
					},
				},
			}),
			wantLen: 0,
		},
		{
			name: "includeGroups has one entry - no result",
			action: makeAction("ca-mfa", ActionUpdate, map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeGroups": []interface{}{"group-1"},
					},
				},
			}),
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := checkEmptyIncludes(tc.action)
			assert.Len(t, results, tc.wantLen)
			if tc.wantLen > 0 {
				assert.Equal(t, SeverityWarning, results[0].Severity)
			}
		})
	}
}

func TestCheckOverlyBroad(t *testing.T) {
	tests := []struct {
		name    string
		action  PolicyAction
		wantLen int
	}{
		{
			name: "includeUsers All with no exclusions and enabled - warning",
			action: makeAction("ca-mfa", ActionUpdate, map[string]interface{}{
				"state": "enabled",
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers":  []interface{}{"All"},
						"excludeUsers":  []interface{}{},
						"excludeGroups": []interface{}{},
					},
				},
			}),
			wantLen: 1,
		},
		{
			name: "includeUsers All but has excludeGroups - no result",
			action: makeAction("ca-mfa", ActionUpdate, map[string]interface{}{
				"state": "enabled",
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers":  []interface{}{"All"},
						"excludeGroups": []interface{}{"group-1"},
					},
				},
			}),
			wantLen: 0,
		},
		{
			name: "state disabled even with All and no exclusions - no result",
			action: makeAction("ca-mfa", ActionUpdate, map[string]interface{}{
				"state": "disabled",
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers":  []interface{}{"All"},
						"excludeUsers":  []interface{}{},
						"excludeGroups": []interface{}{},
					},
				},
			}),
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := checkOverlyBroad(tc.action)
			assert.Len(t, results, tc.wantLen)
			if tc.wantLen > 0 {
				assert.Equal(t, SeverityWarning, results[0].Severity)
			}
		})
	}
}

func TestValidatePlan(t *testing.T) {
	t.Run("combined results from multiple rules", func(t *testing.T) {
		actions := []PolicyAction{
			// Policy with conflicting conditions and missing break-glass
			makeAction("ca-conflict", ActionUpdate, map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers": []interface{}{"user-1"},
						"excludeUsers": []interface{}{"user-1"},
					},
				},
			}),
		}
		cfg := ValidationConfig{
			BreakGlassAccounts: []string{"bg-account-1"},
		}

		results := ValidatePlan(actions, cfg)
		// Should have results from both checkConflictingConditions and checkBreakGlass
		assert.GreaterOrEqual(t, len(results), 2)
	})

	t.Run("noop and untracked actions are skipped", func(t *testing.T) {
		actions := []PolicyAction{
			{Slug: "ca-noop", Action: ActionNoop, BackendJSON: map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers": []interface{}{"user-1"},
						"excludeUsers": []interface{}{"user-1"},
					},
				},
			}},
			{Slug: "ca-untracked", Action: ActionUntracked, BackendJSON: map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers": []interface{}{"user-1"},
						"excludeUsers": []interface{}{"user-1"},
					},
				},
			}},
		}
		cfg := ValidationConfig{}

		results := ValidatePlan(actions, cfg)
		assert.Empty(t, results)
	})
}
