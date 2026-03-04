# Project Research Summary

**Project:** cactl — Entra Conditional Access Policy Deploy Framework
**Domain:** CLI-first IaC tool for Microsoft Entra Conditional Access policies (plan/apply, Git-native state)
**Researched:** 2026-03-04
**Confidence:** HIGH

## Executive Summary

cactl is a domain-specific Infrastructure-as-Code tool for Microsoft Entra Conditional Access (CA) policies, occupying a gap that no existing tool fills well. The closest competitor, Terraform's azuread provider, applies a generic plan/apply workflow to CA policies without domain intelligence — no break-glass validation, no named-location resolution, no policy-specific versioning. PowerShell-based tools (Microsoft365DSC, DCToolbox) offer export/import and drift detection, but none provide a CLI-native plan-preview-apply loop with structured rollback. cactl's opportunity is to be "Terraform for CA policies," combining Terraform's workflow discipline with domain awareness that identity architects actually need.

The recommended implementation is a single Go binary using the established Cobra/Viper CLI stack, the official Microsoft Graph SDK (msgraph-sdk-go v1.0 endpoint) with azidentity for auth, and go-git for Git-native state storage. The architecture centers on a pure reconciliation engine that takes three inputs — desired state (local YAML/JSON), actual state (Graph API), and state manifest (slug-to-ObjectID mapping in Git refs) — and produces a typed plan of actions. All external boundaries (Graph, Git, Auth, Output) are interface-driven to enable testing without real infrastructure. This is a well-documented pattern (Terraform, Kubernetes controllers) applied to the CA policy domain.

The top risks are all addressable at the design level: the Graph API POST endpoint creates silent duplicates (requires read-before-write), server-computed fields must be stripped during normalization before they corrupt the diff engine, and the auth layer must use per-tenant credential instances to prevent wrong-tenant token reuse. Each of these pitfalls must be baked into the lowest-level components in Phase 1 — they cannot be retrofitted later without rewriting core API call sites.

## Key Findings

### Recommended Stack

The stack is well-established and high-confidence. Go 1.24+ with Cobra v1.10.x (not v2, just released Dec 2025 with incomplete ecosystem adoption) and Viper v1.21.0 provides the standard CLI foundation used by kubectl, Terraform, and gh. Authentication uses the official `Azure/azure-sdk-for-go/sdk/azidentity` v1.13.x, which covers all four required auth modes: device code (interactive), service principal secret/cert (CI/CD), and workload identity (GitHub Actions OIDC). Graph API access uses `microsoftgraph/msgraph-sdk-go` v1.96.x against the stable v1.0 endpoint — the beta SDK must be avoided entirely. State storage uses `go-git/go-git/v5` v5.17.0 for pure-Go Git plumbing (no dependency on git binary), `Masterminds/semver/v3` for version parsing, and `r3labs/diff/v3` for structured changelog generation.

**Core technologies:**
- Go 1.24+: Single binary, cross-platform, no runtime deps — standard for modern CLIs
- spf13/cobra v1.10.x: CLI framework with command trees, completion, POSIX flags — de facto standard
- spf13/viper v1.21.0: Config management with flags > env > config file > defaults precedence
- Azure/azure-sdk-for-go/sdk/azidentity v1.13.x: All four auth modes, integrates directly with msgraph-sdk-go
- microsoftgraph/msgraph-sdk-go v1.96.x: Typed fluent API against Graph v1.0, handles pagination and retry
- go-git/go-git/v5 v5.17.0: Pure Go Git plumbing — ref CRUD, blob creation, annotated tags
- r3labs/diff/v3: Typed changelog (create/update/delete) for reconciliation engine input
- Masterminds/semver/v3: Semver parsing and comparison for per-policy version tracking

### Expected Features

The feature landscape is clearly stratified. Import (to onboard existing policies) is the prerequisite for everything — no user starts greenfield. The plan/apply loop with idempotent state manifest is the core value proposition. Semantic versioning, drift detection with three remediation modes, and rollback via Git tags are the differentiators no existing tool offers. Multi-tenant orchestration and ring-based deployment are v2+ territory.

