package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/seb07-cloud/cactl/internal/output"
	"github.com/seb07-cloud/cactl/internal/schema"
	"github.com/seb07-cloud/cactl/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	cactlDir       = ".cactl"
	configFileName = "config.yaml"
	schemaFileName = "schema.json"
	gitignoreFile  = ".gitignore"
)

var configFilePath = cactlDir + "/" + configFileName

// defaultConfig is the template for .cactl/config.yaml.
// No secrets are stored here; ClientID, ClientSecret, CertPath come from env vars only.
const defaultConfig = `# cactl configuration
# All values can be overridden by CACTL_* environment variables or CLI flags
# See: https://github.com/seb07-cloud/cactl
tenant: ""
auth:
  mode: ""  # az-cli | client-secret | client-certificate (auto-detected if empty)
output: human  # human | json
log_level: info
`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a cactl workspace",
	Long:  "Initialize a new cactl workspace in the current directory.\nCreates .cactl/ with config.yaml, schema.json, and updates .gitignore.",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	v := viper.GetViper()
	format := v.GetString("output")
	useColor := output.ShouldUseColor(v)
	r := output.NewRenderer(format, useColor)

	// Step 1: Check if .cactl directory already exists.
	if _, err := os.Stat(cactlDir); err == nil {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: "workspace already initialized (.cactl directory exists)",
		}
	}

	// Step 2: Check if .cactl/config.yaml is tracked by Git (CONF-03).
	if isGitTracked(configFilePath) {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: ".cactl/config.yaml is tracked by Git -- remove it first: git rm --cached .cactl/config.yaml",
		}
	}

	// Step 3: Create .cactl directory.
	if err := os.MkdirAll(cactlDir, 0750); err != nil { //nolint:gosec // G301 - workspace directory
		return fmt.Errorf("creating %s directory: %w", cactlDir, err)
	}

	// Step 4: Update .gitignore BEFORE creating config.yaml (CONF-03 order).
	if err := ensureGitignore(); err != nil {
		// Clean up .cactl dir on failure
		_ = os.RemoveAll(cactlDir)
		return fmt.Errorf("updating .gitignore: %w", err)
	}

	// Step 5: Write .cactl/config.yaml with default content.
	cfgPath := cactlDir + "/" + configFileName
	if err := os.WriteFile(cfgPath, []byte(defaultConfig), 0600); err != nil {
		_ = os.RemoveAll(cactlDir)
		return fmt.Errorf("writing config file: %w", err)
	}

	// Step 6: Fetch CA policy JSON Schema (CONF-04).
	schemaPath := cactlDir + "/" + schemaFileName
	usedFallback, err := schema.FetchOrFallback(schemaPath)
	if err != nil {
		// Even embedded write failed -- this is fatal
		_ = os.RemoveAll(cactlDir)
		return fmt.Errorf("writing schema: %w", err)
	}
	if usedFallback {
		r.Warn("Could not fetch CA policy schema from network; using embedded fallback")
	}

	// Step 7: Output success.
	r.Success("Workspace initialized in .cactl/")
	r.Info("Created " + cfgPath)
	r.Info("Created " + schemaPath)
	r.Info("Updated " + gitignoreFile)

	return nil
}

// isGitTracked returns true if the given path is tracked by Git.
// Returns false if git is not available or the file is not tracked.
func isGitTracked(path string) bool {
	cmd := exec.Command("git", "ls-files", "--error-unmatch", path) //nolint:gosec // G204 - hardcoded binary
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()
	// Exit code 0 means the file IS tracked
	return err == nil
}

// ensureGitignore ensures .gitignore contains the .cactl/config.yaml entry.
func ensureGitignore() error {
	const entry = ".cactl/config.yaml"
	const header = "# cactl workspace"

	// Read existing .gitignore if it exists
	existing, err := os.ReadFile(gitignoreFile) //nolint:gosec // G304 - path from workspace config
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading .gitignore: %w", err)
	}

	// Check if entry already present
	if len(existing) > 0 {
		scanner := bufio.NewScanner(strings.NewReader(string(existing)))
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == entry {
				// Already present, nothing to do
				return nil
			}
		}

		// Append to existing file
		content := string(existing)
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n" + header + "\n" + entry + "\n"
		return os.WriteFile(gitignoreFile, []byte(content), 0644) //nolint:gosec // G306 - not sensitive
	}

	// Create new .gitignore
	content := header + "\n" + entry + "\n"
	return os.WriteFile(gitignoreFile, []byte(content), 0644) //nolint:gosec // G306 - not sensitive
}
