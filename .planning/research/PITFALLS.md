# Pitfalls Research

**Domain:** CLI-first deploy framework for Microsoft Entra Conditional Access policies (Go, Graph API, Git-backed state)
**Researched:** 2026-03-04
**Confidence:** HIGH (Graph API behavior, Git internals, azidentity) / MEDIUM (reconciliation edge cases, go-git custom refs)

## Critical Pitfalls

### Pitfall 1: Graph API POST Creates Silent Duplicates

**What goes wrong:**
The Microsoft Graph `POST /identity/conditionalAccess/policies` endpoint has no idempotency support. If a network timeout occurs and the CLI retries the request, a second policy with an identical configuration but a different GUID is silently created. The user now has two active CA policies enforcing the same controls, with no indication anything went wrong. Worse, a subsequent `cactl apply` may not detect the duplicate because it matches on the stored GUID, not on policy content.

**Why it happens:**
Graph API does not support `Idempotency-Key` headers, does not deduplicate on `displayName`, and returns a new unique `id` for every POST regardless of payload identity. This is a fundamental API design gap, not a bug.

**How to avoid:**
Implement a mandatory "read-before-write" pattern: before every CREATE, query `GET /identity/conditionalAccess/policies` filtered by `displayName` (or a cactl-specific naming convention that embeds a deterministic identifier). Only POST if no match exists. After POST, immediately GET to confirm the created policy and store its `id` in the state manifest. Wrap the entire create flow in a mutex at the application level to prevent concurrent pipeline runs from racing on the same policy name.

**Warning signs:**
- Integration tests that occasionally create two policies with the same name
- State manifest contains an `id` that does not match any live policy (the "other" duplicate was kept)
- Users report "ghost" policies appearing after pipeline retries or timeouts

**Phase to address:**
Phase 1 (Core Graph client). This pattern must be baked into the lowest-level client, not layered on later.

---

### Pitfall 2: PATCH Updates Send Server-Computed Fields Back to the API

**What goes wrong:**
When importing a policy from the tenant, the response includes server-computed read-only fields (`id`, `createdDateTime`, `modifiedDateTime`, and sometimes nested computed fields). If the import normalization is incomplete and these fields leak into the stored policy definition, a subsequent PATCH update sends them back. Graph API may silently ignore them, or -- in edge cases with beta endpoints -- return 400 errors. More critically, including unchanged fields in the PATCH body makes the diff engine report false changes because the server may return slightly different values (e.g., timestamp precision, null vs absent fields).

**Why it happens:**
The Graph API PATCH documentation states "don't include existing values that haven't changed" for performance, but does not throw errors for most extra fields -- creating a false sense of safety. Developers skip strict normalization because "it works."

**How to avoid:**
Define an explicit allowlist of mutable fields for the CA policy schema. During import normalization, strip all fields not on the allowlist. During PATCH construction, diff only allowlisted fields and send only changed values. Never round-trip the raw API response back as a PATCH body. Write a `normalize()` function that is the single entry point for all policy data entering the system.

**Warning signs:**
- Diff engine shows "changes" on every plan even when the user changed nothing
- PATCH requests are larger than expected (sending full objects instead of deltas)
- Flaky tests where timestamp fields cause intermittent diff mismatches

**Phase to address:**
Phase 1 (Import/normalization). Must be correct before the reconciliation engine is built on top of it.

---

### Pitfall 3: Git Custom Refs Namespace Scalability and Push Behavior

**What goes wrong:**
Using `refs/cactl/*` as state storage works at small scale (single tenant, handful of policies), but Git's one-file-per-ref format degrades performance with hundreds of refs. Additionally, `git push` does not automatically transfer custom namespace refs -- developers must explicitly configure refspecs. If a user clones the repo, they get none of the cactl state unless the clone refspec is also configured. Annotated tags under a custom namespace compound this because each tag creates an additional object in the object database.

**Why it happens:**
Git's ref system was designed for branches and tags, not arbitrary state storage. Custom namespaces like `refs/cactl/` are supported but are second-class citizens in terms of tooling, push defaults, and pack-refs optimization. Most Git tutorials and documentation do not cover custom namespace patterns, so developers discover these gaps only after shipping.

