package reconcile

import (
	"testing"
)

func TestComputeDiff(t *testing.T) {
	tests := []struct {
		name     string
		desired  map[string]interface{}
		actual   map[string]interface{}
		expected []FieldDiff
	}{
		{
			name:     "identical maps produce empty diff",
			desired:  map[string]interface{}{"displayName": "MFA Policy", "state": "enabled"},
			actual:   map[string]interface{}{"displayName": "MFA Policy", "state": "enabled"},
			expected: nil,
		},
		{
			name:    "added field detected",
			desired: map[string]interface{}{"displayName": "MFA Policy", "state": "enabled"},
			actual:  map[string]interface{}{"displayName": "MFA Policy"},
			expected: []FieldDiff{
				{Path: "state", Type: DiffAdded, OldValue: nil, NewValue: "enabled"},
			},
		},
		{
			name:    "removed field detected",
			desired: map[string]interface{}{"displayName": "MFA Policy"},
			actual:  map[string]interface{}{"displayName": "MFA Policy", "state": "enabled"},
			expected: []FieldDiff{
				{Path: "state", Type: DiffRemoved, OldValue: "enabled", NewValue: nil},
			},
		},
		{
			name:    "changed field detected",
			desired: map[string]interface{}{"state": "enabled"},
			actual:  map[string]interface{}{"state": "disabled"},
			expected: []FieldDiff{
				{Path: "state", Type: DiffChanged, OldValue: "disabled", NewValue: "enabled"},
			},
		},
		{
			name: "nested map diff with dot-separated path",
			desired: map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeGroups": []interface{}{"group-a", "group-b"},
					},
				},
			},
			actual: map[string]interface{}{
				"conditions": map[string]interface{}{
					"users": map[string]interface{}{
						"includeGroups": []interface{}{"group-a"},
					},
				},
			},
			expected: []FieldDiff{
				{
					Path:     "conditions.users.includeGroups",
					Type:     DiffChanged,
					OldValue: []interface{}{"group-a"},
					NewValue: []interface{}{"group-a", "group-b"},
				},
			},
		},
		{
			name:    "array value change",
			desired: map[string]interface{}{"tags": []interface{}{"prod", "critical"}},
			actual:  map[string]interface{}{"tags": []interface{}{"dev"}},
			expected: []FieldDiff{
				{Path: "tags", Type: DiffChanged, OldValue: []interface{}{"dev"}, NewValue: []interface{}{"prod", "critical"}},
			},
		},
		{
			name: "mixed: some same, some added, some changed",
			desired: map[string]interface{}{
				"displayName": "MFA Policy",
				"state":       "enabled",
				"priority":    float64(10),
			},
			actual: map[string]interface{}{
				"displayName": "MFA Policy",
				"state":       "disabled",
			},
			expected: []FieldDiff{
				{Path: "priority", Type: DiffAdded, OldValue: nil, NewValue: float64(10)},
				{Path: "state", Type: DiffChanged, OldValue: "disabled", NewValue: "enabled"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeDiff(tt.desired, tt.actual)

			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d diffs, got %d: %+v", len(tt.expected), len(got), got)
			}

			for i, exp := range tt.expected {
				if got[i].Path != exp.Path {
					t.Errorf("diff[%d] path: expected %q, got %q", i, exp.Path, got[i].Path)
				}
				if got[i].Type != exp.Type {
					t.Errorf("diff[%d] type: expected %v, got %v", i, exp.Type, got[i].Type)
				}
			}
		})
	}
}
