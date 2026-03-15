# cactl

**The deploy framework for Microsoft Entra Conditional Access policies.**

![CI](https://github.com/seb07-cloud/cactl/actions/workflows/ci.yml/badge.svg)
![Release](https://img.shields.io/github/v/release/seb07-cloud/cactl)
![Go](https://img.shields.io/github/go-mod/go-version/seb07-cloud/cactl)
![License](https://img.shields.io/badge/license-MIT-blue.svg)

---

A misconfigured Conditional Access policy can lock out thousands of users in seconds -- with no version history to roll back to. cactl prevents this by bringing plan/apply discipline, Git-native versioning, and per-policy semantic versioning to Entra ID.

```
cactl plan --tenant contoso.onmicrosoft.com

Tenant: a793bcba-...

  ~ require-mfa-all-users  v1.0.0 → v2.0.0  ⚠ scope-affecting
    conditions.users.excludeGroups:
      - "8f2e1a..."  (Break-Glass Operators)
    state: "enabled" → "enabledForReportingButNotEnforced"

  + block-legacy-auth  (new)

Plan: 1 to create, 1 to update, 0 unchanged.
```

## Why cactl?

| Problem | How cactl solves it |
|---|---|
| No change preview | `cactl plan` shows a Terraform-style diff before anything touches Entra |
| No version history | Every deploy creates a semver-tagged Git object -- full audit trail, zero working-tree noise |
| No rollback | `cactl rollback --policy <slug> --version <semver>` restores any prior version in seconds |
| Portal drift | `cactl drift` detects out-of-band changes and suggests remediation |
| Manual, error-prone CI | `--ci --auto-approve` with structured exit codes (0/1/2/3) plugs directly into GitHub Actions or Azure DevOps |

## Quick Start

```bash
# Install
go install github.com/seb07-cloud/cactl@latest

# Scaffold workspace
cactl init

# Import live policies into Git
cactl import --all --tenant YOUR_TENANT_ID

# Edit a policy JSON, then preview changes
cactl plan --tenant YOUR_TENANT_ID

# Deploy
cactl apply --tenant YOUR_TENANT_ID
```

## Install

**Go install:**

```bash
go install github.com/seb07-cloud/cactl@latest
```

**Binary download (Linux / macOS):**

```bash
# amd64
curl -sL https://github.com/seb07-cloud/cactl/releases/latest/download/cactl_$(uname -s | tr '[:upper:]' '[:lower:]')_amd64.tar.gz | tar xz
sudo mv cactl /usr/local/bin/

# Apple Silicon
curl -sL https://github.com/seb07-cloud/cactl/releases/latest/download/cactl_darwin_arm64.tar.gz | tar xz
sudo mv cactl /usr/local/bin/
```

**Windows:** download the latest `.zip` from [Releases](https://github.com/seb07-cloud/cactl/releases) and add `cactl.exe` to your `PATH`.

## Commands

| Command | Description |
|---|---|
| `cactl init` | Scaffold a workspace (`.cactl/config.yaml`, `.gitignore`, JSON schema) |
| `cactl import` | Fetch live CA policies from Entra and write to Git state |
| `cactl plan` | Compute and display a diff of local vs. live policies |
| `cactl apply` | Apply planned changes to Entra with confirmation prompts |
| `cactl drift` | Detect out-of-band changes (read-only) |
| `cactl rollback` | Restore a policy to a previous semver from Git tag history |
| `cactl status` | Show tracked policies with sync state (in-sync / drifted / missing) |
| `cactl history` | View version history for all or a single policy |
| `cactl test` | Evaluate YAML test scenarios against local policy files (no API calls) |

All commands accept `--tenant`, `--output human|json`, `--no-color`, `--ci`, `--auto-approve`, and `--log-level`.

## How It Works

### Plan / Apply

```
 Local JSON files ──┐
                     ├─ Reconcile ─── Plan ─── Apply ─── Tag + Update Manifest
 Live Entra state ──┘
```

1. **Desired state** is defined in `policies/<tenant-id>/<slug>.json` files you edit and commit.
2. **Live state** is fetched from the Graph API, normalized (server-only fields stripped, nulls removed, keys sorted).
3. The **reconciler** diffs the two and produces actions: `create`, `update`, `recreate`, or `noop`.
4. **Semantic version bumps** are computed per action based on which fields changed:
   - **MAJOR** -- scope-affecting fields (`includeUsers`, `excludeGroups`, `includeApplications`, `state`)
   - **MINOR** -- behavioral fields (`conditions`, `grantControls`, `sessionControls`)
   - **PATCH** -- cosmetic fields (`displayName`)
5. **Validations** run before apply: break-glass account exclusion checks, conflicting include/exclude, overly-broad policies.
6. On apply, changes are written via Graph API, then recorded as annotated Git tags and blob refs.

### Git-Native State

cactl stores all state inside the Git repository itself -- no external backend, no state files in your working tree.

| What | Where | Why |
|---|---|---|
| Policy blobs | `refs/cactl/tenants/<id>/policies/<slug>` | Custom ref namespace -- invisible to branches and worktree |
| Version history | Annotated tags `cactl/<tenant>/<slug>/<semver>` | Immutable audit trail, native `git tag` browsing |
| Manifest | `refs/cactl/tenants/<id>/manifest` | Maps slug to live Entra Object ID, deployed version, timestamp |

Push state to your remote with a single refspec: `+refs/cactl/*:refs/cactl/*`.

### Drift Detection

```bash
cactl drift --tenant YOUR_TENANT_ID
```

Compares local state against live Entra policies without making changes. Exits with code `1` if drift is found, `0` if clean. Designed for scheduled CI runs that open an issue or notify a channel on drift.

### Rollback

```bash
# Direct: restore a specific version
cactl rollback --policy require-mfa-all-users --version 1.0.0

# Interactive: TUI wizard to browse versions, view diffs, and restore
cactl rollback -i
```

Rollback creates a **new forward version** (never rewrites history), preserving a complete audit trail.

### Policy Testing

```bash
cactl test tests/contoso/*.yaml
```

Evaluate sign-in scenarios against local policy files without calling the Graph API. Test specs are YAML files that define a sign-in context (user, groups, app, platform, risk level) and an expected outcome (block, grant, notApplicable, required controls).

## Authentication

cactl supports multiple auth modes, auto-detected in this order:

| Mode | When to use | Configuration |
|---|---|---|
| Azure CLI | Local development | `az login`, then run cactl |
| Client secret | CI with secret-based SP | `CACTL_CLIENT_ID` + `CACTL_CLIENT_SECRET` |
| Client certificate | CI with cert-based SP | `CACTL_CLIENT_ID` + `CACTL_CERT_PATH` |
| Workload identity | GitHub Actions OIDC | Federated credential on the app registration |

Override explicitly with `--auth-mode az-cli|client-secret|client-certificate`.

## Configuration

cactl resolves configuration in priority order: **CLI flags > environment variables > `.cactl/config.yaml` > auto-detection**.

```yaml
# .cactl/config.yaml (created by cactl init, gitignored)
tenant: "a793bcba-c7d6-4169-8e47-79683ee10349"
auth:
  mode: "az-cli"
output: human
log_level: info
```

Secrets (`client_secret`, `cert_path`) are only read from environment variables -- never from the config file.

| Environment Variable | Purpose |
|---|---|
| `CACTL_TENANT` | Default tenant ID |
| `CACTL_CLIENT_ID` | App registration client ID |
| `CACTL_CLIENT_SECRET` | Client secret for SP auth |
| `CACTL_CERT_PATH` | Path to PEM certificate for SP auth |
| `CACTL_OUTPUT` | Output format (`human` or `json`) |
| `CACTL_NO_COLOR` | Disable ANSI colors |
| `CACTL_CI` | Enable CI mode |

## Required Permissions

The app registration or service principal needs these Microsoft Graph API permissions:

| Permission | Type | Purpose |
|---|---|---|
| `Policy.Read.All` | Application | Read CA policies |
| `Policy.ReadWrite.ConditionalAccess` | Application | Create, update, delete CA policies |
| `Application.Read.All` | Application | Resolve app GUIDs to display names |
| `Group.Read.All` | Application | Resolve group GUIDs to display names |
| `RoleManagement.Read.Directory` | Application | Resolve role GUIDs to display names |

## CI/CD

cactl is built for pipelines. Use `--ci --auto-approve` for non-interactive deploys with structured exit codes:

| Exit Code | Meaning | Pipeline action |
|---|---|---|
| `0` | No changes | Pass |
| `1` | Changes detected / drift found | Alert or create PR |
| `2` | Fatal error (auth, network) | Fail build |
| `3` | Validation error | Fail build |

Example workflow files are included for [GitHub Actions](examples/github-actions/) and [Azure DevOps](examples/azure-devops/).

```yaml
# GitHub Actions -- plan on PR, apply on merge
- uses: azure/login@v2
  with:
    client-id: ${{ vars.AZURE_CLIENT_ID }}
    tenant-id: ${{ vars.AZURE_TENANT_ID }}
    subscription-id: ${{ vars.AZURE_SUBSCRIPTION_ID }}

- run: cactl plan --ci --tenant ${{ vars.AZURE_TENANT_ID }}
```

## Multi-Tenant

Pass multiple tenant IDs to process them sequentially with isolated credentials:

```bash
cactl plan --tenant TENANT_A --tenant TENANT_B
```

The overall exit code is the highest severity across all tenants. Fatal errors (exit 2/3) stop execution immediately.

See the [Multi-Tenant Guide](docs/multi-tenant.md) for credential isolation patterns and recommended repo structures.

## Documentation

| Guide | Description |
|---|---|
| [Getting Started](docs/getting-started.md) | Install, authenticate, import, plan, apply |
| [CI/CD Integration](docs/ci-cd.md) | GitHub Actions (OIDC), Azure DevOps, scheduled drift |
| [Multi-Tenant](docs/multi-tenant.md) | Credential isolation, repo-per-tenant patterns |

## License

[MIT](LICENSE)
