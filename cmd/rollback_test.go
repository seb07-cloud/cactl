package cmd

import (
	"testing"
)

func TestRollbackCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "rollback" {
			found = true
			break
		}
	}
	if !found {
		t.Error("rollback command not registered on rootCmd")
	}
}

func TestRollbackCmd_Name(t *testing.T) {
	if rollbackCmd.Name() != "rollback" {
		t.Errorf("expected command name 'rollback', got %q", rollbackCmd.Name())
	}
}

func TestRollbackCmd_HasPolicyFlag(t *testing.T) {
	f := rollbackCmd.Flags().Lookup("policy")
	if f == nil {
		t.Fatal("rollback command should have --policy flag")
	}
	if f.DefValue != "" {
		t.Errorf("expected --policy default to be empty, got %q", f.DefValue)
	}
}

func TestRollbackCmd_HasVersionFlag(t *testing.T) {
	f := rollbackCmd.Flags().Lookup("version")
	if f == nil {
		t.Fatal("rollback command should have --version flag")
	}
	if f.DefValue != "" {
		t.Errorf("expected --version default to be empty, got %q", f.DefValue)
	}
}

func TestRollbackCmd_HasAutoApproveFlag(t *testing.T) {
	f := rollbackCmd.Flags().Lookup("auto-approve")
	if f == nil {
		t.Fatal("rollback command should have --auto-approve flag")
	}
	if f.DefValue != "false" {
		t.Errorf("expected --auto-approve default to be 'false', got %q", f.DefValue)
	}
}

func TestRollbackCmd_RequiresTenant(t *testing.T) {
	if rollbackCmd.RunE == nil {
		t.Error("rollbackCmd should have RunE set")
	}
}

func TestBumpPatchVersion_Basic(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.0.0", "1.0.1"},
		{"2.3.5", "2.3.6"},
		{"0.0.0", "0.0.1"},
		{"1.2.99", "1.2.100"},
	}

	for _, tc := range tests {
		got := bumpPatchVersion(tc.input)
		if got != tc.expected {
			t.Errorf("bumpPatchVersion(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestBumpPatchVersion_InvalidFallback(t *testing.T) {
	got := bumpPatchVersion("invalid")
	if got != "1.0.1" {
		t.Errorf("bumpPatchVersion(\"invalid\") = %q, want \"1.0.1\"", got)
	}
}

func TestRollbackCmd_RequiresPolicyFlag(t *testing.T) {
	// Verify the flag exists and is required by convention (empty default)
	f := rollbackCmd.Flags().Lookup("policy")
	if f == nil {
		t.Fatal("--policy flag must exist")
	}
	if f.DefValue != "" {
		t.Error("--policy should have empty default (required by validation)")
	}
}

func TestRollbackCmd_RequiresVersionFlag(t *testing.T) {
	// Verify the flag exists and is required by convention (empty default)
	f := rollbackCmd.Flags().Lookup("version")
	if f == nil {
		t.Fatal("--version flag must exist")
	}
	if f.DefValue != "" {
		t.Error("--version should have empty default (required by validation)")
	}
}
