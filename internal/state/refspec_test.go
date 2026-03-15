package state

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigureRefspec(t *testing.T) {
	dir := initTempRepo(t)

	// Add a remote origin
	cmd := exec.Command("git", "remote", "add", "origin", "https://example.com/repo.git") //nolint:gosec // G204 - hardcoded binary
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	err := ConfigureRefspec(dir)
	require.NoError(t, err)

	// Verify fetch refspec
	fetchCmd := exec.Command("git", "config", "--get-all", "remote.origin.fetch") //nolint:gosec // G204 - hardcoded binary
	fetchCmd.Dir = dir
	out, err := fetchCmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "+refs/cactl/*:refs/cactl/*")

	// Verify push refspec
	pushCmd := exec.Command("git", "config", "--get-all", "remote.origin.push") //nolint:gosec // G204 - hardcoded binary
	pushCmd.Dir = dir
	out, err = pushCmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "+refs/cactl/*:refs/cactl/*")
}

func TestConfigureRefspecIdempotent(t *testing.T) {
	dir := initTempRepo(t)

	cmd := exec.Command("git", "remote", "add", "origin", "https://example.com/repo.git") //nolint:gosec // G204 - hardcoded binary
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	// Run twice
	require.NoError(t, ConfigureRefspec(dir))
	require.NoError(t, ConfigureRefspec(dir))

	// Count fetch refspec entries -- should be exactly 1
	fetchCmd := exec.Command("git", "config", "--get-all", "remote.origin.fetch") //nolint:gosec // G204 - hardcoded binary
	fetchCmd.Dir = dir
	out, err := fetchCmd.Output()
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	cactlCount := 0
	for _, line := range lines {
		if strings.Contains(line, "refs/cactl/*") {
			cactlCount++
		}
	}
	assert.Equal(t, 1, cactlCount, "should have exactly one cactl fetch refspec")

	// Count push refspec entries
	pushCmd := exec.Command("git", "config", "--get-all", "remote.origin.push") //nolint:gosec // G204 - hardcoded binary
	pushCmd.Dir = dir
	out, err = pushCmd.Output()
	require.NoError(t, err)
	lines = strings.Split(strings.TrimSpace(string(out)), "\n")
	cactlCount = 0
	for _, line := range lines {
		if strings.Contains(line, "refs/cactl/*") {
			cactlCount++
		}
	}
	assert.Equal(t, 1, cactlCount, "should have exactly one cactl push refspec")
}

func TestConfigureRefspecNoRemote(t *testing.T) {
	dir := initTempRepo(t)

	// No remote added -- should not error
	err := ConfigureRefspec(dir)
	assert.NoError(t, err)
}
