package reconcile

import (
	"reflect"
	"sort"
)

// DiffType classifies a field-level difference.
type DiffType int

const (
	// DiffAdded means the field exists in desired but not in actual.
	DiffAdded DiffType = iota
	// DiffRemoved means the field exists in actual but not in desired.
	DiffRemoved
	// DiffChanged means the field exists in both but with different values.
	DiffChanged
)

// FieldDiff represents a single field-level difference between two maps.
type FieldDiff struct {
	Path     string
	Type     DiffType
	OldValue interface{}
	NewValue interface{}
}

// ComputeDiff compares desired and actual maps and returns field-level diffs.
// Paths use dot-separated notation for nested maps.
// Results are sorted by path for deterministic output.
func ComputeDiff(desired, actual map[string]interface{}) []FieldDiff {
	var diffs []FieldDiff
	computeDiffRecursive("", desired, actual, &diffs)

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Path < diffs[j].Path
	})

	if len(diffs) == 0 {
		return nil
	}
	return diffs
}

// computeDiffRecursive walks both maps recursively, collecting diffs.
func computeDiffRecursive(prefix string, desired, actual map[string]interface{}, diffs *[]FieldDiff) {
	// Check for added and changed fields
	for key, desVal := range desired {
		path := joinPath(prefix, key)
		actVal, exists := actual[key]

		if !exists {
			*diffs = append(*diffs, FieldDiff{
				Path:     path,
				Type:     DiffAdded,
				OldValue: nil,
				NewValue: desVal,
			})
			continue
		}

		// Both exist -- check if both are nested maps for recursive descent
		desMap, desIsMap := desVal.(map[string]interface{})
		actMap, actIsMap := actVal.(map[string]interface{})

		if desIsMap && actIsMap {
			computeDiffRecursive(path, desMap, actMap, diffs)
			continue
		}

		// Leaf comparison
		if !reflect.DeepEqual(desVal, actVal) {
			*diffs = append(*diffs, FieldDiff{
				Path:     path,
				Type:     DiffChanged,
				OldValue: actVal,
				NewValue: desVal,
			})
		}
	}

	// Check for removed fields (in actual but not in desired)
	for key, actVal := range actual {
		path := joinPath(prefix, key)
		if _, exists := desired[key]; !exists {
			*diffs = append(*diffs, FieldDiff{
				Path:     path,
				Type:     DiffRemoved,
				OldValue: actVal,
				NewValue: nil,
			})
		}
	}
}

// joinPath builds a dot-separated path from a prefix and key.
func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}
