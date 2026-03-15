package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// schemaURL points to the pre-sliced Identity.SignIns OpenAPI spec from
	// the msgraph-sdk-powershell repo. This is ~1-2MB instead of the full
	// ~30MB v1.0 OpenAPI YAML.
	schemaURL = "https://raw.githubusercontent.com/microsoftgraph/msgraph-sdk-powershell/main/openApiDocs/v1.0/Identity.SignIns.yml"

	// fetchTimeout is the HTTP client timeout for schema fetch attempts.
	fetchTimeout = 30 * time.Second

	// caPrefix is the prefix for CA policy types in the OpenAPI components.
	caPrefix = "microsoft.graph.conditionalAccessPolicy"
)

// relatedTypes are the component schema names we extract alongside the
// main conditionalAccessPolicy definition.
var relatedTypes = []string{
	"microsoft.graph.conditionalAccessConditionSet",
	"microsoft.graph.conditionalAccessGrantControls",
	"microsoft.graph.conditionalAccessSessionControls",
}

// openAPISpec is the minimal structure we need from the OpenAPI YAML.
type openAPISpec struct {
	Components struct {
		Schemas map[string]interface{} `yaml:"schemas"`
	} `yaml:"components"`
}

// Fetch downloads the CA policy schema from the Microsoft Graph metadata
// repository, extracts the conditionalAccessPolicy definition and related
// types, converts them to a standalone JSON Schema (draft-07), and writes
// the result to destPath.
func Fetch(destPath string) error {
	client := &http.Client{Timeout: fetchTimeout}

	resp, err := client.Get(schemaURL)
	if err != nil {
		return fmt.Errorf("fetching schema from %s: %w", schemaURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching schema: HTTP %d from %s", resp.StatusCode, schemaURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading schema response: %w", err)
	}

	var spec openAPISpec
	if err := yaml.Unmarshal(body, &spec); err != nil {
		return fmt.Errorf("parsing OpenAPI YAML: %w", err)
	}

	policySchema, ok := spec.Components.Schemas[caPrefix]
	if !ok {
		return fmt.Errorf("schema %q not found in OpenAPI components", caPrefix)
	}

	jsonSchema, err := convertToJSONSchema(policySchema, spec.Components.Schemas)
	if err != nil {
		return fmt.Errorf("converting to JSON Schema: %w", err)
	}

	out, err := json.MarshalIndent(jsonSchema, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON Schema: %w", err)
	}
	out = append(out, '\n')

	if err := os.WriteFile(destPath, out, 0644); err != nil { //nolint:gosec // G306 - schema files not sensitive
		return fmt.Errorf("writing schema to %s: %w", destPath, err)
	}

	return nil
}

// convertToJSONSchema converts an OpenAPI schema component (and its related
// types) into a standalone JSON Schema draft-07 document.
func convertToJSONSchema(policyDef interface{}, allSchemas map[string]interface{}) (map[string]interface{}, error) {
	props, required, err := extractProperties(policyDef)
	if err != nil {
		return nil, fmt.Errorf("extracting policy properties: %w", err)
	}

	definitions := make(map[string]interface{})
	for _, typeName := range relatedTypes {
		def, ok := allSchemas[typeName]
		if !ok {
			continue
		}
		defProps, _, err := extractProperties(def)
		if err != nil {
			continue
		}
		shortName := strings.TrimPrefix(typeName, "microsoft.graph.")
		definitions[shortName] = map[string]interface{}{
			"type":                 "object",
			"description":          descriptionFrom(def),
			"additionalProperties": true,
			"properties":           defProps,
		}
	}

	// Rewrite $ref-style property references to use local #/definitions/
	rewriteRefs(props, definitions)
	for _, def := range definitions {
		if dm, ok := def.(map[string]interface{}); ok {
			if dp, ok := dm["properties"].(map[string]interface{}); ok {
				rewriteRefs(dp, definitions)
			}
		}
	}

	schema := map[string]interface{}{
		"$schema":              "http://json-schema.org/draft-07/schema#",
		"$id":                  "https://graph.microsoft.com/v1.0/conditionalAccessPolicy",
		"title":                "Conditional Access Policy",
		"description":          "Microsoft Entra Conditional Access policy resource. Fetched from Microsoft Graph OpenAPI spec.",
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}

	if len(required) > 0 {
		schema["required"] = required
	}
	if len(definitions) > 0 {
		schema["definitions"] = definitions
	}

	return schema, nil
}

