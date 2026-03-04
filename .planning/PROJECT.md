# cactl — Conditional Access Policy Deploy Framework

## What This Is

cactl is a CLI-first, open-source deploy framework for Microsoft Entra Conditional Access policies. It gives identity architects and IAM teams a trusted deployment gate — purpose-built plan/apply safety for Entra CA policies, with Git-native versioning, semantic versioning per policy, and first-class multi-tenant support from day one. Single Go binary, MIT licensed.

## Core Value

Reliable, idempotent, state-aware deployment of CA policies that prevents the "Friday afternoon problem" — a misconfigured policy locking out thousands of users with no version history to recover from.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] CLI-first Go binary with cobra command structure
- [ ] Four auth modes: device code, SP secret, SP cert, workload identity (via azidentity)
- [ ] Auth resolution order: flag → env var → config → auto-detect → fallback
- [ ] Backend interface abstraction (Fetch, Write, History, Rollback)
- [ ] GitBackend: state in refs/cactl/* namespace + annotated tags (zero working tree footprint)
- [ ] AzureBlobBackend: manifest.json blob alongside policy JSONs
- [ ] LocalFSBackend: offline dev/testing
- [ ] Multi-tenant: tenant ID flows as explicit parameter through every layer, ClientFactory per tenant
- [ ] State manifest: 1:1 mapping between local policy slug and live Entra Object ID
- [ ] `cactl init`: scaffold workspace, config, .gitignore, refspec setup, schema fetch, optional --import-live
- [ ] `cactl plan`: reconciliation diff (backend vs live), sigils (+, ~, -/+, ?), semver bump suggestions
- [ ] `cactl apply`: deploy with confirmation, --auto-approve, --dry-run, recreate escalation
- [ ] `cactl import`: pull live policies, normalize JSON (strip server fields, remove nulls, alphabetize, pretty-print)
- [ ] `cactl drift`: report-only reconciliation, exit code 0/1/2, never writes
- [ ] `cactl rollback`: restore from annotated tag, plan diff, confirm, PATCH live
- [ ] `cactl status`: tracked policies with version, timestamp, deployer, sync status
- [ ] Full idempotency truth table (create, update, noop, recreate, untracked)
- [ ] Semantic versioning per policy: MAJOR/MINOR/PATCH with configurable field triggers
- [ ] Drift detection with three remediation options: remediate, import live, report only
- [ ] Human-readable output (terraform-style colored diffs) and --output json on all commands
- [ ] Exit code contract: 0=success, 1=changes/drift, 2=fatal error, 3=validation error
- [ ] --ci flag (non-interactive), --no-color, --tenant, --config global flags
- [ ] .cactl/config.yaml with env var overrides (CACTL_*)
- [ ] JSON Schema validation for policy files (fetched on init)
- [ ] CI/CD examples: GitHub Actions (with workload identity), Azure DevOps (with SP cert)
- [ ] GoReleaser binary distribution: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- [ ] Conventional commits, golangci-lint, 80% test coverage on graph + reconcile packages
- [ ] Documentation: getting-started, multi-tenant, ci-cd, backend-azureblob

### Out of Scope

- Web UI / React frontend — CLI only, no dashboard
- CA policy assessment, gap analysis, or security posture review
- CA policy editor or designer
- Replacement for Maester, UTCM, or compliance frameworks
- General-purpose Entra ID management
- Named Locations plugin — post-v1.0 (v1.1+)
- Authentication Strengths plugin — post-v1.0 (v1.1+)
- UTCM integration as drift signal — post-v1.0 (future hook)
- Concurrent pipeline applies — reject in v1, blob lease/Git lock in v1.1
- Multi-tenant parallel apply — sequential in v1, --concurrency in v1.1
- Per-tenant policy overrides in single repo — separate repos per tenant model chosen

## Context

- The Microsoft Graph API has no idempotency for CA policies — POSTing duplicates is silent and dangerous
- The Terraform Graph provider is in beta with state reliability issues and adds unnecessary IaC overhead
- UTCM API (public preview) detects drift but provides no remediation path
- Target users: identity architects (full plan/apply), IAM/M365 teams (day-to-day lifecycle), DevOps/platform engineers (CI/CD pipelines)
- Interaction model mirrors terraform plan/apply — familiar to infrastructure engineers
- State is stored in Git refs/annotated tags to avoid commit noise, merge conflicts, and working tree pollution
- Graph API permissions: Policy.Read.All, Policy.ReadWrite.ConditionalAccess, Application.Read.All, Group.Read.All, RoleManagement.Read.Directory
- Credentials never written to disk — OS credential manager for interactive, env vars for SP

## Constraints

- **Tech stack**: Go 1.22+, single binary, cobra CLI, azidentity for auth — no external runtime dependencies
- **License**: MIT — permissive, commercial-friendly
- **Security**: .cactl/config.yaml must never be tracked by Git; cactl init enforces this. Credentials never logged or written to state.
- **Backwards compatibility**: JSON output schema_version field enables forward-compatible changes. State entry schema_version for migrations.
- **Graph API**: Use json.RawMessage for Conditions/GrantControls/SessionControls to absorb schema changes transparently
- **Slug format**: Enforce kebab-case on import and apply (reject non-conforming)
- **Multi-tenant repos**: Separate repos per tenant (cleanest isolation)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Git refs + annotated tags for state (not .state.json) | Avoids commit noise, merge conflicts, working tree pollution | — Pending |
| Multi-tenant from v0.2, not retrofitted | Retrofitting tenant isolation requires state schema rewrite | — Pending |
| Sequential multi-tenant apply in v1 | Simpler, avoids concurrent state corruption | — Pending |
| Reject concurrent pipeline applies in v1 | Advisory error; blob lease/Git lock in v1.1 | — Pending |
| JSON Schema fetched on init, not bundled | Stays fresh with Graph API changes, zero maintenance | — Pending |
| Enforce kebab-case slug format | Consistent naming, clean Git refs, predictable CLI usage | — Pending |
| Separate repos per tenant | Cleanest isolation for multi-tenant; no override complexity | — Pending |
| Shared refs/cactl/* namespace with type prefix for future plugins | Named Locations: refs/cactl/tenants/<t>/locations/<slug> | — Pending |

---
*Last updated: 2026-03-04 after initialization*
