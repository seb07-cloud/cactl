# Architecture Research

**Domain:** CLI deploy framework for Microsoft Entra Conditional Access policies (plan/apply workflow with Git-native state)
**Researched:** 2026-03-04
**Confidence:** HIGH

## System Overview

```
                            cactl binary (single Go binary)
 ┌──────────────────────────────────────────────────────────────────────┐
 │                                                                      │
 │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐    │
 │  │  cmd/plan  │  │ cmd/apply  │  │ cmd/drift  │  │ cmd/import │ ...│
 │  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘    │
 │        └───────────┬────┴──────────┬────┴───────────────┘           │
 │                    │               │                                 │
 │  ┌─────────────────┴───────────────┴──────────────────────────┐     │
 │  │                    Reconciliation Engine                    │     │
 │  │        (diff, plan, semver bump, idempotency table)        │     │
 │  └──────────┬──────────────────────────────┬──────────────────┘     │
 │             │                              │                         │
 │  ┌──────────┴──────────┐     ┌─────────────┴──────────────┐        │
 │  │    Backend Layer     │     │       Graph Layer           │        │
 │  │  (desired state)     │     │   (actual/live state)       │        │
 │  │                      │     │                             │        │
 │  │  Backend interface:  │     │  GraphClient interface:     │        │
 │  │  Fetch / Write /     │     │  List / Get / Create /      │        │
 │  │  History / Rollback  │     │  Update / Delete             │        │
 │  └──────────┬──────────┘     └─────────────┬──────────────┘        │
 │             │                              │                         │
 │  ┌──────────┴──────────┐     ┌─────────────┴──────────────┐        │
 │  │   State Layer        │     │      Auth Layer             │        │
 │  │  (Git refs / tags)   │     │  (azidentity providers)     │        │
 │  └──────────┬──────────┘     └─────────────┬──────────────┘        │
 │             │                              │                         │
 ├─────────────┴──────────────────────────────┴─────────────────────────┤
 │                        Output Layer                                  │
 │             (human terminal / JSON structured)                       │
 └──────────────────────────────────────────────────────────────────────┘
               │                              │
        ┌──────┴──────┐               ┌───────┴───────┐
        │  Git Repo   │               │  MS Graph API │
        │ (refs/cactl │               │  (Entra ID)   │
        │  + tags)    │               │               │
        └─────────────┘               └───────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| **cmd/** (CLI layer) | Parse flags, load config, wire dependencies, invoke reconciliation, render output | Cobra commands, one file per command. Thin -- delegates to internal packages immediately. |
| **internal/config/** | Load `.cactl/config.yaml`, merge env vars and CLI flags, validate | Viper for config loading with struct binding. Validation as a separate `validate.go`. |
| **internal/auth/** | Resolve credentials, provide `azcore.TokenCredential` to Graph client | Interface `AuthProvider` with `Credential(ctx) (azcore.TokenCredential, error)`. Implementations: device code, SP secret, SP cert, workload identity. Resolution chain in `provider.go`. |
| **internal/backend/** | Read/write policy JSON from/to storage. Discover policy files (`.json` in `policies/`). | `Backend` interface (Fetch, Write, History, Rollback). Implementations: `GitBackend` (reads working tree files + Git refs for state), `AzureBlobBackend`, `LocalFSBackend`. |
| **internal/state/** | Manage state manifest (slug-to-ObjectID mapping) in Git refs namespace or equivalent store | `StateRepository` interface (Get, Set, Delete, History). `GitStateRepository` uses go-git plumbing to store JSON blobs at `refs/cactl/tenants/<tid>/policies/<slug>`. |
| **internal/graph/** | CRUD against Microsoft Graph API for CA policies. Normalize responses. Handle API errors. | `GraphClient` interface wrapping `msgraph-sdk-go`. `normalize.go` strips server-managed fields. `errors.go` maps HTTP status to typed errors (ErrNotFound, ErrConflict, ErrForbidden). |
| **internal/reconcile/** | Compare desired state (backend) to actual state (Graph), produce a plan, suggest semver bumps | `Engine` struct with `Plan(ctx, desired, actual, state) PlanSummary`. Pure logic -- no I/O. Implements the idempotency truth table. |
| **internal/output/** | Render plan/status/drift results for humans (colored terminal) or machines (JSON) | `Renderer` interface with `RenderPlan`, `RenderStatus`, `RenderDrift`. Two implementations: `HumanRenderer`, `JSONRenderer`. |
| **pkg/types/** | Public types shared across packages and potentially by external consumers | `CAPolicy`, `StateEntry`, `PlanAction`, `PlanSummary`, `Version`. Keep minimal -- only types that cross package boundaries. |

## Recommended Project Structure

```
cactl/
├── main.go                         # Entry point: cobra Execute()
├── go.mod
├── go.sum
├── cmd/                            # Cobra command definitions
│   ├── root.go                     # Root command, global flags, config init
│   ├── init.go                     # cactl init
│   ├── plan.go                     # cactl plan
│   ├── apply.go                    # cactl apply
│   ├── drift.go                    # cactl drift
│   ├── import.go                   # cactl import
│   ├── rollback.go                 # cactl rollback
│   └── status.go                   # cactl status
├── internal/
│   ├── auth/
│   │   ├── provider.go             # AuthProvider interface + resolution chain
│   │   ├── device.go               # Device code flow via azidentity
│   │   ├── serviceprincipal.go     # SP secret + cert via azidentity
│   │   └── workloadidentity.go     # Federated OIDC via azidentity
│   ├── backend/
│   │   ├── backend.go              # Backend interface definition
│   │   ├── git.go                  # GitBackend: reads policies/ dir + manages state
│   │   ├── azureblob.go            # AzureBlobBackend
│   │   └── local.go                # LocalFSBackend (dev/test)
│   ├── state/
│   │   ├── repository.go           # StateRepository interface
│   │   ├── git_state.go            # Git refs + annotated tags via go-git plumbing
│   │   └── entry.go                # StateEntry struct + serialization
│   ├── graph/
│   │   ├── client.go               # GraphClient interface + msgraph-sdk-go impl
│   │   ├── normalize.go            # Strip server fields, sort keys, remove nulls
│   │   └── errors.go               # Typed Graph API errors
│   ├── reconcile/
│   │   ├── engine.go               # Reconciliation engine (desired vs actual)
│   │   ├── plan.go                 # PlanSummary, PlanAction types
│   │   └── semver.go               # Semver bump logic based on field diffs
│   ├── output/
│   │   ├── renderer.go             # Renderer interface
│   │   ├── human.go                # Colored terminal output
│   │   └── json.go                 # Structured JSON output
│   └── config/
│       ├── config.go               # Config struct, Viper loading, env merge
│       └── validate.go             # Config validation rules
├── pkg/
│   └── types/
│       ├── policy.go               # CAPolicy type
│       ├── state.go                # StateEntry type
│       └── plan.go                 # PlanAction, PlanSummary types
└── testdata/                       # Fixtures for tests
    ├── policies/                   # Sample policy JSON files
    └── state/                      # Sample state entries
