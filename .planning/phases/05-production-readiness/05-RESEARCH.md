# Phase 5: Production Readiness - Research

**Researched:** 2026-03-04
**Domain:** Multi-tenant CLI orchestration, CI/CD integration, cross-platform binary distribution, code quality enforcement, documentation
**Confidence:** HIGH

## Summary

Phase 5 transforms cactl from a working single-tenant CLI into a production-ready, distributable tool. The work spans four distinct domains: (1) multi-tenant support requiring the `--tenant` flag to accept multiple values and execute commands sequentially per tenant with isolated credentials, (2) CI/CD integration providing non-interactive mode enforcement and example pipelines for GitHub Actions (OIDC workload identity) and Azure DevOps (SP certificate), (3) binary distribution via GoReleaser for five platform targets, and (4) code quality gates including golangci-lint v2, 80% test coverage on graph/reconcile packages, Conventional Commits, MIT license, and user documentation.

The existing codebase already has strong foundations for this phase. The `ClientFactory` already implements per-tenant credential isolation with RWMutex double-check locking. The `--ci` flag exists and is checked in the import command. The `--tenant` flag exists as a single string -- it needs to become a `StringSliceVar` to support multiple tenants. The Graph client is a concrete struct without an interface, which must be addressed for QUAL-02 mockability. No `.goreleaser.yaml`, `.golangci.yml`, or `LICENSE` file exists yet.

**Primary recommendation:** Structure this phase into four plans: (1) multi-tenant threading and sequential execution, (2) CI/CD mode enforcement and GoReleaser binary distribution, (3) code quality gates (linter, tests, coverage, license), and (4) documentation suite.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MTNT-01 | Tenant ID flows as explicit parameter through every layer (CLI -> auth -> Graph -> state) | Already implemented: `ClientFactory.Credential(ctx, tenantID)`, `graph.NewClient(cred, tenantID)`, `state.ReadManifest(backend, tenantID)`. Needs verification that all future commands (plan/apply/drift) thread tenant ID consistently. |
| MTNT-02 | --tenant flag accepts tenant ID or primary domain, supports multiple values | Currently `String` flag in root.go. Change to `StringSliceVar` on `PersistentFlags()`. Cobra/pflag supports both `--tenant a,b` and `--tenant a --tenant b` syntax natively. Domain-to-ID resolution deferred to Graph API lookup (not needed in v1 if user passes tenant ID). |
| MTNT-03 | Sequential multi-tenant apply in v1 (one tenant at a time) | Implement as `for _, tenant := range tenants { runCommand(ctx, tenant) }` loop in each command's RunE. Each iteration creates fresh credentials via ClientFactory. Aggregate exit codes: if any tenant returns exit code 1 (changes/drift), overall exit is 1; if any returns 2 (fatal), overall exit is 2. |
| MTNT-04 | Concurrent pipeline applies rejected with advisory error message | Advisory only -- not a locking mechanism. Detect concurrent execution via a lock file (`.cactl/.lock` with PID) or skip entirely for v1 and document the limitation. Simplest: print warning in docs, defer lock file to v1.1. |
| CICD-01 | --ci flag enables non-interactive mode, suppresses all prompts | Already partially implemented in import command (`ciMode` check). Extend to all commands: any `bufio.Scanner` / `fmt.Scan` call must check `--ci` first and return error if interactive input would be needed. |
| CICD-02 | --ci requires --auto-approve for write operations | Validation rule: if `--ci` is set and command is apply (write op), `--auto-approve` must also be set, else return ExitValidationError (code 3). Add this check in apply/rollback command RunE. |
| CICD-03 | GoReleaser builds for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 | `.goreleaser.yaml` v2 config with `CGO_ENABLED=0`, five goos/goarch combinations, ldflags for version injection, changelog with conventional commit filters. |
| CICD-04 | GitHub Actions example workflow with workload identity auth | `.github/workflows/cactl-plan.yml` using `azure/login@v2` with OIDC federation (`id-token: write` permission). Federated credential on Entra app registration with GitHub OIDC issuer. |
| CICD-05 | Azure DevOps example pipeline with SP cert auth | `examples/azure-devops/azure-pipelines.yml` using `AzureCLI@2` task with `addSpnToEnvironment: true`, mapping to `CACTL_CLIENT_ID`, `CACTL_CERT_PATH` env vars. |
| CICD-06 | Scheduled drift check example (daily cron with alert on exit code 1) | GitHub Actions `schedule: - cron: '0 6 * * *'` running `cactl drift --ci --tenant $TENANT` with step checking exit code and sending notification (e.g., creating GitHub issue or Slack webhook). |
| QUAL-01 | golangci-lint with default ruleset + exhaustive enum checks | `.golangci.yml` with `version: "2"`, enable `exhaustive` linter with `default-signifies-exhaustive: false`. Run via `golangci-lint run ./...`. |
| QUAL-02 | Table-driven unit tests with Graph client fully mockable via interface | Extract `GraphClient` interface from concrete `Client` struct in `internal/graph/`. Interface should declare `ListPolicies`, `GetPolicy`, and future write methods. Tests inject mock implementing this interface. |
| QUAL-03 | 80% test coverage target on internal/graph and internal/reconcile | Use `go test -coverprofile=cover.out -covermode=atomic ./internal/graph/... ./internal/reconcile/...` and `go tool cover -func=cover.out`. Enforce via CI step or `go-test-coverage` tool with `threshold.package: 80`. |
| QUAL-04 | Conventional Commits (feat:, fix:, chore:) for automatic CHANGELOG generation | GoReleaser changelog section with `filters.include: ["^feat:", "^fix:", "^chore:"]` and `groups` for categorization. Enforce in CI via commit message linting (commitlint or simple regex check in pre-commit hook). |
| QUAL-05 | MIT license | Create `LICENSE` file in repo root with MIT license text. |
| DOCS-01 | Getting started guide (install, init, first import, first plan/apply) | `docs/getting-started.md` covering binary install, `cactl init`, `cactl import --all`, `cactl plan`, `cactl apply`. |
| DOCS-02 | Multi-tenant usage guide | `docs/multi-tenant.md` covering `--tenant` flag with multiple values, per-tenant credential setup, sequential execution model. |
| DOCS-03 | CI/CD integration guide (GitHub Actions + Azure DevOps) | `docs/ci-cd.md` referencing example workflows from CICD-04 and CICD-05, explaining OIDC federation setup, SP cert auth, and scheduled drift checks. |
| DOCS-04 | README with badges, install instructions, quick start, architecture overview | `README.md` with CI badge, coverage badge, GoReleaser badge, MIT license badge. Quick install via `go install` or binary download. Architecture diagram from existing ARCHITECTURE.md research. |
</phase_requirements>

