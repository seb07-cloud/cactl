package testengine

import (
	"testing"
)

func TestEvaluatePolicy(t *testing.T) {
	tests := []struct {
		name       string
		policy     map[string]interface{}
		ctx        *SignInContext
		wantResult EvalResult
		wantCtrls  []string
		wantOp     string
	}{
		{
			name: "disabled policy returns notApplicable",
			policy: map[string]interface{}{
				"state": "disabled",
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers": []interface{}{"All"},
					},
					"applications": map[string]interface{}{
						"includeApplications": []interface{}{"All"},
					},
				},
				"grantControls": map[string]interface{}{
					"builtInControls": []interface{}{"block"},
					"operator":        "OR",
				},
			},
			ctx:        &SignInContext{User: "user-1", Application: "app-1"},
			wantResult: ResultNotApplicable,
		},
		{
			name: "enabled block policy matching context returns block",
			policy: map[string]interface{}{
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
					"builtInControls": []interface{}{"block"},
					"operator":        "OR",
				},
			},
			ctx:        &SignInContext{User: "user-1", Application: "app-1"},
			wantResult: ResultBlock,
			wantCtrls:  []string{"block"},
		},
		{
			name: "enabled grant policy matching context returns grant with controls",
			policy: map[string]interface{}{
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
					"operator":        "AND",
				},
			},
			ctx:        &SignInContext{User: "user-1", Application: "app-1"},
			wantResult: ResultGrant,
			wantCtrls:  []string{"mfa"},
			wantOp:     "AND",
		},
		{
			name: "policy with non-matching user returns notApplicable",
			policy: map[string]interface{}{
				"state": "enabled",
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers": []interface{}{"user-2"},
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
			ctx:        &SignInContext{User: "user-1", Application: "app-1"},
			wantResult: ResultNotApplicable,
		},
		{
			name: "report-only policy evaluated as if enabled",
			policy: map[string]interface{}{
				"state": "enabledForReportingButNotEnforced",
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
			ctx:        &SignInContext{User: "user-1", Application: "app-1"},
			wantResult: ResultGrant,
			wantCtrls:  []string{"mfa"},
			wantOp:     "OR",
		},
		{
			name: "policy with no platform condition matches any platform",
			policy: map[string]interface{}{
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
			ctx:        &SignInContext{User: "user-1", Application: "app-1", Platform: "iOS"},
			wantResult: ResultGrant,
			wantCtrls:  []string{"mfa"},
			wantOp:     "OR",
		},
		{
			name: "policy with session controls",
			policy: map[string]interface{}{
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
				"sessionControls": map[string]interface{}{
					"signInFrequency": map[string]interface{}{
						"value":     float64(4),
						"type":      "hours",
						"isEnabled": true,
					},
				},
			},
			ctx:        &SignInContext{User: "user-1", Application: "app-1"},
			wantResult: ResultGrant,
			wantCtrls:  []string{"mfa"},
			wantOp:     "OR",
		},
		{
			name: "policy with non-matching client app type returns notApplicable",
			policy: map[string]interface{}{
				"state": "enabled",
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers": []interface{}{"All"},
					},
					"applications": map[string]interface{}{
						"includeApplications": []interface{}{"All"},
					},
					"clientAppTypes": []interface{}{"browser"},
				},
				"grantControls": map[string]interface{}{
					"builtInControls": []interface{}{"mfa"},
					"operator":        "OR",
				},
			},
			ctx:        &SignInContext{User: "user-1", Application: "app-1", ClientAppType: "mobileAppsAndDesktopClients"},
			wantResult: ResultNotApplicable,
		},
		{
			name: "policy with no grant controls returns grant with empty controls",
			policy: map[string]interface{}{
				"state": "enabled",
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeUsers": []interface{}{"All"},
					},
					"applications": map[string]interface{}{
						"includeApplications": []interface{}{"All"},
					},
				},
			},
			ctx:        &SignInContext{User: "user-1", Application: "app-1"},
			wantResult: ResultGrant,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := EvaluatePolicy("test-policy", tt.policy, tt.ctx)

			if decision.Result != tt.wantResult {
				t.Errorf("Result = %v, want %v", decision.Result, tt.wantResult)
			}

			if len(tt.wantCtrls) > 0 {
				if len(decision.GrantControls) != len(tt.wantCtrls) {
					t.Errorf("GrantControls = %v, want %v", decision.GrantControls, tt.wantCtrls)
				} else {
					for i, c := range tt.wantCtrls {
						if decision.GrantControls[i] != c {
							t.Errorf("GrantControls[%d] = %v, want %v", i, decision.GrantControls[i], c)
						}
					}
				}
			}

			if tt.wantOp != "" && decision.Operator != tt.wantOp {
				t.Errorf("Operator = %v, want %v", decision.Operator, tt.wantOp)
			}
		})
	}
}

