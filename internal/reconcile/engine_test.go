package reconcile

import (
	"testing"

	"github.com/seb07-cloud/cactl/internal/state"
)

func TestReconcile(t *testing.T) {
	tests := []struct {
		name            string
		backend         map[string]BackendPolicy
		live            map[string]LivePolicy
		manifest        *state.Manifest
		expectedActions []PolicyAction
	}{
		{
			name: "create: backend has policy, manifest has no entry",
			backend: map[string]BackendPolicy{
				"ca-mfa": {Data: map[string]interface{}{"displayName": "MFA Policy", "state": "enabled"}},
			},
			live:     map[string]LivePolicy{},
			manifest: newManifest(),
			expectedActions: []PolicyAction{
				{Slug: "ca-mfa", Action: ActionCreate},
			},
		},
		{
			name: "update: backend and live differ",
			backend: map[string]BackendPolicy{
				"ca-mfa": {Data: map[string]interface{}{"displayName": "MFA Policy", "state": "enabled"}},
			},
			live: map[string]LivePolicy{
				"obj-111": {NormalizedData: map[string]interface{}{"displayName": "MFA Policy", "state": "disabled"}, Slug: "ca-mfa"},
			},
			manifest: newManifestWith("ca-mfa", "obj-111"),
			expectedActions: []PolicyAction{
				{Slug: "ca-mfa", Action: ActionUpdate},
			},
		},
		{
			name: "noop: backend and live match (idempotent)",
			backend: map[string]BackendPolicy{
				"ca-mfa": {Data: map[string]interface{}{"displayName": "MFA Policy", "state": "enabled"}},
			},
			live: map[string]LivePolicy{
				"obj-111": {NormalizedData: map[string]interface{}{"displayName": "MFA Policy", "state": "enabled"}, Slug: "ca-mfa"},
			},
			manifest:        newManifestWith("ca-mfa", "obj-111"),
			expectedActions: nil, // No actions emitted for noop
		},
		{
			name: "recreate: manifest tracks ID but live doesn't have it (ghost)",
			backend: map[string]BackendPolicy{
				"ca-mfa": {Data: map[string]interface{}{"displayName": "MFA Policy", "state": "enabled"}},
			},
			live:     map[string]LivePolicy{}, // obj-111 is gone
			manifest: newManifestWith("ca-mfa", "obj-111"),
			expectedActions: []PolicyAction{
				{Slug: "ca-mfa", Action: ActionRecreate},
			},
		},
		{
			name:    "untracked: live has policy not in manifest",
			backend: map[string]BackendPolicy{},
			live: map[string]LivePolicy{
				"xyz789": {NormalizedData: map[string]interface{}{"displayName": "Unknown Policy"}, Slug: "unknown-policy"},
			},
			manifest: newManifest(),
			expectedActions: []PolicyAction{
				{Slug: "unknown-policy", Action: ActionUntracked},
			},
		},
		{
			name: "mixed: two backend policies and one untracked",
			backend: map[string]BackendPolicy{
				"ca-mfa":    {Data: map[string]interface{}{"displayName": "MFA Policy", "state": "enabled"}},
				"ca-legacy": {Data: map[string]interface{}{"displayName": "Legacy", "state": "disabled"}},
			},
			live: map[string]LivePolicy{
				"obj-222": {NormalizedData: map[string]interface{}{"displayName": "Legacy", "state": "enabled"}, Slug: "ca-legacy"},
				"xyz789":  {NormalizedData: map[string]interface{}{"displayName": "Rogue"}, Slug: "rogue-policy"},
			},
			manifest: newManifestWith("ca-legacy", "obj-222"),
			expectedActions: []PolicyAction{
				{Slug: "ca-legacy", Action: ActionUpdate},
				{Slug: "ca-mfa", Action: ActionCreate},
				{Slug: "rogue-policy", Action: ActionUntracked},
			},
		},
		{
			name:            "empty both: no backend, no live -> no actions",
			backend:         map[string]BackendPolicy{},
			live:            map[string]LivePolicy{},
			manifest:        newManifest(),
			expectedActions: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Reconcile(tt.backend, tt.live, tt.manifest)

			if len(got) != len(tt.expectedActions) {
				t.Fatalf("expected %d actions, got %d: %+v", len(tt.expectedActions), len(got), summarizeActions(got))
			}

			for i, exp := range tt.expectedActions {
				if got[i].Slug != exp.Slug {
					t.Errorf("action[%d] slug: expected %q, got %q", i, exp.Slug, got[i].Slug)
				}
				if got[i].Action != exp.Action {
					t.Errorf("action[%d] type: expected %v, got %v", i, exp.Action, got[i].Action)
				}
			}
		})
	}
}

