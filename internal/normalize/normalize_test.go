package normalize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name: "strip server-managed fields",
			input: `{
				"id": "2b31ac51-b855-40a5-a986-0a4ed23e9008",
				"createdDateTime": "2021-11-02T14:17:09Z",
				"modifiedDateTime": "2024-01-03T20:07:59Z",
				"templateId": "some-template",
				"displayName": "Test Policy",
				"state": "enabled"
			}`,
			expected: `{
  "displayName": "Test Policy",
  "state": "enabled"
}
`,
		},
		{
			name: "strip @odata fields at top level and nested",
			input: `{
				"@odata.context": "https://graph.microsoft.com/v1.0/$metadata",
				"displayName": "Test",
				"grantControls": {
					"operator": "OR",
					"authenticationStrength@odata.context": "https://graph.microsoft.com/..."
				}
			}`,
			expected: `{
  "displayName": "Test",
  "grantControls": {
    "operator": "OR"
  }
}
`,
		},
		{
			name: "remove null values recursively",
			input: `{
				"displayName": "Test",
				"sessionControls": null,
				"conditions": {
					"platforms": null,
					"clientAppTypes": ["all"]
				}
			}`,
			expected: `{
  "conditions": {
    "clientAppTypes": [
      "all"
    ]
  },
  "displayName": "Test"
}
`,
		},
		{
			name: "preserve empty arrays",
			input: `{
				"displayName": "Test",
				"conditions": {
					"users": {
						"excludeUsers": [],
						"includeUsers": []
					}
				}
			}`,
			expected: `{
  "conditions": {
    "users": {
      "excludeUsers": [],
      "includeUsers": []
    }
  },
  "displayName": "Test"
}
`,
		},
		{
			name: "sort keys alphabetically at all nesting levels",
			input: `{
				"state": "enabled",
				"displayName": "Test",
				"conditions": {
					"users": {
						"includeUsers": [],
						"excludeUsers": []
					},
					"clientAppTypes": ["all"]
				}
			}`,
			expected: `{
  "conditions": {
    "clientAppTypes": [
      "all"
    ],
    "users": {
      "excludeUsers": [],
      "includeUsers": []
    }
  },
  "displayName": "Test",
  "state": "enabled"
}
`,
		},
		{
			name:  "pretty-print with 2-space indent and trailing newline",
			input: `{"displayName":"Test","state":"enabled"}`,
			expected: `{
  "displayName": "Test",
  "state": "enabled"
}
`,
		},
		{
			name: "full pipeline - Graph API response",
			input: `{
				"id": "2b31ac51-b855-40a5-a986-0a4ed23e9008",
				"templateId": null,
				"displayName": "CA001: Require MFA for admins",
				"createdDateTime": "2021-11-02T14:17:09Z",
				"modifiedDateTime": "2024-01-03T20:07:59Z",
				"state": "enabled",
				"sessionControls": null,
				"conditions": {
					"platforms": null,
					"locations": null,
					"clientAppTypes": ["all"],
					"users": {
						"includeUsers": [],
						"excludeUsers": [],
						"includeGuestsOrExternalUsers": null,
						"excludeGuestsOrExternalUsers": null
					}
				},
				"grantControls": {
					"operator": "OR",
					"builtInControls": ["mfa"],
					"authenticationStrength@odata.context": "https://graph.microsoft.com/...",
					"authenticationStrength": null
				}
			}`,
			expected: `{
  "conditions": {
    "clientAppTypes": [
      "all"
    ],
    "users": {
      "excludeUsers": [],
      "includeUsers": []
    }
  },
  "displayName": "CA001: Require MFA for admins",
  "grantControls": {
    "builtInControls": [
      "mfa"
    ],
    "operator": "OR"
  },
  "state": "enabled"
}
`,
		},
		{
			name:    "invalid JSON returns error",
			input:   `{not valid json}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize([]byte(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(got))
		})
	}
}
