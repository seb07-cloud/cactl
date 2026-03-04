package state

import (
	"fmt"
	"os/exec"
	"strings"
)

const cactlRefspec = "+refs/cactl/*:refs/cactl/*"

// ConfigureRefspec adds cactl custom ref push/pull refspecs to .git/config.
// It is idempotent (no duplicates on repeated calls) and gracefully skips
// when no remote origin is configured.
func ConfigureRefspec(repoDir string) error {
	// Check if remote "origin" exists
	check := exec.Command("git", "config", "--get", "remote.origin.url")
	check.Dir = repoDir
	if err := check.Run(); err != nil {
		// No remote origin -- skip silently
		return nil
	}

	// Configure fetch refspec if not already present
	if err := addRefspecIfMissing(repoDir, "remote.origin.fetch"); err != nil {
		return fmt.Errorf("configuring fetch refspec: %w", err)
	}

	// Configure push refspec if not already present
	if err := addRefspecIfMissing(repoDir, "remote.origin.push"); err != nil {
		return fmt.Errorf("configuring push refspec: %w", err)
	}

	return nil
}

// addRefspecIfMissing checks if the cactl refspec already exists for the given
// config key and adds it if not present.
func addRefspecIfMissing(repoDir, configKey string) error {
	cmd := exec.Command("git", "config", "--get-all", configKey)
	cmd.Dir = repoDir
	out, _ := cmd.Output() // ignore error -- key may not exist yet

	if strings.Contains(string(out), "refs/cactl/*") {
		return nil // Already configured
	}

	add := exec.Command("git", "config", "--add", configKey, cactlRefspec)
	add.Dir = repoDir
	if out, err := add.CombinedOutput(); err != nil {
		return fmt.Errorf("git config --add %s: %s: %w", configKey, strings.TrimSpace(string(out)), err)
	}

	return nil
}