## Standard Stack

### Core

| Library/Tool | Version | Purpose | Why Standard |
|-------------|---------|---------|--------------|
| GoReleaser (OSS) | v2.14.x | Cross-platform binary distribution | De facto standard for Go binary releases. Generates checksums, publishes GitHub Releases, supports Homebrew taps. `.goreleaser.yaml` v2 format. |
| golangci-lint | v2.10.x | Linter aggregator with exhaustive enum checks | v2 is the current major version. New config format with `version: "2"`. Supports `exhaustive` linter for enum switch completeness. |
| go-test-coverage | latest | Coverage threshold enforcement | Lightweight tool for per-package coverage thresholds. Alternative: shell script parsing `go tool cover` output. |

### Supporting

| Library/Tool | Version | Purpose | When to Use |
|-------------|---------|---------|-------------|
| goreleaser/goreleaser-action | v7 | GitHub Actions GoReleaser integration | Release workflow on tag push. Requires `fetch-depth: 0` for full git history. |
| azure/login | v2 | GitHub Actions Azure OIDC login | CI/CD workflow authenticating to Azure via workload identity federation. |
| testifylint | latest | Linter for testify usage patterns | Enable in golangci-lint to catch common testify anti-patterns (require vs assert ordering, etc.) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| GoReleaser OSS | Manual `go build` + `GOOS`/`GOARCH` | GoReleaser handles checksums, archives, changelogs, GitHub Release creation. Manual builds require scripting all of this. Use GoReleaser. |
| go-test-coverage | Shell script threshold check | go-test-coverage supports per-package thresholds and exclusions. Shell script is simpler but less flexible. Either works for v1. |
| golangci-lint exhaustive | Manual switch auditing | Exhaustive catches missing enum cases at lint time. No alternative provides the same compile-time-like safety for switch statements. |
| Conventional Commits + GoReleaser changelog | go-semantic-release | go-semantic-release adds version bumping automation which may conflict with GoReleaser. GoReleaser already supports changelog filtering by conventional commit prefixes. Simpler to use GoReleaser alone. |

