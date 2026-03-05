# cactl

CLI deploy framework for Microsoft Entra Conditional Access policies.

![CI](https://github.com/seb07-cloud/cactl/actions/workflows/ci.yml/badge.svg) ![Release](https://img.shields.io/github/v/release/seb07-cloud/cactl) ![License](https://img.shields.io/badge/license-MIT-blue.svg) ![Go](https://img.shields.io/github/go-mod/go-version/seb07-cloud/cactl)

## What is cactl?

cactl is a CLI-first, open-source deploy framework for Microsoft Entra Conditional Access policies. It gives identity architects and IAM teams a trusted deployment gate with plan/apply safety, Git-native versioning, and semantic versioning per policy.

cactl exists to prevent the "Friday afternoon problem" -- a misconfigured Conditional Access policy locking out thousands of users with no version history to recover from. With cactl, every change is versioned, diffable, and reversible.

## Key Features

- **Plan/Apply safety** -- Preview changes before deploying, just like Terraform
- **Git-native versioning** -- State stored in Git refs and annotated tags, zero working tree pollution
- **Semantic versioning per policy** -- MAJOR/MINOR/PATCH bumps based on field-level change severity
- **Multi-tenant support** -- Sequential per-tenant execution with isolated credentials
- **Drift detection** -- Scheduled or on-demand comparison of local state against live Entra policies
- **Rollback** -- Restore any policy to a previous version from its annotated tag history
- **CI/CD ready** -- Non-interactive `--ci` mode with `--auto-approve` for pipelines
- **Single Go binary** -- No runtime dependencies, cross-platform (Linux, macOS, Windows)

## Quick Install

**Binary download (Linux/macOS):**

```bash
curl -sL https://github.com/seb07-cloud/cactl/releases/latest/download/cactl_$(uname -s | tr '[:upper:]' '[:lower:]')_amd64.tar.gz | tar xz && sudo mv cactl /usr/local/bin/
```

**Go install:**

```bash
go install github.com/seb07-cloud/cactl@latest
```

**Verify:**

```bash
cactl --version
```

## Quick Start

```bash
# 1. Scaffold a new workspace
cactl init

# 2. Import all live policies from your tenant
cactl import --all --tenant YOUR_TENANT_ID

# 3. Edit policy JSON files in your editor
#    (e.g., change state from "enabled" to "enabledForReportingButNotEnforced")

# 4. Preview what will change
cactl plan --tenant YOUR_TENANT_ID

# 5. Deploy changes
cactl apply --tenant YOUR_TENANT_ID
```

## Architecture

cactl follows a pipeline model: CLI commands orchestrate authentication, Graph API calls, normalization, state management, and Git operations.

```
CLI (cobra) -> Auth (azidentity) -> Graph API -> Normalize -> Reconcile -> State (Git refs/tags)
```

**Key packages:**

| Package | Purpose |
|---------|---------|
| `cmd/` | Cobra command definitions (init, import, plan, apply, drift, rollback, status) |
| `internal/auth/` | Authentication provider factory (CLI, SP secret, SP cert, workload identity) |
| `internal/graph/` | Microsoft Graph API client for CA policy CRUD |
| `internal/normalize/` | JSON normalization (strip server fields, remove nulls, alphabetize) |
| `internal/reconcile/` | Diff engine producing plan actions (create, update, delete, recreate, noop) |
| `internal/state/` | Git-backed state storage (refs/cactl/* namespace, annotated tags) |
| `internal/resolve/` | GUID-to-display-name resolution for human-readable output |
| `internal/semver/` | Per-policy semantic versioning with field-level bump rules |
| `pkg/types/` | Shared types (Policy, Config, ExitCodes) |

## Documentation

- [Getting Started](docs/getting-started.md) -- Install, configure, and deploy your first policy
- [Multi-Tenant Guide](docs/multi-tenant.md) -- Managing policies across multiple Entra tenants
- [CI/CD Integration](docs/ci-cd.md) -- GitHub Actions, Azure DevOps, and scheduled drift checks

## Required Permissions

cactl requires the following Microsoft Graph API permissions on your app registration or service principal:

| Permission | Type | Purpose |
|------------|------|---------|
| `Policy.Read.All` | Application | Read Conditional Access policies |
| `Policy.ReadWrite.ConditionalAccess` | Application | Create, update, and delete CA policies |
| `Application.Read.All` | Application | Resolve application GUIDs to display names |
| `Group.Read.All` | Application | Resolve group GUIDs to display names |
| `RoleManagement.Read.Directory` | Application | Resolve directory role GUIDs to display names |

## License

MIT -- see [LICENSE](LICENSE)
