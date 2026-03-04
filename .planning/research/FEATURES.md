# Feature Research

**Domain:** Entra Conditional Access policy deploy framework (CLI/IaC)
**Researched:** 2026-03-04
**Confidence:** MEDIUM-HIGH

## Competitive Landscape Context

The CA-as-code space has existing tools but no single solution covers cactl's intended scope. Key competitors:

| Tool | Type | Strengths | Gaps |
|------|------|-----------|------|
| **Terraform azuread provider** | IaC (HCL) | plan/apply workflow, state management, ecosystem | 1 req/s API limit, missing auth strength support, no CA-specific semantics, no drift remediation, no rollback |
| **Microsoft365DSC** | PowerShell DSC | Desired state enforcement, drift detection via Sentinel, full M365 coverage | Complex setup, DSC learning curve, no plan preview, no semantic versioning, Windows-only |
| **DCToolbox** | PowerShell module | Export/import JSON, ring-based deployment, baseline PoC deploy, Excel reports | No state tracking, no drift detection, no plan/apply, manual diffing |
| **CIPP** | Web portal (MSP) | Multi-tenant templates, auto-redeploy standards, GUI | MSP-focused, no CLI, no version control integration, no diff preview |
| **ScubaGear (CISA)** | Compliance scanner | SCuBA baseline validation, OPA-based, federal mandate (BOD 25-01) | Assessment only -- no deploy, no state management |
| **Maester** | Test framework | What-If API testing, 280+ tests, CI/CD integration | Testing only -- no deploy, no state management |
| **IdPowerToys** | Documentation | PowerPoint CA documentation, visualization | Read-only, no deploy capabilities |
| **Manual PowerShell scripts** | Custom scripts | Flexible, familiar to identity teams | No state tracking, no idempotency, fragile, no diff preview |

**Key insight:** Terraform is the closest competitor for plan/apply workflow, but it treats CA policies as generic resources with no domain-specific intelligence. cactl's opportunity is to be the "Terraform for CA policies" with domain awareness that Terraform lacks.

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete or untrustworthy.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Plan/preview before apply** | Terraform trained users to expect dry-run before mutation. Identity teams will not blind-deploy CA policies that can lock out entire tenants. | HIGH | Core differentiator vs PowerShell scripts. Must show human-readable diff of what changes. |
| **Idempotent apply** | Users expect running apply twice to produce no changes. Non-idempotent deploys cause drift and confusion. | HIGH | Requires state manifest mapping slugs to live Object IDs. Must handle Graph API eventual consistency. |
| **Import existing policies** | Every org already has CA policies. A tool that only creates from scratch is DOA. Import is the onramp. | MEDIUM | Must normalize server-side fields (createdDateTime, modifiedDateTime, id). DCToolbox and M365DSC both offer this. |
| **Export to version-controlled files** | JSON/YAML policy files in Git is the baseline expectation for "as code." Without this, it is just another script. | LOW | One YAML/JSON file per policy. Alphabetize keys for stable diffs. |
| **Multiple auth methods** | Orgs use different auth for interactive vs CI/CD. Device code for humans, SP secret/cert for pipelines, workload identity for GitHub Actions. | MEDIUM | Terraform azuread supports all of these. Parity required. |
| **Break-glass account exclusion** | Every CA best practice guide mandates excluding emergency access accounts. Tool must make this easy and hard to forget. | LOW | Validate at plan time that break-glass group/accounts are excluded. Warn loudly if not. |
| **Report-only mode support** | Microsoft best practice: deploy in report-only first, validate, then enable. Tool must support this lifecycle. | LOW | State field on policy: reportOnly -> enabled. Could be part of ring-based deployment. |
| **CI/CD integration** | DevOps/platform engineers expect non-interactive mode, JSON output, meaningful exit codes. | MEDIUM | --ci flag, --auto-approve, --output json, exit codes (0=no changes, 1=error, 2=changes applied). |
| **Multi-tenant support** | MSPs and large enterprises manage CA across multiple tenants. Single-tenant-only is a dealbreaker for the MSP segment. | MEDIUM | Tenant ID as explicit parameter. ClientFactory per tenant. Config file per tenant or tenant selector. |
| **Colored human-readable diffs** | Identity architects reviewing changes in terminal need clear red/green diffs, not raw JSON blobs. | LOW | Terraform-style output. Property-level diffing, not full-object replacement. |
| **Named location awareness** | CA policies reference named locations by ID. Tool must resolve display names for readability and handle cross-tenant name mapping. | MEDIUM | Import must capture named locations. Plan must show names not GUIDs. |
| **Group/user resolution** | CA policies reference groups/users by Object ID. Diffs must show display names, not opaque GUIDs. | MEDIUM | Resolve at plan time for display. Store IDs in state. Handle deleted groups gracefully. |

