package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/seb07-cloud/cactl/internal/reconcile"
)

const policiesDir = "policies"

// ReadDesiredPolicies reads all policy JSON files from policies/<tenantID>/
// on disk. These are the user-editable desired state files.
// Returns a map of slug -> BackendPolicy.
func ReadDesiredPolicies(tenantID string) (map[string]reconcile.BackendPolicy, error) {
	dir := filepath.Join(policiesDir, tenantID)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no policies directory found at %s -- run 'cactl import' first", dir)
		}
		return nil, fmt.Errorf("reading policies directory %s: %w", dir, err)
	}

	policies := make(map[string]reconcile.BackendPolicy)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		slug := strings.TrimSuffix(entry.Name(), ".json")
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading policy file %s: %w", entry.Name(), err)
		}

		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parsing policy file %s: %w", entry.Name(), err)
		}

		policies[slug] = reconcile.BackendPolicy{Data: m}
	}

	return policies, nil
}

// WritePolicyFile writes normalized policy JSON to policies/<tenantID>/<slug>.json.
func WritePolicyFile(tenantID, slug string, data []byte) error {
	dir := filepath.Join(policiesDir, tenantID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating policies directory: %w", err)
	}
	path := filepath.Join(dir, slug+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing policy file %s: %w", path, err)
	}
	return nil
}