**Installation:**
```bash
# Dev tools (install globally)
# golangci-lint: follow https://golangci-lint.run/docs/welcome/install/local/
# goreleaser: go install github.com/goreleaser/goreleaser/v2@latest
# go-test-coverage: go install github.com/vladopajic/go-test-coverage/v2@latest
```

## Architecture Patterns

### Recommended Project Structure (Phase 5 additions)

```
cactl/
+-- .goreleaser.yaml           # GoReleaser v2 config
+-- .golangci.yml              # golangci-lint v2 config
+-- LICENSE                    # MIT license
+-- README.md                  # Project README with badges
+-- .github/
|   +-- workflows/
|       +-- ci.yml             # Lint + test + coverage on PR
|       +-- release.yml        # GoReleaser on tag push
+-- docs/
|   +-- getting-started.md     # DOCS-01
|   +-- multi-tenant.md        # DOCS-02
|   +-- ci-cd.md               # DOCS-03
+-- examples/
|   +-- github-actions/
|   |   +-- cactl-plan.yml     # CICD-04: workload identity
|   |   +-- cactl-drift.yml    # CICD-06: scheduled drift check
|   +-- azure-devops/
|       +-- azure-pipelines.yml # CICD-05: SP cert auth
```

### Pattern 1: Multi-Tenant Sequential Execution Loop

**What:** Each command that supports `--tenant` wraps its core logic in a for loop over tenant IDs. Each iteration gets a fresh credential from ClientFactory and creates a new Graph client.

**When to use:** Every command's RunE when `len(tenants) > 0`.

**Example:**
```go
// cmd/helpers.go or within each command
func runForTenants(ctx context.Context, tenants []string, factory *auth.ClientFactory, fn func(ctx context.Context, tenantID string, graphClient *graph.Client) error) error {
    var lastErr error
    exitCode := types.ExitSuccess

    for _, tenantID := range tenants {
        cred, err := factory.Credential(ctx, tenantID)
        if err != nil {
            // Log error, set exit code, continue to next tenant
            exitCode = types.ExitFatalError
            lastErr = err
            continue
        }
        graphClient := graph.NewClient(cred, tenantID)
        if err := fn(ctx, tenantID, graphClient); err != nil {
            var exitErr *types.ExitError
            if errors.As(err, &exitErr) && exitErr.Code > exitCode {
                exitCode = exitErr.Code
            }
            lastErr = err
        }
    }

    if exitCode > types.ExitSuccess {
        return &types.ExitError{Code: exitCode, Message: lastErr.Error()}
    }
    return nil
}
```

### Pattern 2: GraphClient Interface Extraction

**What:** Extract an interface from the concrete `graph.Client` struct to enable test mocking without httptest servers.

**When to use:** All packages that consume Graph client functionality.

**Example:**
```go
// internal/graph/client.go
type GraphClient interface {
    ListPolicies(ctx context.Context) ([]Policy, error)
    GetPolicy(ctx context.Context, policyID string) (*Policy, error)
    // Phase 3+ methods:
    // CreatePolicy(ctx context.Context, policy Policy) (*Policy, error)
    // UpdatePolicy(ctx context.Context, policyID string, policy Policy) error
    // DeletePolicy(ctx context.Context, policyID string) error
}

// Client implements GraphClient.
type Client struct { ... }

// Verify interface compliance at compile time.
var _ GraphClient = (*Client)(nil)
```

### Pattern 3: CI Mode Guard

