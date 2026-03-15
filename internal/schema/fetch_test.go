package schema

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConvertToJSONSchema_Basic(t *testing.T) {
	policyDef := map[string]interface{}{
		"type":        "object",
		"description": "A test policy",
		"properties": map[string]interface{}{
			"displayName": map[string]interface{}{
				"type":        "string",
				"description": "The display name",
			},
			"state": map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"enabled", "disabled"},
			},
		},
		"required": []interface{}{"displayName"},
	}

	allSchemas := map[string]interface{}{}

	schema, err := convertToJSONSchema(policyDef, allSchemas)
	if err != nil {
		t.Fatalf("convertToJSONSchema() error = %v", err)
	}

	if schema["$schema"] != "http://json-schema.org/draft-07/schema#" {
		t.Errorf("$schema = %v, want draft-07", schema["$schema"])
	}
	if schema["type"] != "object" {
		t.Errorf("type = %v, want object", schema["type"])
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties is not a map")
	}
	if _, ok := props["displayName"]; !ok {
		t.Error("missing displayName property")
	}
	if _, ok := props["state"]; !ok {
		t.Error("missing state property")
	}

	req, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required is not a string slice")
	}
	if len(req) != 1 || req[0] != "displayName" {
		t.Errorf("required = %v, want [displayName]", req)
	}
}

func TestConvertToJSONSchema_WithRelatedTypes(t *testing.T) {
	policyDef := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"conditions": map[string]interface{}{
				"$ref": "#/components/schemas/microsoft.graph.conditionalAccessConditionSet",
			},
		},
	}

	allSchemas := map[string]interface{}{
		"microsoft.graph.conditionalAccessConditionSet": map[string]interface{}{
			"type":        "object",
			"description": "Condition set",
			"properties": map[string]interface{}{
				"users": map[string]interface{}{
					"type": "object",
				},
			},
		},
	}

	schema, err := convertToJSONSchema(policyDef, allSchemas)
	if err != nil {
		t.Fatalf("convertToJSONSchema() error = %v", err)
	}

	defs, ok := schema["definitions"].(map[string]interface{})
	if !ok {
		t.Fatal("definitions is not a map")
	}
	if _, ok := defs["conditionalAccessConditionSet"]; !ok {
		t.Error("missing conditionalAccessConditionSet definition")
	}

	// Check that the $ref was rewritten
	props := schema["properties"].(map[string]interface{})
	cond := props["conditions"].(map[string]interface{})
	ref, ok := cond["$ref"].(string)
	if !ok {
		t.Fatal("conditions.$ref is not a string")
	}
	if ref != "#/definitions/conditionalAccessConditionSet" {
		t.Errorf("conditions.$ref = %q, want %q", ref, "#/definitions/conditionalAccessConditionSet")
	}
}

func TestConvertToJSONSchema_AllOfComposition(t *testing.T) {
	policyDef := map[string]interface{}{
		"allOf": []interface{}{
			map[string]interface{}{
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type": "string",
					},
				},
			},
			map[string]interface{}{
				"properties": map[string]interface{}{
					"displayName": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []interface{}{"displayName"},
			},
		},
	}

	schema, err := convertToJSONSchema(policyDef, map[string]interface{}{})
	if err != nil {
		t.Fatalf("convertToJSONSchema() error = %v", err)
	}

	props := schema["properties"].(map[string]interface{})
	if _, ok := props["id"]; !ok {
		t.Error("missing id property from allOf merge")
	}
	if _, ok := props["displayName"]; !ok {
		t.Error("missing displayName property from allOf merge")
	}
}

func TestExtractProperties_NotAnObject(t *testing.T) {
	_, _, err := extractProperties("not-a-map")
	if err == nil {
		t.Error("extractProperties(string) = nil, want error")
	}
}

func TestConvertProperty_NonMap(t *testing.T) {
	got := convertProperty("just-a-string")
	if got != "just-a-string" {
		t.Errorf("convertProperty(string) = %v, want original string", got)
	}
}

func TestConvertProperty_WithItems(t *testing.T) {
	prop := map[string]interface{}{
		"type": "array",
		"items": map[string]interface{}{
			"type": "string",
		},
	}
	got := convertProperty(prop).(map[string]interface{})
	if got["type"] != "array" {
		t.Errorf("type = %v, want array", got["type"])
	}
	items, ok := got["items"].(map[string]interface{})
	if !ok {
		t.Fatal("items is not a map")
	}
	if items["type"] != "string" {
		t.Errorf("items.type = %v, want string", items["type"])
	}
}

func TestConvertProperty_InferObject(t *testing.T) {
	prop := map[string]interface{}{
		"properties": map[string]interface{}{
			"foo": map[string]interface{}{"type": "string"},
		},
	}
	got := convertProperty(prop).(map[string]interface{})
	if got["type"] != "object" {
		t.Errorf("inferred type = %v, want object", got["type"])
	}
}

func TestResolveRef_UnknownRef(t *testing.T) {
	m := map[string]interface{}{
		"$ref": "#/components/schemas/microsoft.graph.unknownType",
	}
	definitions := map[string]interface{}{}
	resolveRef(m, definitions)

	if _, hasRef := m["$ref"]; hasRef {
		t.Error("unknown $ref should be removed")
	}
	if m["type"] != "object" {
		t.Errorf("type = %v, want object for unknown ref", m["type"])
	}
}

func TestDescriptionFrom(t *testing.T) {
	tests := []struct {
		name string
		def  interface{}
		want string
	}{
		{"with description", map[string]interface{}{"description": "hello"}, "hello"},
		{"without description", map[string]interface{}{}, ""},
		{"not a map", "string", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := descriptionFrom(tt.def)
			if got != tt.want {
				t.Errorf("descriptionFrom() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteEmbedded(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "schema.json")

	if err := WriteEmbedded(dest); err != nil {
		t.Fatalf("WriteEmbedded() error = %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 {
		t.Error("embedded schema file is empty")
	}
}

func TestFetchOrFallback_UsesEmbedded(t *testing.T) {
	// Fetch will fail (no network in tests or wrong URL), should fall back to embedded
	dir := t.TempDir()
	dest := filepath.Join(dir, "schema.json")

	usedFallback, err := FetchOrFallback(dest)
	if err != nil {
		t.Fatalf("FetchOrFallback() error = %v", err)
	}

	// In CI/test the real fetch may or may not work, but either way should succeed
	_ = usedFallback

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 {
		t.Error("schema file is empty after FetchOrFallback")
	}
}
