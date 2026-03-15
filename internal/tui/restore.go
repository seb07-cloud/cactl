package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/seb07-cloud/cactl/internal/output"
	"github.com/seb07-cloud/cactl/internal/reconcile"
	"github.com/seb07-cloud/cactl/internal/state"
)

// RestoreConfig holds dependencies for the interactive restore wizard.
// Function fields avoid circular dependencies between tui and cmd packages.
type RestoreConfig struct {
	Backend  *state.GitBackend
	Tenant   string
	Manifest *state.Manifest
	UseColor bool
	RepoDir  string

	// WritePolicyFile writes historical JSON to the on-disk desired state file.
	WritePolicyFile func(tenant, slug string, data []byte) error

	// ReadDesiredPolicies returns slug -> data map for all desired state files.
	ReadDesiredPolicies func(tenant string) (map[string]map[string]interface{}, error)

	// RunPlan executes cactl plan programmatically after restore.
	RunPlan func(ctx context.Context) error
}

// RunInteractiveRestore runs the full interactive history browser and restore wizard.
// Flow: select policy -> select version (with diff summaries) -> view diff -> restore/back/quit.
func RunInteractiveRestore(ctx context.Context, cfg RestoreConfig) error {
	// Build sorted slug list from manifest
	slugs := make([]string, 0, len(cfg.Manifest.Policies))
	for slug := range cfg.Manifest.Policies {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	if len(slugs) == 0 {
		fmt.Fprintln(os.Stdout, "No tracked policies found. Run 'cactl import' first.")
		return nil
	}

	// Policy selection loop (outer) with version selection loop (inner)
	for {
		// Step 1: Select policy (Esc or "Quit" exits)
		selectedSlug, err := SelectPolicy(slugs)
		if err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			return fmt.Errorf("selecting policy: %w", err)
		}
		if selectedSlug == "" {
			return nil
		}

		// Step 2: Load version history
		tags, err := cfg.Backend.ListVersionTags(cfg.Tenant, selectedSlug)
		if err != nil {
			return fmt.Errorf("listing versions for %s: %w", selectedSlug, err)
		}
		if len(tags) == 0 {
			fmt.Fprintf(os.Stdout, "No version history for '%s'.\n", selectedSlug)
			continue // back to policy selection
		}

		// Step 3: Compute diff summaries for each version
		summaries := computeDiffSummaries(cfg.Backend, cfg.Tenant, selectedSlug, tags)

		// Version selection loop (supports back-navigation)
		backToPolicies := false
		for {
			// Step 4: Select version (returns "" for "Back to policies", Esc also goes back)
			selectedVersion, err := SelectVersion(tags, summaries)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					backToPolicies = true
					break
				}
				return fmt.Errorf("selecting version: %w", err)
			}
			if selectedVersion == "" {
				backToPolicies = true
				break
			}

			// Step 5: Read historical JSON
			historicalJSON, err := cfg.Backend.ReadTagBlob(cfg.Tenant, selectedSlug, selectedVersion)
			if err != nil {
				return fmt.Errorf("reading version %s: %w", selectedVersion, err)
			}

			// Step 6: Read current desired state
			desiredPolicies, err := cfg.ReadDesiredPolicies(cfg.Tenant)
			if err != nil {
				return fmt.Errorf("reading desired state: %w", err)
			}

			var historicalMap map[string]interface{}
			if err := json.Unmarshal(historicalJSON, &historicalMap); err != nil {
				return fmt.Errorf("parsing historical version: %w", err)
			}

			currentDesiredMap := desiredPolicies[selectedSlug]

			// Step 7: Compute diff (historical is "desired" = what we want, current is "actual" = what we have)
			diffs := reconcile.ComputeDiff(historicalMap, currentDesiredMap)

			if len(diffs) == 0 {
				fmt.Fprintf(os.Stdout, "Selected version %s matches current desired state.\n", selectedVersion)
				continue // back to version selection
			}

			// Step 8: Render diffs
			fmt.Fprintf(os.Stdout, "\nDiff: %s version %s vs current desired state\n\n", selectedSlug, selectedVersion)
			output.RenderFieldDiffs(os.Stdout, diffs, cfg.UseColor)
			fmt.Fprintln(os.Stdout)

			// Step 9: Action selection
			action, err := SelectAction()
			if err != nil {
				return fmt.Errorf("selecting action: %w", err)
			}

			switch action {
			case "back":
				continue // back to version selection
			case "quit":
				return nil
			case "restore":
				return performRestore(ctx, cfg, selectedSlug, selectedVersion, historicalJSON)
			}
		}
		if backToPolicies {
			continue
		}
	}
}

