# Getting Started with cactl

This guide walks you through installing cactl, configuring authentication, and deploying your first Conditional Access policy change.

## Install

### Binary download (Linux/macOS)

```bash
curl -sL https://github.com/seb07-cloud/cactl/releases/latest/download/cactl_$(uname -s | tr '[:upper:]' '[:lower:]')_amd64.tar.gz | tar xz
sudo mv cactl /usr/local/bin/
```

For Apple Silicon (M1/M2/M3), use `arm64` instead of `amd64`:

```bash
curl -sL https://github.com/seb07-cloud/cactl/releases/latest/download/cactl_darwin_arm64.tar.gz | tar xz
sudo mv cactl /usr/local/bin/
```

### Windows

Download the latest `.zip` from [Releases](https://github.com/seb07-cloud/cactl/releases) and add `cactl.exe` to your PATH.

### Go install

```bash
go install github.com/seb07-cloud/cactl@latest
```

### Verify

```bash
cactl --version
```

## Initialize workspace

Run `cactl init` to scaffold a new workspace:

```bash
cactl init
```

This creates:

- `.cactl/config.yaml` -- workspace configuration (tenant ID, auth settings)
- `.gitignore` entry -- ensures `.cactl/config.yaml` is never committed (it may contain references to credentials)
- Fetches the latest JSON Schema for CA policy validation

## Configure authentication

cactl supports three authentication modes. Choose the one that fits your environment.

### Azure CLI (interactive development)

The simplest approach for local development. Sign in with the Azure CLI, then run cactl:

```bash
az login
cactl import --all --tenant YOUR_TENANT_ID
```

cactl automatically detects your Azure CLI session and uses it for Graph API calls.

### Service principal with client secret

Set environment variables for non-interactive authentication:

```bash
export CACTL_TENANT_ID=YOUR_TENANT_ID
export CACTL_CLIENT_ID=YOUR_APP_CLIENT_ID
export CACTL_CLIENT_SECRET=YOUR_CLIENT_SECRET
```

Then run any cactl command:

```bash
cactl import --all --tenant YOUR_TENANT_ID
```

### Service principal with certificate

For certificate-based authentication (recommended for production and CI/CD):

```bash
export CACTL_TENANT_ID=YOUR_TENANT_ID
export CACTL_CLIENT_ID=YOUR_APP_CLIENT_ID
export CACTL_CERT_PATH=/path/to/certificate.pem
```

The certificate file should contain both the certificate and private key in PEM format.

## First import

Import all live Conditional Access policies from your tenant:

```bash
cactl import --all --tenant YOUR_TENANT_ID
```

Expected output:

```
Importing policies from tenant YOUR_TENANT_ID...
  + require-mfa-all-users          (imported)
  + block-legacy-auth               (imported)
  + require-compliant-device        (imported)

Imported 3 policies.
```

Each policy is saved as a normalized JSON file named after its slug (kebab-case of the display name). The state manifest maps each slug to its live Entra Object ID.

## First plan

Edit a policy file to make a change. For example, switch a policy from enforcement to report-only:

```bash
# Open the policy file in your editor
vim require-mfa-all-users.json
```

Change `"state": "enabled"` to `"state": "enabledForReportingButNotEnforced"`, then run:

```bash
cactl plan --tenant YOUR_TENANT_ID
```

Expected output:

```
Tenant: YOUR_TENANT_ID

  ~ require-mfa-all-users  v1.0.0 -> v1.1.0
    state: "enabled" -> "enabledForReportingButNotEnforced"

Plan: 0 to create, 1 to update, 0 to delete.
```

The `~` sigil indicates an update. cactl shows the semantic version bump and the specific field changes.

## First apply

Deploy the planned changes:

```bash
cactl apply --tenant YOUR_TENANT_ID
```

cactl shows the plan and asks for confirmation:

```
Tenant: YOUR_TENANT_ID

  ~ require-mfa-all-users  v1.0.0 -> v1.1.0
    state: "enabled" -> "enabledForReportingButNotEnforced"

Apply 1 change? (yes/no): yes

  ~ require-mfa-all-users  v1.1.0  (applied)

Applied 1 change.
```

The new version is tagged in Git and the state manifest is updated.

## What's next

- [Multi-Tenant Guide](multi-tenant.md) -- Manage policies across multiple Entra tenants
- [CI/CD Integration](ci-cd.md) -- Automate plan/apply in GitHub Actions or Azure DevOps
