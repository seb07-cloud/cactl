# Requirements: cactl

**Defined:** 2026-03-04
**Core Value:** Reliable, idempotent, state-aware deployment of Entra CA policies with Git-native versioning and plan/apply safety

## v1 Requirements

Requirements for v1.0 release. Each maps to roadmap phases.

### CLI Foundation

- [x] **CLI-01**: User can run `cactl init` to scaffold workspace (.cactl/config.yaml, .gitignore, refspec setup, schema fetch)
- [x] **CLI-02**: User can run `cactl plan` to see reconciliation diff between backend and live tenant
- [x] **CLI-03**: User can run `cactl apply` to deploy backend state to live tenant with confirmation prompt
- [x] **CLI-04**: User can run `cactl import` to pull live CA policies into backend with normalization
- [x] **CLI-05**: User can run `cactl drift` to check for drift without making changes
- [x] **CLI-06**: User can run `cactl rollback` to restore a prior policy version from Git tag history
- [x] **CLI-07**: User can run `cactl status` to see tracked policies with version, timestamp, deployer, and sync status
- [x] **CLI-08**: All commands accept --tenant, --output (human|json), --no-color, --ci, --config, --log-level flags
- [x] **CLI-09**: Exit codes follow contract: 0=success/no changes, 1=changes/drift detected, 2=fatal error, 3=validation error
- [x] **CLI-10**: Single Go binary with zero external runtime dependencies, built with cobra/viper

### Authentication

- [x] **AUTH-01**: User can authenticate via Azure CLI credential (`az login` token, default when no SP config)
- [x] **AUTH-02**: User can authenticate via service principal with client secret (CACTL_CLIENT_ID + CACTL_CLIENT_SECRET)
- [x] **AUTH-03**: User can authenticate via service principal with certificate (CACTL_CLIENT_ID + CACTL_CERT_PATH)
- [x] **AUTH-04**: Auth mode resolves in priority order: --auth-mode flag → CACTL_AUTH_MODE env → config file → auto-detect → az login fallback
- [x] **AUTH-05**: Per-tenant credential isolation via ClientFactory (one Graph client per tenant, no shared token state)
- [x] **AUTH-06**: Credentials are never written to disk, never logged, never included in plan output or state manifest

### Configuration

- [x] **CONF-01**: All config lives in .cactl/config.yaml with documented schema (tenant, backend, auth, output, semver)
- [x] **CONF-02**: Every config value can be overridden by environment variable (CACTL_*) or CLI flag
- [x] **CONF-03**: `cactl init` adds .cactl/config.yaml to .gitignore and refuses to continue if already tracked by Git
- [x] **CONF-04**: `cactl init` fetches CA policy JSON Schema from Graph metadata and writes to .cactl/schema.json

### State Management