### Differentiators (Competitive Advantage)

Features that set cactl apart. None of the existing tools do these well (or at all).

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Semantic versioning per policy** | No existing tool versions individual CA policies. Teams can track which policies had MAJOR (breaking) vs MINOR vs PATCH changes. Configurable field triggers (e.g., changing grant controls = MAJOR, changing display name = PATCH). | HIGH | Unique to cactl. Requires field-level change classification. Annotated Git tags per version. |
| **Drift detection with remediation options** | M365DSC detects drift but remediation is all-or-nothing. Terraform detects drift on refresh but offers only "apply to fix." cactl offers three modes: remediate (push local state), import live (accept portal changes), report only. | HIGH | Major differentiator. Must handle the common case where someone tweaks a policy in the portal. Per-policy remediation choice is key. |
| **Rollback via Git tags** | No existing tool offers structured rollback. DCToolbox can re-import a backup JSON, but there is no version selection or atomic rollback. cactl can rollback to any tagged version. | MEDIUM | Depends on semantic versioning. Rollback = apply the policy definition from a prior Git tag. Must handle dependencies between policies. |
| **Slug-based policy identity** | Terraform uses resource names in HCL. M365DSC uses display names. Both break when policies are renamed. Slugs (derived from naming convention) provide stable identity decoupled from display name. | MEDIUM | Slug in state manifest maps to live Object ID. Survives renames. Import must generate slugs from naming conventions. |
| **Import normalization** | DCToolbox exports raw JSON with server fields, nulls, and inconsistent ordering. cactl strips server-only fields, removes nulls, alphabetizes keys, and produces clean YAML ready for version control. | LOW | Quality-of-life differentiator. Makes Git diffs meaningful from day one. |
| **Policy validation at plan time** | No existing tool validates CA policy logic before deploy. cactl can catch: missing break-glass exclusions, conflicting conditions, empty include lists, policies that would block all users. | MEDIUM | Static analysis of policy definitions. Not a replacement for What-If API testing (that is Maester's domain), but catches obvious mistakes. |
| **Dependency-aware deployment ordering** | CA policies can reference named locations, auth strengths, and ToU agreements. Tool must deploy dependencies before dependents and tear down in reverse order. | MEDIUM | Terraform handles this generically via resource graph. cactl needs CA-specific dependency detection. |
| **Ring-based deployment support** | DCToolbox supports rings manually. cactl can automate: deploy to report-only first, validate with What-If, then promote to enabled. | MEDIUM | Could integrate with Maester for validation between rings. |
| **Status command with health overview** | Single command showing: policy count, drift status, last deploy time, version info, pending changes. No existing tool offers this dashboard view. | LOW | Low effort, high visibility. Teams can run `cactl status` in CI to verify tenant health. |
| **Multiple state backends** | Terraform supports many backends. M365DSC uses local files or Azure Automation. cactl offering Git, Azure Blob, and Local FS backends covers the main enterprise patterns. | MEDIUM | Git backend is unique -- state lives alongside policy definitions. Azure Blob for teams needing locking. |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems. Deliberately NOT building these.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **GUI/web portal** | "Not everyone uses CLI" | Splits development effort, creates two codepaths to maintain, CIPP already does this well. cactl's value is in the CLI/IaC workflow. | Produce JSON output that can be consumed by dashboards. Let CIPP/portal users stay in their lane. |
| **Full M365 configuration management** | "While you're at it, manage Exchange, Intune, SharePoint too" | Scope explosion. M365DSC already covers full M365. cactl's value is CA-specific depth, not breadth. | Stay focused on CA policies, named locations, auth strengths, and ToU -- the CA ecosystem. |
| **Policy template marketplace** | "Share templates between orgs" | Security policies are org-specific. Generic templates create false confidence. The Conditional Access Framework community project and CIPP template library already serve this need. | Support import from community frameworks (JSON/YAML). Do not host or curate templates. |
| **Real-time monitoring/alerting** | "Alert me when someone changes a policy in the portal" | This is a monitoring concern, not a deploy concern. Microsoft Sentinel + M365DSC audit log integration handles this. Adding a daemon/webhook listener to a CLI tool is architectural mismatch. | Drift detection on-demand (`cactl drift`) or scheduled in CI. Point users to Sentinel for real-time. |
| **What-If API simulation** | "Simulate policy impact before deploy" | Maester already does this extremely well with 20+ pre-built What-If tests and CI/CD integration. Duplicating this adds complexity without differentiation. | Integrate with Maester: `cactl plan` shows diff, user runs Maester tests, then `cactl apply`. Document the workflow. |
| **Automatic policy generation from compliance frameworks** | "Generate policies from CIS/CISA baselines" | Compliance mappings change frequently, require deep domain expertise, and generate liability concerns. ScubaGear handles CISA SCuBA baselines. | Support importing ScubaGear/community baseline policies. Validate against baselines as a plan-time check (future). |
| **ClickOps detection and prevention** | "Block portal changes entirely" | Requires Entra PIM/RBAC changes outside cactl's scope. Fighting the portal creates user hostility. | Drift detection catches portal changes. Remediate or import. Organizational policy (not tooling) should govern who uses the portal. |
| **Policy rollback across multiple tenants atomically** | "Roll back all tenants at once" | Cross-tenant atomicity is impossible -- each tenant is an independent Graph API endpoint. Partial failures are inevitable. | Rollback per tenant. Orchestrate multi-tenant rollback via CI/CD pipeline that calls `cactl rollback` per tenant. |

## Feature Dependencies

```
[Import] (onramp)
    |
    v
[State Manifest] (slug -> Object ID mapping)
    |
    +---> [Plan/Apply] (requires state to compute diff)
    |         |
    |         +---> [Colored Diffs] (enhances plan output)
    |         |
    |         +---> [CI/CD Integration] (wraps plan/apply with flags)
    |         |
    |         +---> [Policy Validation] (enhances plan with checks)
    |
    +---> [Drift Detection] (requires state to compare against live)
    |         |
    |         +---> [Drift Remediation] (extends drift with actions)
    |
    +---> [Semantic Versioning] (requires state to track changes)
              |
              +---> [Rollback] (requires version tags to exist)
              |
              +---> [Status Command] (displays version + drift info)

[Auth Module] (independent, needed by everything)
    |
    +---> [Multi-tenant] (extends auth with tenant routing)

[Named Location Resolution] (independent, enhances readability)
[Group/User Resolution] (independent, enhances readability)
[Break-glass Validation] (independent, enhances plan)
[Report-only Support] (independent, policy state lifecycle)
    |
    +---> [Ring-based Deployment] (extends report-only with promotion)

[Multiple State Backends] (independent, extends state manifest storage)
```

### Dependency Notes

- **Import -> State Manifest:** Import is the onramp that populates state. Without import, state must be built from scratch via init.
- **State Manifest -> Plan/Apply:** Plan computes diff between local files and state+live. Apply mutates live and updates state. Everything depends on state.
- **State Manifest -> Drift Detection:** Drift compares state manifest against live Graph API responses. Without state, there is nothing to compare.
- **Semantic Versioning -> Rollback:** Rollback targets a specific version tag. Without versioning, there is nothing to roll back to.
- **Auth Module is foundational:** Every Graph API call requires auth. Multi-tenant extends this with tenant routing.
- **Report-only -> Ring-based Deployment:** Rings are a formalization of the report-only -> enabled lifecycle.

## MVP Definition

### Launch With (v1)

Minimum viable product -- what is needed to validate the concept and be useful for a single tenant.

- [ ] **cactl init** -- Initialize project structure and config file
- [ ] **cactl import** -- Import existing CA policies with normalization (strip server fields, remove nulls, alphabetize)
- [ ] **cactl plan** -- Show diff between local policy files and live tenant (colored, human-readable)
- [ ] **cactl apply** -- Deploy changes with idempotency, update state manifest
- [ ] **State manifest** -- Slug to Object ID mapping, local file backend
- [ ] **Auth: device code + SP secret** -- Interactive and CI/CD auth
- [ ] **Break-glass validation** -- Warn at plan time if emergency accounts not excluded
- [ ] **CI/CD basics** -- --ci flag, --auto-approve, --output json, exit codes
- [ ] **Named location + group resolution** -- Display names in diffs, not GUIDs

### Add After Validation (v1.x)

Features to add once core plan/apply loop is working and users provide feedback.

- [ ] **cactl drift** -- On-demand drift detection with remediation options (remediate/import/report)
- [ ] **Semantic versioning** -- Per-policy MAJOR/MINOR/PATCH with configurable field triggers
- [ ] **cactl rollback** -- Rollback to prior version via annotated Git tags
- [ ] **cactl status** -- Health dashboard (policy count, drift status, versions, last deploy)
- [ ] **Report-only lifecycle** -- Deploy as report-only, validate, promote to enabled
- [ ] **SP cert auth + workload identity** -- Additional auth methods for advanced CI/CD
- [ ] **Azure Blob state backend** -- For teams needing state locking
- [ ] **Policy validation rules** -- Catch conflicting conditions, empty includes, overly broad blocks

### Future Consideration (v2+)

Features to defer until product-market fit is established.

- [ ] **Ring-based deployment** -- Automated report-only -> enabled promotion with validation gates
- [ ] **Multi-tenant orchestration** -- Config per tenant, tenant selector, cross-tenant status
- [ ] **Git state backend** -- State as commits alongside policy definitions
- [ ] **Maester integration** -- Run What-If tests between plan and apply
- [ ] **Dependency graph visualization** -- Show named location/auth strength/ToU dependencies
- [ ] **Import from community frameworks** -- Ingest Conditional Access Framework, CIPP templates, ScubaGear baselines
- [ ] **Compliance baseline validation** -- Check policies against CIS/CISA SCuBA baselines at plan time

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Plan/preview before apply | HIGH | HIGH | P1 |
| Idempotent apply | HIGH | HIGH | P1 |
| Import existing policies | HIGH | MEDIUM | P1 |
| State manifest (local) | HIGH | MEDIUM | P1 |
| Auth (device code + SP secret) | HIGH | MEDIUM | P1 |
| Export to version-controlled files | HIGH | LOW | P1 |
| Colored human-readable diffs | HIGH | LOW | P1 |
| Named location/group resolution | HIGH | MEDIUM | P1 |
| Break-glass validation | HIGH | LOW | P1 |
| CI/CD integration | HIGH | LOW | P1 |
| Drift detection + remediation | HIGH | HIGH | P2 |
| Semantic versioning | MEDIUM | HIGH | P2 |
| Rollback via Git tags | MEDIUM | MEDIUM | P2 |
| Status command | MEDIUM | LOW | P2 |
| Report-only lifecycle | MEDIUM | LOW | P2 |
| Policy validation rules | MEDIUM | MEDIUM | P2 |
| Additional auth methods | MEDIUM | LOW | P2 |
| Azure Blob backend | LOW | MEDIUM | P2 |
| Ring-based deployment | MEDIUM | HIGH | P3 |
| Multi-tenant orchestration | MEDIUM | HIGH | P3 |
| Git state backend | LOW | HIGH | P3 |
| Maester integration | MEDIUM | MEDIUM | P3 |
| Compliance baseline validation | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for launch -- the plan/apply loop with import
- P2: Should have -- drift, versioning, and rollback complete the story
- P3: Nice to have -- multi-tenant, integrations, advanced deployment patterns

## Competitor Feature Analysis

| Feature | Terraform azuread | Microsoft365DSC | DCToolbox | CIPP | cactl (planned) |
|---------|-------------------|-----------------|-----------|------|-----------------|
| Plan/preview | Yes (generic) | No | No | No | Yes (CA-specific) |
| Idempotent apply | Yes | Yes (DSC) | No | Partial | Yes |
| Import existing | Yes (import block) | Yes (export cmd) | Yes (JSON) | Yes (template) | Yes (normalized) |
| State management | Yes (tfstate) | No (DSC pulls) | No | No | Yes (manifest) |
| Drift detection | Yes (refresh) | Yes (Sentinel) | No | Partial (standards) | Yes (3 modes) |
| Rollback | No (manual) | No | No (re-import backup) | No | Yes (Git tags) |
| Versioning | No | No | No | No | Yes (semantic) |
| CI/CD native | Yes | Partial | No | No | Yes |
| Multi-tenant | Via workspaces | Yes | No | Yes (MSP) | Yes (planned) |
| CA-specific validation | No | No | No | No | Yes |
| Named location resolution | No (raw IDs) | Yes (display names) | No | Yes | Yes |
| Break-glass checks | No | No | No | No | Yes |
| Ring deployment | No | No | Manual | Via standards | Planned |
| Report-only lifecycle | Manual | Manual | Yes | Yes | Planned |
| API rate handling | Auto-retry | N/A (DSC) | No | N/A | Must implement |

## Sources

- [Terraform azuread_conditional_access_policy](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/conditional_access_policy) -- Official Terraform registry docs (MEDIUM confidence)
- [Microsoft365DSC GitHub](https://github.com/microsoft/Microsoft365DSC) -- Official repo (HIGH confidence)
- [DCToolbox GitHub](https://github.com/DanielChronlund/DCToolbox) -- Community tool (MEDIUM confidence)
- [CIPP Documentation](https://docs.cipp.app/user-documentation/tenant/conditional/list-policies) -- Official docs (MEDIUM confidence)
- [ScubaGear GitHub](https://github.com/cisagov/ScubaGear) -- CISA official tool (HIGH confidence)
- [Maester](https://maester.dev/) -- Official site (MEDIUM confidence)
- [IdPowerToys GitHub](https://github.com/merill/idPowerToys) -- Community tool (MEDIUM confidence)
- [Microsoft Learn: CA deployment planning](https://learn.microsoft.com/en-us/entra/identity/conditional-access/plan-conditional-access) -- Official docs (HIGH confidence)
- [Microsoft Learn: CA audit logs](https://learn.microsoft.com/en-us/entra/identity/conditional-access/troubleshoot-policy-changes-audit-log) -- Official docs (HIGH confidence)
- [Terraform azuread provider issues](https://github.com/hashicorp/terraform-provider-azuread/issues) -- GitHub issues for limitation tracking (MEDIUM confidence)
- [UCDWraith/entra_conditional_access_as_code](https://github.com/UCDWraith/entra_conditional_access_as_code) -- Community Terraform + Graph hybrid (LOW confidence)
- [CISA BOD 25-01](https://www.cisa.gov/news-events/directives/bod-25-01-implementation-guidance-implementing-secure-practices-cloud-services) -- Federal mandate (HIGH confidence)
- [CA naming conventions](https://www.welkasworld.com/post/conditional-access-naming-conventions-personas-design-process) -- Community best practice (MEDIUM confidence)
- [Conditional Access Framework](https://www.joeyverlinden.com/conditional-access-framework-5/) -- Community framework (MEDIUM confidence)

---
*Feature research for: Entra Conditional Access policy deploy framework (cactl)*
*Researched: 2026-03-04*