```

### Structure Rationale

- **cmd/** contains only CLI wiring. Each command file is ~50-100 lines: parse flags, construct dependencies, call internal packages, render output. No business logic here.
- **internal/** enforces Go's compiler-level import restriction. External consumers cannot import these packages, which is correct since the internal architecture is an implementation detail.
- **internal/state/** is separated from **internal/backend/** because the Backend reads policy JSON files (desired state source), while State manages the slug-to-ObjectID manifest (deployment tracking). These are distinct responsibilities even though both may use Git under the hood.
- **pkg/types/** is the only public package. It contains types that appear in the `Backend` and `GraphClient` interfaces, so they must be importable. Keep this surface area small.
- **testdata/** follows Go convention for test fixtures.

## Architectural Patterns

### Pattern 1: Reconciliation Engine (Desired State vs. Actual State)

**What:** The reconciliation engine is a pure function that takes three inputs -- desired state (from backend), actual state (from Graph API), and current manifest (from state store) -- and produces a plan of actions. It makes no I/O calls itself.

**When to use:** Every command that compares state (`plan`, `apply`, `drift`, `rollback`).

**Trade-offs:** Pure reconciliation logic is easy to test with table-driven tests (no mocks needed for the engine itself). The trade-off is that the caller must orchestrate fetching all three inputs before calling the engine.

**Example:**
```go
// internal/reconcile/engine.go
type Engine struct {
    MajorFields []string // from config: fields that trigger MAJOR bump
    MinorFields []string // from config: fields that trigger MINOR bump
}