// performRestore handles the file write, auto-commit, and auto-plan steps.
func performRestore(ctx context.Context, cfg RestoreConfig, slug, version string, historicalJSON []byte) error {
	filePath := fmt.Sprintf("policies/%s/%s.json", cfg.Tenant, slug)

	// Check for uncommitted changes
	dirty, err := hasUncommittedChanges(cfg.RepoDir, filePath)
	if err != nil {
		// Non-fatal: warn but continue
		fmt.Fprintf(os.Stderr, "Warning: could not check git status for %s: %v\n", filePath, err)
	}
	if dirty {
		confirmed, err := ConfirmOverwrite(filePath)
		if err != nil {
			return fmt.Errorf("confirmation prompt: %w", err)
		}
		if !confirmed {
			fmt.Fprintln(os.Stdout, "Restore cancelled.")
			return nil
		}
	}

	// Write historical JSON to desired state file
	if err := cfg.WritePolicyFile(cfg.Tenant, slug, historicalJSON); err != nil {
		return fmt.Errorf("writing policy file: %w", err)
	}

	// Auto-commit
	commitMsg := fmt.Sprintf("restore: %s to v%s", slug, version)
	if err := autoCommit(cfg.RepoDir, filePath, commitMsg); err != nil {
		return fmt.Errorf("auto-commit: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Restored %s to version %s and committed.\n\n", slug, version)

	// Auto-run plan
	fmt.Fprintf(os.Stdout, "Running plan to show what would change in Entra...\n\n")
	if err := cfg.RunPlan(ctx); err != nil {
		// Plan errors are non-fatal for the restore flow
		// (plan returns exit code 1 when there are changes, which is expected)
		fmt.Fprintf(os.Stderr, "Plan completed with: %v\n", err)
	}

	return nil
}

// computeDiffSummaries computes a diff summary string for each version tag
// by comparing each version to its predecessor.
func computeDiffSummaries(backend *state.GitBackend, tenant, slug string, tags []state.VersionTag) []string {
	summaries := make([]string, len(tags))

	for i := range tags {
		if i == len(tags)-1 {
			// Oldest version (tags are sorted descending)
			summaries[i] = "initial version"
			continue
		}

		// Read current version blob
		currentJSON, err := backend.ReadTagBlob(tenant, slug, tags[i].Version)
		if err != nil {
			summaries[i] = "error reading version"
			continue
		}

		// Read previous version blob (next in array since descending order)
		prevJSON, err := backend.ReadTagBlob(tenant, slug, tags[i+1].Version)
		if err != nil {
			summaries[i] = "error reading previous version"
			continue
		}

		var currentMap, prevMap map[string]interface{}
		if err := json.Unmarshal(currentJSON, &currentMap); err != nil {
			summaries[i] = "error parsing version"
			continue
		}
		if err := json.Unmarshal(prevJSON, &prevMap); err != nil {
			summaries[i] = "error parsing previous version"
			continue
		}

		diffs := reconcile.ComputeDiff(currentMap, prevMap)
		summaries[i] = output.DiffSummary(diffs)
	}

	return summaries
}

// hasUncommittedChanges checks if a file has uncommitted local changes.
func hasUncommittedChanges(repoDir, filePath string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain", filePath)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// autoCommit stages and commits a file with the given message.
func autoCommit(repoDir, filePath, message string) error {
	add := exec.Command("git", "add", filePath)
	add.Dir = repoDir
	if out, err := add.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s: %w", strings.TrimSpace(string(out)), err)
	}
	commit := exec.Command("git", "commit", "-m", message)
	commit.Dir = repoDir
	if out, err := commit.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

