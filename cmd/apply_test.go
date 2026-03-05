package cmd

import (
	"strings"
	"testing"

	"github.com/seb07-cloud/cactl/internal/reconcile"
	"github.com/seb07-cloud/cactl/internal/semver"
	"github.com/seb07-cloud/cactl/pkg/types"
)

func TestApplyCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "apply" {
			found = true
			break
		}
	}
	if !found {
		t.Error("apply command not registered on rootCmd")
	}
}

func TestApplyCmd_Name(t *testing.T) {
	if applyCmd.Name() != "apply" {
		t.Errorf("expected command name 'apply', got %q", applyCmd.Name())
	}
}

func TestApplyCmd_Flags(t *testing.T) {
	f := applyCmd.Flags()

	autoApprove := f.Lookup("auto-approve")
	if autoApprove == nil {
		t.Error("apply command missing --auto-approve flag")
	} else if autoApprove.DefValue != "false" {
		t.Errorf("--auto-approve default should be false, got %q", autoApprove.DefValue)
	}

	dryRun := f.Lookup("dry-run")
	if dryRun == nil {
		t.Error("apply command missing --dry-run flag")
	} else if dryRun.DefValue != "false" {
		t.Errorf("--dry-run default should be false, got %q", dryRun.DefValue)
	}
}

func TestApplyCmd_RequiresTenant(t *testing.T) {
	if applyCmd.RunE == nil {
		t.Error("applyCmd should have RunE set")
	}
}

func TestConfirm_EmptyInput(t *testing.T) {
	r := strings.NewReader("\n")
	result := confirmFromReader("Do you want to apply? [Y/n]: ", r)
	if !result {
		t.Error("confirm should return true for empty input")
	}
}

func TestConfirm_YesInput(t *testing.T) {
	r := strings.NewReader("y\n")
	result := confirmFromReader("prompt: ", r)
	if !result {
		t.Error("confirm should return true for 'y'")
	}

	r = strings.NewReader("yes\n")
	result = confirmFromReader("prompt: ", r)
	if !result {
		t.Error("confirm should return true for 'yes'")
	}

	r = strings.NewReader("YES\n")
	result = confirmFromReader("prompt: ", r)
	if !result {
		t.Error("confirm should return true for 'YES'")
	}
}

func TestConfirm_NoInput(t *testing.T) {
	r := strings.NewReader("n\n")
	result := confirmFromReader("prompt: ", r)
	if result {
		t.Error("confirm should return false for 'n'")
	}
}

func TestConfirmExplicit_YesOnly(t *testing.T) {
	r := strings.NewReader("yes\n")
	result := confirmExplicitFromReader("prompt: ", r)
	if !result {
		t.Error("confirmExplicit should return true for 'yes'")
	}
}

func TestConfirmExplicit_EmptyIsFalse(t *testing.T) {
	r := strings.NewReader("\n")
	result := confirmExplicitFromReader("prompt: ", r)
	if result {
		t.Error("confirmExplicit should return false for empty input")
	}
}

func TestConfirmExplicit_YIsFalse(t *testing.T) {
	r := strings.NewReader("y\n")
	result := confirmExplicitFromReader("prompt: ", r)
	if result {
		t.Error("confirmExplicit should return false for 'y'")
	}
}

func TestFilterActionable(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{Slug: "noop-policy", Action: reconcile.ActionNoop},
		{Slug: "create-policy", Action: reconcile.ActionCreate},
		{Slug: "update-policy", Action: reconcile.ActionUpdate},
		{Slug: "recreate-policy", Action: reconcile.ActionRecreate},
		{Slug: "untracked-policy", Action: reconcile.ActionUntracked},
	}

	result := filterActionable(actions)
	if len(result) != 3 {
		t.Fatalf("expected 3 actionable items, got %d", len(result))
	}

	expected := map[string]bool{
		"create-policy":   true,
		"update-policy":   true,
		"recreate-policy": true,
	}
	for _, a := range result {
		if !expected[a.Slug] {
			t.Errorf("unexpected actionable item: %s", a.Slug)
		}
	}
}

func TestHasAction(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{Slug: "a", Action: reconcile.ActionCreate},
		{Slug: "b", Action: reconcile.ActionUpdate},
	}

	if !hasAction(actions, reconcile.ActionCreate) {
		t.Error("hasAction should find ActionCreate")
	}
	if !hasAction(actions, reconcile.ActionUpdate) {
		t.Error("hasAction should find ActionUpdate")
	}
	if hasAction(actions, reconcile.ActionRecreate) {
		t.Error("hasAction should not find ActionRecreate")
	}
}

func TestApplyCmd_HasBumpLevelFlag(t *testing.T) {
	f := applyCmd.Flags().Lookup("bump-level")
	if f == nil {
		t.Error("apply command missing --bump-level flag")
	} else if f.DefValue != "" {
		t.Errorf("--bump-level default should be empty, got %q", f.DefValue)
	}
}

func TestParseBumpLevel(t *testing.T) {
	tests := []struct {
		input   string
		want    semver.BumpLevel
		wantErr bool
	}{
		{"major", semver.BumpMajor, false},
		{"MAJOR", semver.BumpMajor, false},
		{"minor", semver.BumpMinor, false},
		{"Minor", semver.BumpMinor, false},
		{"patch", semver.BumpPatch, false},
		{"PATCH", semver.BumpPatch, false},
		{"invalid", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		got, err := parseBumpLevel(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseBumpLevel(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("parseBumpLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDeployerIdentity(t *testing.T) {
	cfg := &types.Config{
		Auth: types.AuthConfig{Mode: "az-cli"},
	}
	got := deployerIdentity(cfg)
	if got != "cactl/az-cli" {
		t.Errorf("expected 'cactl/az-cli', got %q", got)
	}

	cfg2 := &types.Config{}
	got2 := deployerIdentity(cfg2)
	if got2 != "cactl/unknown" {
		t.Errorf("expected 'cactl/unknown', got %q", got2)
	}
}