func (e *Engine) Plan(
    desired []types.CAPolicy,    // from backend
    actual  []types.CAPolicy,    // from Graph API
    state   []types.StateEntry,  // from state store
) *types.PlanSummary {
    // Implements the idempotency truth table:
    // - desired present + state missing + live missing  = CREATE (+)
    // - desired changed + state present + live present  = UPDATE (~)
    // - desired same    + state present + live present   = NOOP
    // - desired present + state present + live missing  = RECREATE (-/+)
    // - desired absent  + state missing + live present  = UNTRACKED (?)
    // Returns PlanSummary with []PlanAction
}
```

### Pattern 2: Interface-Driven Dependency Injection

**What:** All external boundaries (Backend, State, Graph, Auth, Output) are defined as Go interfaces in their respective packages. Concrete implementations are selected and wired in `cmd/` based on config. Tests inject mocks/fakes.

**When to use:** Every cross-boundary interaction.

**Trade-offs:** Slightly more boilerplate (interface + implementation files), but enables testability without the msgraph SDK or a real Git repo. Critical for a tool where the external dependencies (Graph API, Git) are expensive to set up in tests.

**Example:**
```go
// internal/graph/client.go
type GraphClient interface {
    ListPolicies(ctx context.Context, tenantID string) ([]types.CAPolicy, error)
    GetPolicy(ctx context.Context, tenantID, objectID string) (*types.CAPolicy, error)
    CreatePolicy(ctx context.Context, tenantID string, policy types.CAPolicy) (string, error)
    UpdatePolicy(ctx context.Context, tenantID, objectID string, policy types.CAPolicy) error
    DeletePolicy(ctx context.Context, tenantID, objectID string) error
}

// internal/graph/msgraph_client.go (implementation)
type MSGraphClient struct {
    clientFactory *ClientFactory  // one client per tenant
}
```

### Pattern 3: Git Plumbing for Zero-Footprint State

**What:** State is stored as JSON blobs in custom Git refs (`refs/cactl/tenants/<tid>/policies/<slug>`) using go-git's plumbing API. No files appear in the working tree. Immutable snapshots are annotated tags with the policy JSON in the tag message body.

**When to use:** GitBackend state operations.

**Trade-offs:** Non-standard Git usage -- refs typically point to commits, not blobs. This is technically valid but may confuse Git GUIs or hooks that expect standard ref patterns. The benefit is zero working tree pollution, no commit noise, and no merge conflicts on state.

**Example:**
```go
// internal/state/git_state.go
func (s *GitStateRepository) Set(ctx context.Context, tenantID, slug string, entry types.StateEntry) error {
    // 1. Serialize StateEntry to JSON
    data, _ := json.Marshal(entry)

    // 2. Create a blob object in the Git object store
    obj := s.repo.Storer.NewEncodedObject()
    obj.SetType(plumbing.BlobObject)
    w, _ := obj.Writer()
    w.Write(data)
    w.Close()
    hash, _ := s.repo.Storer.SetEncodedObject(obj)

    // 3. Point a custom ref to the blob
    refName := plumbing.ReferenceName(
        fmt.Sprintf("refs/cactl/tenants/%s/policies/%s", tenantID, slug),
    )
    ref := plumbing.NewHashReference(refName, hash)
    return s.repo.Storer.SetReference(ref)
}
```

### Pattern 4: ClientFactory for Multi-Tenant Graph Access

**What:** A factory that creates and caches one authenticated `GraphServiceClient` per tenant ID. Each client holds its own token credential and token cache. The factory is initialized once in `cmd/root.go` and passed down.

**When to use:** Any command targeting one or more tenants.

**Trade-offs:** Caching clients avoids re-authentication per API call. The factory must be concurrency-safe if multi-tenant parallel operations are added later (v1.1). For v1 sequential execution, a simple map suffices.

**Example:**
```go
// internal/graph/factory.go
type ClientFactory struct {
    authProvider auth.AuthProvider
    clients      map[string]*MSGraphClient  // keyed by tenantID
}