**Must have (table stakes):**
- Plan/preview before apply — identity architects will not blind-deploy policies that can lock out tenants
- Idempotent apply with state manifest — running apply twice must produce no changes
- Import existing policies with normalization — every org has existing policies; a greenfield-only tool is unusable
- Named location and group/user display name resolution — diffs showing raw GUIDs are unacceptable
- Break-glass account exclusion validation — warn loudly at plan time if emergency accounts not excluded
- CI/CD integration — non-interactive mode, JSON output, distinct exit codes (0=changes, 1=error, 2=no-op)
- Multiple auth methods — device code for humans, SP secret/cert for CI, workload identity for GitHub Actions

**Should have (competitive):**
- Drift detection with three modes (remediate/import/report) — M365DSC detects drift but remediation is all-or-nothing
- Semantic versioning per policy — no existing tool versions individual CA policies with MAJOR/MINOR/PATCH
- Rollback to prior version via annotated Git tags — no existing tool offers structured rollback with version selection
- Status command — single dashboard view of policy count, drift status, last deploy, versions
- Report-only lifecycle support — deploy as report-only, validate, promote to enabled
- Policy validation at plan time — catch missing break-glass exclusions, conflicting conditions, empty include lists

**Defer (v2+):**
- Ring-based deployment (automated report-only → enabled promotion with validation gates)
- Multi-tenant orchestration (config per tenant, cross-tenant status)
- Git state backend (state as commits alongside policy definitions — complex, low demand initially)
- Maester integration (What-If API testing between plan and apply)
- Compliance baseline validation (CIS/CISA SCuBA checks at plan time)

### Architecture Approach

The architecture follows a clean layered design with a pure reconciliation engine at its core. The engine is a pure function (no I/O) that takes desired state, actual state, and the state manifest as inputs and produces a typed plan — this isolation enables table-driven tests with no mocks and makes the engine reusable by any future UI. All external boundaries (Backend, StateRepository, GraphClient, AuthProvider, Renderer) are Go interfaces; concrete implementations are wired in `cmd/` based on config. State is stored as JSON blobs in custom Git refs (`refs/cactl/tenants/<tid>/policies/<slug>`) with annotated tags for version snapshots — zero working tree footprint, no commit noise, no merge conflicts. For multi-tenant support, a ClientFactory creates and caches one authenticated GraphServiceClient per tenant ID; tenant ID flows explicitly as a parameter through every layer.

**Major components:**
1. `cmd/` (CLI layer) — thin Cobra commands that wire dependencies and delegate to internal packages; ~50-100 lines each
2. `internal/reconcile/` — pure reconciliation engine: desired vs actual vs state manifest → typed PlanSummary
3. `internal/graph/` — GraphClient interface + msgraph-sdk-go implementation with normalization and typed errors
4. `internal/state/` — StateRepository interface + GitStateRepository using go-git plumbing API
5. `internal/auth/` — AuthProvider interface + per-mode implementations (device code, SP, workload identity)
6. `internal/backend/` — Backend interface for reading policy JSON files (LocalFS first, Git/Blob later)
7. `internal/output/` — Renderer interface with HumanRenderer (colored terminal) and JSONRenderer

### Critical Pitfalls

Seven critical pitfalls were identified. The top five demand immediate architectural attention in Phase 1:

1. **Graph API POST creates silent duplicates** — The POST endpoint has no idempotency support; timeouts cause duplicate policies. Prevention: mandatory read-before-write (GET by displayName before POST; GET after POST to confirm and store Object ID).

2. **Server-computed field leakage into PATCH** — Importing raw API responses and patching them back causes false diffs and potential 400 errors. Prevention: strict allowlist-based normalization function as the single entry point for all policy data; never round-trip the raw API response.

3. **Wrong-tenant token acquisition via azidentity** — A known SDK issue means silent token acquisition may use the home tenant instead of the target tenant. Prevention: one credential instance per tenant (ClientFactory-per-tenant pattern is mandatory, not optional); validate `tid` JWT claim before every Graph API call.

4. **Git custom refs namespace design** — One ref per policy per tenant creates hundreds of loose refs and push/fetch complexity. Prevention: design with aggregated manifest per tenant (`refs/cactl/manifests/<tenant-id>`) from day one; retrofitting a ref namespace scheme is extremely painful.

5. **Config file leaks secrets into Git** — `config.yaml` committed before `.gitignore` is set up permanently exposes credentials. Prevention: ship `.gitignore` template in `cactl init`; support env-var-only secrets; add `cactl init` warning if config.yaml is tracked.

