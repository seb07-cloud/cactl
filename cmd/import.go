package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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

func init() {
	rootCmd.AddCommand(newImportCmd())
}

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import Conditional Access policies from Entra",
		Long: `Import Conditional Access policies from Microsoft Entra via Graph API.

Fetches live policies, normalizes the JSON, writes to Git state backend,
creates version tags, and updates the manifest.

Without --all or --policy, shows untracked policies and prompts for selection.`,
		RunE: runImport,
	}

	cmd.Flags().Bool("all", false, "Import all policies from tenant")
	cmd.Flags().String("policy", "", "Import a single policy by slug or display name")
	cmd.Flags().Bool("force", false, "Overwrite already-tracked policies (bumps patch version)")

	return cmd
}

func runImport(cmd *cobra.Command, args []string) error {
	v := viper.GetViper()
	format := v.GetString("output")
	useColor := output.ShouldUseColor(v)
	r := output.NewRenderer(format, useColor)
	ctx := cmd.Context()

	importAll, _ := cmd.Flags().GetBool("all")
	policyFilter, _ := cmd.Flags().GetString("policy")
	force, _ := cmd.Flags().GetBool("force")
	ciMode := v.GetBool("ci")

	// Step 1: Validate flags
	if importAll && policyFilter != "" {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: "cannot use --all and --policy together",
		}
	}
	if ciMode && !importAll && policyFilter == "" {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: "--ci mode requires --all or --policy (interactive selection not available)",
		}
	}

	// Step 2: Load config and create dependencies
	cfg, err := config.LoadFromGlobal()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if cfg.Tenant == "" {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: "tenant is required (set via --tenant flag, CACTL_TENANT env, or config file)",
		}
	}

	factory, err := auth.NewClientFactory(cfg.Auth)
	if err != nil {
		return fmt.Errorf("creating auth factory: %w", err)
	}

	cred, err := factory.Credential(ctx, cfg.Tenant)
	if err != nil {
		return fmt.Errorf("acquiring credential: %w", err)
	}

	graphClient := graph.NewClient(cred, cfg.Tenant)

	backend, err := state.NewGitBackend(".")
	if err != nil {
		return fmt.Errorf("initializing state backend: %w", err)
	}

	// Safety net: ensure refspec exists before first import
	if err := state.ConfigureRefspec("."); err != nil {
		return fmt.Errorf("configuring refspec: %w", err)
	}

	// Step 3: Fetch policies from Graph API
	r.Info("Fetching policies from Microsoft Graph...")
	policies, err := graphClient.ListPolicies(ctx)
	if err != nil {
		return fmt.Errorf("fetching policies: %w", err)
	}
	r.Info(fmt.Sprintf("Found %d policies in tenant", len(policies)))

	// Step 4: Read existing manifest
	manifest, err := state.ReadManifest(backend, cfg.Tenant)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	// Step 5: Determine which policies to import
	var toImport []graph.Policy

	if importAll {
		toImport = policies
	} else if policyFilter != "" {
		toImport, err = filterPolicy(policies, policyFilter)
		if err != nil {
			return err
		}
	} else {
		// Interactive selection
		toImport, err = interactiveSelect(ctx, r, policies, manifest, ciMode)
		if err != nil {
			return err
		}
		if toImport == nil {
			return nil // User selected "none" or all tracked
		}
	}

	// Step 6: Process each policy
	imported := 0
	for _, p := range toImport {
		slug := normalize.Slugify(p.DisplayName)

		// Check if already tracked
		if entry, exists := manifest.Policies[slug]; exists {
			// Check for slug collision (different LiveObjectID)
			if entry.LiveObjectID != p.ID {
				return &types.ExitError{
					Code:    types.ExitValidationError,
					Message: fmt.Sprintf("slug collision: %q maps to existing policy %s, but live policy has ID %s", slug, entry.LiveObjectID, p.ID),
				}
			}
			if !force {
				r.Info(fmt.Sprintf("  - %s (already tracked, use --force to overwrite)", slug))
				continue
			}
		}

		// Normalize RawJSON
		normalized, err := normalize.Normalize(p.RawJSON)
		if err != nil {
			return fmt.Errorf("normalizing policy %s: %w", slug, err)
		}

		// Write to state
		blobSHA, err := backend.WritePolicy(cfg.Tenant, slug, normalized)
		if err != nil {
			return fmt.Errorf("writing policy %s to state: %w", slug, err)
		}

		// Determine version
		version := "1.0.0"
		if entry, exists := manifest.Policies[slug]; exists && force {
			version = bumpPatchVersion(entry.Version)
		}

		// Create annotated tag
		tagMsg := fmt.Sprintf("cactl import: %s %s", slug, version)
		if err := backend.CreateVersionTag(cfg.Tenant, slug, version, blobSHA, tagMsg); err != nil {
			return fmt.Errorf("creating version tag for %s: %w", slug, err)
		}

		// Update manifest entry
		manifest.Policies[slug] = state.Entry{
			Slug:         slug,
			Tenant:       cfg.Tenant,
			LiveObjectID: p.ID,
			Version:      version,
			LastDeployed: time.Now().UTC().Format(time.RFC3339),
			DeployedBy:   factory.Mode(),
			AuthMode:     factory.Mode(),
			BackendSHA:   blobSHA,
		}

		r.Print(fmt.Sprintf("  + %s (v%s) -- %s", slug, version, p.DisplayName))
		imported++
	}

	// Step 7: Write manifest
	if imported > 0 {
		if err := state.WriteManifest(backend, cfg.Tenant, manifest); err != nil {
			return fmt.Errorf("writing manifest: %w", err)
		}
	}

	r.Success(fmt.Sprintf("Imported %d policies for tenant %s", imported, cfg.Tenant))
	return nil
}

