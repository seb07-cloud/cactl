package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/seb07-cloud/cactl/internal/auth"
	"github.com/seb07-cloud/cactl/internal/config"
	"github.com/seb07-cloud/cactl/internal/graph"
	"github.com/seb07-cloud/cactl/internal/normalize"
	"github.com/seb07-cloud/cactl/internal/output"
	"github.com/seb07-cloud/cactl/internal/state"
	"github.com/seb07-cloud/cactl/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show tracked policies with sync status",
	Long:  "Display all tracked policies with version, deployment info, and sync status.\nSync status compares backend state against live Entra tenant.\nDegrades gracefully when authentication or network is unavailable.",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().String("history", "", "Show version history for a specific policy slug")
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

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
			Message: "tenant is required: use --tenant or set CACTL_TENANT",
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

	// Check --history mode
	historySlug, _ := cmd.Flags().GetString("history")
	if historySlug != "" {
		return runStatusHistory(backend, cfg, historySlug, useColor)
	}

	// 5. Normal status mode: check for tracked policies
	if len(manifest.Policies) == 0 {
		fmt.Fprintln(os.Stdout, "No tracked policies. Run 'cactl import' to get started.")
		return nil
	}

	// 6. Attempt sync check with graceful degradation
	syncAvailable := false
	liveIndex := make(map[string]string) // policy ID -> git SHA of normalized JSON

	factory, factoryErr := auth.NewClientFactory(cfg.Auth)
	if factoryErr != nil {
		if cfg.Output != "json" {
			fmt.Fprintln(os.Stderr, "Warning: Could not authenticate -- sync status will show as 'unknown'.")
		}
	} else {
		cred, credErr := factory.Credential(ctx, cfg.Tenant)
		if credErr != nil {
			if cfg.Output != "json" {
				fmt.Fprintln(os.Stderr, "Warning: Could not authenticate -- sync status will show as 'unknown'.")
			}
		} else {
			graphClient := graph.NewClient(cred, cfg.Tenant)
			livePolicies, listErr := graphClient.ListPolicies(ctx)
			if listErr != nil {
				if cfg.Output != "json" {
					fmt.Fprintln(os.Stderr, "Warning: Could not fetch live policies -- sync status will show as 'unknown'.")
				}
			} else {
				syncAvailable = true
				for _, p := range livePolicies {
					normalized, normErr := normalize.Normalize(p.RawJSON)
					if normErr != nil {
						continue
					}
					sha, hashErr := backend.HashObject(normalized)
					if hashErr != nil {
						continue
					}
					liveIndex[p.ID] = sha
				}
			}
		}
	}

	// 7. Build PolicyStatus entries from manifest
	entries := buildPolicyStatuses(manifest, syncAvailable, liveIndex)

	// Sort by slug for deterministic output
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Slug < entries[j].Slug
	})

	// 8. Build summary
	summary := output.BuildSummary(entries)

	// 9. Render
	if cfg.Output == "json" {
		statusOutput := types.StatusOutput{
			SchemaVersion: 1,
			Tenant:        cfg.Tenant,
			Policies:      entries,
			Summary:       summary,
		}
		if err := output.RenderStatusJSON(os.Stdout, statusOutput); err != nil {
			return &types.ExitError{
				Code:    types.ExitFatalError,
				Message: fmt.Sprintf("rendering JSON: %v", err),
			}
		}
	} else {
		output.RenderStatus(os.Stdout, entries, useColor)
	}

	// Status always exits 0 (informational, not a gate)
	return nil
}

// runStatusHistory handles the --history flag to show version timeline.
func runStatusHistory(backend *state.GitBackend, cfg *types.Config, slug string, useColor bool) error {
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

	if cfg.Output == "json" {
		// For JSON history, output a simple JSON array of version info
		type historyEntry struct {
			Version   string `json:"version"`
			Timestamp string `json:"timestamp"`
			Message   string `json:"message"`
		}
		histEntries := make([]historyEntry, len(tags))
		for i, t := range tags {
			histEntries[i] = historyEntry{
				Version:   t.Version,
				Timestamp: t.Timestamp,
				Message:   t.Message,
			}
		}
		histOutput := struct {
			SchemaVersion int            `json:"schema_version"`
			Slug          string         `json:"slug"`
			History       []historyEntry `json:"history"`
		}{
			SchemaVersion: 1,
			Slug:          slug,
			History:       histEntries,
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
		output.RenderHistory(os.Stdout, slug, tags, useColor)
	}

	return nil
}

// buildPolicyStatuses converts manifest entries to PolicyStatus slice with sync status.
func buildPolicyStatuses(manifest *state.Manifest, syncAvailable bool, liveIndex map[string]string) []types.PolicyStatus {
	entries := make([]types.PolicyStatus, 0, len(manifest.Policies))
	for _, e := range manifest.Policies {
		ps := types.PolicyStatus{
			Slug:         e.Slug,
			Version:      e.Version,
			LastDeployed: e.LastDeployed,
			DeployedBy:   e.DeployedBy,
			LiveObjectID: e.LiveObjectID,
		}

		if !syncAvailable {
			ps.SyncStatus = "unknown"
		} else {
			liveSHA, found := liveIndex[e.LiveObjectID]
			if !found {
				ps.SyncStatus = "missing"
			} else if liveSHA == e.BackendSHA {
				ps.SyncStatus = "in-sync"
			} else {
				ps.SyncStatus = "drifted"
			}
		}

		entries = append(entries, ps)
	}
	return entries
}

