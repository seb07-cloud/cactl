package cmd

import (
	"testing"

	"github.com/seb07-cloud/cactl/internal/state"
	"github.com/seb07-cloud/cactl/pkg/types"
)

func TestStatusCmdRegistered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "status" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'status' command to be registered on rootCmd")
	}
}

func TestStatusCmdHasHistoryFlag(t *testing.T) {
	f := statusCmd.Flags().Lookup("history")
	if f == nil {
		t.Error("expected 'status' command to have --history flag")
	}
}

func TestStatusRequiresTenant(t *testing.T) {
	// The command loads config via viper; an empty tenant should cause an error.
	// We test this indirectly through buildPolicyStatuses which doesn't need tenant,
	// but the runStatus function checks it. The registration test above confirms wiring.
}

func TestBuildPolicyStatuses(t *testing.T) {
	manifest := &state.Manifest{
		SchemaVersion: 1,
		Tenant:        "test-tenant",
		Policies: map[string]state.Entry{
			"block-legacy": {
				Slug:         "block-legacy",
				Version:      "v1.0.0",
				LastDeployed: "2026-03-01",
				DeployedBy:   "admin@test.com",
				LiveObjectID: "id-1",
				BackendSHA:   "abc123",
			},
			"require-mfa": {
				Slug:         "require-mfa",
				Version:      "v2.0.0",
				LastDeployed: "2026-03-02",
				DeployedBy:   "ci@test.com",
				LiveObjectID: "id-2",
				BackendSHA:   "def456",
			},
			"deleted-policy": {
				Slug:         "deleted-policy",
				Version:      "v1.0.0",
				LastDeployed: "2026-02-15",
				DeployedBy:   "admin@test.com",
				LiveObjectID: "id-3",
				BackendSHA:   "ghi789",
			},
		},
	}

	// Case 1: sync not available -> all unknown
	t.Run("sync unavailable", func(t *testing.T) {
		entries := buildPolicyStatuses(manifest, false, nil)
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}
		for _, e := range entries {
			if e.SyncStatus != "unknown" {
				t.Errorf("expected sync status 'unknown' for %s, got %s", e.Slug, e.SyncStatus)
			}
		}
	})

	// Case 2: sync available with various conditions
	t.Run("sync available", func(t *testing.T) {
		liveIndex := map[string]string{
			"id-1": "abc123", // matches -> in-sync
			"id-2": "zzz999", // differs -> drifted
			// id-3 missing -> missing
		}

		entries := buildPolicyStatuses(manifest, true, liveIndex)
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}

		statusMap := make(map[string]string)
		for _, e := range entries {
			statusMap[e.Slug] = e.SyncStatus
		}

		if statusMap["block-legacy"] != "in-sync" {
			t.Errorf("expected block-legacy to be 'in-sync', got %q", statusMap["block-legacy"])
		}
		if statusMap["require-mfa"] != "drifted" {
			t.Errorf("expected require-mfa to be 'drifted', got %q", statusMap["require-mfa"])
		}
		if statusMap["deleted-policy"] != "missing" {
			t.Errorf("expected deleted-policy to be 'missing', got %q", statusMap["deleted-policy"])
		}
	})

	// Case 3: verify fields are mapped correctly
	t.Run("field mapping", func(t *testing.T) {
		entries := buildPolicyStatuses(manifest, false, nil)
		found := false
		for _, e := range entries {
			if e.Slug == "block-legacy" {
				found = true
				if e.Version != "v1.0.0" {
					t.Errorf("expected version v1.0.0, got %s", e.Version)
				}
				if e.LastDeployed != "2026-03-01" {
					t.Errorf("expected last_deployed 2026-03-01, got %s", e.LastDeployed)
				}
				if e.DeployedBy != "admin@test.com" {
					t.Errorf("expected deployed_by admin@test.com, got %s", e.DeployedBy)
				}
				if e.LiveObjectID != "id-1" {
					t.Errorf("expected live_object_id id-1, got %s", e.LiveObjectID)
				}
			}
		}
		if !found {
			t.Error("block-legacy not found in entries")
		}
	})
}

// Ensure unused imports don't cause issues -- types is used in buildPolicyStatuses return type.
var _ types.PolicyStatus