- [x] **STATE-01**: State manifest records 1:1 mapping between local policy slug and live Entra Object ID
- [x] **STATE-02**: GitBackend stores state in refs/cactl/tenants/<tenant-id>/policies/<slug> (current state blobs)
- [x] **STATE-03**: Every `cactl apply` creates an immutable annotated Git tag (cactl/<tenant>/<slug>/<semver>) containing full policy JSON
- [x] **STATE-04**: `cactl init` writes refspec configuration to .git/config for automatic push/pull of refs/cactl/*
- [x] **STATE-05**: State entry schema includes: schema_version, slug, tenant, live_object_id, version, last_deployed, deployed_by, auth_mode, backend_sha

### Import & Normalization

- [x] **IMPORT-01**: `cactl import --all` fetches all live CA policies and imports them as v1.0.0
- [x] **IMPORT-02**: `cactl import --policy <slug>` imports a specific policy by slug or display name
- [x] **IMPORT-03**: Import strips server-managed fields (id, createdDateTime, modifiedDateTime, templateId)
- [x] **IMPORT-04**: Import removes explicit null fields from Graph API responses
- [x] **IMPORT-05**: Import normalizes field order (alphabetical) and pretty-prints with 2-space indent
- [x] **IMPORT-06**: Import enforces kebab-case slug format derived from filename
- [x] **IMPORT-07**: `cactl import --force` overwrites existing backend JSON for already-tracked policies
- [x] **IMPORT-08**: Without --policy or --all, import lists untracked (?) policies and prompts for selection

### Plan & Apply

- [x] **PLAN-01**: `cactl plan` compares backend JSON files against live tenant state via Graph API
- [x] **PLAN-02**: Plan output shows sigils: + (create), ~ (update), -/+ (recreate with warning), ? (untracked)
- [x] **PLAN-03**: Plan output shows semver bump suggestion per policy (MAJOR/MINOR/PATCH) based on configurable field triggers
- [x] **PLAN-04**: Plan summary line shows counts: N to create, N to update, N to recreate, N untracked
- [x] **PLAN-05**: `cactl apply` presents plan diff and requests confirmation before making changes
- [x] **PLAN-06**: `cactl apply --auto-approve` skips confirmation (required in --ci mode)
- [x] **PLAN-07**: `cactl apply --dry-run` generates full plan and runs Graph API validation but makes no writes
- [x] **PLAN-08**: Recreate (-/+) actions escalate confirmation: user must type 'yes' (not just Enter)
- [x] **PLAN-09**: Apply is idempotent: running apply on unchanged policy set produces no changes
- [x] **PLAN-10**: Full idempotency truth table implemented: create, update, noop, recreate (ghost cleanup), untracked warning

### Semantic Versioning

- [x] **SEMV-01**: Every tracked policy is versioned independently using MAJOR.MINOR.PATCH
- [x] **SEMV-02**: MAJOR bump triggered by scope expansion (configurable via semver.major_fields in config)
- [x] **SEMV-03**: MINOR bump triggered by conditions/controls changes (configurable via semver.minor_fields)
- [x] **SEMV-04**: PATCH bump for state-only or cosmetic changes (all other fields)
- [x] **SEMV-05**: User can override the suggested bump level at apply time
- [x] **SEMV-06**: MAJOR bumps display explicit warning in plan output

### Drift Detection

- [x] **DRIFT-01**: `cactl drift` outputs diff between backend state and live tenant without making changes
- [x] **DRIFT-02**: Drift types identified: policy modified (~), policy missing (-/+), untracked policy (?)
- [x] **DRIFT-03**: Exit codes: 0=no drift, 1=drift detected, 2=error
- [x] **DRIFT-04**: Three remediation options presented: remediate (apply backend), import live (update backend), report only

### Rollback

- [x] **ROLL-01**: `cactl rollback --policy <slug> --version <semver>` reads policy JSON from annotated tag
- [x] **ROLL-02**: Rollback runs plan diff against current live state and presents for confirmation
- [x] **ROLL-03**: On confirmation: PATCHes live policy, writes new state manifest entry
- [x] **ROLL-04**: Tag history is never modified — full audit trail preserved; rollback becomes new deployment event

### Display & Output

- [x] **DISP-01**: Human-readable output uses terraform-style colored diffs with sigils
- [x] **DISP-02**: All commands support --output json with stable schema (schema_version field)
- [x] **DISP-03**: Named locations resolved to display names in plan/diff output (not raw GUIDs)
- [x] **DISP-04**: Groups and users resolved to display names in plan/diff output
- [x] **DISP-05**: `cactl status` shows per-policy version tree with timestamp and deployer identity
- [x] **DISP-06**: --no-color flag disables ANSI color output (also via CACTL_NO_COLOR=1)

### Validation

- [x] **VALID-01**: Break-glass account exclusion validated at plan time — warn loudly if emergency access accounts not excluded
- [x] **VALID-02**: Policy JSON validated against schema fetched during init
- [x] **VALID-03**: Detect conflicting conditions (e.g., include and exclude same group)
- [x] **VALID-04**: Detect empty include lists (policy applies to no one)
- [x] **VALID-05**: Detect policies that would block all users (overly broad with no exclusions)

### CI/CD & Distribution

- [x] **CICD-01**: --ci flag enables non-interactive mode, suppresses all prompts
- [x] **CICD-02**: --ci requires --auto-approve for write operations
- [x] **CICD-03**: GoReleaser builds for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- [x] **CICD-04**: GitHub Actions example workflow with workload identity auth
- [x] **CICD-05**: Azure DevOps example pipeline with SP cert auth
- [x] **CICD-06**: Scheduled drift check example (daily cron with alert on exit code 1)

### Multi-Tenant

- [x] **MTNT-01**: Tenant ID flows as explicit parameter through every layer (CLI → auth → Graph → state)
- [x] **MTNT-02**: --tenant flag accepts tenant ID or primary domain, supports multiple values
- [x] **MTNT-03**: Sequential multi-tenant apply in v1 (one tenant at a time)
- [x] **MTNT-04**: Concurrent pipeline applies rejected with advisory error message

### Code Quality

- [x] **QUAL-01**: golangci-lint with default ruleset + exhaustive enum checks
- [x] **QUAL-02**: Table-driven unit tests with Graph client fully mockable via interface
- [x] **QUAL-03**: 80% test coverage target on internal/graph and internal/reconcile
- [x] **QUAL-04**: Conventional Commits (feat:, fix:, chore:) for automatic CHANGELOG generation
- [x] **QUAL-05**: MIT license

### Documentation

- [ ] **DOCS-01**: Getting started guide (install, init, first import, first plan/apply)
- [ ] **DOCS-02**: Multi-tenant usage guide
- [ ] **DOCS-03**: CI/CD integration guide (GitHub Actions + Azure DevOps)
- [ ] **DOCS-04**: README with badges, install instructions, quick start, architecture overview

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Extended Auth

- **AUTH-V2-01**: Device code flow for interactive browser auth
- **AUTH-V2-02**: Workload identity (OIDC) for GitHub Actions / Azure DevOps native

### Extended Backends

- **BACK-V2-01**: AzureBlobBackend with blob leasing for distributed state locking
- **BACK-V2-02**: LocalFSBackend for offline development and testing

### Advanced Features

- **ADV-V2-01**: Named Locations plugin (shared refs/cactl/* namespace with type prefix)
- **ADV-V2-02**: Authentication Strengths plugin
- **ADV-V2-03**: UTCM integration as drift signal source
- **ADV-V2-04**: Concurrent multi-tenant apply with --concurrency flag
- **ADV-V2-05**: Ring-based deployment (report-only → enabled promotion with validation gates)
- **ADV-V2-06**: Maester integration for What-If API testing between plan and apply
- **ADV-V2-07**: Compliance baseline validation (CIS/CISA SCuBA baselines at plan time)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Web UI / React dashboard | CLI-only product; CIPP already serves GUI users |
| CA policy assessment / gap analysis | Not a security posture tool; Maester/ScubaGear cover this |
| CA policy editor or designer | Deploy tool, not design tool |
| General Entra ID management | Scope discipline — CA policies only |
| Real-time monitoring/alerting | Monitoring concern, not deploy; Sentinel handles this |
| Policy template marketplace | Security policies are org-specific; community frameworks exist |
| What-If API simulation | Maester does this well; integrate don't duplicate |
| ClickOps prevention | Organizational policy, not tooling |
| Cross-tenant atomic rollback | Impossible — each tenant is independent Graph endpoint |
| Per-tenant policy overrides in single repo | Separate repos per tenant model chosen |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| CLI-01 | Phase 1 | Complete |
| CLI-02 | Phase 3 | Complete |
| CLI-03 | Phase 3 | Complete |
| CLI-04 | Phase 2 | Complete |
| CLI-05 | Phase 4 | Complete |
| CLI-06 | Phase 4 | Complete |
| CLI-07 | Phase 4 | Complete |
| CLI-08 | Phase 1 | Complete |
| CLI-09 | Phase 1 | Complete |
| CLI-10 | Phase 1 | Complete |
| AUTH-01 | Phase 1 | Complete |
| AUTH-02 | Phase 1 | Complete |
| AUTH-03 | Phase 1 | Complete |
| AUTH-04 | Phase 1 | Complete |
| AUTH-05 | Phase 1 | Complete |
| AUTH-06 | Phase 1 | Complete |
| CONF-01 | Phase 1 | Complete |
| CONF-02 | Phase 1 | Complete |
| CONF-03 | Phase 1 | Complete |
| CONF-04 | Phase 1 | Complete |
| STATE-01 | Phase 2 | Complete |
| STATE-02 | Phase 2 | Complete |
| STATE-03 | Phase 2 | Complete |
| STATE-04 | Phase 2 | Complete |
| STATE-05 | Phase 2 | Complete |
| IMPORT-01 | Phase 2 | Complete |
| IMPORT-02 | Phase 2 | Complete |
| IMPORT-03 | Phase 2 | Complete |
| IMPORT-04 | Phase 2 | Complete |
| IMPORT-05 | Phase 2 | Complete |
| IMPORT-06 | Phase 2 | Complete |
| IMPORT-07 | Phase 2 | Complete |
| IMPORT-08 | Phase 2 | Complete |
| PLAN-01 | Phase 3 | Complete |
| PLAN-02 | Phase 3 | Complete |
| PLAN-03 | Phase 3 | Complete |
| PLAN-04 | Phase 3 | Complete |
| PLAN-05 | Phase 3 | Complete |
| PLAN-06 | Phase 3 | Complete |
| PLAN-07 | Phase 3 | Complete |
| PLAN-08 | Phase 3 | Complete |
| PLAN-09 | Phase 3 | Complete |
| PLAN-10 | Phase 3 | Complete |
| SEMV-01 | Phase 3 | Complete |
| SEMV-02 | Phase 3 | Complete |
| SEMV-03 | Phase 3 | Complete |
| SEMV-04 | Phase 3 | Complete |
| SEMV-05 | Phase 3 | Complete |
| SEMV-06 | Phase 3 | Complete |
| DRIFT-01 | Phase 4 | Complete |
| DRIFT-02 | Phase 4 | Complete |
| DRIFT-03 | Phase 4 | Complete |
| DRIFT-04 | Phase 4 | Complete |
| ROLL-01 | Phase 4 | Complete |
| ROLL-02 | Phase 4 | Complete |
| ROLL-03 | Phase 4 | Complete |
| ROLL-04 | Phase 4 | Complete |
| DISP-01 | Phase 3 | Complete |
| DISP-02 | Phase 3 | Complete |
| DISP-03 | Phase 3 | Complete |
| DISP-04 | Phase 3 | Complete |
| DISP-05 | Phase 3 | Complete |
| DISP-06 | Phase 1 | Complete |
| VALID-01 | Phase 3 | Complete |
| VALID-02 | Phase 3 | Complete |
| VALID-03 | Phase 3 | Complete |
| VALID-04 | Phase 3 | Complete |
| VALID-05 | Phase 3 | Complete |
| CICD-01 | Phase 5 | Complete |
| CICD-02 | Phase 5 | Complete |
| CICD-03 | Phase 5 | Complete |
| CICD-04 | Phase 5 | Complete |
| CICD-05 | Phase 5 | Complete |
| CICD-06 | Phase 5 | Complete |
| MTNT-01 | Phase 5 | Complete |
| MTNT-02 | Phase 5 | Complete |
| MTNT-03 | Phase 5 | Complete |
| MTNT-04 | Phase 5 | Complete |
| QUAL-01 | Phase 5 | Complete |
| QUAL-02 | Phase 5 | Complete |
| QUAL-03 | Phase 5 | Complete |
| QUAL-04 | Phase 5 | Complete |
| QUAL-05 | Phase 5 | Complete |
| DOCS-01 | Phase 5 | Pending |
| DOCS-02 | Phase 5 | Pending |
| DOCS-03 | Phase 5 | Pending |
| DOCS-04 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 87 total
- Mapped to phases: 87
- Unmapped: 0

---
*Requirements defined: 2026-03-04*
*Last updated: 2026-03-04 after roadmap creation*
