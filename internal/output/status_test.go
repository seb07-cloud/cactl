package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/seb07-cloud/cactl/internal/state"
	"github.com/seb07-cloud/cactl/pkg/types"
)

func TestRenderStatus(t *testing.T) {
	entries := []types.PolicyStatus{
		{Slug: "block-legacy", Version: "v1.2.0", LastDeployed: "2026-03-01", DeployedBy: "admin@test.com", SyncStatus: "in-sync", LiveObjectID: "id-1"},
		{Slug: "require-mfa", Version: "v2.0.0", LastDeployed: "2026-03-02", DeployedBy: "ci@test.com", SyncStatus: "drifted", LiveObjectID: "id-2"},
		{Slug: "deleted-policy", Version: "v1.0.0", LastDeployed: "2026-02-15", DeployedBy: "admin@test.com", SyncStatus: "missing", LiveObjectID: "id-3"},
	}

	var buf bytes.Buffer
	RenderStatus(&buf, entries, false)
	out := buf.String()

	// Verify all slugs present
	for _, e := range entries {
		if !strings.Contains(out, e.Slug) {
			t.Errorf("expected output to contain slug %q", e.Slug)
		}
		if !strings.Contains(out, e.Version) {
			t.Errorf("expected output to contain version %q", e.Version)
		}
		if !strings.Contains(out, e.SyncStatus) {
			t.Errorf("expected output to contain sync status %q", e.SyncStatus)
		}
	}

	// Verify header
	if !strings.Contains(out, "POLICY") {
		t.Error("expected output to contain POLICY header")
	}
	if !strings.Contains(out, "SYNC") {
		t.Error("expected output to contain SYNC header")
	}
}

func TestRenderStatusNoColor(t *testing.T) {
	entries := []types.PolicyStatus{
		{Slug: "test-policy", Version: "v1.0.0", LastDeployed: "2026-03-01", DeployedBy: "admin@test.com", SyncStatus: "in-sync"},
		{Slug: "test-drifted", Version: "v1.0.0", LastDeployed: "2026-03-01", DeployedBy: "admin@test.com", SyncStatus: "drifted"},
	}

	var buf bytes.Buffer
	RenderStatus(&buf, entries, false)
	out := buf.String()

	// Verify no ANSI escape codes
	if strings.Contains(out, "\033[") {
		t.Error("expected no ANSI escape codes in no-color output")
	}
}

func TestRenderStatusWithColor(t *testing.T) {
	entries := []types.PolicyStatus{
		{Slug: "test-policy", Version: "v1.0.0", LastDeployed: "2026-03-01", DeployedBy: "admin@test.com", SyncStatus: "in-sync"},
		{Slug: "test-drifted", Version: "v1.0.0", LastDeployed: "2026-03-01", DeployedBy: "admin@test.com", SyncStatus: "drifted"},
		{Slug: "test-missing", Version: "v1.0.0", LastDeployed: "2026-03-01", DeployedBy: "admin@test.com", SyncStatus: "missing"},
	}

	var buf bytes.Buffer
	RenderStatus(&buf, entries, true)
	out := buf.String()

	// Verify ANSI codes present for colored statuses
	if !strings.Contains(out, "\033[32m") {
		t.Error("expected green ANSI code for in-sync")
	}
	if !strings.Contains(out, "\033[33m") {
		t.Error("expected yellow ANSI code for drifted")
	}
	if !strings.Contains(out, "\033[31m") {
		t.Error("expected red ANSI code for missing")
	}
}

func TestRenderStatusJSON(t *testing.T) {
	output := types.StatusOutput{
		SchemaVersion: 1,
		Tenant:        "test-tenant",
		Policies: []types.PolicyStatus{
			{Slug: "block-legacy", Version: "v1.0.0", SyncStatus: "in-sync"},
		},
		Summary: types.StatusSummary{Total: 1, InSync: 1},
	}

	var buf bytes.Buffer
	if err := RenderStatusJSON(&buf, output); err != nil {
		t.Fatalf("RenderStatusJSON failed: %v", err)
	}

	// Verify valid JSON
	var parsed types.StatusOutput
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed.SchemaVersion != 1 {
		t.Errorf("expected schema_version=1, got %d", parsed.SchemaVersion)
	}
	if parsed.Tenant != "test-tenant" {
		t.Errorf("expected tenant=test-tenant, got %s", parsed.Tenant)
	}
	if len(parsed.Policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(parsed.Policies))
	}
}

func TestRenderHistory(t *testing.T) {
	tags := []state.VersionTag{
		{Version: "v1.2.0", Timestamp: "2026-03-03", Message: "Update MFA settings"},
		{Version: "v1.1.0", Timestamp: "2026-03-02", Message: "Add device compliance"},
		{Version: "v1.0.0", Timestamp: "2026-03-01", Message: "Initial import"},
	}

	var buf bytes.Buffer
	RenderHistory(&buf, "block-legacy", tags, false)
	out := buf.String()

	// Verify header
	if !strings.Contains(out, "VERSION") {
		t.Error("expected output to contain VERSION header")
	}
	if !strings.Contains(out, "MESSAGE") {
		t.Error("expected output to contain MESSAGE header")
	}

	// Verify all versions and timestamps
	for _, tag := range tags {
		if !strings.Contains(out, tag.Version) {
			t.Errorf("expected output to contain version %q", tag.Version)
		}
		if !strings.Contains(out, tag.Timestamp) {
			t.Errorf("expected output to contain timestamp %q", tag.Timestamp)
		}
	}

	// Verify slug in header
	if !strings.Contains(out, "block-legacy") {
		t.Error("expected output to contain policy slug")
	}
}

func TestBuildSummary(t *testing.T) {
	entries := []types.PolicyStatus{
		{SyncStatus: "in-sync"},
		{SyncStatus: "in-sync"},
		{SyncStatus: "drifted"},
		{SyncStatus: "missing"},
		{SyncStatus: "unknown"},
	}

	s := BuildSummary(entries)

	if s.Total != 5 {
		t.Errorf("expected total=5, got %d", s.Total)
	}
	if s.InSync != 2 {
		t.Errorf("expected in_sync=2, got %d", s.InSync)
	}
	if s.Drifted != 1 {
		t.Errorf("expected drifted=1, got %d", s.Drifted)
	}
	if s.Missing != 1 {
		t.Errorf("expected missing=1, got %d", s.Missing)
	}
	if s.Unknown != 1 {
		t.Errorf("expected unknown=1, got %d", s.Unknown)
	}
}

func TestRenderStatusSummaryLine(t *testing.T) {
	entries := []types.PolicyStatus{
		{SyncStatus: "in-sync"},
		{SyncStatus: "drifted"},
		{SyncStatus: "missing"},
	}

	var buf bytes.Buffer
	RenderStatus(&buf, entries, false)
	out := buf.String()

	if !strings.Contains(out, "3 total") {
		t.Error("expected summary to contain '3 total'")
	}
	if !strings.Contains(out, "1 in-sync") {
		t.Error("expected summary to contain '1 in-sync'")
	}
	if !strings.Contains(out, "1 drifted") {
		t.Error("expected summary to contain '1 drifted'")
	}
	if !strings.Contains(out, "1 missing") {
		t.Error("expected summary to contain '1 missing'")
	}
	if !strings.Contains(out, "0 unknown") {
		t.Error("expected summary to contain '0 unknown'")
	}
}