**What:** A utility function that checks `--ci` mode before any interactive prompt, returning a validation error instead of blocking on stdin.

**When to use:** Every interactive prompt in the codebase.

**Example:**
```go
// internal/cli/ciguard.go or inline in commands
func requireNonInteractive(ciMode bool, operation string) error {
    if ciMode {
        return &types.ExitError{
            Code:    types.ExitValidationError,
            Message: fmt.Sprintf("--%s requires explicit flags in --ci mode (interactive prompts suppressed)", operation),
        }
    }
    return nil
}
```

### Anti-Patterns to Avoid

- **Global tenant state:** Never store the "current tenant" in a package-level variable. Thread tenant ID explicitly through every function call. The existing codebase already does this correctly.
- **Shared exit code across tenants:** Each tenant execution should track its own exit code. The aggregate exit code uses the highest severity (2 > 1 > 0).
- **Hardcoded platform list in build scripts:** Use GoReleaser's declarative `goos`/`goarch` lists rather than shell scripts with `GOOS=linux GOARCH=amd64 go build`. GoReleaser handles the matrix automatically.
- **Coverage gaming:** Do not write tests that exercise code paths without meaningful assertions just to hit 80%. Focus on testing the graph client interface mock interactions and reconcile engine truth table.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-platform binary builds | Shell scripts with GOOS/GOARCH matrix | GoReleaser `.goreleaser.yaml` | GoReleaser handles archives, checksums, changelogs, GitHub Releases, Homebrew taps. A shell script would need to replicate all of this. |
| Lint aggregation | Individual linter invocations | golangci-lint v2 | Runs 50+ linters in parallel with caching. Single config file. Faster than running each linter separately. |
| Coverage threshold enforcement | Custom shell parsing of `go tool cover` output | `go-test-coverage` or simple `go tool cover -func` with threshold script | Coverage output parsing is fragile. Dedicated tools handle edge cases (excluded files, per-package thresholds). |
| Changelog from commits | Custom git log parsing | GoReleaser changelog with conventional commit filters | GoReleaser already parses commits and groups by prefix. No need for a separate tool. |
| OIDC token exchange in CI | Custom HTTP calls to Entra token endpoint | `azure/login@v2` GitHub Action + azidentity `DefaultAzureCredential` | The action handles the OIDC dance. azidentity's `DefaultAzureCredential` automatically picks up the environment in CI. |

**Key insight:** Phase 5 is almost entirely about configuration files and wiring, not custom code. The heavy lifting is done by GoReleaser, golangci-lint, GitHub Actions, and Azure DevOps. The custom code is limited to: (1) `--tenant` flag change to StringSlice, (2) sequential execution loop, (3) GraphClient interface extraction, and (4) CI mode guards.

## Common Pitfalls

### Pitfall 1: StringSlice Flag Breaks Single-Tenant Usage

**What goes wrong:** Changing `--tenant` from `String` to `StringSlice` breaks existing usage patterns. `v.GetString("tenant")` returns empty when the underlying type is a string slice.
**Why it happens:** Viper's type coercion does not automatically convert between `String` and `StringSlice`.
**How to avoid:** Use `v.GetStringSlice("tenant")` everywhere. For backward compatibility, if a single string is provided without commas, pflag's StringSlice still returns a single-element slice. Test both `--tenant abc` and `--tenant abc,def` syntax.
**Warning signs:** Empty tenant ID in commands that previously worked.

### Pitfall 2: GoReleaser Requires Full Git History

**What goes wrong:** GoReleaser fails in CI with "git log" errors or produces empty changelogs.
**Why it happens:** `actions/checkout@v4` defaults to `fetch-depth: 1` (shallow clone). GoReleaser needs the full history to compute changelog between tags.
**How to avoid:** Always set `fetch-depth: 0` in the checkout step of the release workflow.
**Warning signs:** "could not find previous tag" error in GoReleaser output.

### Pitfall 3: golangci-lint v2 Config Format Incompatibility

