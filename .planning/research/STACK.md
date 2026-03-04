# Stack Research

**Domain:** Go CLI tool for Microsoft Entra Conditional Access policy management via Graph API
**Researched:** 2026-03-04
**Confidence:** HIGH

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.24+ (target 1.25/1.26) | Language runtime | Single binary, cross-platform, no runtime deps. Set `go 1.24` in go.mod for broad compat; CI can test against 1.25 and 1.26. Go 1.26 released Feb 2026, but 1.24 is the safe floor for all dependency compat. |
| spf13/cobra | v1.10.x (latest v1 line) | CLI command framework | De facto standard for Go CLIs (used by kubectl, docker, terraform, gh). Command-tree abstraction, POSIX flags via pflag, auto-generated help/completions. Use `RunE` for all commands (error returns). Do NOT jump to v2.0.0 yet -- it was just released Dec 2025 and the ecosystem has not fully validated it. |
| spf13/viper | v1.21.0 | Configuration management | Companion to Cobra. Handles YAML config files, env vars, flag binding with clear precedence: flags > env > config file > defaults. Bind flags in `init()`, load config in `PersistentPreRunE`. Not concurrency-safe -- fine for CLI init-time config loading. |
| Azure/azure-sdk-for-go/sdk/azidentity | v1.13.x | Authentication to Microsoft Entra ID | Official Microsoft SDK. Provides `DeviceCodeCredential`, `ClientSecretCredential`, `ClientCertificateCredential`, `WorkloadIdentityCredential` -- all four auth modes cactl needs. Integrates directly with msgraph-sdk-go via `GraphServiceClientWithCredentials`. |
| microsoftgraph/msgraph-sdk-go | v1.96.x | Microsoft Graph API client (v1.0 endpoint) | Official Microsoft Graph SDK for Go. Typed fluent API: `client.Identity().ConditionalAccess().Policies().Get()`. Auto-generated from OpenAPI spec, so it tracks API changes. Use v1.0 (stable), not beta SDK. |
| go-git/go-git/v5 | v5.17.0 | Git operations (refs, tags, objects) | Pure Go Git implementation. No dependency on git binary. Supports ref manipulation (`refs/cactl/*`), annotated tag creation, object reading/writing. Used by Gitea, Pulumi. Security patches applied in 2025 -- pin to latest. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Masterminds/semver/v3 | v3.4.0 | Semantic version parsing and comparison | State versioning for plan/apply. Parse, compare, bump semver strings. Handles prerelease correctly since v3.4.0. |
| r3labs/diff/v3 | v3.x (latest) | Struct diffing with changelog | Reconciliation engine: diff desired vs actual CA policy state. Produces typed changelog (create/update/delete) serializable to JSON. Supports patch/merge. Tagged field comparison. |
| google/go-cmp | v0.7.x | Deep equality in tests | Use in test assertions, NOT in production diff logic. More flexible than reflect.DeepEqual, with options to ignore fields, handle unexported types. |
| Azure/azure-sdk-for-go/sdk/storage/azblob | v1.6.0 | Azure Blob Storage client | AzureBlobBackend state storage. Same azidentity auth chain. Only import when blob backend is selected. |
| stretchr/testify | v1.11.x | Test assertions and mocks | `require` for fatal assertions, `assert` for non-fatal. `mock` for interface mocking (Graph client, Git backend). Standard in Go ecosystem. |
| olekukonko/tablewriter | v1.0.7 | Human-readable table output | CLI table formatting for `plan` output, `list` commands. Use v1.0.7+ (v1.0.0 had missing functionality). Supports ASCII, Unicode, Markdown output modes. |

### Development Tools

| Tool | Version | Purpose | Notes |
|------|---------|---------|-------|
| golangci-lint | v2.10.x | Linter aggregator | v2 has new config format (`linters.default` replaces `enable-all`/`disable-all`). Use `golangci-lint migrate` if starting from v1 config. Run `golangci-lint fmt` for formatting. |
| GoReleaser | v2.14.x (OSS) | Cross-platform binary distribution | Builds darwin/linux/windows binaries, generates checksums, publishes to Homebrew/Scoop. Use `.goreleaser.yaml` in repo root. OSS edition is sufficient (MIT project). |
| cobra-cli | latest | Scaffold new commands | `go install github.com/spf13/cobra-cli@latest`. One-time scaffolding tool, not a runtime dependency. |
| mockgen (go.uber.org/mock) | v0.5.x | Interface mock generation | Generate mocks for Graph client interface, Backend interface. Prefer uber fork over deprecated golang/mock. |

## Installation

