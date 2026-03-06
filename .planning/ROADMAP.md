# Roadmap: cactl

## Overview

cactl delivers a CLI-first deploy framework for Entra Conditional Access policies across five phases. Phase 1 establishes the Go binary, authentication, and Graph API client -- the foundation every subsequent phase depends on. Phase 2 adds Git-backed state storage and policy import, giving users version-controlled policy files. Phase 3 builds the core plan/apply reconciliation loop with semantic versioning and safety validations. Phase 4 extends with drift detection, rollback, and status reporting. Phase 5 completes v1.0 with multi-tenant support, CI/CD integration, quality gates, and documentation.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation** - Go binary, auth chain, Graph client, workspace init, and config (completed 2026-03-04)
- [x] **Phase 2: State and Import** - Git-backed state storage, policy import with normalization (completed 2026-03-04)
- [x] **Phase 3: Plan and Apply** - Reconciliation engine, plan/apply loop, semver, validations, display (completed 2026-03-05)
- [x] **Phase 4: Drift, Rollback, and Status** - Drift detection, version rollback, status dashboard (completed 2026-03-05)
- [x] **Phase 5: Production Readiness** - Multi-tenant, CI/CD, code quality, docs, binary distribution (completed 2026-03-05)
- [ ] **Phase 6: Point-in-Time Restore** - Git history timeline, point-in-time policy restore with full diffs (not started)

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
**Plans**: 3 plans

Plans:
- [ ] 02-01-PLAN.md — JSON normalization pipeline and kebab-case slug derivation (TDD)
- [ ] 02-02-PLAN.md — Git-backed state store: GitBackend, manifest, refspec (TDD)
- [ ] 02-03-PLAN.md — Graph API client and `cactl import` command with full pipeline wiring

### Phase 3: Plan and Apply
**Goal**: User can preview and deploy CA policy changes with colored diffs, semantic versioning, safety validations, and display name resolution
**Depends on**: Phase 2
**Requirements**: CLI-02, CLI-03, PLAN-01, PLAN-02, PLAN-03, PLAN-04, PLAN-05, PLAN-06, PLAN-07, PLAN-08, PLAN-09, PLAN-10, SEMV-01, SEMV-02, SEMV-03, SEMV-04, SEMV-06, DISP-01, DISP-02, DISP-03, DISP-04, VALID-01, VALID-03, VALID-04, VALID-05
**Success Criteria** (what must be TRUE):
  1. User can run `cactl plan` and see a terraform-style colored diff with sigils (+, ~, -/+, ?) showing what would change, with named locations and groups resolved to display names
  2. User can run `cactl apply` and deploy changes with a confirmation prompt, or skip confirmation with --auto-approve; recreate actions require explicit 'yes'
  3. Running `cactl apply` on an unchanged policy set produces no changes (full idempotency)
  4. Each policy change shows a semver bump suggestion (MAJOR/MINOR/PATCH) based on configurable field triggers, with MAJOR bumps displaying explicit warnings
  5. Plan-time validations catch break-glass account exclusion gaps, schema violations, conflicting conditions, empty include lists, and overly broad policies
**Plans**: 5 plans

Plans:
- [ ] 03-01-PLAN.md — Reconciliation engine and field-level JSON diff (TDD)
- [ ] 03-02-PLAN.md — Semantic versioning with field triggers and plan-time validations (TDD)
- [ ] 03-03-PLAN.md — Graph API write operations and display name resolver
- [ ] 03-04-PLAN.md — Diff output renderer and `cactl plan` command wiring
- [ ] 03-05-PLAN.md — `cactl apply` command with confirmation, dry-run, and state updates

### Phase 4: Drift, Rollback, and Status
**Goal**: User can detect configuration drift, roll back to prior policy versions, and view deployment status across tracked policies
**Depends on**: Phase 3
**Requirements**: CLI-05, CLI-06, CLI-07, DRIFT-01, DRIFT-02, DRIFT-03, DRIFT-04, ROLL-01, ROLL-02, ROLL-03, ROLL-04, SEMV-05, VALID-02, DISP-05
**Success Criteria** (what must be TRUE):
  1. User can run `cactl drift` and see a diff of backend vs live state without any changes being made, with exit code 0 for no drift and 1 for drift detected
  2. Drift output identifies modification types (~, -/+, ?) and presents three remediation options (remediate, import live, report only)
  3. User can run `cactl rollback --policy <slug> --version <semver>` to restore a prior version from Git tag history, with plan diff and confirmation before applying
  4. User can run `cactl status` and see all tracked policies with version, timestamp, deployer identity, and sync status
**Plans**: 5 plans

Plans:
- [ ] 04-01-PLAN.md — Git tag operations: ListVersionTags and ReadTagBlob on GitBackend (TDD)
- [ ] 04-02-PLAN.md — `cactl drift` command: read-only reconciliation with remediation suggestions
- [ ] 04-03-PLAN.md — `cactl rollback` command: tag read, diff, confirm, PATCH, new version tag
- [ ] 04-04-PLAN.md — `cactl status` command: status table, sync check, version history
- [ ] 04-05-PLAN.md — Gap closure: SEMV-05 --bump-level flag + stale REQUIREMENTS.md checkboxes

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
**Plans**: 4 plans

Plans:
- [ ] 05-01-PLAN.md — Multi-tenant sequential execution, CI mode guards, --auto-approve enforcement
- [ ] 05-02-PLAN.md — GoReleaser config, CI/release workflows, example pipelines (GitHub Actions OIDC, Azure DevOps SP cert, scheduled drift)
- [ ] 05-03-PLAN.md — golangci-lint v2 config, GraphClient interface extraction, mock-based tests, MIT license
- [ ] 05-04-PLAN.md — Documentation: README with badges, getting-started, multi-tenant, and CI/CD guides

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4 -> 5 -> 6 -> 7

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 0/3 | Complete    | 2026-03-04 |
| 2. State and Import | 0/3 | Complete    | 2026-03-04 |
| 3. Plan and Apply | 0/3 | Complete    | 2026-03-05 |
| 4. Drift, Rollback, and Status | 0/4 | Complete    | 2026-03-05 |
| 5. Production Readiness | 0/4 | Complete    | 2026-03-05 |
| 6. Point-in-Time Restore | 0/2 | Not Started | - |
| 7. Codebase DRY Simplification | 0/3 | Not Started | - |

### Phase 6: Point-in-Time Restore
**Goal**: User can restore any policy to its state at any previous point in time, with full diff preview and confirmation
**Depends on**: Phase 5
**Plans:** 2 plans

Plans:
- [ ] 06-01-PLAN.md — TUI package with huh selectors and interactive rollback restore wizard (-i flag)
- [ ] 06-02-PLAN.md — Standalone `cactl history` command with version timeline and diff summaries

### Phase 7: Codebase DRY Simplification
**Goal:** Behavior-preserving refactoring to eliminate ~600 lines of duplication concentrated in the cmd/ layer, extracting shared pipeline helpers and consolidating mirror types
**Depends on:** Phase 6
**Plans:** 3 plans

Plans:
- [ ] 07-01-PLAN.md — Extract CommandPipeline struct with shared bootstrap, normalization, semver, validation, resolution, and rendering helpers; refactor plan.go
- [ ] 07-02-PLAN.md — Refactor apply/drift/rollback to use pipeline, consolidate apply action handlers, eliminate bumpPatchVersion duplicate
- [ ] 07-03-PLAN.md — Eliminate mirror type definitions (semver.FieldDiff, validate.ActionType), consolidate history JSON structure and diff summary logic
