# Phase 1: Foundation - Research

**Researched:** 2026-03-04
**Domain:** Go CLI scaffolding, Microsoft Entra ID authentication, Graph API client bootstrap, workspace initialization
**Confidence:** HIGH

## Summary

Phase 1 establishes the three pillars every subsequent phase depends on: the CLI binary skeleton (Cobra/Viper with global flags and config loading), the authentication layer (three auth modes with per-tenant credential isolation), and the workspace scaffolding (`cactl init`). The prior project research (STACK.md, ARCHITECTURE.md, PITFALLS.md) already validated the core stack and architectural patterns at HIGH confidence. This phase-specific research narrows the focus to implementation details the planner needs: the exact Cobra/Viper wiring pattern for config precedence, the azidentity credential types and their per-tenant behavior, the CA policy JSON Schema source for CONF-04, and the exit code / `--no-color` contract.

The primary technical risk in this phase is the auth layer. The azidentity SDK has a documented issue where multi-tenant token acquisition can silently use the wrong tenant's cached token (Azure/azure-sdk-for-go#19726). The mitigation -- one credential instance per tenant via ClientFactory -- must be the architectural default from day one; it cannot be retrofitted. The second risk is the JSON Schema fetch for CONF-04: Microsoft does not publish a standalone JSON Schema for the `conditionalAccessPolicy` resource. The schema must be extracted from the OpenAPI YAML in the `microsoftgraph/msgraph-metadata` repository, or derived from the CSDL `$metadata` endpoint, or hand-maintained from the documented properties. This research recommends fetching the OpenAPI YAML from GitHub and extracting the relevant `components/schemas` section at init time.

**Primary recommendation:** Build four packages in dependency order -- `pkg/types` (shared types), `internal/config` (Viper-based config + validation), `internal/auth` (credential factory with per-tenant isolation), `internal/output` (renderer with `--no-color` support) -- then wire them in `cmd/root.go` and `cmd/init.go`. Defer Graph client implementation to a minimal "auth verification" call (`GET /organization`) to confirm credentials work without importing the full reconciliation surface.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.24+ | Language runtime | Single binary, cross-platform. Set `go 1.24` in go.mod. |
| spf13/cobra | v1.10.x | CLI framework | De facto Go CLI standard (kubectl, docker, gh). Use `RunE` for all commands. Do NOT use v2.0.0 (Dec 2025, ecosystem not ready). |
| spf13/viper | v1.21.0 | Config management | Companion to Cobra. Handles YAML config, env vars, flag binding. Precedence: flags > env > config > defaults. |
| Azure/azure-sdk-for-go/sdk/azidentity | v1.13.x | Entra ID authentication | Official Microsoft SDK. Provides `AzureCLICredential`, `ClientSecretCredential`, `ClientCertificateCredential`. All implement `azcore.TokenCredential`. |
| microsoftgraph/msgraph-sdk-go | v1.96.x | Graph API client (v1.0) | Official typed SDK. Only needed in Phase 1 for auth verification (`GET /organization`). Full CA policy usage starts Phase 2. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stretchr/testify | v1.11.x | Test assertions | `require` for fatal, `assert` for non-fatal. Standard Go test companion. |
| go.uber.org/mock | v0.5.x | Interface mock generation | Generate mocks for AuthProvider, Renderer interfaces. Active fork of deprecated golang/mock. |
| golangci-lint | v2.10.x | Linter | Set up from Phase 1. Config format uses `linters.default`. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| spf13/viper | koanf | koanf is goroutine-safe; viper is not. For CLI init-time config loading, viper's Cobra integration is superior. |
| Separate viper instance | Global viper singleton | Global singleton makes testing harder. Use `viper.New()` and pass the instance, but the cobra integration works best with the global instance for a CLI tool where config is loaded once. |
| AzureCLICredential | DefaultAzureCredential | DefaultAzureCredential has unpredictable chain in shared environments. Use explicit credential types based on resolved auth mode. |

### Installation

```bash
go mod init github.com/[org]/cactl

go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get github.com/Azure/azure-sdk-for-go/sdk/azidentity@latest
go get github.com/microsoftgraph/msgraph-sdk-go@latest
go get github.com/stretchr/testify@latest
go get go.uber.org/mock/mockgen@latest
```

## Architecture Patterns

### Recommended Project Structure (Phase 1 scope)

