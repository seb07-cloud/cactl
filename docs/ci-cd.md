# CI/CD Integration Guide

cactl is designed for CI/CD pipelines. This guide covers non-interactive mode, authentication patterns for GitHub Actions and Azure DevOps, and scheduled drift detection.

## Overview

Use the `--ci` flag to run cactl in non-interactive (CI) mode:

```bash
cactl plan --ci --tenant YOUR_TENANT_ID
cactl apply --ci --auto-approve --tenant YOUR_TENANT_ID
```

## CI mode behavior

The `--ci` flag changes cactl's behavior for pipeline environments:

| Behavior | Without `--ci` | With `--ci` |
|----------|---------------|-------------|
| Confirmation prompts | Interactive yes/no | Suppressed |
| Write operations (apply, rollback) | Prompt for confirmation | Requires `--auto-approve` flag |
| Import | Interactive selection | Requires `--all` or `--policy` flag |
| Color output | Auto-detected | Disabled (unless `--color` forced) |

**Key rule:** `--ci` alone does not authorize write operations. You must also pass `--auto-approve` for `apply` and `rollback`. Without it, cactl exits with code 2.

## Exit code contract

All cactl commands follow a consistent exit code contract for pipeline integration:

| Exit Code | Meaning | Pipeline Action |
|-----------|---------|-----------------|
| 0 | Success, no changes | Pass |
| 1 | Changes detected (plan) or drift found (drift) | Alert / create PR |
| 2 | Fatal error (auth failure, network error, missing --auto-approve) | Fail build |
| 3 | Validation error (invalid policy JSON, schema violation) | Fail build |

Use these exit codes to drive pipeline logic (e.g., create an issue on exit 1 from drift, fail the build on exit 2 or 3).

## GitHub Actions with OIDC (workload identity)

This is the recommended approach for GitHub Actions. It uses federated credentials -- no secrets to rotate.

### Setup

1. **Create an app registration** in Microsoft Entra ID with the [required permissions](../README.md#required-permissions).

2. **Add a federated credential** for your GitHub repository:
   - Issuer: `https://token.actions.githubusercontent.com`
   - Subject: `repo:YOUR_ORG/YOUR_REPO:environment:production` (adjust to your branch/environment)
   - Audience: `api://AzureADTokenExchange`

3. **Set GitHub environment variables:**
   - `AZURE_CLIENT_ID` -- App registration client ID
   - `AZURE_TENANT_ID` -- Entra tenant ID
   - `AZURE_SUBSCRIPTION_ID` -- Azure subscription ID (required by azure/login)

### Example workflow

See [examples/github-actions/cactl-plan.yml](../examples/github-actions/cactl-plan.yml) for a complete working example.

The key steps are:

```yaml
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

- name: Plan changes
  run: cactl plan --ci --tenant ${{ vars.AZURE_TENANT_ID }}
```

For apply workflows, add `--auto-approve`:

```yaml
- name: Apply changes
  run: cactl apply --ci --auto-approve --tenant ${{ vars.AZURE_TENANT_ID }}
```

## Azure DevOps with SP certificate

For Azure DevOps pipelines, use service principal certificate authentication.

### Setup

1. **Create an app registration** with the [required permissions](../README.md#required-permissions).

2. **Generate a certificate** and upload the public key to the app registration:
   ```bash
   openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes
   cat cert.pem key.pem > combined.pem
   ```

3. **Upload `cert.pem`** (public key only) to the app registration's Certificates section.

4. **Create a variable group** in Azure DevOps with:
   - `CACTL_CLIENT_ID` -- App registration client ID
   - `CACTL_TENANT_ID` -- Entra tenant ID
   - `CACTL_CERT_PATH` -- Path to the combined PEM file (stored as a secure file)

### Example pipeline

See [examples/azure-devops/azure-pipelines.yml](../examples/azure-devops/azure-pipelines.yml) for a complete working example.

The key steps are:

```yaml
- task: AzureCLI@2
  inputs:
    azureSubscription: 'your-service-connection'
    scriptType: 'bash'
    scriptLocation: 'inlineScript'
    inlineScript: |
      cactl plan --ci --tenant $(CACTL_TENANT_ID)
```

## Scheduled drift detection

Run drift checks on a schedule to catch out-of-band changes made directly in the Entra portal.

### GitHub Actions cron example

See [examples/github-actions/cactl-drift.yml](../examples/github-actions/cactl-drift.yml) for a complete working example.

```yaml
on:
  schedule:
    - cron: '0 6 * * *'  # Daily at 06:00 UTC

jobs:
  drift-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Azure Login (OIDC)
        uses: azure/login@v2
        with:
          client-id: ${{ vars.AZURE_CLIENT_ID }}
          tenant-id: ${{ vars.AZURE_TENANT_ID }}
          subscription-id: ${{ vars.AZURE_SUBSCRIPTION_ID }}

      - name: Check for drift
        id: drift
        continue-on-error: true
        run: cactl drift --ci --tenant ${{ vars.AZURE_TENANT_ID }}

      - name: Create issue on drift
        if: steps.drift.outcome == 'failure'
        uses: actions/github-script@v7
        with:
          script: |
            await github.rest.issues.create({
              owner: context.repo.owner,
              repo: context.repo.repo,
              title: 'CA Policy Drift Detected',
              body: 'Scheduled drift check found differences between local state and live Entra policies. Run `cactl drift` locally to investigate.',
              labels: ['drift', 'conditional-access']
            });
```

### Exit codes for drift

| Exit Code | Meaning | Suggested Action |
|-----------|---------|-----------------|
| 0 | No drift | No action needed |
| 1 | Drift detected | Create issue, notify team |
| 2 | Fatal error | Investigate auth/network failure |

## Pipeline concurrency

When running `cactl apply` in CI/CD, ensure only one pipeline applies to a given tenant at a time:

### GitHub Actions

```yaml
concurrency:
  group: cactl-apply-${{ vars.AZURE_TENANT_ID }}
  cancel-in-progress: false
```

### Azure DevOps

Use [exclusive lock checks](https://learn.microsoft.com/en-us/azure/devops/pipelines/process/approvals?view=azure-devops&tabs=check-pass#exclusive-lock) on your environment to prevent concurrent deploys.

Concurrent apply safety (lock files) is planned for cactl v1.1.
