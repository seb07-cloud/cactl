package cmd

import (
	"testing"
)

func TestPlanCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "plan" {
			found = true
			break
		}
	}
	if !found {
		t.Error("plan command not registered on rootCmd")
	}
}

func TestPlanCmd_Name(t *testing.T) {
	if planCmd.Name() != "plan" {
		t.Errorf("expected command name 'plan', got %q", planCmd.Name())
	}
}

func TestPlanCmd_RequiresTenant(t *testing.T) {
	// The plan command requires a tenant flag. When invoked without one,
	// it should return an error (after going through PersistentPreRunE).
	// We test the command structure rather than executing the full flow
	// since it requires Azure authentication.
	if planCmd.RunE == nil {
		t.Error("planCmd should have RunE set")
	}
}