func (f *ClientFactory) Client(ctx context.Context, tenantID string) (GraphClient, error) {
    if c, ok := f.clients[tenantID]; ok {
        return c, nil
    }
    cred, err := f.authProvider.Credential(ctx, tenantID)
    if err != nil {
        return nil, err
    }
    client, _ := msgraphsdk.NewGraphServiceClientWithCredentials(cred, requiredScopes)
    f.clients[tenantID] = &MSGraphClient{client: client}
    return f.clients[tenantID], nil
}
```

### Pattern 5: Renderer Interface for Dual Output

**What:** All user-facing output goes through a `Renderer` interface. Commands never write directly to stdout. The renderer is selected based on `--output` flag (human vs json).

**When to use:** Every command that produces output.

**Trade-offs:** Slight indirection, but it ensures JSON output is structurally correct (not ad-hoc print statements mixed with structured data) and that `--ci` mode never accidentally emits ANSI codes.

**Example:**
```go
// internal/output/renderer.go
type Renderer interface {
    RenderPlan(plan *types.PlanSummary) error
    RenderStatus(statuses []types.PolicyStatus) error
    RenderDrift(drift *types.DriftReport) error
    RenderError(err error) error
}
```

## Data Flow

### Plan/Apply Flow (Primary Workflow)

```
User runs: cactl plan --tenant contoso.onmicrosoft.com

cmd/plan.go
    │
    ├─── config.Load()                    → Config struct
    ├─── auth.NewProvider(config.Auth)     → AuthProvider
    ├─── backend.New(config.Backend)       → Backend
    ├─── state.New(config.Backend)         → StateRepository
    ├─── graph.NewFactory(authProvider)    → ClientFactory
    │
    │  ┌──── Parallel fetch (conceptually) ────────────────────┐
    │  │                                                        │
    │  ├─── backend.Fetch(ctx, tenantID)   → []CAPolicy        │ (desired)
    │  ├─── graphClient.ListPolicies(ctx)  → []CAPolicy        │ (actual)
    │  ├─── state.List(ctx, tenantID)      → []StateEntry      │ (manifest)
    │  │                                                        │
    │  └────────────────────────────────────────────────────────┘
    │
    ├─── reconcile.Engine.Plan(desired, actual, state) → PlanSummary
    │
    └─── renderer.RenderPlan(planSummary)  → stdout (human or JSON)

Exit code: 0 (no changes) or 1 (changes detected)
```

### Apply Flow (extends Plan)

```
User runs: cactl apply --tenant contoso.onmicrosoft.com

cmd/apply.go
    │
    ├─── [same fetch + plan as above]
    │
    ├─── renderer.RenderPlan(planSummary)  → show diff to user
    │
    ├─── prompt for confirmation (unless --auto-approve)
    │    (escalated prompt if plan contains recreate actions)
    │
    ├─── For each PlanAction in plan:
    │    │
    │    ├── CREATE (+):  graphClient.CreatePolicy() → objectID
    │    │                state.Set(slug, objectID, v1.0.0)
    │    │                tag: cactl/<tenant>/<slug>/1.0.0
    │    │
    │    ├── UPDATE (~):  graphClient.UpdatePolicy(objectID)
    │    │                state.Set(slug, objectID, newVersion)
    │    │                tag: cactl/<tenant>/<slug>/<newVersion>
    │    │
    │    ├── RECREATE (-/+): graphClient.CreatePolicy() → newObjectID
    │    │                   state.Set(slug, newObjectID, newVersion)
    │    │                   tag: cactl/<tenant>/<slug>/<newVersion>
    │    │
    │    └── NOOP: skip
    │
    └─── renderer.RenderApplyResult()
         Print: "Run git push --follow-tags to sync state"
```

### Import Flow

```
User runs: cactl import --all --tenant contoso.onmicrosoft.com

cmd/import.go
    │
    ├─── graphClient.ListPolicies(ctx)     → []CAPolicy (raw from Graph)
    ├─── state.List(ctx, tenantID)         → []StateEntry (already tracked)
    │
    ├─── Filter: only untracked policies (no existing state entry)
    │
    ├─── For each untracked policy:
    │    │
    │    ├── normalize.Normalize(policy)    → strip id, timestamps, nulls
    │    ├── Derive slug from displayName   → kebab-case filename
    │    ├── Write to policies/<slug>.json  → working tree file
    │    ├── state.Set(slug, objectID, v1.0.0)
    │    └── tag: cactl/<tenant>/<slug>/1.0.0
    │
    └─── renderer.RenderImportResult()
