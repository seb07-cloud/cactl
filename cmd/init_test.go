package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})
}

func TestInitHappyPath(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	err := runInit(nil, nil)
	require.NoError(t, err)

	// .cactl/config.yaml exists
	_, err = os.Stat(filepath.Join(cactlDir, configFileName))
	require.NoError(t, err, ".cactl/config.yaml should exist")

	// .cactl/schema.json exists
	_, err = os.Stat(filepath.Join(cactlDir, schemaFileName))
	require.NoError(t, err, ".cactl/schema.json should exist")

	// .gitignore contains the entry
	content, err := os.ReadFile(gitignoreFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), ".cactl/config.yaml")
}

func TestInitAlreadyInitialized(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// Pre-create .cactl directory
	require.NoError(t, os.MkdirAll(cactlDir, 0755))

	err := runInit(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace already initialized")
}

func TestInitGitignoreAppend(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// Pre-create .gitignore with other content
	existingContent := "node_modules/\n*.log\n"
	require.NoError(t, os.WriteFile(gitignoreFile, []byte(existingContent), 0644))

	err := runInit(nil, nil)
	require.NoError(t, err)

	content, err := os.ReadFile(gitignoreFile)
	require.NoError(t, err)
	s := string(content)

	// Original content preserved
	assert.Contains(t, s, "node_modules/")
	assert.Contains(t, s, "*.log")
	// New entry added
	assert.Contains(t, s, ".cactl/config.yaml")
}

func TestInitGitignoreIdempotent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// Pre-create .gitignore already containing the entry
	existingContent := "# cactl workspace\n.cactl/config.yaml\n"
	require.NoError(t, os.WriteFile(gitignoreFile, []byte(existingContent), 0644))

	err := runInit(nil, nil)
	require.NoError(t, err)

	content, err := os.ReadFile(gitignoreFile)
	require.NoError(t, err)

	// Entry should appear exactly once
	count := strings.Count(string(content), ".cactl/config.yaml")
	assert.Equal(t, 1, count, ".cactl/config.yaml should appear exactly once in .gitignore")
}

func TestInitConfigContent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	err := runInit(nil, nil)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(cactlDir, configFileName))
	require.NoError(t, err)

	// Parse as YAML
	var cfg map[string]interface{}
	require.NoError(t, yaml.Unmarshal(content, &cfg))

	// Verify expected keys exist
	assert.Contains(t, cfg, "tenant")
	assert.Contains(t, cfg, "auth")
	assert.Contains(t, cfg, "output")
	assert.Contains(t, cfg, "log_level")

	// Verify auth has mode key
	auth, ok := cfg["auth"].(map[string]interface{})
	require.True(t, ok, "auth should be a map")
	assert.Contains(t, auth, "mode")

	// Verify output default
	assert.Equal(t, "human", cfg["output"])
	assert.Equal(t, "info", cfg["log_level"])
}
