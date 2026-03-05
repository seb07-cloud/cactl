package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// BumpLevel represents the semantic versioning bump level.
type BumpLevel int

const (
	BumpPatch BumpLevel = iota
	BumpMinor
	BumpMajor
)

// String returns the human-readable name of the bump level.
func (b BumpLevel) String() string {
	switch b {
	case BumpPatch:
		return "PATCH"
	case BumpMinor:
		return "MINOR"
	case BumpMajor:
		return "MAJOR"
	default:
		return "UNKNOWN"
	}
}

// FieldDiff represents a single field-level difference between two JSON documents.
// This is a local type matching the reconcile.FieldDiff interface to avoid
// circular dependencies. The reconcile package defines the canonical type.
type FieldDiff struct {
	Path     string
	OldValue interface{}
	NewValue interface{}
}

// DetermineBump analyzes field diffs against configured triggers and returns the
// highest applicable bump level. Any major field triggers BumpMajor immediately
// (short-circuit). Any minor field triggers BumpMinor. Default is BumpPatch.
//
// Field matching uses prefix matching: trigger "conditions" matches
// "conditions.users.includeGroups".
func DetermineBump(diffs []FieldDiff, majorFields, minorFields []string) BumpLevel {
	bump := BumpPatch
	for _, d := range diffs {
		if matchesAny(d.Path, majorFields) {
			return BumpMajor // Short-circuit: any major field = MAJOR
		}
		if matchesAny(d.Path, minorFields) {
			bump = BumpMinor
		}
	}
	return bump
}

// matchesAny checks if a field path matches any configured trigger.
// Supports exact match and prefix matching: "conditions" matches
// "conditions.users.includeGroups" but not "conditionsExtra".
func matchesAny(path string, triggers []string) bool {
	for _, trigger := range triggers {
		if path == trigger || strings.HasPrefix(path, trigger+".") {
			return true
		}
	}
	return false
}

// BumpVersion increments a semantic version string by the given level.
// The version string must be in "X.Y.Z" format (no "v" prefix).
// BumpMajor resets minor and patch to 0. BumpMinor resets patch to 0.
func BumpVersion(current string, level BumpLevel) (string, error) {
	parts := strings.Split(current, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid version format %q: expected X.Y.Z", current)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid major version %q: %w", parts[0], err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid minor version %q: %w", parts[1], err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid patch version %q: %w", parts[2], err)
	}

	switch level {
	case BumpMajor:
		major++
		minor = 0
		patch = 0
	case BumpMinor:
		minor++
		patch = 0
	case BumpPatch:
		patch++
	}

	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}