// filterPolicy finds a single policy matching by slug or display name (case-insensitive).
func filterPolicy(policies []graph.Policy, filter string) ([]graph.Policy, error) {
	filterLower := strings.ToLower(filter)
	for _, p := range policies {
		slug := normalize.Slugify(p.DisplayName)
		if strings.ToLower(slug) == filterLower || strings.ToLower(p.DisplayName) == filterLower {
			return []graph.Policy{p}, nil
		}
	}
	return nil, &types.ExitError{
		Code:    types.ExitValidationError,
		Message: fmt.Sprintf("no policy found matching %q", filter),
	}
}

// interactiveSelect shows untracked policies and prompts for selection.
func interactiveSelect(_ context.Context, r output.Renderer, policies []graph.Policy, manifest *state.Manifest, ciMode bool) ([]graph.Policy, error) {
	// Find untracked policies
	var untracked []graph.Policy
	for _, p := range policies {
		slug := normalize.Slugify(p.DisplayName)
		if _, exists := manifest.Policies[slug]; !exists {
			untracked = append(untracked, p)
		}
	}

	if len(untracked) == 0 {
		r.Info("All policies are already tracked")
		return nil, nil
	}

	if ciMode {
		return nil, &types.ExitError{
			Code:    types.ExitValidationError,
			Message: "--ci mode requires --all or --policy (interactive selection not available)",
		}
	}

	// Print untracked list with ? sigil
	r.Print("Untracked policies:")
	for i, p := range untracked {
		r.Print(fmt.Sprintf("  ? [%d] %s (%s)", i+1, p.DisplayName, p.ID))
	}

	// Prompt user
	fmt.Print("\nImport which policies? (all, none, or comma-separated numbers): ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil, nil
	}
	input := strings.TrimSpace(scanner.Text())

	switch strings.ToLower(input) {
	case "none", "":
		return nil, nil
	case "all":
		return untracked, nil
	default:
		// Parse comma-separated numbers
		var selected []graph.Policy
		for _, part := range strings.Split(input, ",") {
			num, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || num < 1 || num > len(untracked) {
				return nil, &types.ExitError{
					Code:    types.ExitValidationError,
					Message: fmt.Sprintf("invalid selection: %q (expected 1-%d, all, or none)", strings.TrimSpace(part), len(untracked)),
				}
			}
			selected = append(selected, untracked[num-1])
		}
		return selected, nil
	}
}

// bumpPatchVersion increments the patch component of a semver string.
// For example, "1.0.0" becomes "1.0.1".
func bumpPatchVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "1.0.1" // Fallback
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "1.0.1"
	}
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch+1)
}
