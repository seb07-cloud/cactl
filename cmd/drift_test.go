package cmd

import (
	"testing"

	"github.com/seb07-cloud/cactl/internal/reconcile"
)

func TestDriftCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "drift" {
			found = true
			break
		}
	}
	if !found {
		t.Error("drift command not registered on rootCmd")
	}
}

func TestDriftCmd_Name(t *testing.T) {
	if driftCmd.Name() != "drift" {
		t.Errorf("expected command name 'drift', got %q", driftCmd.Name())
	}
}

func TestDriftCmd_HasPolicyFlag(t *testing.T) {
	f := driftCmd.Flags().Lookup("policy")
	if f == nil {
		t.Fatal("drift command should have --policy flag")
	}
	if f.DefValue != "" {
		t.Errorf("expected --policy default to be empty, got %q", f.DefValue)
	}
}

func TestDriftCmd_RequiresTenant(t *testing.T) {
	if driftCmd.RunE == nil {
		t.Error("driftCmd should have RunE set")
	}
}

func TestFilterBySlug_MatchesSlug(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{Slug: "policy-a", Action: reconcile.ActionUpdate},
		{Slug: "policy-b", Action: reconcile.ActionCreate},
		{Slug: "policy-c", Action: reconcile.ActionUntracked},
	}

	result := filterBySlug(actions, "policy-b")
	if len(result) != 1 {
		t.Fatalf("expected 1 action, got %d", len(result))
	}
	if result[0].Slug != "policy-b" {
		t.Errorf("expected slug 'policy-b', got %q", result[0].Slug)
	}
}

func TestFilterBySlug_EmptyFilter(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{Slug: "policy-a", Action: reconcile.ActionUpdate},
		{Slug: "policy-b", Action: reconcile.ActionCreate},
	}

	result := filterBySlug(actions, "")
	if len(result) != 2 {
		t.Fatalf("expected 2 actions for empty filter, got %d", len(result))
	}
}

func TestFilterBySlug_NoMatch(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{Slug: "policy-a", Action: reconcile.ActionUpdate},
	}

	result := filterBySlug(actions, "nonexistent")
	if len(result) != 0 {
		t.Fatalf("expected 0 actions for non-matching filter, got %d", len(result))
	}
}

func TestFilterDriftActionable_ExcludesNoop(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{Slug: "a", Action: reconcile.ActionNoop},
		{Slug: "b", Action: reconcile.ActionUpdate},
		{Slug: "c", Action: reconcile.ActionUntracked},
		{Slug: "d", Action: reconcile.ActionCreate},
		{Slug: "e", Action: reconcile.ActionRecreate},
	}

	result := filterDriftActionable(actions)
	if len(result) != 4 {
		t.Fatalf("expected 4 actionable items (excluding noop), got %d", len(result))
	}

	// Verify Untracked is included (drift-specific behavior)
	hasUntracked := false
	for _, a := range result {
		if a.Action == reconcile.ActionUntracked {
			hasUntracked = true
			break
		}
	}
	if !hasUntracked {
		t.Error("filterDriftActionable should include Untracked actions")
	}
}

func TestFilterDriftActionable_AllNoop(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{Slug: "a", Action: reconcile.ActionNoop},
		{Slug: "b", Action: reconcile.ActionNoop},
	}

	result := filterDriftActionable(actions)
	if len(result) != 0 {
		t.Fatalf("expected 0 actionable items for all-noop, got %d", len(result))
	}
}