Additional: Reconciliation must handle untracked live resources explicitly (warn, never auto-delete) from Phase 3; concurrent pipeline applies require advisory locking from Phase 4.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Foundation — Auth, Graph Client, and Normalization

**Rationale:** The Graph client pitfalls (silent duplicates, server field leakage, wrong-tenant token) and the config secret exposure pitfall are all Phase 1 concerns that cannot be retrofitted. The architecture's build order mandates `pkg/types/` → `internal/config/` → `internal/auth/` → `internal/graph/` before anything else. Getting these right is the prerequisite for all other phases.

**Delivers:** Working auth chain (device code + SP secret), GraphClient interface with read-before-write CREATE, PATCH with allowlisted fields only, per-tenant ClientFactory, normalized policy response handling, typed Graph errors, 429 retry with backoff, `.gitignore` and config scaffolding via `cactl init`.

**Addresses:** Auth (device code + SP secret), CI/CD basics (--ci flag, --output json, exit codes), `cactl init` project scaffolding.

**Avoids:** Silent duplicate creation, server-field PATCH corruption, wrong-tenant token, config secret exposure, pagination failures, missing auth scopes.

**Research flag:** Standard patterns — azidentity and msgraph-sdk-go are well-documented. No research-phase needed.

---

### Phase 2: State Storage — Git Refs and State Manifest

**Rationale:** The state manifest (slug → Object ID mapping) is the dependency for plan, apply, import, and drift. The Git ref namespace design is the pitfall that is most expensive to fix after shipping — must be correct before any state is written. Architecture mandates `internal/state/` (GitStateRepository) + `internal/backend/` (LocalFSBackend first, then GitBackend) as Level 2 dependencies.

**Delivers:** StateRepository interface with GitStateRepository (go-git plumbing, `refs/cactl/manifests/<tid>` per-tenant aggregated manifest), annotated tag version snapshots, LocalFSBackend for development, refspec configuration in `cactl init`, `cactl import` command with full normalization.

**Addresses:** Import existing policies, export to version-controlled files, state manifest (local).

**Avoids:** Flat ref namespace scalability trap, push/fetch refspec misconfiguration, import-overwriting-uncommitted-changes without warning.

**Research flag:** Git plumbing with go-git is less commonly documented — may benefit from targeted research into go-git's `Storer` API behavior for annotated tags and custom refs.

---

### Phase 3: Reconciliation Engine and Plan/Apply Loop

**Rationale:** With auth, Graph client, and state storage in place, the reconciliation engine can be built as a pure function and fully tested with table-driven tests. Plan and apply are the core user-facing value. The untracked-resource pitfall must be baked into the engine's truth table from the start.

**Delivers:** `internal/reconcile/Engine` (pure function, full idempotency truth table including untracked state), `cmd/plan` (read-only, shows colored diff), `cmd/apply` (mutations with confirmation prompt, --auto-approve), HumanRenderer and JSONRenderer, semantic versioning bump logic (configurable MAJOR/MINOR/PATCH field triggers), break-glass validation at plan time, named location and group/user display name resolution.

**Addresses:** Plan/preview before apply, idempotent apply, colored diffs, break-glass validation, named location/group resolution, semantic versioning.

**Avoids:** Business logic in cmd/, untracked live resources causing silent destruction, displayName-as-identity trap, raw msgraph-sdk-go types in domain logic.

**Research flag:** Standard patterns — reconciliation engine design is well-documented in Kubernetes controller and Terraform patterns.

---

### Phase 4: Drift Detection and CI/CD Integration

**Rationale:** With the plan/apply loop validated, drift detection extends the core logic with three remediation modes (remediate/import/report). CI/CD integration polishes exit codes, concurrent apply protection, and the --ci flag behavior. This completes the v1 product.

**Delivers:** `cmd/drift` with three modes (remediate, import, report), advisory locking for concurrent applies, CI/CD integration (distinct exit codes 0/1/2, --auto-approve, --ci flag enforcing non-interactive mode), `cmd/status` health dashboard, report-only lifecycle support, workload identity and SP cert auth methods.

**Addresses:** Drift detection + remediation, CI/CD integration, status command, report-only lifecycle, additional auth methods.

