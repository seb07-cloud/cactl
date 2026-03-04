// Package normalize provides JSON normalization and slug derivation for CA policies.
package normalize

import (
	"encoding/json"
	"fmt"
	"strings"
)

// serverManagedFields are read-only fields that must be stripped during import.
var serverManagedFields = []string{
	"id",
	"createdDateTime",
	"modifiedDateTime",
	"templateId",
}

// Normalize takes raw Graph API policy JSON and returns canonical form.
// It strips server-managed fields, removes @odata metadata recursively,
// removes null values recursively, sorts keys alphabetically, and
// pretty-prints with 2-space indent and a trailing newline.
func Normalize(raw []byte) ([]byte, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unmarshalling: %w", err)
	}

	// Step 1: Strip server-managed fields
	for _, field := range serverManagedFields {
		delete(m, field)
	}

	// Step 2: Strip @odata metadata fields (recursive)
	stripODataFields(m)

	// Step 3: Recursively remove null values
	removeNulls(m)

	// Step 4: Marshal with sorted keys and 2-space indent
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling: %w", err)
	}

	// Ensure trailing newline
	out = append(out, '\n')
	return out, nil
}

// stripODataFields recursively removes any key containing "@odata." from nested maps.
func stripODataFields(m map[string]interface{}) {
	for k, v := range m {
		if strings.Contains(k, "@odata.") {
			delete(m, k)
			continue
		}
		if nested, ok := v.(map[string]interface{}); ok {
			stripODataFields(nested)
		}
		if arr, ok := v.([]interface{}); ok {
			for _, item := range arr {
				if nestedMap, ok := item.(map[string]interface{}); ok {
					stripODataFields(nestedMap)
				}
			}
		}
	}
}

// removeNulls recursively deletes nil values from nested maps and arrays of maps.
func removeNulls(m map[string]interface{}) {
	for k, v := range m {
		if v == nil {
			delete(m, k)
			continue
		}
		if nested, ok := v.(map[string]interface{}); ok {
			removeNulls(nested)
		}
		if arr, ok := v.([]interface{}); ok {
			for _, item := range arr {
				if nestedMap, ok := item.(map[string]interface{}); ok {
					removeNulls(nestedMap)
				}
			}
		}
	}
}
