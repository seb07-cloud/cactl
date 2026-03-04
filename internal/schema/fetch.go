package schema

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	// schemaURL is the raw URL for the Microsoft Graph v1.0 OpenAPI spec.
	// The full file is ~30MB YAML; Phase 1 attempts the download but falls
	// back to embedded schema on any failure.
	schemaURL = "https://raw.githubusercontent.com/microsoftgraph/msgraph-metadata/master/openapi/v1.0/openapi.yaml"

	// fetchTimeout is the HTTP client timeout for schema fetch attempts.
	fetchTimeout = 30 * time.Second
)

// Fetch attempts to download the CA policy JSON Schema from the Microsoft
// Graph metadata repository and write it to destPath.
//
// Phase 1 pragmatic approach: the full OpenAPI spec is ~30MB YAML which is
// expensive to download and parse during init. This function attempts the
// download but is expected to fail in most environments (timeout, offline).
// The caller should fall back to WriteEmbedded on any error.
//
// Future phases will implement a smarter approach: either use a pre-sliced
// endpoint, cache the spec, or fetch only the CA policy fragment via CSDL.
func Fetch(destPath string) error {
	client := &http.Client{Timeout: fetchTimeout}

	resp, err := client.Get(schemaURL)
	if err != nil {
		return fmt.Errorf("fetching schema from %s: %w", schemaURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching schema: HTTP %d from %s", resp.StatusCode, schemaURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading schema response: %w", err)
	}

	// Phase 1: We downloaded the full OpenAPI YAML but extracting and
	// converting the CA policy fragment to JSON Schema is complex.
	// For now, return an error so the caller falls back to embedded schema.
	// The downloaded content is not written because it's raw YAML, not
	// the JSON Schema format we need.
	_ = body

	return fmt.Errorf("schema extraction from OpenAPI spec not yet implemented (Phase 1 uses embedded fallback)")
}

// FetchOrFallback attempts to fetch the schema from the network. On any
// failure it falls back to the embedded schema. Returns true if the embedded
// fallback was used.
func FetchOrFallback(destPath string) (usedFallback bool, err error) {
	if fetchErr := Fetch(destPath); fetchErr != nil {
		// Network fetch failed or not yet implemented; use embedded fallback
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
