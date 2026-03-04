# Roadmap: cactl

## Overview

cactl delivers a CLI-first deploy framework for Entra Conditional Access policies across five phases. Phase 1 establishes the Go binary, authentication, and Graph API client -- the foundation every subsequent phase depends on. Phase 2 adds Git-backed state storage and policy import, giving users version-controlled policy files. Phase 3 builds the core plan/apply reconciliation loop with semantic versioning and safety validations. Phase 4 extends with drift detection, rollback, and status reporting. Phase 5 completes v1.0 with multi-tenant support, CI/CD integration, quality gates, and documentation.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation** - Go binary, auth chain, Graph client, workspace init, and config (completed 2026-03-04)
- [ ] **Phase 2: State and Import** - Git-backed state storage, policy import with normalization
- [ ] **Phase 3: Plan and Apply** - Reconciliation engine, plan/apply loop, semver, validations, display
- [ ] **Phase 4: Drift, Rollback, and Status** - Drift detection, version rollback, status dashboard
- [ ] **Phase 5: Production Readiness** - Multi-tenant, CI/CD, code quality, docs, binary distribution

## Phase Details

### Phase 1: Foundation
**Goal**: User can initialize a cactl workspace and authenticate to an Entra tenant via the CLI
**Depends on**: Nothing (first phase)
**Requirements**: CLI-01, CLI-08, CLI-09, CLI-10, AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-06, CONF-01, CONF-02, CONF-03, CONF-04, DISP-06
**Success Criteria** (what must be TRUE):
  1. User can run `cactl init` and get a working workspace with config file, .gitignore, and fetched JSON Schema
  2. User can authenticate via az login credential, SP with secret, or SP with certificate and see confirmation of successful auth
  3. Auth mode resolves correctly through the priority chain (flag > env > config > auto-detect > fallback)
  4. Global flags (--tenant, --output, --no-color, --ci, --config, --log-level) are accepted on all commands
  5. Credentials are never written to disk, logged, or exposed in any output
**Plans**: 3 plans

Plans:
- [ ] 01-01-PLAN.md — Go project scaffold, CLI skeleton with global flags, config loading, output renderer, exit codes
- [ ] 01-02-PLAN.md — Auth layer: AuthProvider interface, three credential types, ClientFactory with per-tenant isolation
- [ ] 01-03-PLAN.md — `cactl init` command: workspace scaffolding, .gitignore safety, schema fetch with embedded fallback