func TestEvaluatePolicy_SessionControls(t *testing.T) {
	policy := map[string]interface{}{
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
		"sessionControls": map[string]interface{}{
			"signInFrequency": map[string]interface{}{
				"value":     float64(4),
				"type":      "hours",
				"isEnabled": true,
			},
		},
	}

	ctx := &SignInContext{User: "user-1", Application: "app-1"}
	decision := EvaluatePolicy("test-policy", policy, ctx)

	if decision.SessionControls == nil {
		t.Fatal("SessionControls should not be nil")
	}
	freq, ok := decision.SessionControls["signInFrequency"]
	if !ok {
		t.Fatal("Expected signInFrequency in session controls")
	}
	freqMap, ok := freq.(map[string]interface{})
	if !ok {
		t.Fatal("signInFrequency should be a map")
	}
	if freqMap["type"] != "hours" {
		t.Errorf("signInFrequency.type = %v, want hours", freqMap["type"])
	}
}

func TestEvaluateAll(t *testing.T) {
	tests := []struct {
		name           string
		policies       []PolicyWithSlug
		ctx            *SignInContext
		wantResult     EvalResult
		wantCtrls      []string
		wantMatchCount int
	}{
		{
			name: "block wins over grant",
			policies: []PolicyWithSlug{
				{
					Slug: "grant-mfa",
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
				},
				{
					Slug: "block-all",
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
							"builtInControls": []interface{}{"block"},
							"operator":        "OR",
						},
					},
				},
			},
			ctx:            &SignInContext{User: "user-1", Application: "app-1"},
			wantResult:     ResultBlock,
			wantMatchCount: 2,
		},
		{
			name: "two grant policies combine controls",
			policies: []PolicyWithSlug{
				{
					Slug: "require-mfa",
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
				},
				{
					Slug: "require-compliant",
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
							"builtInControls": []interface{}{"compliantDevice"},
							"operator":        "AND",
						},
					},
				},
			},
			ctx:            &SignInContext{User: "user-1", Application: "app-1"},
			wantResult:     ResultGrant,
			wantCtrls:      []string{"mfa", "compliantDevice"},
			wantMatchCount: 2,
		},
		{
			name: "no matching policies returns notApplicable",
			policies: []PolicyWithSlug{
				{
					Slug: "disabled-policy",
					Data: map[string]interface{}{
						"state": "disabled",
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
				},
			},
			ctx:            &SignInContext{User: "user-1", Application: "app-1"},
			wantResult:     ResultNotApplicable,
			wantMatchCount: 0,
		},
		{
			name:           "empty policies returns notApplicable",
			policies:       []PolicyWithSlug{},
			ctx:            &SignInContext{User: "user-1", Application: "app-1"},
			wantResult:     ResultNotApplicable,
			wantMatchCount: 0,
		},
		{
			name: "report-only policy evaluated in combined result",
			policies: []PolicyWithSlug{
				{
					Slug: "report-only-mfa",
					Data: map[string]interface{}{
						"state": "enabledForReportingButNotEnforced",
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
				},
			},
			ctx:            &SignInContext{User: "user-1", Application: "app-1"},
			wantResult:     ResultGrant,
			wantCtrls:      []string{"mfa"},
			wantMatchCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			combined := EvaluateAll(tt.policies, tt.ctx)

			if combined.Result != tt.wantResult {
				t.Errorf("Result = %v, want %v", combined.Result, tt.wantResult)
			}

			if len(tt.wantCtrls) > 0 {
				if len(combined.GrantControls) != len(tt.wantCtrls) {
					t.Errorf("GrantControls = %v, want %v", combined.GrantControls, tt.wantCtrls)
				} else {
					for i, c := range tt.wantCtrls {
						if combined.GrantControls[i] != c {
							t.Errorf("GrantControls[%d] = %v, want %v", i, combined.GrantControls[i], c)
						}
					}
				}
			}

			if len(combined.MatchingPolicies) != tt.wantMatchCount {
				t.Errorf("MatchingPolicies count = %d, want %d (policies: %v)",
					len(combined.MatchingPolicies), tt.wantMatchCount, combined.MatchingPolicies)
			}
		})
	}
}