```

### Key Data Flows

1. **Desired state path:** `policies/*.json` (working tree) --> `backend.Fetch()` --> `reconcile.Engine` (as desired state input)
2. **Actual state path:** MS Graph API --> `graphClient.ListPolicies()` --> `reconcile.Engine` (as actual state input)
3. **Manifest state path:** `refs/cactl/tenants/<tid>/policies/<slug>` (Git blob) --> `state.Get()` --> `reconcile.Engine` (as mapping input) + used by Graph layer to know which objectID to PATCH
4. **Version history path:** Annotated tags `cactl/<tid>/<slug>/<semver>` --> `state.History()` --> `cmd/status` and `cmd/rollback`
5. **Auth credential path:** Config/env/flags --> `auth.AuthProvider.Credential()` --> `graph.ClientFactory` --> `GraphServiceClient`

## Internal Boundaries and Communication

| Boundary | Communication | Key Contract |
|----------|---------------|--------------|
| cmd/ --> internal/* | Direct function calls | Commands construct dependencies and call internal package functions. No global state. |
| reconcile/ --> (nothing) | Pure computation | Engine receives data, returns plan. Zero side effects. Does not import backend, state, or graph. |
| cmd/ --> backend/ | `Backend` interface | Backend knows nothing about Graph API or reconciliation. |
| cmd/ --> graph/ | `GraphClient` interface | Graph client knows nothing about backends or state. |
| cmd/ --> state/ | `StateRepository` interface | State layer is independent. Backend layer may share the Git repo handle but state owns its own ref namespace. |
| backend/ --> state/ | No direct communication | Both may use the same underlying Git repo, but they access different namespaces (working tree files vs refs/cactl/*). The cmd/ layer coordinates between them. |
| graph/ --> auth/ | `AuthProvider` via factory | Graph factory receives an AuthProvider. Auth knows nothing about Graph. |
| output/ --> (nothing) | Receives types, writes stdout | Renderer has no dependencies on other internal packages. Takes `types.*` structs and produces output. |

### Critical Isolation: Reconciliation Engine

The reconciliation engine is the architectural centerpiece and must remain a **pure function** with zero I/O dependencies:

```
                    ┌─────────────────────────┐
     desired ──────>│                         │
                    │   reconcile.Engine       │──────> PlanSummary
     actual  ──────>│                         │
                    │   (pure computation)     │
     state   ──────>│                         │
                    └─────────────────────────┘
                      No I/O. No interfaces.
                      Only pkg/types imports.
```

This isolation means:
- Table-driven tests with no mocks, no network, no filesystem
- The truth table from the spec maps directly to test cases
- Semver bump logic is independently testable
- The engine can be reused by any future UI (web dashboard, VS Code extension)

## Integration Points

### External Services

| Service | Integration Pattern | Gotchas |
|---------|---------------------|---------|
| **Microsoft Graph API** | `msgraph-sdk-go` with `azidentity` credentials. Fluent API: `client.Identity().ConditionalAccess().Policies()` | No uniqueness constraint on POST -- creating duplicates is silent. This is the entire reason cactl exists. SDK uses Kiota-generated code; types are pointer-heavy. Rate limiting (429) needs retry with backoff. |
| **Git repository** | `go-git/go-git/v5` plumbing API for refs and blobs; standard file I/O for policy JSON | Custom refs (`refs/cactl/*`) require explicit refspec in `.git/config` for push/fetch. `cactl init` must configure this. go-git v5 has known vulnerabilities (GO-2025-3367, GO-2025-3368) -- pin to patched version. |
| **Azure Blob Storage** | `azblob` SDK for AzureBlobBackend | Blob lease needed for concurrent access protection (v1.1). Container must exist before use. |
| **OS Credential Manager** | Via `azidentity` SDK (macOS Keychain, Windows Credential Manager, libsecret) | Token cache is per-user. CI environments have no credential manager -- must use SP or workload identity. |

### go-git vs. Shell-Out to git CLI

**Recommendation: Use go-git for state operations, shell out to `git` for push/fetch.**

Rationale:
- go-git excels at plumbing operations (creating blobs, setting refs, creating annotated tags) without needing a working tree or commit
- go-git's remote operations (push/fetch) have historically been less reliable than the native `git` CLI, especially with SSH keys and credential helpers
- For `cactl init` writing refspecs to `.git/config`, go-git's config API works well
- For the "reminder to push" after apply, shelling out to `git push --follow-tags` is simpler and more reliable

## Anti-Patterns

### Anti-Pattern 1: Business Logic in cmd/

**What people do:** Put reconciliation logic, API calls, or state management directly in cobra command handlers.
**Why it is wrong:** Untestable without running the full CLI. Mixes flag parsing with domain logic. Makes it impossible to reuse logic in a future API server or library.
**Do this instead:** Each command file is a thin orchestrator: parse flags, construct dependencies from config, call internal packages, render output. Target ~50-100 lines per command file.

### Anti-Pattern 2: Shared Mutable State Across Tenants

**What people do:** Use a global variable or singleton for the Graph client, tenant ID, or state repository.
**Why it is wrong:** Breaks multi-tenant support. Creates hidden coupling. Race conditions if concurrency is added later.
**Do this instead:** Thread tenant ID as an explicit parameter through every function. Use ClientFactory to manage per-tenant clients. The spec explicitly mandates this: "The tenant ID flows as an explicit parameter through every layer."

### Anti-Pattern 3: Mixing State Storage with Backend Storage

**What people do:** Store both policy JSON files and the state manifest in the same abstraction, making the Backend interface responsible for both.
**Why it is wrong:** The state manifest (slug-to-ObjectID mapping) has different lifecycle, storage mechanism (Git refs vs files), and access patterns than policy JSON files. Combining them creates a god-object backend.
**Do this instead:** Separate `Backend` interface (policy JSON CRUD) from `StateRepository` interface (manifest CRUD). Both may use Git, but they own different namespaces. The cmd/ layer coordinates between them.

### Anti-Pattern 4: Directly Using msgraph-sdk-go Types in Domain Logic

**What people do:** Pass `graphmodels.ConditionalAccessPolicy` directly to the reconciliation engine.
**Why it is wrong:** Couples domain logic to the SDK's generated types (pointer-heavy, OData-aware, subject to SDK version changes). Makes the reconciler untestable without the SDK.
**Do this instead:** Define `types.CAPolicy` in `pkg/types/` as a clean domain type. The Graph client translates between SDK types and domain types at the boundary. The reconciler only sees `types.CAPolicy`.

### Anti-Pattern 5: Storing State as Committed Files

**What people do:** Write `.state.json` or `.cactl-state/` files that get committed to the repo.
**Why it is wrong:** Creates commit noise on every deploy, pollutes `git log`, causes merge conflicts in concurrent pipeline runs, and exposes Object IDs in the working tree.
**Do this instead:** Use Git refs (`refs/cactl/*`) for current state and annotated tags for immutable snapshots. Zero working tree footprint. This is a core architectural decision in the spec.

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| 1-10 tenants, manual deploys | Single binary, sequential execution, Git backend. This is the v1.0 target. No changes needed. |
| 10-50 tenants, CI/CD pipelines | AzureBlobBackend with blob leases for concurrent pipeline protection. `--concurrency` flag for parallel tenant processing (v1.1). Consider a central state store rather than per-repo Git refs. |
| 50+ tenants, MSP scale | Plugin architecture for Named Locations and Auth Strengths. Central state service (not per-repo). Webhook integration for drift alerts. This is post-v1.0 territory. |

### Scaling Priorities

1. **First bottleneck: Graph API rate limiting.** Microsoft Graph enforces per-tenant and per-app throttling. At 10+ tenants with many policies, sequential API calls will hit 429s. Mitigation: implement retry with exponential backoff from day one (in the Graph client layer), even though v1 is sequential. Add `--concurrency` for parallel tenant processing in v1.1.

2. **Second bottleneck: Git refs namespace size.** With hundreds of policies across dozens of tenants, the `refs/cactl/` namespace could contain thousands of refs. Git handles this fine (the Linux kernel repo has thousands of refs), but `git fetch` of the full refspec may become slow. Mitigation: allow selective refspec fetch per-tenant in v1.1.

## Dependency Graph and Build Order

The following shows which components depend on which, dictating the order they should be built:

```
Level 0 (no internal dependencies):
  pkg/types/          -- shared types, built first
  internal/config/    -- config loading, only depends on stdlib + viper

Level 1 (depends on types only):
  internal/auth/      -- depends on pkg/types (for config types)
  internal/output/    -- depends on pkg/types (for PlanSummary, etc.)

Level 2 (depends on types + level 1):
  internal/state/     -- depends on pkg/types
  internal/graph/     -- depends on pkg/types, internal/auth
  internal/backend/   -- depends on pkg/types

Level 3 (depends on types only, but logically needs all data sources to exist):
  internal/reconcile/ -- depends ONLY on pkg/types (pure logic)

Level 4 (wires everything together):
  cmd/                -- depends on all internal packages
```

### Suggested Build Order for Phases

1. **Foundation:** `pkg/types/` + `internal/config/` + `internal/auth/` + `internal/output/` -- these have no cross-dependencies and provide the base for everything else
2. **Graph integration:** `internal/graph/` -- enables fetching live state, needs auth
3. **Backend + State:** `internal/backend/` (LocalFSBackend first, then GitBackend) + `internal/state/` -- enables reading desired state and managing manifest
4. **Reconciliation:** `internal/reconcile/` -- the core engine, depends only on types but needs backend and graph to exist for integration testing
5. **Commands:** `cmd/plan` first (read-only, safest), then `cmd/apply`, then `cmd/import`, `cmd/drift`, `cmd/rollback`, `cmd/status`, `cmd/init`
6. **Polish:** AzureBlobBackend, `--ci` mode, `--dry-run`, exit codes, JSON schema validation

This build order matches the spec's release plan (v0.1 through v1.0) and ensures each phase produces a testable, runnable increment.

## Sources

- [Terraform Core Workflow](https://developer.hashicorp.com/terraform/intro/core-workflow) -- plan/apply pattern reference (HIGH confidence)
- [go-git/go-git GitHub](https://github.com/go-git/go-git) -- pure Go Git implementation (HIGH confidence)
- [go-git plumbing package](https://pkg.go.dev/github.com/go-git/go-git/v5/plumbing) -- reference and blob APIs (HIGH confidence)
- [go-git storer package](https://pkg.go.dev/github.com/go-git/go-git/v5/plumbing/storer) -- ReferenceStorer, SetReference, SetEncodedObject (HIGH confidence)
- [msgraph-sdk-go GitHub](https://github.com/microsoftgraph/msgraph-sdk-go) -- Microsoft Graph SDK for Go (HIGH confidence)
- [Conditional Access Policy API](https://learn.microsoft.com/en-us/graph/api/resources/conditionalaccesspolicy?view=graph-rest-1.0) -- Graph API resource type (HIGH confidence)
- [Create CA Policy](https://learn.microsoft.com/en-us/graph/api/conditionalaccessroot-post-policies?view=graph-rest-1.0) -- POST endpoint (HIGH confidence)
- [Update CA Policy](https://learn.microsoft.com/en-us/graph/api/conditionalaccesspolicy-update?view=graph-rest-1.0) -- PATCH endpoint (HIGH confidence)
- [Cobra CLI framework](https://pkg.go.dev/github.com/spf13/cobra) -- CLI command structure (HIGH confidence)
- [Reactive Planning and Reconciliation in Go](https://blog.gopheracademy.com/reactive-planning-go/) -- reconciliation engine patterns (MEDIUM confidence)
- [Reconciliation Pattern, Control Theory and Cluster API](https://archive.fosdem.org/2023/schedule/event/goreconciliation/) -- FOSDEM talk on reconciliation (MEDIUM confidence)
- [The Principle of Reconciliation](https://www.chainguard.dev/unchained/the-principle-of-reconciliation) -- idempotency in reconcilers (MEDIUM confidence)
- [Go Project Structure: Practices & Patterns](https://www.glukhov.org/post/2025/12/go-project-structure/) -- cmd/internal/pkg layout (MEDIUM confidence)
- [go-git annotated tag example](https://medium.com/@clm160/tag-example-with-go-git-library-4377a84bbf17) -- CreateTag API usage (MEDIUM confidence)

---
*Architecture research for: cactl -- Conditional Access Policy Deploy Framework*
*Researched: 2026-03-04*