// extractProperties pulls the properties map and required list from an
// OpenAPI schema definition (represented as a generic map). It handles
// allOf composition by merging properties from all allOf entries.
func extractProperties(def interface{}) (map[string]interface{}, []string, error) {
	m, ok := def.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("schema definition is not an object")
	}

	props := make(map[string]interface{})
	var required []string

	// Collect properties directly on the definition
	collectProps(m, props, &required)

	// Handle allOf: merge properties from each entry
	if allOf, ok := m["allOf"].([]interface{}); ok {
		for _, entry := range allOf {
			if em, ok := entry.(map[string]interface{}); ok {
				collectProps(em, props, &required)
			}
		}
	}

	return props, required, nil
}

// collectProps extracts properties and required fields from a single schema map.
func collectProps(m map[string]interface{}, props map[string]interface{}, required *[]string) {
	if p, ok := m["properties"].(map[string]interface{}); ok {
		for k, v := range p {
			props[k] = convertProperty(v)
		}
	}
	if r, ok := m["required"].([]interface{}); ok {
		for _, v := range r {
			if s, ok := v.(string); ok {
				*required = append(*required, s)
			}
		}
	}
}

// convertProperty converts an OpenAPI property definition to a JSON Schema
// property definition.
func convertProperty(v interface{}) interface{} {
	m, ok := v.(map[string]interface{})
	if !ok {
		return v
	}

	result := make(map[string]interface{})

	// Copy basic fields
	for _, field := range []string{"type", "description", "enum", "format", "readOnly", "default"} {
		if val, ok := m[field]; ok {
			result[field] = val
		}
	}

	// Handle $ref to another component
	if ref, ok := m["$ref"].(string); ok {
		// Store original ref for later rewriting
		result["$ref"] = ref
	}

	// Handle array items
	if items, ok := m["items"].(map[string]interface{}); ok {
		result["items"] = convertProperty(items)
	}

	// If no type and no $ref, infer object
	if _, hasType := result["type"]; !hasType {
		if _, hasRef := result["$ref"]; !hasRef {
			if _, hasProps := m["properties"]; hasProps {
				result["type"] = "object"
				result["additionalProperties"] = true
			}
		}
	}

	return result
}

// rewriteRefs rewrites OpenAPI $ref values (e.g.
// "#/components/schemas/microsoft.graph.conditionalAccessConditionSet")
// to JSON Schema local refs (e.g. "#/definitions/conditionalAccessConditionSet"),
// but only if the referenced type exists in our definitions.
func rewriteRefs(props map[string]interface{}, definitions map[string]interface{}) {
	for k, v := range props {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		resolveRef(m, definitions)
		// Also handle $ref inside array items
		if items, ok := m["items"].(map[string]interface{}); ok {
			resolveRef(items, definitions)
		}
		props[k] = m
	}
}

// resolveRef rewrites a single $ref in a property map.
func resolveRef(m map[string]interface{}, definitions map[string]interface{}) {
	ref, ok := m["$ref"].(string)
	if !ok {
		return
	}
	typeName := ref
	if idx := strings.LastIndex(ref, "/"); idx >= 0 {
		typeName = ref[idx+1:]
	}
	shortName := strings.TrimPrefix(typeName, "microsoft.graph.")
	if _, exists := definitions[shortName]; exists {
		m["$ref"] = "#/definitions/" + shortName
	} else {
		// Unknown ref: convert to generic object
		delete(m, "$ref")
		m["type"] = "object"
		m["additionalProperties"] = true
	}
}

// descriptionFrom extracts the description field from a schema definition.
func descriptionFrom(def interface{}) string {
	if m, ok := def.(map[string]interface{}); ok {
		if d, ok := m["description"].(string); ok {
			return d
		}
	}
	return ""
}

// FetchOrFallback attempts to fetch the schema from the network. On any
// failure it falls back to the embedded schema. Returns true if the embedded
// fallback was used.
func FetchOrFallback(destPath string) (usedFallback bool, err error) {
	if fetchErr := Fetch(destPath); fetchErr != nil {
		// Network fetch failed; use embedded fallback
		if writeErr := WriteEmbedded(destPath); writeErr != nil {
			return true, fmt.Errorf("embedded schema fallback failed: %w (fetch error: %v)", writeErr, fetchErr)
		}
		return true, nil
	}

	// Verify written file exists
	if _, err := os.Stat(destPath); err != nil {
		return false, fmt.Errorf("schema written but not found at %s: %w", destPath, err)
	}

	return false, nil
}
