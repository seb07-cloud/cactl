package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/seb07-cloud/cactl/internal/config"
	"github.com/seb07-cloud/cactl/internal/output"
	"github.com/seb07-cloud/cactl/internal/reconcile"
	"github.com/seb07-cloud/cactl/internal/state"
	"github.com/seb07-cloud/cactl/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// historyEntry is the JSON structure for version history entries.
// Used by both history and status --history commands.
type historyEntry struct {
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
	Changes   string `json:"changes,omitempty"`
}

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View version history for tracked policies",
	Long: `Display version history for tracked Conditional Access policies.
Without --policy, shows all tracked policies with version counts.
With --policy, shows full version timeline with diff summaries.`,
	RunE: runHistory,
}

func init() {
	rootCmd.AddCommand(historyCmd)
	historyCmd.Flags().String("policy", "", "Policy slug to show detailed history for")
	historyCmd.Flags().Bool("json", false, "Output in JSON format")
}

func runHistory(cmd *cobra.Command, args []string) error {
	// 1. Load config
	cfg, err := config.LoadFromGlobal()
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("loading config: %v", err),
		}
	}

	// 2. Validate tenant
	if cfg.Tenant == "" {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: "tenant is required: use --tenant, set CACTL_TENANT, or log in with az login",
		}
	}

	v := viper.GetViper()
	useColor := output.ShouldUseColor(v)

	// 3. Create git backend
	backend, err := state.NewGitBackend(".")
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("git backend: %v", err),
		}
	}

	// 4. Load manifest
	manifest, err := state.ReadManifest(backend, cfg.Tenant)
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("reading manifest: %v", err),
		}
	}

	jsonFlag, _ := cmd.Flags().GetBool("json")
	policySlug, _ := cmd.Flags().GetString("policy")

	if policySlug == "" {
		return runHistoryListAll(backend, cfg, manifest, jsonFlag, useColor)
	}
	return runHistorySinglePolicy(backend, cfg, manifest, policySlug, jsonFlag, useColor)
}

// runHistoryListAll shows all tracked policies with version counts.
func runHistoryListAll(backend *state.GitBackend, cfg *types.Config, manifest *state.Manifest, jsonOutput bool, useColor bool) error {
	if len(manifest.Policies) == 0 {
		fmt.Fprintln(os.Stdout, "No tracked policies. Run 'cactl import' to get started.")
		return nil
	}

	type policySummary struct {
		Slug           string `json:"slug"`
		VersionCount   int    `json:"version_count"`
		CurrentVersion string `json:"current_version"`
		LastDeployed   string `json:"last_deployed"`
	}

	// Collect and sort by slug
	slugs := make([]string, 0, len(manifest.Policies))
	for slug := range manifest.Policies {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	summaries := make([]policySummary, 0, len(slugs))
	for _, slug := range slugs {
		entry := manifest.Policies[slug]
		tags, err := backend.ListVersionTags(cfg.Tenant, slug)
		if err != nil {
			// Graceful: show 0 versions if tag listing fails
			tags = nil
		}
		summaries = append(summaries, policySummary{
			Slug:           slug,
			VersionCount:   len(tags),
			CurrentVersion: entry.Version,
			LastDeployed:   entry.LastDeployed,
		})
	}

	if jsonOutput {
		data, err := json.MarshalIndent(summaries, "", "  ")
		if err != nil {
			return &types.ExitError{
				Code:    types.ExitFatalError,
				Message: fmt.Sprintf("rendering JSON: %v", err),
			}
		}
		fmt.Fprintln(os.Stdout, string(data))
	} else {
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "POLICY\tVERSIONS\tCURRENT VERSION\tLAST DEPLOYED")
		for _, s := range summaries {
			fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", s.Slug, s.VersionCount, s.CurrentVersion, s.LastDeployed)
		}
		tw.Flush()
	}

	return nil
}

// runHistorySinglePolicy shows full version timeline with diff summaries for a single policy.
func runHistorySinglePolicy(backend *state.GitBackend, cfg *types.Config, manifest *state.Manifest, slug string, jsonOutput bool, useColor bool) error {
	// Validate slug exists in manifest
	if _, exists := manifest.Policies[slug]; !exists {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("policy '%s' is not tracked -- run 'cactl import' first", slug),
		}
	}

	tags, err := backend.ListVersionTags(cfg.Tenant, slug)
	if err != nil {
		return &types.ExitError{
			Code:    types.ExitFatalError,
			Message: fmt.Sprintf("listing version tags: %v", err),
		}
	}

	if len(tags) == 0 {
		fmt.Fprintf(os.Stdout, "No version history for policy '%s'.\n", slug)
		return nil
	}

	// Compute diff summaries for each version
	diffSummaries := computeDiffSummaries(backend, cfg.Tenant, slug, tags)

	if jsonOutput {
		entries := make([]historyEntry, len(tags))
		for i, t := range tags {
			entries[i] = historyEntry{
				Version:   t.Version,
				Timestamp: t.Timestamp,
				Message:   t.Message,
				Changes:   diffSummaries[i],
			}
		}
		histOutput := struct {
			SchemaVersion int            `json:"schema_version"`
			Slug          string         `json:"slug"`
			History       []historyEntry `json:"history"`
		}{
			SchemaVersion: 1,
			Slug:          slug,
			History:       entries,
		}
		data, err := json.MarshalIndent(histOutput, "", "  ")
		if err != nil {
			return &types.ExitError{
				Code:    types.ExitFatalError,
				Message: fmt.Sprintf("rendering JSON: %v", err),
			}
		}
		fmt.Fprintln(os.Stdout, string(data))
	} else {
		fmt.Fprintf(os.Stdout, "Version history for %s:\n\n", slug)
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "VERSION\tDATE\tCHANGES")
		for i, t := range tags {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", t.Version, t.Timestamp, diffSummaries[i])
		}
		tw.Flush()
	}

	return nil
}

// computeDiffSummaries returns a string slice parallel to tags with diff summary for each version.
// For the oldest version (last in the slice, since tags are sorted newest first), returns "initial import".
// For each other version, reads both blobs, computes diff, and builds a summary string.
func computeDiffSummaries(backend *state.GitBackend, tenant, slug string, tags []state.VersionTag) []string {
	summaries := make([]string, len(tags))
	if len(tags) == 0 {
		return summaries
	}

	// Tags are sorted newest first. The oldest is at the end.
	summaries[len(tags)-1] = "initial import"

	for i := 0; i < len(tags)-1; i++ {
		currentVersion := tags[i].Version
		previousVersion := tags[i+1].Version

		currentBlob, err := backend.ReadTagBlob(tenant, slug, currentVersion)
		if err != nil {
			summaries[i] = "unknown"
			continue
		}
		previousBlob, err := backend.ReadTagBlob(tenant, slug, previousVersion)
		if err != nil {
			summaries[i] = "unknown"
			continue
		}

		// Unmarshal both to maps
		var currentMap, previousMap map[string]interface{}
		if err := json.Unmarshal(currentBlob, &currentMap); err != nil {
			summaries[i] = "unknown"
			continue
		}
		if err := json.Unmarshal(previousBlob, &previousMap); err != nil {
			summaries[i] = "unknown"
			continue
		}

		// ComputeDiff: current is desired (newer), previous is actual (older)
		diffs := reconcile.ComputeDiff(currentMap, previousMap)
		summaries[i] = output.DiffSummary(diffs)
	}

	return summaries
}