func TestReconcileUpdateHasDiff(t *testing.T) {
	backend := map[string]BackendPolicy{
		"ca-mfa": {Data: map[string]interface{}{"displayName": "MFA Policy", "state": "enabled"}},
	}
	live := map[string]LivePolicy{
		"obj-111": {NormalizedData: map[string]interface{}{"displayName": "MFA Policy", "state": "disabled"}, Slug: "ca-mfa"},
	}
	manifest := newManifestWith("ca-mfa", "obj-111")

	actions := Reconcile(backend, live, manifest)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if len(actions[0].Diff) == 0 {
		t.Error("expected non-empty Diff for update action")
	}
}

// Helper to create an empty manifest.
func newManifest() *state.Manifest {
	return &state.Manifest{
		SchemaVersion: 1,
		Tenant:        "test-tenant",
		Policies:      make(map[string]state.Entry),
	}
}

// Helper to create a manifest with one tracked policy.
func newManifestWith(slug, liveObjectID string) *state.Manifest {
	m := newManifest()
	m.Policies[slug] = state.Entry{
		Slug:         slug,
		LiveObjectID: liveObjectID,
	}
	return m
}

func TestDetectDuplicates(t *testing.T) {
	live := map[string]LivePolicy{
		"id-1": {NormalizedData: map[string]interface{}{"displayName": "MFA Policy"}, Slug: "mfa-policy"},
		"id-2": {NormalizedData: map[string]interface{}{"displayName": "MFA Policy"}, Slug: "mfa-policy"},
		"id-3": {NormalizedData: map[string]interface{}{"displayName": "MFA Policy"}, Slug: "mfa-policy"},
		"id-4": {NormalizedData: map[string]interface{}{"displayName": "Unique Policy"}, Slug: "unique-policy"},
	}

	actions := DetectDuplicates(live)
	if len(actions) != 1 {
		t.Fatalf("expected 1 duplicate action, got %d", len(actions))
	}
	if actions[0].Action != ActionDuplicate {
		t.Errorf("expected ActionDuplicate, got %v", actions[0].Action)
	}
	if actions[0].DisplayName != "MFA Policy" {
		t.Errorf("expected displayName 'MFA Policy', got %q", actions[0].DisplayName)
	}
	if len(actions[0].DuplicateIDs) != 3 {
		t.Errorf("expected 3 duplicate IDs, got %d", len(actions[0].DuplicateIDs))
	}
}

func TestDetectDuplicatesNoDuplicates(t *testing.T) {
	live := map[string]LivePolicy{
		"id-1": {NormalizedData: map[string]interface{}{"displayName": "Policy A"}, Slug: "policy-a"},
		"id-2": {NormalizedData: map[string]interface{}{"displayName": "Policy B"}, Slug: "policy-b"},
	}

	actions := DetectDuplicates(live)
	if len(actions) != 0 {
		t.Fatalf("expected 0 duplicate actions, got %d", len(actions))
	}
}

func TestReconcileIncludesDuplicates(t *testing.T) {
	backend := map[string]BackendPolicy{}
	live := map[string]LivePolicy{
		"id-1": {NormalizedData: map[string]interface{}{"displayName": "Dup Policy"}, Slug: "dup-policy"},
		"id-2": {NormalizedData: map[string]interface{}{"displayName": "Dup Policy"}, Slug: "dup-policy"},
	}
	manifest := newManifest()

	actions := Reconcile(backend, live, manifest)

	// Should have 2 untracked + 1 duplicate = 3 actions
	var duplicateCount, untrackedCount int
	for _, a := range actions {
		switch a.Action {
		case ActionDuplicate:
			duplicateCount++
		case ActionUntracked:
			untrackedCount++
		case ActionNoop, ActionCreate, ActionUpdate, ActionRecreate:
			// not relevant for this test
		}
	}
	if duplicateCount != 1 {
		t.Errorf("expected 1 duplicate action, got %d", duplicateCount)
	}
	if untrackedCount != 2 {
		t.Errorf("expected 2 untracked actions, got %d", untrackedCount)
	}
}

// Helper for debug output.
func summarizeActions(actions []PolicyAction) []string {
	var s []string
	for _, a := range actions {
		s = append(s, a.Slug+"="+a.Action.String())
	}
	return s
}
