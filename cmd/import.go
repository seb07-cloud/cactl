package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/seb07-cloud/cactl/internal/config"
	"github.com/seb07-cloud/cactl/internal/graph"
	"github.com/seb07-cloud/cactl/internal/normalize"
	"github.com/seb07-cloud/cactl/internal/output"
	"github.com/seb07-cloud/cactl/internal/semver"
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

	// Step 2: Load config and resolve tenants
	cfg, err := config.LoadFromGlobal()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if len(cfg.Tenants) == 0 {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: "tenant is required: use --tenant, set CACTL_TENANT, or log in with az login",
		}
	}

	// Step 3: Execute import for each tenant sequentially
	return runForTenants(ctx, cfg.Tenants, cfg.Auth, func(ctx context.Context, tenantID string, cred azcore.TokenCredential) error {
		return importForTenant(ctx, r, cred, tenantID, importAll, policyFilter, force, ciMode)
	})
}

// importForTenant runs the import pipeline for a single tenant.
func importForTenant(
	ctx context.Context,
	r output.Renderer,
	cred azcore.TokenCredential,
	tenantID string,
	importAll bool,
	policyFilter string,
	force bool,
	ciMode bool,
) error {
	graphClient := graph.NewClient(cred, tenantID)

	backend, err := state.NewGitBackend(".")
	if err != nil {
		return fmt.Errorf("initializing state backend: %w", err)
	}

	// Safety net: ensure refspec exists before first import
	if err := state.ConfigureRefspec("."); err != nil {
		return fmt.Errorf("configuring refspec: %w", err)
	}

	// Fetch policies from Graph API
	r.Info("Fetching policies from Microsoft Graph...")
	policies, err := graphClient.ListPolicies(ctx)
	if err != nil {
		return fmt.Errorf("fetching policies: %w", err)
	}
	r.Info(fmt.Sprintf("Found %d policies in tenant", len(policies)))

	// Read existing manifest
	manifest, err := state.ReadManifest(backend, tenantID)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	// Determine which policies to import
	var toImport []graph.Policy

	if importAll || (force && policyFilter == "") {
		// --all or --force without --policy: import everything
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

	// Process each policy
	imported := 0
	// Get auth mode for manifest entries -- use the factory mode via credential type name
	authMode := "az-cli" // default; will be overridden by factory mode in runForTenants
	for _, p := range toImport {
		slug := normalize.Slugify(p.DisplayName)

		// Check if already tracked
		if entry, exists := manifest.Policies[slug]; exists {
			// Slug collision: same display name but different live object ID.
			// Disambiguate by appending a short ID suffix.
			if entry.LiveObjectID != p.ID {
				slug = disambiguateSlug(slug, p.ID, manifest)
				r.Warn(fmt.Sprintf("Slug collision for %q -- using disambiguated slug: %s", p.DisplayName, slug))
			} else if !force {
				r.Info(fmt.Sprintf("  - %s (already tracked, use --force to overwrite)", slug))
				continue
			}
		}

		// Normalize RawJSON
		normalized, err := normalize.Normalize(p.RawJSON)
		if err != nil {
			return fmt.Errorf("normalizing policy %s: %w", slug, err)
		}

		// Write policy JSON file to working tree for review/editing
		if err := WritePolicyFile(tenantID, slug, normalized); err != nil {
			return fmt.Errorf("writing policy file for %s: %w", slug, err)
		}

		// Write to state (Git blob -- records last-deployed state)
		blobSHA, err := backend.WritePolicy(tenantID, slug, normalized)
		if err != nil {
			return fmt.Errorf("writing policy %s to state: %w", slug, err)
		}

		// Determine version
		version := "1.0.0"
		if entry, exists := manifest.Policies[slug]; exists && force {
			newVer, verErr := semver.BumpVersion(entry.Version, semver.BumpPatch)
			if verErr != nil {
				newVer = "1.0.1"
			}
			version = newVer
		}

		// Create annotated tag
		tagMsg := fmt.Sprintf("cactl import: %s %s", slug, version)
		actualVersion, tagErr := backend.CreateVersionTag(tenantID, slug, version, blobSHA, tagMsg)
		if tagErr != nil {
			return fmt.Errorf("creating version tag for %s: %w", slug, tagErr)
		}
		version = actualVersion

		// Update manifest entry
		manifest.Policies[slug] = state.Entry{
			Slug:         slug,
			Tenant:       tenantID,
			LiveObjectID: p.ID,
			Version:      version,
			LastDeployed: time.Now().UTC().Format(time.RFC3339),
			DeployedBy:   authMode,
			AuthMode:     authMode,
			BackendSHA:   blobSHA,
		}

		r.Print(fmt.Sprintf("  + %s (v%s) -- %s", slug, version, p.DisplayName))
		imported++
	}

	// Write manifest
	if imported > 0 {
		if err := state.WriteManifest(backend, tenantID, manifest); err != nil {
			return fmt.Errorf("writing manifest: %w", err)
		}
	}

	r.Success(fmt.Sprintf("Imported %d policies for tenant %s", imported, tenantID))
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

// disambiguateSlug appends a short suffix from the policy's object ID to
// resolve slug collisions when two live policies share the same display name.
// Uses the first 8 characters of the object ID (before the first hyphen).
func disambiguateSlug(baseSlug, objectID string, manifest *state.Manifest) string {
	suffix := objectID
	if idx := strings.Index(objectID, "-"); idx >= 0 {
		suffix = objectID[:idx]
	}
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	candidate := baseSlug + "-" + suffix
	// Ensure the disambiguated slug itself doesn't collide
	if entry, exists := manifest.Policies[candidate]; exists && entry.LiveObjectID != objectID {
		// Extremely unlikely: use full ID
		candidate = baseSlug + "-" + objectID
	}
	return candidate
}