**How to avoid:**
1. Design the ref namespace to be shallow -- use `refs/cactl/manifests/<tenant-id>` (one ref per tenant) rather than one ref per policy per tenant.
2. Document explicit push/fetch refspec configuration in the setup guide: `push = +refs/cactl/*:refs/cactl/*` in `.git/config`.
3. Periodically run `git pack-refs --all` in CI to consolidate loose refs.
4. For annotated tags (version snapshots), use a naming scheme that allows garbage collection of old versions: `refs/cactl/versions/<tenant>/<semver>`.
5. Consider whether the state that must travel with the repo should live in annotated tags (immutable, pushed explicitly) vs. lightweight refs (mutable, local only).

**Warning signs:**
- `git push` succeeds but remote repo has no cactl refs
- Clone of the repo has empty state; `cactl status` shows no tracked policies
- Repository operations slow down as tenant/policy count grows
- `git gc` warnings about many loose objects

**Phase to address:**
Phase 2 (Git state storage). Must be designed correctly before any state is written. Retrofitting a ref namespace scheme is extremely painful because existing refs must be migrated.

---

### Pitfall 4: Reconciliation Engine Fails on Untracked Live Resources

**What goes wrong:**
The reconciliation engine compares "desired state" (Git) against "live state" (tenant). But if a policy exists in the tenant that was never imported into cactl (created manually in the portal, by another tool, or by another cactl instance), the reconciler has no reference for it. The naive approach is to ignore untracked policies, but this means `cactl apply` can silently leave conflicting policies active. The dangerous approach is to delete untracked policies, which destroys policies the user intentionally manages outside cactl.

**Why it happens:**
The Terraform ecosystem calls this the "import problem." Real-world tenants are never greenfield -- they have existing policies, policies from other tools, and policies that should remain unmanaged. The reconciliation truth table must handle the "untracked" state explicitly, but developers often defer this to "later" and build the engine assuming full ownership.

**How to avoid:**
Implement the full truth table from day one:
- **Tracked + Live**: normal reconcile (update/noop)
- **Tracked + Missing**: recreate or error (configurable)
- **Untracked + Live**: warn, optionally import, never auto-delete
- **Tracked + Deleted from desired state**: explicit `cactl destroy` required, not implicit

Add a `cactl import` command early. Add an `--untracked-policy` flag (`warn`, `ignore`, `import`, `error`) to `cactl plan` so operators can choose behavior per environment.

**Warning signs:**
- Users ask "how do I add my existing policies to cactl?"
- `cactl plan` output is confusing because it does not mention policies visible in the portal
- An operator accidentally deletes a production policy they did not intend to manage

**Phase to address:**
Phase 3 (Reconciliation engine). But the data model must support untracked states from Phase 2 (state storage).

---

### Pitfall 5: azidentity Multi-Tenant Token Acquisition Uses Wrong Tenant

**What goes wrong:**
When using `ClientSecretCredential` or `ClientCertificateCredential` with azidentity, the credential is initialized with a "home" tenant ID. When cactl needs to operate on a different tenant, the SDK has a known issue where `opts.TenantID` in `GetToken()` is not correctly used for silent token acquisition -- the token may be acquired for the home tenant instead of the target tenant. This means cactl could apply policies to the wrong tenant, a catastrophic failure mode in a multi-tenant tool.