```
cactl/
├── main.go                         # Entry point: cobra Execute()
├── go.mod
├── go.sum
├── cmd/
│   ├── root.go                     # Root command, global flags, PersistentPreRunE config
│   └── init.go                     # cactl init (workspace scaffolding)
├── internal/
│   ├── auth/
│   │   ├── provider.go             # AuthProvider interface + resolution chain
│   │   ├── azurecli.go             # AzureCLICredential wrapper
│   │   ├── secret.go               # ClientSecretCredential wrapper
│   │   ├── certificate.go          # ClientCertificateCredential wrapper
│   │   └── factory.go              # ClientFactory: per-tenant credential caching
│   ├── config/
│   │   ├── config.go               # Config struct, Viper loading, env merge
│   │   └── validate.go             # Config validation rules
│   └── output/
│       ├── renderer.go             # Renderer interface
│       ├── human.go                # Human-readable output (with color support)
│       └── json.go                 # Structured JSON output
├── pkg/
│   └── types/
│       ├── config.go               # Config types (AuthConfig, OutputConfig, etc.)
│       └── exitcodes.go            # Exit code constants
└── testdata/
    └── config/                     # Sample config YAML files for tests
```

### Pattern 1: Cobra + Viper Config Precedence Chain

**What:** All configuration resolves through a single precedence chain: CLI flags > env vars (CACTL_*) > config file (.cactl/config.yaml) > defaults. This is wired in `PersistentPreRunE` on the root command so it runs before any subcommand.

**When to use:** Every command invocation.

**Implementation details (verified against official Cobra docs):**

```go
// cmd/root.go
var rootCmd = &cobra.Command{
    Use:   "cactl",
    Short: "Conditional Access policy deploy framework",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        return initConfig(cmd)
    },
    SilenceUsage:  true, // Don't show usage on runtime errors
    SilenceErrors: true, // We handle error output ourselves
}

func init() {
    // Global flags (CLI-08)
    rootCmd.PersistentFlags().String("tenant", "", "Entra tenant ID or primary domain")
    rootCmd.PersistentFlags().String("output", "human", "Output format: human|json")
    rootCmd.PersistentFlags().Bool("no-color", false, "Disable ANSI color output")
    rootCmd.PersistentFlags().Bool("ci", false, "Non-interactive CI mode")
    rootCmd.PersistentFlags().String("config", "", "Config file (default: .cactl/config.yaml)")
    rootCmd.PersistentFlags().String("log-level", "info", "Log level: debug|info|warn|error")
    rootCmd.PersistentFlags().String("auth-mode", "", "Auth mode: az-cli|client-secret|client-certificate")
}

func initConfig(cmd *cobra.Command) error {
    v := viper.GetViper()

    // 1. Config file
    cfgFile, _ := cmd.Flags().GetString("config")
    if cfgFile != "" {
        v.SetConfigFile(cfgFile)
    } else {
        v.SetConfigName("config")
        v.SetConfigType("yaml")
        v.AddConfigPath(".cactl")
    }

    // 2. Env vars: CACTL_TENANT, CACTL_OUTPUT, CACTL_NO_COLOR, etc.
    v.SetEnvPrefix("CACTL")
    v.AutomaticEnv()
    v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

    // 3. Read config file (ignore if not found -- init hasn't been run yet)
    if err := v.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return fmt.Errorf("reading config: %w", err)
        }
    }

    // 4. Bind flags to viper (flags override everything)
    v.BindPFlags(cmd.Flags())

    return nil
}
```

**Critical:** Always use `viper.GetString("key")` to read values, never `cmd.Flag("key").Value.String()`. The flag variable holds the default; viper resolves the full precedence chain.

### Pattern 2: Auth Provider Resolution Chain (AUTH-04)

**What:** Auth mode resolves in priority order: `--auth-mode` flag > `CACTL_AUTH_MODE` env > config file `auth.mode` > auto-detect > az-cli fallback.

**Auto-detect logic:** If `CACTL_CLIENT_ID` + `CACTL_CLIENT_SECRET` are set, use `client-secret`. If `CACTL_CLIENT_ID` + `CACTL_CERT_PATH` are set, use `client-certificate`. Otherwise fall back to `az-cli`.

```go
// internal/auth/provider.go
type AuthProvider interface {
    // Credential returns a TokenCredential for the given tenant.
    // Each call with a different tenantID returns an isolated credential instance.
    Credential(ctx context.Context, tenantID string) (azcore.TokenCredential, error)
    // Mode returns the resolved auth mode name for display/logging.
    Mode() string
}

func ResolveAuthMode(cfg types.AuthConfig) string {
    // Priority: explicit mode > auto-detect > fallback
    if cfg.Mode != "" {
        return cfg.Mode  // Already resolved from flag > env > config by viper
    }
    // Auto-detect
    if cfg.ClientID != "" && cfg.ClientSecret != "" {
        return "client-secret"
    }
    if cfg.ClientID != "" && cfg.CertPath != "" {
        return "client-certificate"
    }
    return "az-cli"  // Fallback (AUTH-01)
}
```

### Pattern 3: ClientFactory for Per-Tenant Credential Isolation (AUTH-05)

**What:** A factory that creates one credential instance per tenant ID. Never reuses credentials across tenants.

