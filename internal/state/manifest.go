package state

import (
	"encoding/json"
	"fmt"
)

// Manifest holds the state of all tracked policies for a tenant.
type Manifest struct {
	SchemaVersion int              `json:"schema_version"`
	Tenant        string           `json:"tenant"`
	Policies      map[string]Entry `json:"policies"`
}

// Entry tracks a single policy's state. All STATE-05 fields are present.
type Entry struct {
	Slug         string `json:"slug"`
	Tenant       string `json:"tenant"`
	LiveObjectID string `json:"live_object_id"`
	Version      string `json:"version"`
	LastDeployed string `json:"last_deployed"`
	DeployedBy   string `json:"deployed_by"`
	AuthMode     string `json:"auth_mode"`
	BackendSHA   string `json:"backend_sha"`
}

// manifestRef returns the ref path for a tenant's manifest.
func manifestRef(tenantID string) string {
	return fmt.Sprintf("refs/cactl/tenants/%s/manifest", tenantID)
}

// ReadManifest reads the state manifest from Git refs for the given tenant.
// If the manifest does not exist yet, returns an empty Manifest with SchemaVersion=1.
func ReadManifest(backend *GitBackend, tenantID string) (*Manifest, error) {
	ref := manifestRef(tenantID)
	data, err := backend.catFile(ref)
	if err != nil {
		// Ref doesn't exist -- return empty manifest
		return &Manifest{
			SchemaVersion: 1,
			Tenant:        tenantID,
			Policies:      make(map[string]Entry),
		}, nil
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshalling manifest: %w", err)
	}

	// Ensure Policies map is initialized
	if m.Policies == nil {
		m.Policies = make(map[string]Entry)
	}

	return &m, nil
}

// WriteManifest writes the state manifest as a JSON blob to Git refs.
func WriteManifest(backend *GitBackend, tenantID string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling manifest: %w", err)
	}

	hash, err := backend.hashObject(data)
	if err != nil {
		return fmt.Errorf("writing manifest blob: %w", err)
	}

	ref := manifestRef(tenantID)
	if err := backend.updateRef(ref, hash); err != nil {
		return fmt.Errorf("updating manifest ref %s: %w", ref, err)
	}

	return nil
}