**Why it happens:**
This is a documented upstream SDK issue (Azure/azure-sdk-for-go#19726). The `AdditionallyAllowedTenants` option exists but must be explicitly configured. Even when configured, the silent token cache may return a token for the wrong tenant if the cache key does not include tenant ID.

**How to avoid:**
1. Create a **separate credential instance per tenant** -- do not reuse a single credential across tenants. The `ClientFactory` pattern in the spec is correct: each tenant gets its own credential + client pair.
2. Always set `AdditionallyAllowedTenants` even when using per-tenant credentials, as a defense-in-depth measure.
3. After acquiring a token, decode the JWT and assert the `tid` (tenant ID) claim matches the expected tenant before making any Graph API calls.
4. Write an integration test that operates on two tenants sequentially and verifies policies land in the correct tenant.

**Warning signs:**
- Policies appearing in the wrong tenant during multi-tenant testing
- Token refresh errors after switching between tenants in the same session
- `403 Forbidden` errors that resolve when the tool is restarted (stale cached token for wrong tenant)

**Phase to address:**
Phase 1 (Auth layer). Must be architecturally correct from the start -- the ClientFactory-per-tenant pattern cannot be bolted on later without rewriting every Graph call site.

---

### Pitfall 6: Concurrent Pipeline Applies Corrupt State

**What goes wrong:**
Two CI/CD pipelines run `cactl apply` simultaneously for the same tenant. Both read the same state manifest, both compute the same diff, and both attempt the same mutations. Result: duplicate policy creation (see Pitfall 1), conflicting PATCH updates, or a state manifest that reflects only one pipeline's changes while the other's mutations are orphaned.

**Why it happens:**
Git refs have file-level locking (`.lock` files), but this only prevents concurrent writes to the same ref on the same machine. In CI/CD, pipelines run on different machines with separate clones, so Git's local locking provides zero protection. There is no distributed lock built into Git.

**How to avoid:**
1. Implement advisory locking in the state manifest: write a `refs/cactl/locks/<tenant-id>` ref containing the pipeline run ID and timestamp before starting apply. Check for this ref before proceeding. Delete after completion.
2. In CI, use the CI platform's native concurrency controls (e.g., GitHub Actions `concurrency` groups keyed on tenant ID) as the primary guard.
3. Make `cactl apply` detect stale state: before writing, verify the state ref has not been updated since the plan was computed. If it has, abort with a clear error.
4. Design the state manifest to be append-only where possible (annotated tags for versions are naturally safe because they are immutable).

**Warning signs:**
- CI logs show two `cactl apply` runs overlapping in time for the same tenant
- State manifest `modifiedDateTime` jumps backward
- "Ghost" policies in the tenant that are not in the state manifest

**Phase to address:**
Phase 4 (CI/CD integration). But the state storage design (Phase 2) must support lock refs from the beginning.

---

### Pitfall 7: Config File Leaks Secrets into Git

**What goes wrong:**
`config.yaml` contains tenant IDs, client IDs, and potentially client secrets for service principal authentication. If this file is committed to the repository, secrets are exposed in Git history permanently (even if later removed from the working tree). Tenant IDs alone can be sensitive in enterprise contexts.

**Why it happens:**
During development, it is natural to test with a config file in the repo root. The `.gitignore` is not set up yet, or the developer adds `config.yaml` before adding the gitignore entry. Once committed, the secret is in history forever unless the repo is rewritten.

**How to avoid:**
1. Ship a `.gitignore` template that excludes `config.yaml`, `*.secret`, and common credential file patterns from the very first commit.
2. Support environment variable overrides for all secrets (`CACTL_CLIENT_SECRET`, etc.) and document this as the recommended CI approach.
3. Never store secrets in config.yaml -- only reference them (e.g., `client_secret_env: CACTL_CLIENT_SECRET`).
4. Add a `cactl doctor` or `cactl init` check that warns if config.yaml is tracked by Git.
5. For certificate-based auth, store only the path to the cert file, never the cert content.

**Warning signs:**
- `git log --all --diff-filter=A -- config.yaml` returns results
- CI pipeline has credentials hardcoded in YAML instead of using secrets/vault
- Security scanner flags the repository

**Phase to address:**
Phase 1 (Project scaffolding / `cactl init`). The `.gitignore` and env-var pattern must exist before any user touches the tool.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Storing full API responses as state (no normalization) | Fast import implementation | Every diff shows phantom changes from server-computed fields; schema changes break storage | Never -- normalization is mandatory from day one |
| Single credential shared across tenants | Simpler auth code | Wrong-tenant mutations, impossible to debug token issues | Never for multi-tenant tools |
| Hardcoding Graph API v1.0 paths without version abstraction | Fewer indirection layers | Painful migration when Microsoft ships v2.0 or changes beta endpoints | Acceptable in MVP if the abstraction boundary is documented for later |
| Skipping advisory locking for applies | Simpler state management | Corrupt state in any CI/CD environment with parallelism | Only acceptable for single-user local development |
| Using `displayName` as the policy identity key | No need for state manifest | Breaks when two policies share a name (Graph allows it); rename = orphaned state | Never -- use the Graph-assigned `id` as the primary key, `displayName` as a human label |
| Flat ref namespace (one ref per policy) | Simple mental model | Hundreds of loose refs per tenant; slow Git operations; push/fetch configuration nightmare | Never at scale -- use aggregated manifests per tenant |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Graph API POST (create) | Retrying on timeout without checking if the policy was created | Read-before-write: GET by displayName before POST; GET after POST to confirm |
| Graph API PATCH (update) | Sending the full policy object including read-only fields | Send only changed mutable fields; maintain an explicit allowlist |
| Graph API GET (list) | Not handling pagination; assuming all policies fit in one response | Always follow `@odata.nextLink`; use `$top` and iterate |
| Graph API (auth scopes) | Requesting `Policy.ReadWrite.ConditionalAccess` without `Application.Read.All` | Both scopes are required; the API returns misleading 403 without the read scope |
| Graph API (throttling) | No retry logic; failing on first 429 | Parse `Retry-After` header; implement exponential backoff; batch reads where possible (max 20 per batch) |
| Git custom refs | Assuming `git push` transfers custom namespace refs | Explicitly configure push refspec: `+refs/cactl/*:refs/cactl/*` |
| Git annotated tags | Using `go-git` `ResolveRevision` on annotated tag and treating the result as a commit hash | Must dereference the tag object to get the commit hash; use `TagObject` then `Target` |
| azidentity device code flow | Assuming device code works in CI | Device code requires interactive browser login; CI must use service principal or workload identity |
| azidentity token caching | Assuming cached tokens are scoped to the correct tenant | Validate `tid` claim in JWT after acquisition; use separate credential instances per tenant |
| Viper + Cobra config | Binding pflags in `init()` instead of `PersistentPreRunE` | Flags bound in `init()` override config file values unpredictably; bind in PreRun for correct precedence |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Fetching all policies per reconcile cycle | `cactl plan` takes 10+ seconds | Cache policy list with short TTL; use `$select` to fetch only needed fields | At 50+ policies per tenant, or across 5+ tenants |
| One Graph API call per policy (no batching) | Throttling (429) during apply | Batch reads into groups of 20 (Graph batch limit); serialize writes with backoff | At 20+ policy mutations in a single apply |
| Full Git ref scan on every operation | CLI startup becomes slow | Cache ref state in memory per session; only scan refs on `plan`/`apply` | At 100+ refs in `refs/cactl/` namespace |
| Annotated tag creation for every field change | Object database bloat; slow `git gc` | Only create version tags on actual applies, not on plans or imports | At 200+ version tags per tenant |
| Deserializing entire state manifest for single-policy lookup | Memory pressure, slow operations | Index policies by ID in the manifest; consider per-tenant manifest files | At 100+ policies per tenant manifest |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Storing `client_secret` in `config.yaml` committed to Git | Credential exposure; anyone with repo access can impersonate the service principal | Use environment variables or secret references only; warn in `cactl init` if secrets detected in config |
| Using `DefaultAzureCredential` in production | Unpredictable credential chain; may authenticate as wrong identity in shared environments | Use explicit credential type (`ClientSecretCredential`, `ClientCertificateCredential`, `WorkloadIdentityCredential`) based on context |
| Not validating tenant ID in acquired tokens | Policies applied to wrong tenant | Decode JWT and assert `tid` claim matches target tenant before every Graph API call |
| Over-scoped Graph API permissions | Compromised credential can modify all Entra resources, not just CA policies | Request minimum scopes: `Policy.ReadWrite.ConditionalAccess` + `Policy.Read.All` + `Application.Read.All`; avoid `Directory.ReadWrite.All` |
| Device code flow token persisted to disk without encryption | Token theft from CI artifacts or local filesystem | Use in-memory token cache only; never write tokens to disk; rely on azidentity's built-in cache |
| CA policy with `state: "enabled"` deployed without testing | Lockout: policy blocks all users including admins | Default new policies to `state: "enabledForReportingButNotEnforced"`; require explicit `--enable` flag for production |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| `cactl apply` with no plan preview | User cannot see what will change before it happens | Always show plan output before apply; require `--auto-approve` to skip confirmation |
| Diff output shows raw JSON with no highlighting | Users cannot quickly identify what changed | Use colored, field-level diff output (red/green); group changes by policy |
| Error messages expose raw Graph API error JSON | Users cannot understand what went wrong | Wrap Graph errors with human-readable context: "Failed to update policy 'Block Legacy Auth': 403 Forbidden -- check that the service principal has Policy.ReadWrite.ConditionalAccess permission" |
| No distinction between "nothing to do" and "error" exit codes | CI pipelines cannot distinguish success from failure from no-op | Use distinct exit codes: 0 = success with changes, 1 = error, 2 = no changes needed |
| Silent behavior in `--ci` mode | Operators cannot debug pipeline failures | Even in CI mode, emit structured JSON logs to stderr; use `--output json` for machine-parseable stdout |
| `cactl import` overwrites local changes without warning | User loses uncommitted policy edits | Check for uncommitted changes before import; require `--force` to overwrite |

## "Looks Done But Isn't" Checklist

- [ ] **Policy creation:** Often missing duplicate detection -- verify that POST checks for existing policy by displayName before creating
- [ ] **Import normalization:** Often missing null-field stripping -- verify that `null` values, empty arrays, and server-computed fields are removed
- [ ] **Git ref push:** Often missing custom refspec configuration -- verify that `refs/cactl/*` refs actually arrive at the remote after `git push`
- [ ] **Multi-tenant auth:** Often missing per-tenant credential isolation -- verify that two sequential tenant operations use different credentials
- [ ] **Reconciliation plan:** Often missing the "untracked" policy state -- verify that policies in the tenant but not in state are surfaced to the user
- [ ] **Semantic versioning:** Often missing MAJOR version bump triggers -- verify that removing a policy or changing `state` from disabled to enabled triggers a MAJOR bump
- [ ] **Throttle handling:** Often missing 429 retry -- verify that a burst of 30 API calls does not crash the tool
- [ ] **CI exit codes:** Often conflating "no changes" with "success" -- verify exit code 2 when plan shows no drift
- [ ] **Config secret detection:** Often missing commit guard -- verify that `cactl init` warns if `config.yaml` contains secrets and is not gitignored
- [ ] **Annotated tag dereferencing:** Often missing peel-to-commit -- verify that reading a version tag returns the commit hash, not the tag object hash

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Duplicate policies created via Graph API | MEDIUM | List all policies by displayName; identify duplicates by comparing full JSON (minus server fields); delete the newer duplicate; update state manifest with surviving ID |
| State manifest out of sync with live tenant | MEDIUM | Run `cactl import --force` to rebuild state from live tenant; diff against Git history to verify no policies were lost; commit new state |
| Wrong-tenant policy application | HIGH | Immediately disable all policies created in wrong tenant (`state: "disabled"`); delete them; verify correct tenant state is intact; audit token acquisition code |
| Secrets committed to Git history | HIGH | Rotate the compromised credential immediately; use `git filter-repo` to remove the file from history; force-push (requires coordination with all consumers); enable secret scanning |
| Corrupt state from concurrent applies | MEDIUM | Identify which pipeline's changes "won" by comparing state manifest timestamps; re-import from live tenant; reconcile the lost pipeline's intended changes manually |
| Git ref namespace migration needed | HIGH | Write a migration script that reads old refs, creates new refs, and deletes old ones; must be atomic (all or nothing); test on a clone first; communicate to all users |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Graph API duplicate creation | Phase 1: Core Graph client | Integration test: create policy twice with same name; assert only one exists |
| Server-computed field leakage | Phase 1: Import/normalization | Unit test: normalize API response and assert no read-only fields remain |
| Config secret exposure | Phase 1: Project scaffolding | `cactl init` test: create config with secret; assert warning emitted; assert .gitignore covers it |
| Wrong-tenant token acquisition | Phase 1: Auth/ClientFactory | Integration test: sequential multi-tenant ops; decode tokens and assert correct `tid` |
| Git ref push/fetch behavior | Phase 2: State storage | End-to-end test: push state, clone repo, verify cactl refs present in clone |
| Ref namespace scalability | Phase 2: State storage design | Load test: create 200 policies across 5 tenants; measure `cactl plan` latency |
| Untracked policy handling | Phase 3: Reconciliation engine | Integration test: add policy in portal; run `cactl plan`; assert untracked warning |
| Concurrent apply corruption | Phase 4: CI/CD integration | Test: two parallel applies; assert one fails with lock error or state conflict |
| Throttle handling | Phase 1: Graph client | Integration test: send 50 rapid requests; assert no unhandled 429 crashes |
| Graph schema migration | Ongoing: maintenance | Monitor Microsoft Graph changelog; pin API version; test against beta periodically |

## Sources

- [Microsoft Graph: conditionalAccessPolicy resource type](https://learn.microsoft.com/en-us/graph/api/resources/conditionalaccesspolicy?view=graph-rest-1.0) -- HIGH confidence
- [Microsoft Graph: Create conditionalAccessPolicy](https://learn.microsoft.com/en-us/graph/api/conditionalaccessroot-post-policies?view=graph-rest-1.0) -- HIGH confidence
- [Microsoft Graph: Update conditionalAccessPolicy (PATCH)](https://learn.microsoft.com/en-us/graph/api/conditionalaccesspolicy-update?view=graph-rest-1.0) -- HIGH confidence
- [Microsoft Graph throttling guidance](https://learn.microsoft.com/en-us/graph/throttling) -- HIGH confidence
- [Microsoft Graph service-specific throttling limits](https://learn.microsoft.com/en-us/graph/throttling-limits) -- HIGH confidence
- [Microsoft Graph versioning and breaking change policies](https://learn.microsoft.com/en-us/graph/versioning-and-support) -- HIGH confidence
- [Azure/azure-sdk-for-go: Multi-tenant auth issue #19726](https://github.com/Azure/azure-sdk-for-go/issues/19726) -- HIGH confidence
- [azidentity TROUBLESHOOTING.md](https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/TROUBLESHOOTING.md) -- HIGH confidence
- [Git internals: Git References](https://git-scm.com/book/en/v2/Git-Internals-Git-References) -- HIGH confidence
- [Git reftable documentation](https://git-scm.com/docs/reftable) -- HIGH confidence
- [Git pack-refs documentation](https://git-scm.com/docs/git-pack-refs) -- HIGH confidence
- [GitHub Blog: Git Concurrency in GitHub Desktop](https://github.blog/2015-10-20-git-concurrency-in-github-desktop/) -- MEDIUM confidence
- [go-git: ResolveRevision annotated tag issue #772](https://github.com/src-d/go-git/issues/772) -- HIGH confidence
- [State Reconciliation Defects in IaC (ACM)](https://dl.acm.org/doi/10.1145/3660790) -- MEDIUM confidence
- [Terraform drift detection guidance (HashiCorp)](https://developer.hashicorp.com/terraform/tutorials/state/resource-drift) -- MEDIUM confidence
- [Cobra GitHub repository](https://github.com/spf13/cobra) -- HIGH confidence
- [Graph API 429 throttling workaround (blog.atwork.at)](https://blog.atwork.at/post/2025/microsoft-graph-api-429-too-many-requests-workaround/) -- MEDIUM confidence

---
*Pitfalls research for: cactl -- CLI-first Entra Conditional Access deploy framework*
*Researched: 2026-03-04*