**Avoids:** Concurrent pipeline state corruption (advisory locking), conflating "no changes" with "error" in exit codes, device code in CI environments.

**Research flag:** Advisory locking via Git refs is a non-standard pattern — research the reliability of `refs/cactl/locks/<tid>` as a distributed lock signal.

---

### Phase 5: Azure Blob Backend and v1.x Polish

**Rationale:** Azure Blob state backend is a lower-priority differentiator (P2 in feature matrix) that serves teams needing distributed state locking across CI agents. Policy validation rules (static analysis at plan time) and rollback complete the "should have" feature set.

**Delivers:** `internal/backend/AzureBlobBackend` (azblob SDK, blob leasing for concurrency), `cmd/rollback` (apply policy definition from prior annotated tag), policy validation rules (conflicting conditions, empty includes, overly broad blocks).

**Addresses:** Azure Blob state backend, rollback via Git tags, policy validation rules.

**Avoids:** Azure Blob without blob leasing (partial locking protection), rollback without version tags (depends on Phase 3 semver tags being present).

**Research flag:** Azure Blob lease semantics for distributed locking — verify lease acquisition/release behavior under CI failure scenarios.

---

### Phase 6: v2+ — Multi-Tenant Orchestration and Integrations

**Rationale:** Multi-tenant orchestration, ring-based deployment, and external integrations (Maester, ScubaGear baselines) are deferred until product-market fit is established with the single-tenant plan/apply loop.

**Delivers:** Multi-tenant config and tenant selector, ring-based deployment automation (report-only → enabled promotion), Maester integration workflow, compliance baseline validation, dependency graph visualization.

**Addresses:** Multi-tenant orchestration, ring-based deployment, Maester integration, compliance baseline validation.

**Research flag:** Multi-tenant at scale requires research into Graph API per-app throttling across multiple tenant tokens and potential Entra multi-tenant application registration patterns.

---

### Phase Ordering Rationale

- **Auth and Graph client first** because every subsequent phase makes Graph API calls; pitfalls in this layer (wrong-tenant tokens, silent duplicates) are architectural and cannot be retrofitted
- **State storage before reconciliation** because the engine needs all three inputs (desired, actual, manifest) to be defined; the Git ref namespace design is the highest-cost-to-fix pitfall
- **Plan before apply** because plan is read-only and validates the reconciliation logic before any mutations run; apply extends plan with execution
- **Drift and CI/CD together** because drift detection is conceptually the "plan without local changes" case, and CI/CD polish is the finishing coat on the plan/apply core
- **Blob backend and rollback in Phase 5** because they depend on Phase 2 (state) and Phase 3 (semver tags) being stable
- **Multi-tenant in Phase 6** because the single-tenant loop must be validated first; premature multi-tenant optimization creates cross-tenant token risk before it is needed

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 2 (State Storage):** go-git plumbing API for annotated tag creation and custom refs is less commonly documented; verify `Storer.SetEncodedObject` behavior for blob objects and confirm `ResolveRevision` annotated-tag dereference behavior
- **Phase 4 (CI/CD):** Advisory locking via Git refs (`refs/cactl/locks/<tid>`) as a distributed lock signal — verify cross-clone behavior and race condition windows
- **Phase 5 (Blob Backend):** Azure Blob lease acquisition/release semantics under CI failure (leaked locks) — verify lease TTL and renewal patterns
- **Phase 6 (Multi-tenant):** Graph API per-app throttling across many tenant tokens; Entra multi-tenant application registration patterns for MSP scenarios

Phases with standard patterns (skip research-phase):
- **Phase 1 (Auth/Graph):** azidentity and msgraph-sdk-go are official Microsoft SDKs with comprehensive documentation; patterns are well-established
- **Phase 3 (Reconciliation/Plan/Apply):** Reconciliation engine is a documented pattern from Kubernetes and Terraform; cobra/viper CLI wiring is a standard pattern

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All core libraries verified against pkg.go.dev and official release notes. One MEDIUM: r3labs/diff exact version unverified; olekukonko/tablewriter v1.0.7 MEDIUM. No blocking gaps. |
| Features | MEDIUM-HIGH | Competitor landscape well-researched with official docs for Terraform, M365DSC, ScubaGear. Community tool claims (DCToolbox, CIPP) are MEDIUM confidence. Feature prioritization is based on domain analysis, not user research. |
| Architecture | HIGH | Patterns are grounded in Go conventions (cmd/internal/pkg layout), Terraform's reconciliation model, and go-git's documented plumbing API. Anti-patterns are well-reasoned from the spec. |
| Pitfalls | HIGH | Graph API behavior (duplicate creation, PATCH semantics, throttling) verified against official Microsoft Learn docs. azidentity multi-tenant issue is a documented GitHub issue. Git ref pitfalls verified against Git internals documentation. |