**What goes wrong:** golangci-lint fails to parse config file or ignores linter settings.
**Why it happens:** v2 changed the config format. `enable-all`/`disable-all` became `linters.default: all`. The `run.go` key moved. `issues.exclude-rules` format changed.
**How to avoid:** Use `version: "2"` at the top of `.golangci.yml`. Run `golangci-lint migrate` if starting from a v1 config. Test with `golangci-lint run --config .golangci.yml ./...` locally before CI.
**Warning signs:** "unknown linter" or "deprecated configuration" warnings.

### Pitfall 4: OIDC Federated Credential Subject Mismatch

**What goes wrong:** GitHub Actions workflow fails with `AADSTS70021: No matching federated identity record found`.
**Why it happens:** The `subject` claim in the federated credential must exactly match the GitHub OIDC token's subject, including case. For branch-based subjects, it's `repo:org/repo:ref:refs/heads/main`. For environment-based, it's `repo:org/repo:environment:production`.
**How to avoid:** Document the exact subject format in the CI/CD guide. Use environment-based subjects for production deployments.
**Warning signs:** Auth failures only in CI, not locally.

### Pitfall 5: Coverage Measurement Scope Confusion

**What goes wrong:** Coverage reports show low numbers because tests in `internal/graph` only cover `internal/graph`, not `internal/reconcile`, and vice versa.
**Why it happens:** `go test -cover ./internal/graph/...` measures coverage of `internal/graph` by tests in `internal/graph`. Integration tests in `cmd/` that exercise graph code don't count toward graph package coverage.
**How to avoid:** Use `-coverpkg=./internal/graph/...,./internal/reconcile/...` to measure cross-package coverage. Or focus per-package: ensure each package's own tests cover 80% of that package.
**Warning signs:** Passing tests but low coverage numbers.

### Pitfall 6: Exit Code Aggregation in Multi-Tenant Mode

**What goes wrong:** Tool exits with code 0 even though one of three tenants had drift detected (code 1).
**Why it happens:** Naive error handling returns the last tenant's result, not the worst-case.
**How to avoid:** Track the maximum exit code across all tenant executions. Return the highest severity code: fatal (2) > changes/drift (1) > success (0).
**Warning signs:** CI pipeline reports success when one tenant has drift.

## Code Examples

### GoReleaser Config (`.goreleaser.yaml`)

```yaml
# Source: https://goreleaser.com/customization/builds/go/
version: 2

project_name: cactl

before:
  hooks:
    - go mod tidy

builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.CommitDate}}
    flags:
      - -trimpath

archives:
  - formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  groups:
    - title: Features
      regexp: '^feat(\(.+\))?:'
      order: 0
    - title: Bug Fixes
      regexp: '^fix(\(.+\))?:'
      order: 1
    - title: Others
      order: 999
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore\\(deps\\):"

release:
  github:
    owner: seb07-cloud
    name: cactl
```

### golangci-lint Config (`.golangci.yml`)

```yaml
# Source: https://golangci-lint.run/docs/configuration/file/
version: "2"

linters:
  default: standard
  enable:
    - exhaustive
    - testifylint
    - errorlint
    - gocritic
    - gosec
    - prealloc
  settings:
    exhaustive:
      check:
        - switch
        - map
      default-signifies-exhaustive: false
    testifylint:
      enable-all: true

formatters:
  enable:
    - gofmt
    - goimports

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

### GitHub Actions CI Workflow (`.github/workflows/ci.yml`)

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: golangci/golangci-lint-action@v6
        with:
          version: v2.10

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go test -race -coverprofile=cover.out -covermode=atomic ./...
      - name: Check coverage threshold
        run: |
          COVERAGE=$(go tool cover -func=cover.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "Total coverage: ${COVERAGE}%"
          # Per-package check for graph and reconcile
          for pkg in internal/graph internal/reconcile; do
            PKG_COV=$(go tool cover -func=cover.out | grep "$pkg" | awk '{sum+=$3; n++} END {if(n>0) print sum/n; else print 0}')
            echo "$pkg coverage: ${PKG_COV}%"
          done
```

### GitHub Actions Release Workflow (`.github/workflows/release.yml`)

