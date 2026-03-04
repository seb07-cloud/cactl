// Package schema provides CA policy JSON Schema fetching and embedded fallback.
package schema

import (
	_ "embed"
	"fmt"
	"os"
)

// EmbeddedSchema contains the embedded fallback JSON Schema for
// conditionalAccessPolicy, compiled into the binary at build time.
//
//go:embed schema.json
var EmbeddedSchema []byte

// WriteEmbedded writes the embedded fallback schema to the given path.
func WriteEmbedded(path string) error {
	if err := os.WriteFile(path, EmbeddedSchema, 0644); err != nil {
		return fmt.Errorf("writing embedded schema: %w", err)
	}
	return nil
}
