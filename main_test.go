package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExitErrorPrintsToStderr verifies that ExitError.Message is printed to
// stderr before os.Exit. This requires running the built binary because
// os.Exit cannot be intercepted in-process.
func TestExitErrorPrintsToStderr(t *testing.T) {
	// Build binary to a temp location
	binPath := filepath.Join(t.TempDir(), "cactl")
	build := exec.Command("go", "build", "-o", binPath, ".") //nolint:gosec // G204 - test binary
	build.Dir = projectRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", string(out))

	// Create a temp dir with .cactl already present to trigger ExitError
	workDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workDir, ".cactl"), 0755))

	// Run cactl init in the workspace that already has .cactl
	cmd := exec.Command(binPath, "init") //nolint:gosec // G204 - test binary
	cmd.Dir = workDir
	stderr, err := cmd.CombinedOutput()

	// Should fail with exit code 3
	require.Error(t, err)
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "expected ExitError, got %T", err)
	assert.Equal(t, 3, exitErr.ExitCode(), "expected exit code 3")

	// Stderr should contain the error message
	assert.Contains(t, string(stderr), "workspace already initialized")
	assert.Contains(t, string(stderr), "Error:")
}

// TestInvalidOutputPrintsToStderr verifies that --output invalid produces
// a validation error on stderr with exit code 3.
func TestInvalidOutputPrintsToStderr(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "cactl")
	build := exec.Command("go", "build", "-o", binPath, ".") //nolint:gosec // G204 - test binary
	build.Dir = projectRoot(t)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", string(out))

	workDir := t.TempDir()

	cmd := exec.Command(binPath, "--output", "invalid", "init") //nolint:gosec // G204 - test binary
	cmd.Dir = workDir
	stderr, err := cmd.CombinedOutput()

	require.Error(t, err)
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "expected ExitError, got %T", err)
	assert.Equal(t, 3, exitErr.ExitCode(), "expected exit code 3")

	assert.Contains(t, string(stderr), "invalid output format")
	assert.Contains(t, string(stderr), "Error:")
}

// projectRoot returns the module root directory.
func projectRoot(t *testing.T) string {
	t.Helper()
	// We're in the root already since main_test.go is at project root
	wd, err := os.Getwd()
	require.NoError(t, err)
	return wd
}