```yaml
# Source: https://goreleaser.com/ci/actions/
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### GitHub Actions OIDC Workflow Example (`examples/github-actions/cactl-plan.yml`)

```yaml
# Source: https://docs.github.com/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-azure
name: "cactl plan"

on:
  pull_request:
    paths:
      - "policies/**"

permissions:
  id-token: write
  contents: read

jobs:
  plan:
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Azure Login (OIDC)
        uses: azure/login@v2
        with:
          client-id: ${{ vars.AZURE_CLIENT_ID }}
          tenant-id: ${{ vars.AZURE_TENANT_ID }}
          subscription-id: ${{ vars.AZURE_SUBSCRIPTION_ID }}

      - name: Install cactl
        run: |
          curl -sL https://github.com/seb07-cloud/cactl/releases/latest/download/cactl_linux_amd64.tar.gz | tar xz
          sudo mv cactl /usr/local/bin/

      - name: Run plan
        run: cactl plan --ci --tenant ${{ vars.AZURE_TENANT_ID }}
```

### Azure DevOps Pipeline Example (`examples/azure-devops/azure-pipelines.yml`)

```yaml
# Source: https://christosmonogios.com/2024/08/11/The-Complete-Guide-On-How-To-Access-Azure-Key-Vault-Secrets-Using-A-Service-Principle-With-Certificate-Within-An-Azure-DevOps-Pipeline/
trigger:
  branches:
    include:
      - main
  paths:
    include:
      - policies/*

pool:
  vmImage: ubuntu-latest

variables:
  - group: cactl-credentials

steps:
  - task: AzureCLI@2
    displayName: "Run cactl plan"
    inputs:
      azureSubscription: "cactl-service-connection"
      scriptType: bash
      addSpnToEnvironment: true
      scriptLocation: inlineScript
      inlineScript: |
        export CACTL_CLIENT_ID=$servicePrincipalId
        export CACTL_CERT_PATH=$(CACTL_CERT_PATH)
        export CACTL_AUTH_MODE=client-certificate
        cactl plan --ci --tenant $(AZURE_TENANT_ID)
```

### StringSlice Tenant Flag

```go
// cmd/root.go -- change from String to StringSlice
func init() {
    rootCmd.PersistentFlags().StringSlice("tenant", nil, "Entra tenant ID(s) or primary domain(s)")
    // ... other flags unchanged
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| golangci-lint v1 config | golangci-lint v2 config (`version: "2"`) | 2025 | New YAML structure: `linters.default`, `linters.enable`, `linters.settings`. Must use v2 format. |
| GoReleaser v1 `format:` (singular) | GoReleaser v2 `formats:` (plural) | 2024 | YAML key renamed. Old key causes parse error. |
| Azure SP secret for CI/CD | Workload identity federation (OIDC) | 2023-2024 GA | No secrets to rotate. GitHub Actions uses `azure/login@v2` with `id-token: write` permission. Recommended for all new setups. |
| `golang/mock` for mockgen | `go.uber.org/mock` | 2023 | golang/mock repository archived. uber fork is the active maintained version. |
| Manual coverage scripts | `go-test-coverage` tool | 2024 | Per-package thresholds, exclusion patterns, CI integration. But simple shell scripts still work fine. |

**Deprecated/outdated:**
- `golangci-lint` v1 config format: Still parsed with warnings but will be removed in future v2 releases
- `azure/login@v1`: Does not support OIDC federation; must use `v2`
- `goreleaser/goreleaser-action@v5`: Superseded by v7; v5 does not support GoReleaser v2

## Open Questions

1. **Domain-to-tenant-ID resolution for MTNT-02**
   - What we know: The requirement says "--tenant flag accepts tenant ID or primary domain"
   - What's unclear: Whether to resolve domain to tenant ID via Graph API (`/organization` endpoint) or accept domain as-is and let azidentity resolve it. azidentity accepts both tenant ID (GUID) and domain in the `TenantID` field.
   - Recommendation: Accept both formats and pass directly to azidentity, which handles resolution. Document that tenant ID (GUID) is preferred for reliability. LOW priority -- azidentity handles this transparently.

2. **MTNT-04 concurrent pipeline rejection mechanism**
   - What we know: Requirement says "advisory error message" -- not a hard lock
   - What's unclear: Whether to implement a lock file, use Git refs as a lock, or simply document the limitation
   - Recommendation: For v1, add a documentation note that concurrent `cactl apply` against the same tenant from multiple pipelines is not safe. No code-level enforcement needed for advisory-only. Defer lock file to v1.1 where blob lease is planned anyway.

3. **Coverage measurement scope for QUAL-03**
   - What we know: 80% target on `internal/graph` and `internal/reconcile`
   - What's unclear: Phase 3 and 4 create `internal/reconcile` -- it does not exist yet. Coverage requirement applies to whatever state the code is in after all prior phases complete.
   - Recommendation: Create the golangci-lint and coverage CI jobs now. They will initially apply to `internal/graph` (which exists) and `internal/reconcile` will be checked once Phase 3 creates it. Use per-package threshold configuration.

4. **Conventional Commits enforcement timing**
   - What we know: QUAL-04 requires conventional commits
   - What's unclear: Whether to enforce retroactively on existing commits or start enforcing from this phase forward
   - Recommendation: Enforce going forward only. Existing commits already follow conventional format (e.g., `feat(02-03): ...`). Add a commit message lint check to CI (simple regex in GitHub Actions step) and document the convention.

## Sources

### Primary (HIGH confidence)
- [GoReleaser Official Docs - Builds](https://goreleaser.com/customization/builds/go/) -- GoReleaser v2 build configuration
- [GoReleaser Official Docs - GitHub Actions](https://goreleaser.com/ci/actions/) -- CI workflow integration
- [GoReleaser Official Docs - Changelog](https://goreleaser.com/customization/changelog/) -- Conventional commit changelog filtering
- [golangci-lint Configuration File](https://golangci-lint.run/docs/configuration/file/) -- v2 config format
- [golangci-lint Linter Settings](https://golangci-lint.run/docs/linters/configuration/) -- exhaustive linter config
- [GitHub Docs - Configuring OIDC in Azure](https://docs.github.com/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-azure) -- GitHub Actions OIDC federation
- [Microsoft Learn - Workload Identity Federation](https://learn.microsoft.com/en-us/entra/workload-id/workload-identity-federation) -- Entra OIDC federation
- [goreleaser/goreleaser-action on GitHub](https://github.com/goreleaser/goreleaser-action) -- v7 action, fetch-depth requirement

### Secondary (MEDIUM confidence)
- [go-test-coverage](https://github.com/vladopajic/go-test-coverage) -- Coverage threshold tool
- [Azure DevOps SP Certificate Auth](https://christosmonogios.com/2024/08/11/The-Complete-Guide-On-How-To-Access-Azure-Key-Vault-Secrets-Using-A-Service-Principle-With-Certificate-Within-An-Azure-DevOps-Pipeline/) -- Certificate-based auth pattern
- [Cobra Enterprise Guide](https://cobra.dev/docs/explanations/enterprise-guide/) -- Multi-value flag patterns
- [Azure DevOps Workload Identity Federation](https://devblogs.microsoft.com/devops/public-preview-of-workload-identity-federation-for-azure-pipelines/) -- Modern alternative to SP cert

### Tertiary (LOW confidence)
- [Conventional Commits + GoReleaser changelog integration patterns](https://github.com/goreleaser/chglog) -- chglog library, not directly needed if using GoReleaser's built-in changelog

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- GoReleaser, golangci-lint, and GitHub Actions are verified with official docs
- Architecture: HIGH -- Multi-tenant sequential execution pattern is straightforward; existing ClientFactory already supports it
- Pitfalls: HIGH -- GoReleaser fetch-depth, golangci-lint v2 migration, OIDC subject mismatch are well-documented issues
- Code quality: HIGH -- golangci-lint v2 config and coverage tooling verified with official sources
- CI/CD: MEDIUM -- Azure DevOps pipeline examples are less standardized than GitHub Actions; patterns verified but not from Microsoft official docs

**Research date:** 2026-03-04
**Valid until:** 2026-04-04 (tools are stable; GoReleaser and golangci-lint release monthly but config format is stable)
