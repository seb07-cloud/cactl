package state

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTempRepo creates a temporary git repo with user config for annotated tags.
func initTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "test"},
		{"git", "config", "user.email", "test@test.com"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "setup %v: %s", args, string(out))
	}
	return dir
}

func TestWriteAndReadPolicy(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	data := []byte(`{"displayName":"test-policy","state":"enabled"}`)
	_, err = b.WritePolicy("tenant-abc", "test-policy", data)
	require.NoError(t, err)

	got, err := b.ReadPolicy("tenant-abc", "test-policy")
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestWritePolicyCreatesRef(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	data := []byte(`{"displayName":"ref-check"}`)
	_, err = b.WritePolicy("tenant-123", "ref-check", data)
	require.NoError(t, err)

	cmd := exec.Command("git", "for-each-ref", "refs/cactl/tenants/tenant-123/policies/ref-check")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.NotEmpty(t, strings.TrimSpace(string(out)), "ref should exist")
}

func TestReadPolicyNotFound(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	_, err = b.ReadPolicy("tenant-abc", "nonexistent")
	assert.Error(t, err)
}

func TestListPolicies(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	slugs := []string{"policy-a", "policy-b", "policy-c"}
	for _, s := range slugs {
		_, err := b.WritePolicy("tenant-list", s, []byte(`{"slug":"`+s+`"}`))
		require.NoError(t, err)
	}

	got, err := b.ListPolicies("tenant-list")
	require.NoError(t, err)
	assert.ElementsMatch(t, slugs, got)
}

func TestListPoliciesEmpty(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	got, err := b.ListPolicies("tenant-empty")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestCreateVersionTag(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	data := []byte(`{"displayName":"tag-test"}`)
	blobHash, err := b.WritePolicy("tenant-tag", "tag-test", data)
	require.NoError(t, err)

	err = b.CreateVersionTag("tenant-tag", "tag-test", "1.0.0", blobHash, "cactl import: tag-test 1.0.0")
	require.NoError(t, err)

	// Verify tag exists
	tagName := "cactl/tenant-tag/tag-test/1.0.0"
	cmd := exec.Command("git", "tag", "-l", tagName)
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, tagName, strings.TrimSpace(string(out)))

	// Verify it is annotated (cat-file -t on the tag ref returns "tag")
	cmd2 := exec.Command("git", "cat-file", "-t", tagName)
	cmd2.Dir = dir
	out2, err := cmd2.Output()
	require.NoError(t, err)
	assert.Equal(t, "tag", strings.TrimSpace(string(out2)))
}

func TestOverwritePolicy(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	original := []byte(`{"version":"v1"}`)
	_, err = b.WritePolicy("tenant-ow", "overwrite-test", original)
	require.NoError(t, err)

	updated := []byte(`{"version":"v2"}`)
	_, err = b.WritePolicy("tenant-ow", "overwrite-test", updated)
	require.NoError(t, err)

	got, err := b.ReadPolicy("tenant-ow", "overwrite-test")
	require.NoError(t, err)
	assert.Equal(t, updated, got)
}

func TestNewGitBackendInvalidDir(t *testing.T) {
	_, err := NewGitBackend("/nonexistent/path")
	assert.Error(t, err)
}

func TestNewGitBackendNonGitDir(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)
	_, err := NewGitBackend(dir)
	assert.Error(t, err)
}
