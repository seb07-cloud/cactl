package testengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTestBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
		check   func(t *testing.T, spec *TestSpec)
	}{
		{
			name: "valid full spec",
			input: `
name: "Block legacy auth"
description: "Verify CAP001 blocks legacy auth"
policies:
  - "cap001-block-legacy-auth"
scenarios:
  - name: "Legacy client blocked"
    context:
      user: "any"
      groups:
        - "group-1"
      roles:
        - "role-1"
      application: "All"
      clientAppType: "exchangeActiveSync"
      platform: "windows"
      location: "trusted"
      signInRiskLevel: "none"
      userRiskLevel: "low"
    expect:
      result: "block"
  - name: "Modern browser allowed"
    context:
      user: "any"
      clientAppType: "browser"
    expect:
      result: "notApplicable"
`,
			check: func(t *testing.T, spec *TestSpec) {
				assert.Equal(t, "Block legacy auth", spec.Name)
				assert.Equal(t, "Verify CAP001 blocks legacy auth", spec.Description)
				assert.Equal(t, []string{"cap001-block-legacy-auth"}, spec.Policies)
				assert.Len(t, spec.Scenarios, 2)

				s0 := spec.Scenarios[0]
				assert.Equal(t, "Legacy client blocked", s0.Name)
				assert.Equal(t, "any", s0.Context.User)
				assert.Equal(t, []string{"group-1"}, s0.Context.Groups)
				assert.Equal(t, []string{"role-1"}, s0.Context.Roles)
				assert.Equal(t, "All", s0.Context.Application)
				assert.Equal(t, "exchangeActiveSync", s0.Context.ClientAppType)
				assert.Equal(t, "windows", s0.Context.Platform)
				assert.Equal(t, "trusted", s0.Context.Location)
				assert.Equal(t, "none", s0.Context.SignInRiskLevel)
				assert.Equal(t, "low", s0.Context.UserRiskLevel)
				assert.Equal(t, "block", s0.Expect.Result)

				s1 := spec.Scenarios[1]
				assert.Equal(t, "notApplicable", s1.Expect.Result)
			},
		},
		{
			name: "minimal valid spec",
			input: `
name: "Minimal"
scenarios:
  - name: "Basic scenario"
    expect:
      result: "grant"
`,
			check: func(t *testing.T, spec *TestSpec) {
				assert.Equal(t, "Minimal", spec.Name)
				assert.Empty(t, spec.Description)
				assert.Empty(t, spec.Policies)
				assert.Len(t, spec.Scenarios, 1)
				assert.Equal(t, "grant", spec.Scenarios[0].Expect.Result)
			},
		},
		{
			name: "grant with controls",
			input: `
name: "MFA required"
scenarios:
  - name: "User needs MFA"
    expect:
      result: "grant"
      controls:
        - "mfa"
      sessionControls:
        signInFrequency:
          value: 9
          type: "hours"
`,
			check: func(t *testing.T, spec *TestSpec) {
				s := spec.Scenarios[0]
				assert.Equal(t, "grant", s.Expect.Result)
				assert.Equal(t, []string{"mfa"}, s.Expect.Controls)
				assert.NotNil(t, s.Expect.SessionControls)
				sif := s.Expect.SessionControls["signInFrequency"]
				assert.NotNil(t, sif)
			},
		},
		{
			name:    "missing name",
			input:   `scenarios: [{name: "s1", expect: {result: "block"}}]`,
			wantErr: "missing required field: name",
		},
		{
			name:    "no scenarios",
			input:   `name: "Empty"`,
			wantErr: "has no scenarios",
		},
		{
			name: "scenario missing name",
			input: `
name: "Test"
scenarios:
  - expect:
      result: "block"
`,
			wantErr: "missing required field: name",
		},
		{
			name: "scenario missing expect result",
			input: `
name: "Test"
scenarios:
  - name: "No result"
    expect: {}
`,
			wantErr: "missing required field: expect.result",
		},
		{
			name: "invalid result value",
			input: `
name: "Test"
scenarios:
  - name: "Bad result"
    expect:
      result: "deny"
`,
			wantErr: `invalid expect.result "deny"`,
		},
		{
			name:    "invalid YAML",
			input:   `{{{invalid`,
			wantErr: "parsing YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := ParseTestBytes([]byte(tt.input))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, spec)
			}
		})
	}
}