**Overall confidence:** HIGH

### Gaps to Address

- **r3labs/diff exact version:** MEDIUM confidence on exact version — verify the latest v3.x release tag at implementation time before adding to go.mod
- **go-git annotated tag API:** The `CreateTag` API usage for annotated tags was referenced from a community blog post — verify against the go-git v5.17.0 source before building Phase 2 state storage
- **Graph API `$filter` by displayName:** Read-before-write duplicate prevention assumes `GET /identity/conditionalAccess/policies?$filter=displayName eq '...'` works as expected — verify this filter is supported for CA policies (some Graph resources do not support all OData filter operators)
- **Feature priority validation:** The feature prioritization matrix is based on domain analysis and competitor research, not user interviews. The relative priority of drift detection vs. rollback vs. semantic versioning should be validated with target users early in v1.x.

## Sources

### Primary (HIGH confidence)
- [Microsoft Graph CA Policy API](https://learn.microsoft.com/en-us/graph/api/resources/conditionalaccesspolicy?view=graph-rest-1.0) — CA policy schema, PATCH/POST semantics, auth scopes
- [Microsoft Graph throttling guidance](https://learn.microsoft.com/en-us/graph/throttling) — rate limiting behavior and Retry-After header
- [azidentity on pkg.go.dev](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity) — v1.13.1 credential types and multi-tenant behavior
- [Azure/azure-sdk-for-go issue #19726](https://github.com/Azure/azure-sdk-for-go/issues/19726) — documented multi-tenant token acquisition bug
- [msgraph-sdk-go on pkg.go.dev](https://pkg.go.dev/github.com/microsoftgraph/msgraph-sdk-go) — v1.96.0, typed fluent API
- [go-git/go-git on pkg.go.dev](https://pkg.go.dev/github.com/go-git/go-git/v5) — v5.17.0 plumbing API
- [spf13/cobra releases](https://github.com/spf13/cobra/releases) — v1.10.x stable, v2.0.0 Dec 2025 caution
- [golangci-lint v2 releases](https://github.com/golangci/golangci-lint/releases) — v2.10.1 Feb 2026
- [GoReleaser v2.14](https://goreleaser.com/blog/goreleaser-v2.14/) — cross-platform binary distribution
- [Microsoft365DSC GitHub](https://github.com/microsoft/Microsoft365DSC) — competitor feature analysis
- [ScubaGear GitHub](https://github.com/cisagov/ScubaGear) — CISA BOD 25-01 compliance scanner
- [Git Internals: Git References](https://git-scm.com/book/en/v2/Git-Internals-Git-References) — custom refs behavior

### Secondary (MEDIUM confidence)
- [DCToolbox GitHub](https://github.com/DanielChronlund/DCToolbox) — competitor feature analysis
- [CIPP Documentation](https://docs.cipp.app/) — MSP-focused competitor
- [Maester](https://maester.dev/) — What-If testing framework
- [Terraform azuread provider](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/conditional_access_policy) — closest competitor, plan/apply workflow
- [Cobra + Viper integration patterns](https://www.glukhov.org/post/2025/11/go-cli-applications-with-cobra-and-viper/) — community pattern
- [Go Project Structure](https://www.glukhov.org/post/2025/12/go-project-structure/) — cmd/internal/pkg layout
- [r3labs/diff v3](https://pkg.go.dev/github.com/r3labs/diff/v3) — struct diffing, version unverified
- [olekukonko/tablewriter](https://github.com/olekukonko/tablewriter) — v1.0.7 table formatting

### Tertiary (LOW confidence)
- [UCDWraith/entra_conditional_access_as_code](https://github.com/UCDWraith/entra_conditional_access_as_code) — community Terraform + Graph hybrid, referenced for feature landscape only

---
*Research completed: 2026-03-04*
*Ready for roadmap: yes*
