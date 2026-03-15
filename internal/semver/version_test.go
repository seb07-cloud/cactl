package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetermineBump(t *testing.T) {
	cfg := DefaultSemverConfig()

	tests := []struct {
		name     string
		diffs    []FieldDiff
		expected BumpLevel
	}{
		{
			name: "field in major_fields list returns BumpMajor",
			diffs: []FieldDiff{
				{Path: "conditions.users.includeUsers"},
			},
			expected: BumpMajor,
		},
		{
			name: "field matching major_fields exactly returns BumpMajor",
			diffs: []FieldDiff{
				{Path: "state"},
			},
			expected: BumpMajor,
		},
		{
			name: "field matching minor_fields prefix returns BumpMinor",
			diffs: []FieldDiff{
				{Path: "grantControls.builtInControls"},
			},
			expected: BumpMinor,
		},
		{
			name: "field not in major or minor returns BumpPatch",
			diffs: []FieldDiff{
				{Path: "displayName"},
			},
			expected: BumpPatch,
		},
		{
			name: "mixed major and patch returns BumpMajor (highest wins)",
			diffs: []FieldDiff{
				{Path: "displayName"},
				{Path: "conditions.users.includeUsers"},
			},
			expected: BumpMajor,
		},
		{
			name: "mixed minor and patch returns BumpMinor",
			diffs: []FieldDiff{
				{Path: "displayName"},
				{Path: "grantControls.builtInControls"},
			},
			expected: BumpMinor,
		},
		{
			name:     "empty diffs returns BumpPatch",
			diffs:    []FieldDiff{},
			expected: BumpPatch,
		},
		{
			name: "prefix matching conditions trigger matches nested path",
			diffs: []FieldDiff{
				{Path: "conditions.users.includeGroups"},
			},
			expected: BumpMajor,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DetermineBump(tc.diffs, cfg.MajorFields, cfg.MinorFields)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBumpLevel_String(t *testing.T) {
	tests := []struct {
		level BumpLevel
		want  string
	}{
		{BumpPatch, "PATCH"},
		{BumpMinor, "MINOR"},
		{BumpMajor, "MAJOR"},
		{BumpLevel(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.level.String())
		})
	}
}

func TestBumpVersion(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		level    BumpLevel
		expected string
		wantErr  bool
	}{
		{
			name:     "patch bump 1.0.0",
			current:  "1.0.0",
			level:    BumpPatch,
			expected: "1.0.1",
		},
		{
			name:     "minor bump 1.0.0 resets patch",
			current:  "1.0.0",
			level:    BumpMinor,
			expected: "1.1.0",
		},
		{
			name:     "major bump 1.0.0 resets minor and patch",
			current:  "1.0.0",
			level:    BumpMajor,
			expected: "2.0.0",
		},
		{
			name:     "minor bump 1.2.3 resets patch",
			current:  "1.2.3",
			level:    BumpMinor,
			expected: "1.3.0",
		},
		{
			name:     "major bump 2.5.9 resets minor and patch",
			current:  "2.5.9",
			level:    BumpMajor,
			expected: "3.0.0",
		},
		{
			name:    "invalid version returns error",
			current: "abc",
			level:   BumpPatch,
			wantErr: true,
		},
		{
			name:    "invalid major version",
			current: "x.1.2",
			level:   BumpPatch,
			wantErr: true,
		},
		{
			name:    "invalid minor version",
			current: "1.y.2",
			level:   BumpPatch,
			wantErr: true,
		},
		{
			name:    "invalid patch version",
			current: "1.2.z",
			level:   BumpPatch,
			wantErr: true,
		},
		{
			name:    "too few parts",
			current: "1.2",
			level:   BumpPatch,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := BumpVersion(tc.current, tc.level)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}