**Why per-tenant instances:** azidentity has a documented issue (Azure/azure-sdk-for-go#19726) where `ClientSecretCredential` initialized with one tenant may acquire tokens for a different tenant from its cache. Creating separate credential instances per tenant eliminates this risk entirely.

```go
// internal/auth/factory.go
type ClientFactory struct {
    mode        string
    cfg         types.AuthConfig
    credentials map[string]azcore.TokenCredential  // keyed by tenantID
}

func NewClientFactory(cfg types.AuthConfig) *ClientFactory {
    return &ClientFactory{
        mode:        ResolveAuthMode(cfg),
        cfg:         cfg,
        credentials: make(map[string]azcore.TokenCredential),
    }
}

func (f *ClientFactory) Credential(ctx context.Context, tenantID string) (azcore.TokenCredential, error) {
    if cred, ok := f.credentials[tenantID]; ok {
        return cred, nil
    }

    var cred azcore.TokenCredential
    var err error

    switch f.mode {
    case "az-cli":
        opts := &azidentity.AzureCLICredentialOptions{TenantID: tenantID}
        cred, err = azidentity.NewAzureCLICredential(opts)
    case "client-secret":
        cred, err = azidentity.NewClientSecretCredential(
            tenantID, f.cfg.ClientID, f.cfg.ClientSecret, nil,
        )
    case "client-certificate":
        certData, readErr := os.ReadFile(f.cfg.CertPath)
        if readErr != nil {
            return nil, fmt.Errorf("reading certificate: %w", readErr)
        }
        certs, key, parseErr := azidentity.ParseCertificates(certData, nil)
        if parseErr != nil {
            return nil, fmt.Errorf("parsing certificate: %w", parseErr)
        }
        cred, err = azidentity.NewClientCertificateCredential(
            tenantID, f.cfg.ClientID, certs, key, nil,
        )
    default:
        return nil, fmt.Errorf("unknown auth mode: %s", f.mode)
    }

    if err != nil {
        return nil, fmt.Errorf("creating %s credential for tenant %s: %w", f.mode, tenantID, err)
    }
    f.credentials[tenantID] = cred
    return cred, nil
}
```

### Pattern 4: Exit Code Contract (CLI-09)

**What:** Custom exit codes mapped to domain-specific outcomes.

```go
// pkg/types/exitcodes.go
const (
    ExitSuccess        = 0  // Success, no changes needed
    ExitChanges        = 1  // Changes or drift detected
    ExitFatalError     = 2  // Fatal error (auth failure, network error, etc.)
    ExitValidationError = 3  // Validation error (invalid config, schema violation)
)

// Custom error type that carries an exit code
type ExitError struct {
    Code    int
    Message string
    Err     error
}

func (e *ExitError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %v", e.Message, e.Err)
    }
    return e.Message
}
```

In `main.go`, extract the exit code after `rootCmd.Execute()`:

```go
func main() {
    if err := cmd.Execute(); err != nil {
        var exitErr *types.ExitError
        if errors.As(err, &exitErr) {
            os.Exit(exitErr.Code)
        }
        os.Exit(types.ExitFatalError)
    }
}
```

### Pattern 5: No-Color Output (DISP-06)

**What:** `--no-color` flag and `CACTL_NO_COLOR=1` env var disable ANSI color output. Also respects the `NO_COLOR` convention (https://no-color.org/).

```go
// internal/output/color.go
func ShouldUseColor(v *viper.Viper) bool {
    // Explicit flag/env takes priority
    if v.GetBool("no-color") {
        return false
    }
    // Respect NO_COLOR convention (any non-empty value)
    if os.Getenv("NO_COLOR") != "" {
        return false
    }
    // CI mode disables color by default
    if v.GetBool("ci") {
        return false
    }
    // Check if stdout is a terminal
    if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
        return false  // piped output, no color
    }
    return true
}
```

### Anti-Patterns to Avoid

- **Business logic in cmd/:** Each command file must be a thin orchestrator (~50-100 lines). Parse flags, construct dependencies, delegate to internal packages, render output. No auth logic, no config parsing, no Graph calls in cmd/ files.
- **Using DefaultAzureCredential:** Unpredictable credential chain in shared environments. Always resolve to a specific credential type based on explicit auth mode.
- **Global mutable state:** No package-level variables for config, credentials, or clients. Thread dependencies explicitly through function parameters.
- **Reading flags directly instead of through viper:** `cmd.Flag("tenant").Value.String()` bypasses the precedence chain. Always use `viper.GetString("tenant")`.
- **Logging credentials:** Never log client secrets, certificate contents, or token values. The auth provider must sanitize all log output.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Azure AD authentication | Custom OAuth2 token flow | azidentity `NewClientSecretCredential`, `NewAzureCLICredential`, etc. | Token refresh, cache management, certificate parsing, PKCE -- all handled. Hundreds of edge cases. |
| CLI flag parsing + config precedence | Custom config loader with manual env var reading | Cobra pflags + Viper `AutomaticEnv()` + `BindPFlags()` | Precedence chain is subtle (flag vs env vs config vs default). Viper handles it correctly. |
| YAML config file parsing | Custom YAML parser | Viper `ReadInConfig()` with `SetConfigType("yaml")` | Viper handles file discovery, format detection, and merging with other sources. |
| ANSI color stripping | Regex-based ANSI escape removal | Check `ShouldUseColor()` before emitting; use a color library that respects a disable flag | Stripping after the fact is fragile. Disable at the source. |
| Certificate PEM parsing | Custom PEM/PKCS parser | `azidentity.ParseCertificates()` | Handles PEM, PKCS#12, certificate chains, and password-protected keys. |
| JSON Schema for CA policies | Hand-written schema file | Extract from `microsoftgraph/msgraph-metadata` OpenAPI spec | Schema stays current with Graph API changes. |

**Key insight:** Phase 1's value is wiring well-tested libraries together correctly, not building custom implementations. The azidentity and Cobra/Viper libraries handle the hard parts. The skill is in the integration pattern (per-tenant isolation, config precedence, exit codes).

## Common Pitfalls

### Pitfall 1: Wrong-Tenant Token Acquisition

**What goes wrong:** azidentity's `ClientSecretCredential` initialized with tenant A may acquire tokens for tenant A when asked for tenant B, due to silent token cache reuse.
**Why it happens:** Documented SDK issue (Azure/azure-sdk-for-go#19726). The cache key may not include tenant ID in all code paths.
**How to avoid:** Create one credential instance per tenant. The ClientFactory pattern above enforces this. Never share a credential across tenants.
**Warning signs:** Policies appearing in wrong tenant during multi-tenant testing; 403 errors that resolve on restart.

### Pitfall 2: Config File Tracked by Git (CONF-03)

**What goes wrong:** `.cactl/config.yaml` committed to Git exposes tenant IDs and potentially client IDs. Once in Git history, secrets persist even after deletion.
**Why it happens:** User runs `git add .` before `cactl init` creates the `.gitignore`.
**How to avoid:** `cactl init` must: (1) create `.gitignore` BEFORE creating `config.yaml`, (2) check if `config.yaml` is already tracked (`git ls-files --error-unmatch`), (3) refuse to continue if tracked, (4) never store secrets in config -- only reference env vars.
**Warning signs:** `git status` shows `.cactl/config.yaml` as tracked.

### Pitfall 3: Viper Flag Binding Timing

**What goes wrong:** Flags bound in `init()` with `viper.BindPFlag()` can override config file values unexpectedly because Cobra sets flag defaults before viper reads the config.
**Why it happens:** When `BindPFlag` is called, viper treats the flag's default value as if it were explicitly set, overriding the config file value.
**How to avoid:** Bind flags in `PersistentPreRunE` (after config is loaded), or use `viper.BindPFlags(cmd.Flags())` which only overrides when the flag was actually set by the user. The pattern in this research binds in `initConfig()` which runs in `PersistentPreRunE`.
**Warning signs:** Config file values being ignored; defaults always winning.

### Pitfall 4: AzureCLICredential Requires Active az login Session

**What goes wrong:** `AzureCLICredential` calls `az account get-access-token` under the hood. If the user's `az login` session has expired or they're logged into the wrong tenant, the credential returns a token for an unexpected tenant.
**Why it happens:** `AzureCLICredential` uses the active subscription's tenant, not necessarily the tenant the user specified via `--tenant`.
**How to avoid:** When using `az-cli` auth mode, pass `TenantID` in `AzureCLICredentialOptions` to force token acquisition for the specified tenant. After acquiring the credential, verify auth by making a test API call.
**Warning signs:** "Run az login first" errors; wrong tenant in Graph API responses.

### Pitfall 5: JSON Schema Fetch Fails Without Network

**What goes wrong:** `cactl init` tries to fetch the CA policy JSON Schema from GitHub (CONF-04) but fails when offline or behind a corporate proxy.
**Why it happens:** The schema source is a GitHub raw URL.
**How to avoid:** Make schema fetch optional with a warning. Bundle a fallback schema in the binary as an embedded resource (`embed` package). Fetch from network if available; use bundled version if not. Add `--skip-schema` flag to bypass entirely.
**Warning signs:** `cactl init` hangs or fails in air-gapped environments.

### Pitfall 6: PersistentPreRunE Inheritance in Subcommands

**What goes wrong:** If a subcommand defines its own `PersistentPreRunE`, it overrides the root command's `PersistentPreRunE` instead of chaining with it. Config loading never runs.
**Why it happens:** Cobra does not chain `PersistentPreRunE` calls -- the most specific one wins.
**How to avoid:** If a subcommand needs pre-run logic, explicitly call the parent's pre-run function first. Or better: keep all global pre-run logic in the root command's `PersistentPreRunE` and use per-command `PreRunE` (non-persistent) for command-specific setup.
**Warning signs:** Config values are empty in subcommands; env vars not resolved.

## Code Examples

### cactl init Workspace Scaffolding (CLI-01, CONF-01, CONF-03, CONF-04)

```go
// cmd/init.go
func runInit(cmd *cobra.Command, args []string) error {
    // 1. Check if .cactl already exists
    if _, err := os.Stat(".cactl"); err == nil {
        return &types.ExitError{
            Code:    types.ExitValidationError,
            Message: "workspace already initialized (.cactl directory exists)",
        }
    }

    // 2. Create .cactl directory
    if err := os.MkdirAll(".cactl", 0755); err != nil {
        return fmt.Errorf("creating .cactl directory: %w", err)
    }

    // 3. Write .gitignore FIRST (CONF-03 -- before config.yaml exists)
    gitignoreContent := "# cactl workspace\nconfig.yaml\n*.secret\n"
    if err := os.WriteFile(".gitignore", []byte(gitignoreContent), 0644); err != nil {
        // If .gitignore already exists, append rather than overwrite
        // ... (append logic)
    }

    // 4. Check if config.yaml is already tracked by Git (CONF-03)
    trackCheck := exec.Command("git", "ls-files", "--error-unmatch", ".cactl/config.yaml")
    if trackCheck.Run() == nil {
        return &types.ExitError{
            Code:    types.ExitValidationError,
            Message: ".cactl/config.yaml is tracked by Git. Remove it from tracking first: git rm --cached .cactl/config.yaml",
        }
    }

    // 5. Write default config.yaml (CONF-01)
    defaultConfig := `# cactl configuration
# All values can be overridden by CACTL_* environment variables or CLI flags
tenant: ""
auth:
  mode: ""  # az-cli | client-secret | client-certificate (auto-detected if empty)
output: human  # human | json
log_level: info
`
    if err := os.WriteFile(".cactl/config.yaml", []byte(defaultConfig), 0644); err != nil {
        return fmt.Errorf("writing config: %w", err)
    }

    // 6. Fetch CA policy JSON Schema (CONF-04)
    if err := fetchSchema(".cactl/schema.json"); err != nil {
        // Non-fatal: warn and continue
        fmt.Fprintf(os.Stderr, "Warning: could not fetch CA policy schema: %v\n", err)
        fmt.Fprintf(os.Stderr, "Using bundled schema. Run 'cactl init --update-schema' to retry.\n")
        // Write bundled fallback schema
        if writeErr := writeBundledSchema(".cactl/schema.json"); writeErr != nil {
            return fmt.Errorf("writing bundled schema: %w", writeErr)
        }
    }

    return nil
}
```

### Config Struct (CONF-01, CONF-02)

```go
// internal/config/config.go
type Config struct {
    Tenant   string       `mapstructure:"tenant"`
    Auth     AuthConfig   `mapstructure:"auth"`
    Output   string       `mapstructure:"output"`
    LogLevel string       `mapstructure:"log_level"`
    NoColor  bool         `mapstructure:"no_color"`
    CI       bool         `mapstructure:"ci"`
}

type AuthConfig struct {
    Mode         string `mapstructure:"mode"`
    ClientID     string `mapstructure:"client_id"`
    ClientSecret string `mapstructure:"client_secret"`  // NEVER from config file; env var only
    CertPath     string `mapstructure:"cert_path"`
}

func Load(v *viper.Viper) (*Config, error) {
    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("unmarshalling config: %w", err)
    }

    // Override auth secrets from env vars exclusively (AUTH-06)
    cfg.Auth.ClientID = v.GetString("client_id")       // CACTL_CLIENT_ID
    cfg.Auth.ClientSecret = v.GetString("client_secret") // CACTL_CLIENT_SECRET
    cfg.Auth.CertPath = v.GetString("cert_path")       // CACTL_CERT_PATH

    return &cfg, nil
}
```

### Auth Verification (Testing Auth Works)

```go
// After credential is obtained, verify it works with a lightweight Graph call
func VerifyAuth(ctx context.Context, cred azcore.TokenCredential, tenantID string) error {
    client, err := msgraphsdk.NewGraphServiceClientWithCredentials(cred, []string{
        "https://graph.microsoft.com/.default",
    })
    if err != nil {
        return fmt.Errorf("creating graph client: %w", err)
    }

    // Minimal call to verify auth: get organization info
    org, err := client.Organization().Get(ctx, nil)
    if err != nil {
        return fmt.Errorf("auth verification failed: %w", err)
    }

    // Verify we're authenticated to the correct tenant
    orgs := org.GetValue()
    if len(orgs) > 0 {
        orgTenantID := orgs[0].GetId()
        if orgTenantID != nil && *orgTenantID != tenantID {
            return fmt.Errorf("authenticated to tenant %s but expected %s", *orgTenantID, tenantID)
        }
    }
    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| cobra v1.x with manual flag binding | cobra v1.10.x with `BindPFlags(cmd.Flags())` in PreRunE | 2024 | Cleaner precedence; flags only override when user explicitly sets them |
| `golang/mock` for interface mocking | `go.uber.org/mock` (active fork) | 2023 | golang/mock is archived; uber fork is maintained |
| golangci-lint v1 config | golangci-lint v2 config (`linters.default`) | 2025 | New config format; use `golangci-lint migrate` if starting from v1 examples |
| `DefaultAzureCredential` for all auth | Explicit credential type selection | Always recommended | `DefaultAzureCredential` is convenient for demos but unpredictable in production |
| cobra v2.0.0 | cobra v1.10.x | Dec 2025 (v2 released) | v2 has breaking changes; ecosystem adoption is incomplete; stick with v1 line |

**Deprecated/outdated:**
- `golang/mock`: Archived. Use `go.uber.org/mock`.
- `gopkg.in/yaml.v2`: Use `gopkg.in/yaml.v3` (viper uses v3 internally).
- cobra v2.0.0: Too new (Dec 2025). Ecosystem not validated.

## CA Policy JSON Schema (CONF-04)

Microsoft does not publish a standalone JSON Schema (draft-07 or later) for the `conditionalAccessPolicy` resource type. The schema information is available in three forms:

### Option A: Extract from OpenAPI spec (RECOMMENDED)

The `microsoftgraph/msgraph-metadata` repository contains the full OpenAPI 3.0.4 spec at:
```
https://raw.githubusercontent.com/microsoftgraph/msgraph-metadata/master/openapi/v1.0/openapi.yaml
```

The CA policy schema is at `components/schemas/microsoft.graph.conditionalAccessPolicy` and related types (`microsoft.graph.conditionalAccessConditionSet`, `microsoft.graph.conditionalAccessGrantControls`, `microsoft.graph.conditionalAccessSessionControls`).

**Implementation:** At `cactl init` time, fetch the OpenAPI YAML, extract the relevant schema components, and convert to JSON Schema. Store as `.cactl/schema.json`.

**Tradeoff:** The full OpenAPI YAML is ~30MB. Consider using the pre-sliced files from `msgraph-sdk-powershell/openApiDocs/v1.0/Identity.SignIns.yml` which contains only the identity/sign-ins subset.

### Option B: CSDL $metadata endpoint

```
https://graph.microsoft.com/v1.0/$metadata
```

This returns the full CSDL (XML) for Graph v1.0. The CA policy entity type is defined there. Requires XML parsing and CSDL-to-JSON-Schema conversion.

**Tradeoff:** More authoritative (live from the API) but requires XML parsing and a CSDL-to-JSON-Schema converter. More complex than Option A.

### Option C: Bundled schema derived from documentation

Hand-maintain a JSON Schema file based on the documented properties at `learn.microsoft.com/en-us/graph/api/resources/conditionalaccesspolicy`.

**Tradeoff:** Simplest implementation, but requires manual updates when Microsoft adds properties. Does not stay fresh automatically.

### Recommendation

Use Option A with an embedded fallback (Option C). `cactl init` attempts to fetch and extract from the OpenAPI spec. If it fails (offline, proxy), fall back to a bundled schema embedded in the binary via Go's `embed` package. The bundled schema is updated with each cactl release.

### CA Policy Properties (from official docs, verified)

**Mutable properties (safe to PATCH):**
- `displayName` (String)
- `state` (conditionalAccessPolicyState: enabled | disabled | enabledForReportingButNotEnforced)
- `conditions` (conditionalAccessConditionSet)
- `grantControls` (conditionalAccessGrantControls)
- `sessionControls` (conditionalAccessSessionControls)

**Read-only properties (MUST strip during normalization):**
- `id` (String) -- server-assigned GUID
- `createdDateTime` (DateTimeOffset) -- server-managed
- `modifiedDateTime` (DateTimeOffset) -- server-managed
- `templateId` (String) -- inherited from template, read-only

**Required for POST (create):**
- `displayName`
- `state`
- `conditions`

**PATCH returns:** `204 No Content` with empty body.

**Required permissions:**
- `Policy.Read.All` AND `Policy.ReadWrite.ConditionalAccess` (minimum)
- `Application.Read.All` may also be required (documented known issue)

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CLI-01 | `cactl init` scaffolds workspace (.cactl/config.yaml, .gitignore, refspec setup, schema fetch) | Pattern: cactl init code example; creates .cactl dir, .gitignore before config.yaml, fetches schema with fallback. Refspec setup deferred to Phase 2 (no state refs yet). |
| CLI-08 | All commands accept --tenant, --output, --no-color, --ci, --config, --log-level flags | Pattern 1: Cobra PersistentFlags on rootCmd; viper binds in PersistentPreRunE; all subcommands inherit. |
| CLI-09 | Exit codes: 0=success, 1=changes/drift, 2=fatal, 3=validation | Pattern 4: ExitError type with Code field; main.go extracts code after Execute(). |
| CLI-10 | Single Go binary with cobra/viper, zero external runtime deps | Stack: Go 1.24+, cobra v1.10.x, viper v1.21.0. `go build` produces single binary. |
| AUTH-01 | Azure CLI credential (az login token, default fallback) | Pattern 2-3: AzureCLICredential via azidentity with TenantID option. Fallback mode in ResolveAuthMode(). |
| AUTH-02 | Service principal with client secret (CACTL_CLIENT_ID + CACTL_CLIENT_SECRET) | Pattern 3: ClientSecretCredential via azidentity. Env vars resolved through viper CACTL_ prefix. |
| AUTH-03 | Service principal with certificate (CACTL_CLIENT_ID + CACTL_CERT_PATH) | Pattern 3: ClientCertificateCredential via azidentity. ParseCertificates() handles PEM/PKCS#12. |
| AUTH-04 | Auth mode resolution: flag > env > config > auto-detect > fallback | Pattern 2: ResolveAuthMode() checks viper-resolved mode, then auto-detects from env vars, then falls back to az-cli. |
| AUTH-05 | Per-tenant credential isolation via ClientFactory | Pattern 3: ClientFactory creates separate credential instance per tenant ID. Map keyed by tenantID. Addresses azidentity issue #19726. |
| AUTH-06 | Credentials never written to disk, logged, or in output | Config struct: ClientSecret loaded from env var only (CACTL_CLIENT_SECRET), never from config file. Auth provider sanitizes log output. Renderer never emits credential values. |
| CONF-01 | Config in .cactl/config.yaml with documented schema | Config struct example: tenant, auth (mode, client_id), output, log_level. Default config written by init. |
| CONF-02 | Every config value overridable by CACTL_* env var or CLI flag | Pattern 1: viper.SetEnvPrefix("CACTL") + AutomaticEnv() + BindPFlags(). Full precedence chain. |
| CONF-03 | `cactl init` adds config to .gitignore, refuses if already tracked | Init code example: writes .gitignore before config.yaml; checks git ls-files --error-unmatch; exits with code 3 if tracked. |
| CONF-04 | `cactl init` fetches CA policy JSON Schema from Graph metadata | CA Policy JSON Schema section: fetch from msgraph-metadata OpenAPI spec with embedded fallback. Pre-sliced Identity.SignIns.yml for smaller download. |
| DISP-06 | --no-color disables ANSI color (also CACTL_NO_COLOR=1) | Pattern 5: ShouldUseColor() checks --no-color flag, CACTL_NO_COLOR, NO_COLOR convention, CI mode, and terminal detection. |
</phase_requirements>

## Open Questions

1. **Refspec setup in Phase 1 or Phase 2?**
   - What we know: CLI-01 mentions "refspec setup" as part of `cactl init`. But refspecs configure push/fetch for `refs/cactl/*`, which is the state storage namespace created in Phase 2.
   - What's unclear: Should `cactl init` write the refspec now (Phase 1) even though no refs exist yet, or defer to Phase 2 when state storage is implemented?
   - Recommendation: Defer refspec setup to Phase 2. Phase 1's `cactl init` creates the config file, .gitignore, and schema. Phase 2 extends init (or adds a separate step) to configure refspecs. This avoids writing Git config for a namespace that doesn't exist yet.

2. **Schema fetch: full OpenAPI vs. pre-sliced?**
   - What we know: The full OpenAPI YAML is ~30MB. The `msgraph-sdk-powershell` repo has pre-sliced files per service area.
   - What's unclear: Whether the pre-sliced `Identity.SignIns.yml` contains the complete CA policy schema including all nested types.
   - Recommendation: Start with the pre-sliced file. If it's incomplete, fall back to the full spec. Either way, bundle a static fallback via `embed` for offline use.

3. **Auth verification: should `cactl init` verify auth?**
   - What we know: `cactl init` creates the workspace. Auth is configured but may not be verified until a command like `plan` or `import` runs.
   - What's unclear: Should init include an optional `--verify-auth` step that makes a test Graph API call?
   - Recommendation: Make auth verification a separate concern. `cactl init` focuses on workspace scaffolding. A future `cactl auth verify` or `cactl whoami` command handles auth testing. This keeps init fast and offline-capable.

4. **viper instance: global singleton vs. passed instance?**
   - What we know: Viper's global instance is convenient but makes testing harder. A passed instance is cleaner but requires more plumbing.
   - What's unclear: How much test friction the global singleton creates in practice.
   - Recommendation: Use the global singleton for Phase 1 (standard Cobra pattern). If testing friction emerges, refactor to a passed instance in a later phase. The config loading happens once at startup; concurrency is not a concern.

## Sources

### Primary (HIGH confidence)
- [azidentity on pkg.go.dev](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity) -- v1.13.x credential types, AzureCLICredential, ClientSecretCredential, ClientCertificateCredential
- [azidentity README on GitHub](https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/README.md) -- auth patterns, credential chains, token behavior
- [Azure/azure-sdk-for-go issue #19726](https://github.com/Azure/azure-sdk-for-go/issues/19726) -- multi-tenant token acquisition bug
- [Azure/azure-sdk-for-go issue #21651](https://github.com/Azure/azure-sdk-for-go/issues/21651) -- AzureCLICredential tenant behavior
- [Microsoft Learn: Credential chains in Azure Identity for Go](https://learn.microsoft.com/en-us/azure/developer/go/sdk/authentication/credential-chains) -- credential chain patterns
- [Microsoft Learn: conditionalAccessPolicy resource type](https://learn.microsoft.com/en-us/graph/api/resources/conditionalaccesspolicy?view=graph-rest-1.0) -- CA policy properties, read-only fields, required fields
- [Microsoft Learn: Update conditionalAccessPolicy](https://learn.microsoft.com/en-us/graph/api/conditionalaccesspolicy-update?view=graph-rest-1.0) -- PATCH semantics, permissions, Go SDK example
- [Cobra official docs: 12-Factor App](https://cobra.dev/docs/tutorials/12-factor-app/) -- PersistentPreRunE config loading pattern
- [Cobra official docs: Working with Commands](https://cobra.dev/docs/how-to-guides/working-with-commands/) -- RunE, SilenceUsage, SilenceErrors
- [Cobra GitHub: Exit code issue #2124](https://github.com/spf13/cobra/issues/2124) -- custom exit code patterns
- [spf13/viper on GitHub](https://github.com/spf13/viper) -- SetEnvPrefix, AutomaticEnv, SetEnvKeyReplacer
- [msgraph-metadata OpenAPI v1.0](https://github.com/microsoftgraph/msgraph-metadata/blob/master/openapi/v1.0/openapi.yaml) -- CA policy schema source
- [msgraph-metadata OpenAPI directory](https://github.com/microsoftgraph/msgraph-metadata/tree/master/openapi) -- sliced specs, Hidi tool

### Secondary (MEDIUM confidence)
- [Cobra + Viper integration patterns (Glukhov blog)](https://www.glukhov.org/post/2025/11/go-cli-applications-with-cobra-and-viper/) -- community pattern for PersistentPreRunE
- [JetBrains Guide: Error Handling in Cobra](https://www.jetbrains.com/guide/go/tutorials/cli-apps-go-cobra/error_handling/) -- RunE patterns, SilenceUsage
- [Gopher Advent: Taming Cobras](https://gopheradvent.com/calendar/2022/taming-cobras-making-most-of-cobra-clis/) -- advanced Cobra patterns
- [Kosli: Configure CLI Tools with Viper](https://www.kosli.com/blog/how-to-configure-cli-tools-in-standard-formats-with-viper-in-golang/) -- env prefix and YAML config
- [DeepWiki: azidentity Authentication and Identity](https://deepwiki.com/Azure/azure-sdk-for-go/2.2-authentication-and-identity) -- credential type overview
- [Microsoft Learn: Local Dev with Service Principals](https://learn.microsoft.com/en-us/azure/developer/go/sdk/authentication/local-development-service-principal) -- SP auth Go examples

### Tertiary (LOW confidence)
- [no-color.org](https://no-color.org/) -- NO_COLOR convention (community standard, widely adopted)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries verified on pkg.go.dev with version numbers and release dates
- Architecture patterns: HIGH -- patterns from official Cobra/Viper docs and verified azidentity examples
- Auth layer: HIGH -- azidentity credential types and multi-tenant issue verified against official GitHub repo
- Config precedence: HIGH -- verified against official Cobra 12-Factor App tutorial
- CA policy schema: MEDIUM -- schema extraction approach is sound but the pre-sliced file completeness is unverified
- Pitfalls: HIGH -- all pitfalls verified against official docs or documented GitHub issues

**Research date:** 2026-03-04
**Valid until:** 2026-04-04 (stable libraries, 30-day validity)
