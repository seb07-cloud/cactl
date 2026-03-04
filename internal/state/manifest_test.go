package state

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestRoundTrip(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	m := &Manifest{
		SchemaVersion: 1,
		Tenant:        "tenant-rt",
		Policies: map[string]Entry{
			"policy-a": {
				Slug:         "policy-a",
				Tenant:       "tenant-rt",
				LiveObjectID: "aaaa-bbbb-cccc",
				Version:      "1.0.0",
				LastDeployed: "2026-03-04T12:00:00Z",
				DeployedBy:   "test-user",
				AuthMode:     "device-code",
				BackendSHA:   "abc123",
			},
			"policy-b": {
				Slug:         "policy-b",
				Tenant:       "tenant-rt",
				LiveObjectID: "dddd-eeee-ffff",
				Version:      "1.0.0",
				LastDeployed: "2026-03-04T12:00:00Z",
				DeployedBy:   "test-user",
				AuthMode:     "sp-secret",
				BackendSHA:   "def456",
			},
		},
	}

	err = WriteManifest(b, "tenant-rt", m)
	require.NoError(t, err)

	got, err := ReadManifest(b, "tenant-rt")
	require.NoError(t, err)
	assert.Equal(t, m.SchemaVersion, got.SchemaVersion)
	assert.Equal(t, m.Tenant, got.Tenant)
	assert.Equal(t, len(m.Policies), len(got.Policies))
	assert.Equal(t, m.Policies["policy-a"], got.Policies["policy-a"])
	assert.Equal(t, m.Policies["policy-b"], got.Policies["policy-b"])
}

func TestManifestReadNotFound(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	got, err := ReadManifest(b, "tenant-none")
	require.NoError(t, err)
	assert.Equal(t, 1, got.SchemaVersion)
	assert.NotNil(t, got.Policies)
	assert.Empty(t, got.Policies)
}

func TestManifestAddEntry(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	m, err := ReadManifest(b, "tenant-add")
	require.NoError(t, err)

	entry := Entry{
		Slug:         "new-policy",
		Tenant:       "tenant-add",
		LiveObjectID: "1111-2222-3333",
		Version:      "1.0.0",
		LastDeployed: "2026-03-04T13:00:00Z",
		DeployedBy:   "admin",
		AuthMode:     "sp-cert",
		BackendSHA:   "sha789",
	}
	m.Policies["new-policy"] = entry

	err = WriteManifest(b, "tenant-add", m)
	require.NoError(t, err)

	got, err := ReadManifest(b, "tenant-add")
	require.NoError(t, err)
	assert.Contains(t, got.Policies, "new-policy")
	assert.Equal(t, entry, got.Policies["new-policy"])
}

func TestEntryHasAllFields(t *testing.T) {
	entry := Entry{
		Slug:         "test-slug",
		Tenant:       "test-tenant",
		LiveObjectID: "obj-id",
		Version:      "1.0.0",
		LastDeployed: "2026-03-04T14:00:00Z",
		DeployedBy:   "deployer",
		AuthMode:     "device-code",
		BackendSHA:   "shasum",
	}

	data, err := json.Marshal(entry)
	require.NoError(t, err)

	var m map[string]interface{}
	err = json.Unmarshal(data, &m)
	require.NoError(t, err)

	expectedFields := []string{
		"slug", "tenant", "live_object_id", "version",
		"last_deployed", "deployed_by", "auth_mode", "backend_sha",
	}
	for _, field := range expectedFields {
		assert.Contains(t, m, field, "missing field: %s", field)
	}
}
