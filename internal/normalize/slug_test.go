package normalize

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "policy name with colon and spaces",
			input:    "CA001: Require MFA for admins",
			expected: "ca001-require-mfa-for-admins",
		},
		{
			name:     "title case words",
			input:    "Block Legacy Authentication",
			expected: "block-legacy-authentication",
		},
		{
			name:     "spaces between alphanumeric groups",
			input:    "CA 001",
			expected: "ca-001",
		},
		{
			name:     "leading and trailing spaces",
			input:    "  Leading Spaces  ",
			expected: "leading-spaces",
		},
		{
			name:     "consecutive special characters collapse to single dash",
			input:    "ALL---CAPS---POLICY",
			expected: "all-caps-policy",
		},
		{
			name:     "underscores replaced with dashes",
			input:    "policy_with_underscores",
			expected: "policy-with-underscores",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "already kebab-case",
			input:    "simple",
			expected: "simple",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
