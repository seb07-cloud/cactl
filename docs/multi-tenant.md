# Multi-Tenant Guide

cactl supports managing Conditional Access policies across multiple Microsoft Entra tenants. This guide explains the multi-tenant execution model, credential isolation, and best practices.

## Overview

cactl processes tenants sequentially -- each tenant gets its own authentication context, Graph API calls, and state operations. This ensures complete isolation between tenants and avoids concurrent state corruption.

## Passing multiple tenants

Use the `--tenant` flag to specify one or more tenant IDs:

```bash
# Repeated flag syntax
cactl plan --tenant TENANT_A --tenant TENANT_B

# Comma-separated syntax
cactl plan --tenant TENANT_A,TENANT_B
```

Both forms are equivalent. cactl processes each tenant in order, showing results for each.

## Per-tenant credentials

cactl's `ClientFactory` maintains isolated credentials per tenant. Each tenant authenticates independently using the configured auth mode.

### Same credentials for all tenants

If your service principal is registered as a multi-tenant app, set credentials once:

```bash
export CACTL_CLIENT_ID=YOUR_MULTI_TENANT_APP_ID
export CACTL_CLIENT_SECRET=YOUR_CLIENT_SECRET

cactl plan --tenant TENANT_A --tenant TENANT_B
```

The same credentials are used for both tenants, but each gets its own token and Graph API session.

### Different credentials per tenant

For tenants with separate app registrations, configure credentials in `.cactl/config.yaml` or use the environment variable approach with wrapper scripts:

```bash
#!/bin/bash
# plan-all-tenants.sh

# Tenant A
CACTL_CLIENT_ID=APP_ID_A CACTL_CLIENT_SECRET=SECRET_A \
  cactl plan --tenant TENANT_A

# Tenant B
CACTL_CLIENT_ID=APP_ID_B CACTL_CLIENT_SECRET=SECRET_B \
  cactl plan --tenant TENANT_B
```

### Environment variable

For single-tenant workflows, the `CACTL_TENANT` environment variable is supported for backward compatibility:

```bash
export CACTL_TENANT=YOUR_TENANT_ID
cactl plan
```

This is automatically wrapped into the multi-tenant format internally.

## Exit code aggregation

When processing multiple tenants, cactl returns the highest severity exit code across all tenant executions:

| Exit Code | Meaning | Behavior |
|-----------|---------|----------|
| 0 | Success (no changes) | Continue to next tenant |
| 1 | Changes detected / drift found | Continue to next tenant |
| 2 | Fatal error (auth failure, network) | Stop immediately |
| 3 | Validation error | Stop immediately |

**Examples:**

- Tenant A returns 0, Tenant B returns 1 --> overall exit code is 1
- Tenant A returns 1, Tenant B returns 2 --> execution stops at Tenant B, exit code is 2
- Tenant A returns 0, Tenant B returns 0 --> overall exit code is 0

Fatal (2) and validation (3) errors cause immediate termination -- remaining tenants are skipped.

## Separate repos per tenant

cactl uses a **separate repository per tenant** model. Each tenant's policies live in their own Git repository with their own state (refs/cactl/* namespace and annotated tags).

This provides:

- **Clean isolation** -- no risk of cross-tenant policy leakage
- **Independent versioning** -- each tenant's policies have their own semantic version history
- **Simple CI/CD** -- each repo triggers its own pipeline
- **Clear ownership** -- repository permissions map directly to tenant access

```
# Recommended directory structure
~/tenants/
  contoso-corp/     # git repo for tenant A
    .cactl/
    require-mfa.json
    block-legacy.json
  fabrikam-inc/     # git repo for tenant B
    .cactl/
    require-mfa.json
    require-compliant.json
```

## Limitations

### Concurrent pipeline applies (v1 advisory)

In v1, concurrent `cactl apply` runs against the same tenant are **not safe**. If two pipelines attempt to apply changes simultaneously, state corruption may occur.

**Mitigation:** Use CI/CD pipeline concurrency controls to ensure only one apply runs at a time per tenant:

```yaml
# GitHub Actions
concurrency:
  group: cactl-apply-${{ vars.TENANT_ID }}
  cancel-in-progress: false
```

Lock-file based concurrency control is planned for v1.1.

### Sequential execution only

In v1, multi-tenant execution is strictly sequential. A `--concurrency` flag for parallel tenant processing is planned for v1.1.