```bash
# Initialize module
go mod init github.com/[org]/cactl

# Core dependencies
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get github.com/Azure/azure-sdk-for-go/sdk/azidentity@latest
go get github.com/microsoftgraph/msgraph-sdk-go@latest
go get github.com/go-git/go-git/v5@latest
go get github.com/Masterminds/semver/v3@latest
go get github.com/r3labs/diff/v3@latest
go get github.com/olekukonko/tablewriter@latest

# Azure Blob backend (optional, add when implementing)
go get github.com/Azure/azure-sdk-for-go/sdk/storage/azblob@latest

# Test dependencies
go get github.com/stretchr/testify@latest
go get github.com/google/go-cmp@latest
go get go.uber.org/mock/mockgen@latest

# Dev tools (install globally)
go install github.com/spf13/cobra-cli@latest
# golangci-lint: follow https://golangci-lint.run/docs/welcome/install/local/
# goreleaser: follow https://goreleaser.com/install/
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| spf13/cobra | urfave/cli/v2 | Never for this project. Cobra has deeper Viper integration and is what Terraform/kubectl use. urfave/cli is simpler but lacks command grouping and completion generation quality. |
| spf13/viper | koanf | If you need concurrent config access (viper is not goroutine-safe). For a CLI that loads config once at startup, viper's Cobra integration is unbeatable. |
| msgraph-sdk-go (typed) | Raw HTTP to Graph API | If the typed SDK's generated code is too heavy or has bugs for a specific endpoint. Keep raw HTTP as escape hatch but start with typed SDK -- it handles pagination, retry, and serialization. |
| go-git/go-git/v5 | os/exec shelling to git | If go-git cannot handle a specific git operation (e.g., advanced merge). For ref/tag CRUD which is all cactl needs, go-git is sufficient and avoids requiring git on PATH. |
| r3labs/diff/v3 | viant/godiff | If diffing becomes a performance bottleneck (godiff is ~5x faster). Unlikely for CA policies (tens to hundreds of objects, not millions). r3labs/diff has better changelog/patch semantics for plan/apply. |
| olekukonko/tablewriter | charmbracelet/lipgloss/table | If you want richer TUI styling (colors, borders, adaptive themes). tablewriter is more focused on data tables and has Markdown output mode which is useful for CI logs. |
| stretchr/testify | stdlib testing only | If you want zero test dependencies. testify's `require`/`assert` and `mock` packages save significant boilerplate for a project targeting 80% coverage. |
| golangci-lint v2 | golangci-lint v1 | Never. v1 is deprecated. v2 has migration tooling (`golangci-lint migrate`) and better defaults. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| msgraph-beta-sdk-go | Beta API has breaking changes without notice. CA policy endpoints are fully available in v1.0. | msgraph-sdk-go (v1.0 endpoint) |
| gopkg.in/src-d/go-git.v4 | Deprecated, relocated to go-git/go-git. Security vulnerabilities unfixed. | go-git/go-git/v5 |
| golang/mock (mockgen) | Deprecated. Repository archived. | go.uber.org/mock (active fork) |
| Azure/azure-storage-blob-go | Legacy SDK, replaced by azblob in the unified azure-sdk-for-go. | Azure/azure-sdk-for-go/sdk/storage/azblob |
| cobra v2.0.0 | Just released Dec 2025. Ecosystem adoption is still early. Breaking changes from v1 line. | cobra v1.10.x (stable, battle-tested) |
| reflect.DeepEqual in production | No diff output, panics on some types, no customization. | r3labs/diff for production, google/go-cmp for tests |
| joho/godotenv for config | Unnecessary complexity. Viper handles env vars natively via `AutomaticEnv()` and `BindEnv()`. | spf13/viper |

## Stack Patterns by Variant

**If running in CI/CD (GitHub Actions, Azure DevOps):**
- Use `WorkloadIdentityCredential` from azidentity (OIDC federation, no secrets)
- Set `--ci` flag to force JSON output and disable interactive prompts
- GoReleaser handles release artifact generation from CI

**If running interactively (developer workstation):**
- Use `DeviceCodeCredential` for user auth (no client secret needed)
- Table output with color by default, respect `--no-color` and `NO_COLOR` env var
- Config file at `.cactl/config.yaml` in project root, discovered via viper

**If using service principal (automated scripts):**
- Use `ClientSecretCredential` or `ClientCertificateCredential`
- Credentials via env vars: `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET` or `AZURE_CLIENT_CERTIFICATE_PATH`
- azidentity reads these automatically -- no custom env parsing needed

**If state backend is Git (default):**
- go-git for all ref/tag operations under `refs/cactl/*`
- Annotated tags for state snapshots with semver
- No dependency on git binary being installed

**If state backend is Azure Blob:**
- azblob client with same azidentity credential chain
- Blob leasing for concurrency control (prevents concurrent apply)
- Container per tenant, blob per state version

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| msgraph-sdk-go v1.96.x | azidentity v1.13.x | Both use azcore. Must use compatible azcore versions -- go mod handles this automatically. |
| cobra v1.10.x | viper v1.21.x | Designed to work together. `viper.BindPFlag()` accepts cobra pflag values. |
| go-git v5.17.x | Go 1.22+ | go-git v5.17 requires Go 1.22 minimum. Our Go 1.24+ floor satisfies this. |
| golangci-lint v2.10.x | Go 1.24+ | Supports same two latest Go versions as Go team policy. |
| GoReleaser v2.14.x | Go 1.24+ | Builds with any Go version; cross-compilation via `GOOS`/`GOARCH`. |

## Key Architecture Decisions Driven by Stack

1. **azidentity credential factory pattern:** Create a `NewCredential(cfg AuthConfig) (azcore.TokenCredential, error)` that switches on auth mode. All credential types implement `azcore.TokenCredential`, so the Graph client does not care which one is used.

2. **msgraph-sdk-go typed client:** Use the fluent API `client.Identity().ConditionalAccess().Policies()` for all CRUD. Wrap in a `GraphClient` interface for testability (mock the interface, not the SDK).

3. **go-git ref-based state:** Store state as JSON blobs in git objects, referenced by `refs/cactl/state/[tenant-id]`. Annotated tags for versioned snapshots. go-git's `Storer` interface allows in-memory testing.

4. **r3labs/diff for reconciliation:** Diff desired (from YAML files) vs actual (from Graph API) CA policy sets. The changelog drives the plan output and the apply execution.

5. **Viper config precedence:** `PersistentPreRunE` on root command loads config. Order: CLI flags -> env vars (prefixed `CACTL_`) -> `.cactl/config.yaml` -> defaults. This is the standard cobra+viper pattern.

## Sources

- [spf13/cobra on pkg.go.dev](https://pkg.go.dev/github.com/spf13/cobra) -- version and publication date verified (HIGH confidence)
- [spf13/cobra releases on GitHub](https://github.com/spf13/cobra/releases) -- v2.0.0 release confirmed Dec 2025 (HIGH confidence)
- [azidentity on pkg.go.dev](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity) -- v1.13.1 verified Nov 2025 (HIGH confidence)
- [Azure SDK releases](https://azure.github.io/azure-sdk/releases/latest/go.html) -- release cadence verified (HIGH confidence)
- [msgraph-sdk-go on pkg.go.dev](https://pkg.go.dev/github.com/microsoftgraph/msgraph-sdk-go) -- v1.96.0 verified Feb 2026 (HIGH confidence)
- [Microsoft Graph CA Policy API docs](https://learn.microsoft.com/en-us/graph/api/resources/conditionalaccesspolicy?view=graph-rest-1.0) -- v1.0 endpoint verified (HIGH confidence)
- [go-git/go-git on pkg.go.dev](https://pkg.go.dev/github.com/go-git/go-git/v5) -- v5.17.0 verified Feb 2026 (HIGH confidence)
- [spf13/viper on pkg.go.dev](https://pkg.go.dev/github.com/spf13/viper) -- v1.21.0 verified Sep 2025 (HIGH confidence)
- [Masterminds/semver on GitHub](https://github.com/Masterminds/semver) -- v3.4.0 verified (HIGH confidence)
- [r3labs/diff on pkg.go.dev](https://pkg.go.dev/github.com/r3labs/diff/v3) -- v3 branch confirmed active (MEDIUM confidence, exact version unverified)
- [azblob on pkg.go.dev](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/storage/azblob) -- v1.6.0 verified Jan 2026 (HIGH confidence)
- [golangci-lint releases](https://github.com/golangci/golangci-lint/releases) -- v2.10.1 verified Feb 2026 (HIGH confidence)
- [GoReleaser v2.14 announcement](https://goreleaser.com/blog/goreleaser-v2.14/) -- verified (HIGH confidence)
- [Go 1.26 release blog](https://go.dev/blog/go1.26) -- verified Feb 2026 (HIGH confidence)
- [stretchr/testify on pkg.go.dev](https://pkg.go.dev/github.com/stretchr/testify) -- v1.11.x verified Aug 2025 (HIGH confidence)
- [olekukonko/tablewriter on GitHub](https://github.com/olekukonko/tablewriter) -- v1.0.7 verified (MEDIUM confidence)
- [Cobra + Viper integration patterns](https://www.glukhov.org/post/2025/11/go-cli-applications-with-cobra-and-viper/) -- community pattern verified (MEDIUM confidence)

---
*Stack research for: cactl -- Entra Conditional Access policy deploy framework*
*Researched: 2026-03-04*