### Phase 2: State and Import
**Goal**: User can import live CA policies into a Git-backed state store with full normalization and version tracking
**Depends on**: Phase 1
**Requirements**: CLI-04, STATE-01, STATE-02, STATE-03, STATE-04, STATE-05, IMPORT-01, IMPORT-02, IMPORT-03, IMPORT-04, IMPORT-05, IMPORT-06, IMPORT-07, IMPORT-08
**Success Criteria** (what must be TRUE):
  1. User can run `cactl import --all` and see all live CA policies pulled into normalized JSON files with kebab-case slugs
  2. User can run `cactl import --policy <slug>` to import a specific policy by slug or display name
  3. State manifest correctly maps each policy slug to its live Entra Object ID, and every apply creates an immutable annotated Git tag
  4. Git refs (refs/cactl/*) are configured for automatic push/pull via refspec, with zero working tree footprint
  5. Import normalizes JSON (strips server fields, removes nulls, alphabetizes keys, pretty-prints with 2-space indent)
**Plans**: TBD

Plans:
- [ ] 02-01: TBD
- [ ] 02-02: TBD

### Phase 3: Plan and Apply
**Goal**: User can preview and deploy CA policy changes with colored diffs, semantic versioning, safety validations, and display name resolution
**Depends on**: Phase 2
**Requirements**: CLI-02, CLI-03, PLAN-01, PLAN-02, PLAN-03, PLAN-04, PLAN-05, PLAN-06, PLAN-07, PLAN-08, PLAN-09, PLAN-10, SEMV-01, SEMV-02, SEMV-03, SEMV-04, SEMV-05, SEMV-06, DISP-01, DISP-02, DISP-03, DISP-04, DISP-05, VALID-01, VALID-02, VALID-03, VALID-04, VALID-05
**Success Criteria** (what must be TRUE):
  1. User can run `cactl plan` and see a terraform-style colored diff with sigils (+, ~, -/+, ?) showing what would change, with named locations and groups resolved to display names
  2. User can run `cactl apply` and deploy changes with a confirmation prompt, or skip confirmation with --auto-approve; recreate actions require explicit 'yes'
  3. Running `cactl apply` on an unchanged policy set produces no changes (full idempotency)
  4. Each policy change shows a semver bump suggestion (MAJOR/MINOR/PATCH) based on configurable field triggers, with MAJOR bumps displaying explicit warnings
  5. Plan-time validations catch break-glass account exclusion gaps, schema violations, conflicting conditions, empty include lists, and overly broad policies
**Plans**: TBD

Plans:
- [ ] 03-01: TBD
- [ ] 03-02: TBD
- [ ] 03-03: TBD

### Phase 4: Drift, Rollback, and Status
**Goal**: User can detect configuration drift, roll back to prior policy versions, and view deployment status across tracked policies
**Depends on**: Phase 3
**Requirements**: CLI-05, CLI-06, CLI-07, DRIFT-01, DRIFT-02, DRIFT-03, DRIFT-04, ROLL-01, ROLL-02, ROLL-03, ROLL-04
**Success Criteria** (what must be TRUE):
  1. User can run `cactl drift` and see a diff of backend vs live state without any changes being made, with exit code 0 for no drift and 1 for drift detected
  2. Drift output identifies modification types (~, -/+, ?) and presents three remediation options (remediate, import live, report only)
  3. User can run `cactl rollback --policy <slug> --version <semver>` to restore a prior version from Git tag history, with plan diff and confirmation before applying
  4. User can run `cactl status` and see all tracked policies with version, timestamp, deployer identity, and sync status
**Plans**: TBD

Plans:
- [ ] 04-01: TBD
- [ ] 04-02: TBD

### Phase 5: Production Readiness
**Goal**: Tool is production-ready with multi-tenant support, CI/CD integration, quality enforcement, documentation, and cross-platform binary distribution
**Depends on**: Phase 4
**Requirements**: MTNT-01, MTNT-02, MTNT-03, MTNT-04, CICD-01, CICD-02, CICD-03, CICD-04, CICD-05, CICD-06, QUAL-01, QUAL-02, QUAL-03, QUAL-04, QUAL-05, DOCS-01, DOCS-02, DOCS-03, DOCS-04
**Success Criteria** (what must be TRUE):
  1. User can pass --tenant with one or more tenant IDs and have plan/apply/drift/import execute sequentially against each tenant with isolated credentials
  2. CI/CD pipelines can run cactl with --ci --auto-approve for non-interactive deploys, with workload identity or SP cert auth and distinct exit codes
  3. GoReleaser produces binaries for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
  4. Getting-started, multi-tenant, and CI/CD guides exist alongside a README with badges, install instructions, and architecture overview
  5. golangci-lint passes, table-driven tests with mockable Graph client achieve 80% coverage on graph and reconcile packages, and all commits follow Conventional Commits
**Plans**: TBD

Plans:
- [ ] 05-01: TBD
- [ ] 05-02: TBD
- [ ] 05-03: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4 -> 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 0/3 | Complete    | 2026-03-04 |
| 2. State and Import | 0/2 | Not started | - |
| 3. Plan and Apply | 0/3 | Not started | - |
| 4. Drift, Rollback, and Status | 0/2 | Not started | - |
| 5. Production Readiness | 0/3 | Not started | - |
