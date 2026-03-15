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

	actualVersion, err := b.CreateVersionTag("tenant-tag", "tag-test", "1.0.0", blobHash, "cactl import: tag-test 1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", actualVersion)

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

func TestListVersionTags(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	data := []byte(`{"displayName":"ca-mfa"}`)
	blobHash, err := b.WritePolicy("test-tenant", "ca-mfa", data)
	require.NoError(t, err)

	// Create 3 annotated tags
	versions := []string{"1.0.0", "1.1.0", "2.0.0"}
	for _, v := range versions {
		_, err := b.CreateVersionTag("test-tenant", "ca-mfa", v, blobHash, "cactl: ca-mfa "+v)
		require.NoError(t, err)
	}

	tags, err := b.ListVersionTags("test-tenant", "ca-mfa")
	require.NoError(t, err)
	require.Len(t, tags, 3)

	// Should be sorted by semver descending (2.0.0 first)
	assert.Equal(t, "2.0.0", tags[0].Version)
	assert.Equal(t, "1.1.0", tags[1].Version)
	assert.Equal(t, "1.0.0", tags[2].Version)

	// Each should have non-empty fields
	for _, tag := range tags {
		assert.NotEmpty(t, tag.Version)
		assert.NotEmpty(t, tag.Timestamp)
		assert.NotEmpty(t, tag.Message)
	}
}

func TestListVersionTags_Empty(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	tags, err := b.ListVersionTags("test-tenant", "no-such-slug")
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestListVersionTags_OtherSlugs(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	data := []byte(`{"displayName":"test"}`)
	blobHash, err := b.WritePolicy("test-tenant", "ca-mfa", data)
	require.NoError(t, err)
	blobHash2, err := b.WritePolicy("test-tenant", "ca-block", data)
	require.NoError(t, err)

	_, err = b.CreateVersionTag("test-tenant", "ca-mfa", "1.0.0", blobHash, "mfa v1")
	require.NoError(t, err)
	_, err = b.CreateVersionTag("test-tenant", "ca-mfa", "2.0.0", blobHash, "mfa v2")
	require.NoError(t, err)
	_, err = b.CreateVersionTag("test-tenant", "ca-block", "1.0.0", blobHash2, "block v1")
	require.NoError(t, err)

	tags, err := b.ListVersionTags("test-tenant", "ca-mfa")
	require.NoError(t, err)
	require.Len(t, tags, 2)
	assert.Equal(t, "2.0.0", tags[0].Version)
	assert.Equal(t, "1.0.0", tags[1].Version)
}

func TestReadTagBlob(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	data := []byte(`{"displayName":"test"}`)
	blobHash, err := b.WritePolicy("test-tenant", "ca-mfa", data)
	require.NoError(t, err)

	_, err = b.CreateVersionTag("test-tenant", "ca-mfa", "1.0.0", blobHash, "initial import")
	require.NoError(t, err)

	got, err := b.ReadTagBlob("test-tenant", "ca-mfa", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestReadTagBlob_NotFound(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	_, err = b.ReadTagBlob("test-tenant", "ca-mfa", "9.9.9")
	assert.Error(t, err)
}

func TestHashObject(t *testing.T) {
	dir := initTempRepo(t)
	b, err := NewGitBackend(dir)
	require.NoError(t, err)

	data := []byte(`{"displayName":"hash-test","state":"enabled"}`)

	hash, err := b.HashObject(data)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 40, "SHA-1 hash should be 40 hex characters")

	// Verify it matches what WritePolicy returns for the same content
	writeHash, err := b.WritePolicy("test-tenant", "hash-test", data)
	require.NoError(t, err)
	assert.Equal(t, writeHash, hash, "HashObject should return same SHA as WritePolicy for identical content")
}
