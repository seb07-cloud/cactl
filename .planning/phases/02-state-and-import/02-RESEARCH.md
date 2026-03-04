# Phase 2: State and Import - Research

**Researched:** 2026-03-04
**Domain:** Git-backed state storage (custom refs, blob plumbing), Microsoft Graph API for CA policy import, JSON normalization, slug generation
**Confidence:** HIGH

## Summary

Phase 2 adds two capabilities on top of the Phase 1 foundation: (1) a Git-backed state store that uses custom refs (`refs/cactl/...`) to hold policy blobs and state manifests without polluting the working tree, and (2) a `cactl import` command that fetches live CA policies from Microsoft Graph, normalizes the JSON, and writes them into this state store with version tracking.

The primary technical decision is how to interact with Git. The go-git library (v5) provides a pure Go API for creating blobs and custom refs, but it has documented issues with refs pointing directly to blobs (returns `ErrUnsupportedObject` when tag/commit operations expect a commit). The alternative -- shelling out to `git hash-object -w --stdin` and `git update-ref` -- is simpler, fully featured, and aligns with the existing codebase pattern (Phase 1 already uses `os/exec` for `git ls-files`). This research recommends `os/exec` with git plumbing commands for Phase 2, avoiding the go-git dependency entirely.

The second technical decision is how to call the Microsoft Graph API. Phase 1 research recommended `msgraph-sdk-go` but the SDK was never imported -- the codebase uses raw `net/http` for schema fetch. The official MS Graph Go SDK (`msgraph-sdk-go`) causes severe binary bloat and build time regression (documented: `go test` from 7s to 8m50s) due to its massive auto-generated models package. Since cactl only needs a handful of Graph endpoints (list policies, get policy, update policy, create policy), raw `net/http` calls with `azcore.TokenCredential` for auth is the correct approach. This keeps the binary lean and avoids pulling in thousands of unused generated types.

**Primary recommendation:** Build three packages -- `internal/state` (Git-backed state store using exec git plumbing), `internal/graph` (thin HTTP client for CA policy CRUD using azcore auth), `internal/normalize` (JSON normalization: strip server fields, remove nulls, sort keys, pretty-print) -- then wire them into `cmd/import.go`. Use `os/exec` for all Git operations rather than go-git.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `os/exec` | N/A | Git plumbing commands (hash-object, update-ref, cat-file, tag) | Zero dependency; full git feature parity; Phase 1 already uses this pattern for `git ls-files`. Avoids go-git blob-ref issues. |
| Go stdlib `net/http` | N/A | Microsoft Graph API calls | Zero dependency; avoids msgraph-sdk-go binary bloat. Phase 1 already uses this for schema fetch. |
| Go stdlib `encoding/json` | N/A | JSON normalization (marshal/unmarshal, sorted keys, pretty-print) | Maps auto-sort keys on marshal. `json.MarshalIndent` for 2-space pretty-print. No external dependency needed. |
| Azure/azure-sdk-for-go/sdk/azcore | v1.20.0 | TokenCredential for Graph API auth | Already in go.mod from Phase 1. Provides `BearerTokenPolicy` for HTTP pipeline auth. |
| Azure/azure-sdk-for-go/sdk/azidentity | v1.13.1 | Credential creation (via ClientFactory from Phase 1) | Already in go.mod. Phase 2 does not add new auth types; reuses Phase 1 ClientFactory. |
| spf13/cobra | v1.10.2 | `import` command registration | Already in go.mod. New subcommand follows existing `cmd/init.go` pattern. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stretchr/testify | v1.11.1 | Test assertions | Already in go.mod. Table-driven tests for normalize, state, graph packages. |
| Go stdlib `regexp` | N/A | Kebab-case slug derivation from display names | Simple regex replacement: strip non-alphanumeric, lowercase, join with hyphens. No external library needed. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `os/exec` git plumbing | go-git/go-git v5 | go-git is pure Go (no git binary dependency) but has documented issues with refs pointing to blobs (`ErrUnsupportedObject`), incomplete custom ref support, and adds ~5MB to binary. `os/exec` is simpler and fully featured for our use case. Requiring git binary is acceptable -- users already need git for the repo. |
| Raw `net/http` | msgraph-sdk-go v1.96.x | Official SDK provides typed models and auto-pagination but causes severe binary bloat (documented: build times 7s->8m50s, massive models package). cactl needs only 4 Graph endpoints. Raw HTTP with manual JSON unmarshalling is proportionate. |
| stdlib `regexp` for slugs | gobeam/stringy, ettle/strcase | External slug libraries handle Unicode and edge cases but add a dependency for a simple operation (lowercase, replace spaces/special chars with hyphens). CA policy display names are ASCII-dominant. Hand-roll with tests is appropriate here. |

### Installation

No new dependencies required. Phase 2 uses only what is already in `go.mod` plus the Go standard library.

## Architecture Patterns

### Recommended Project Structure (Phase 2 additions)

```
cactl/
├── cmd/
│   ├── import.go              # cactl import command (CLI-04, IMPORT-01..08)
│   └── ... (existing)
├── internal/
│   ├── graph/
│   │   ├── client.go          # Graph HTTP client with azcore auth
│   │   ├── client_test.go     # Tests with httptest mock server
│   │   └── policies.go        # CA policy CRUD operations (list, get)
│   ├── normalize/
│   │   ├── normalize.go       # JSON normalization (strip, null removal, sort, format)
│   │   ├── normalize_test.go  # Table-driven tests
│   │   └── slug.go            # Kebab-case slug derivation
│   ├── state/
│   │   ├── backend.go         # GitBackend interface and implementation
│   │   ├── backend_test.go    # Tests using temp git repos
│   │   ├── manifest.go        # State manifest types (STATE-05 schema)
│   │   └── refspec.go         # Refspec configuration for .git/config (STATE-04)
│   └── ... (existing)
└── ... (existing)
```

### Pattern 1: Git Plumbing State Store (STATE-02)

**What:** Store normalized policy JSON as Git blobs in custom refs (`refs/cactl/tenants/<tenant-id>/policies/<slug>`), with no working tree footprint.

**When to use:** Every state read/write operation (import, plan, apply).

**How it works:**
1. Write blob: `echo '<json>' | git hash-object -w --stdin` -> returns SHA
2. Point ref: `git update-ref refs/cactl/tenants/<tenant>/policies/<slug> <sha>`
3. Read blob: `git cat-file blob refs/cactl/tenants/<tenant>/policies/<slug>`
4. List refs: `git for-each-ref refs/cactl/tenants/<tenant>/policies/`

**Why custom refs, not files in the working tree:**
- Policies are managed state, not source code. They should not appear in `git status` or be accidentally edited.
- Custom refs push/pull via refspec without affecting the working tree.
- Immutable history via annotated tags provides audit trail.

```go
// internal/state/backend.go

// GitBackend stores state in Git refs using plumbing commands.
type GitBackend struct {
    repoDir string // Path to the git repo root
}

// WritePolicy writes normalized policy JSON as a blob and updates the ref.
func (b *GitBackend) WritePolicy(tenantID, slug string, data []byte) error {
    // 1. Write blob to object store
    hash, err := b.hashObject(data)
    if err != nil {
        return fmt.Errorf("writing blob for %s: %w", slug, err)
    }

    // 2. Update ref to point to blob
    ref := fmt.Sprintf("refs/cactl/tenants/%s/policies/%s", tenantID, slug)
    if err := b.updateRef(ref, hash); err != nil {
        return fmt.Errorf("updating ref %s: %w", ref, err)
    }

    return nil
}

func (b *GitBackend) hashObject(data []byte) (string, error) {
    cmd := exec.Command("git", "hash-object", "-w", "--stdin")
    cmd.Dir = b.repoDir
    cmd.Stdin = bytes.NewReader(data)
    out, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("git hash-object: %w", err)
    }
    return strings.TrimSpace(string(out)), nil
}

func (b *GitBackend) updateRef(ref, hash string) error {
    cmd := exec.Command("git", "update-ref", ref, hash)
    cmd.Dir = b.repoDir
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("git update-ref: %s: %w", string(out), err)
    }
    return nil
}

// ReadPolicy reads the policy JSON blob from the ref.
func (b *GitBackend) ReadPolicy(tenantID, slug string) ([]byte, error) {
    ref := fmt.Sprintf("refs/cactl/tenants/%s/policies/%s", tenantID, slug)
    cmd := exec.Command("git", "cat-file", "blob", ref)
    cmd.Dir = b.repoDir
    out, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("reading %s: %w", ref, err)
    }
    return out, nil
}
```

### Pattern 2: State Manifest (STATE-01, STATE-05)

**What:** A JSON manifest blob stored at `refs/cactl/tenants/<tenant-id>/manifest` containing metadata for all tracked policies.

**Schema (STATE-05):**

```go
// internal/state/manifest.go

// Manifest holds the state of all tracked policies for a tenant.
type Manifest struct {
    SchemaVersion int              `json:"schema_version"`
    Tenant        string           `json:"tenant"`
    Policies      map[string]Entry `json:"policies"` // keyed by slug
}

// Entry tracks a single policy's state.
type Entry struct {
    Slug         string `json:"slug"`
    Tenant       string `json:"tenant"`
    LiveObjectID string `json:"live_object_id"` // Entra object ID from Graph API
    Version      string `json:"version"`        // semver e.g. "1.0.0"
    LastDeployed string `json:"last_deployed"`   // ISO 8601 timestamp
    DeployedBy   string `json:"deployed_by"`     // identity that last deployed
    AuthMode     string `json:"auth_mode"`       // auth mode used for last deploy
    BackendSHA   string `json:"backend_sha"`     // SHA of the policy blob in git
}
```

The manifest is read/written as a blob, same as policy data:
- Read: `git cat-file blob refs/cactl/tenants/<tenant>/manifest`
- Write: `echo '<json>' | git hash-object -w --stdin` then `git update-ref refs/cactl/tenants/<tenant>/manifest <sha>`

### Pattern 3: Annotated Tags for Immutable Version History (STATE-03)

**What:** Every imported or deployed policy version creates an annotated tag at `cactl/<tenant>/<slug>/<semver>` containing the full policy JSON.

**Why annotated tags (not lightweight):** Annotated tags are stored as full objects with tagger identity, timestamp, and message. This provides a complete audit trail: who imported what, when.

```go
// CreateVersionTag creates an annotated tag for a policy version.
func (b *GitBackend) CreateVersionTag(tenantID, slug, version string, blobHash string) error {
    tagName := fmt.Sprintf("cactl/%s/%s/%s", tenantID, slug, version)
    // Create annotated tag pointing to the blob
    cmd := exec.Command("git", "tag", "-a", tagName, blobHash,
        "-m", fmt.Sprintf("cactl import: %s %s", slug, version))
    cmd.Dir = b.repoDir
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("creating tag %s: %s: %w", tagName, string(out), err)
    }
    return nil
}
```

Note: `git tag -a <name> <object>` can tag any object, including blobs. This is a standard git feature.

### Pattern 4: Refspec Configuration (STATE-04)

**What:** `cactl init` (extended in Phase 2) writes refspec entries to `.git/config` so that `git push` and `git fetch` automatically include `refs/cactl/*`.

```bash
# Added to .git/config by cactl init (Phase 2 extension)
[remote "origin"]
    fetch = +refs/cactl/*:refs/cactl/*
    push = +refs/cactl/*:refs/cactl/*
```

```go
// internal/state/refspec.go

// ConfigureRefspec adds cactl ref push/pull to .git/config.
func ConfigureRefspec(repoDir string) error {
    // Check if refspec already configured
    check := exec.Command("git", "config", "--get-all", "remote.origin.fetch")
    check.Dir = repoDir
    out, _ := check.Output()
    if strings.Contains(string(out), "refs/cactl/*") {
        return nil // Already configured
    }

    // Add fetch refspec
    addFetch := exec.Command("git", "config", "--add",
        "remote.origin.fetch", "+refs/cactl/*:refs/cactl/*")
    addFetch.Dir = repoDir
    if err := addFetch.Run(); err != nil {
        return fmt.Errorf("adding fetch refspec: %w", err)
    }

    // Add push refspec
    addPush := exec.Command("git", "config", "--add",
        "remote.origin.push", "+refs/cactl/*:refs/cactl/*")
    addPush.Dir = repoDir
    if err := addPush.Run(); err != nil {
        return fmt.Errorf("adding push refspec: %w", err)
    }

    return nil
}
```

### Pattern 5: Graph API Client with azcore Auth

**What:** A thin HTTP client that uses the existing `azcore.TokenCredential` from Phase 1's ClientFactory to call Microsoft Graph endpoints.

```go
// internal/graph/client.go

// Client is a thin HTTP client for Microsoft Graph API.
type Client struct {
    baseURL    string
    httpClient *http.Client
    credential azcore.TokenCredential
    tenantID   string
}

// NewClient creates a Graph API client with the given credential.
func NewClient(credential azcore.TokenCredential, tenantID string) *Client {
    return &Client{
        baseURL:    "https://graph.microsoft.com/v1.0",
        httpClient: &http.Client{Timeout: 30 * time.Second},
        credential: credential,
        tenantID:   tenantID,
    }
}

// do executes an authenticated HTTP request to Graph API.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
    url := c.baseURL + path

    req, err := http.NewRequestWithContext(ctx, method, url, body)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }

    // Acquire token using azcore credential
    token, err := c.credential.GetToken(ctx, policy.TokenRequestOptions{
        Scopes: []string{"https://graph.microsoft.com/.default"},
    })
    if err != nil {
        return nil, fmt.Errorf("acquiring token: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+token.Token)
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("executing request: %w", err)
    }

    return resp, nil
}
```

```go
// internal/graph/policies.go

// Policy represents a Conditional Access policy from Graph API.
// Only includes fields we need; server-managed fields are in RawPolicy.
type Policy struct {
    ID               string          `json:"id"`
    DisplayName      string          `json:"displayName"`
    State            string          `json:"state"`
    CreatedDateTime  string          `json:"createdDateTime"`
    ModifiedDateTime string          `json:"modifiedDateTime"`
    TemplateID       *string         `json:"templateId"`
    Conditions       json.RawMessage `json:"conditions"`
    GrantControls    json.RawMessage `json:"grantControls"`
    SessionControls  json.RawMessage `json:"sessionControls"`
}

// ListPolicies retrieves all CA policies from the tenant.
func (c *Client) ListPolicies(ctx context.Context) ([]Policy, error) {
    resp, err := c.do(ctx, "GET", "/identity/conditionalAccess/policies", nil)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("list policies: HTTP %d: %s", resp.StatusCode, string(body))
    }

    var result struct {
        Value []Policy `json:"value"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decoding policies: %w", err)
    }

    return result.Value, nil
}
```

### Pattern 6: JSON Normalization Pipeline (IMPORT-03..05)

**What:** A deterministic normalization pipeline that transforms Graph API responses into canonical JSON for storage.

Steps: (1) strip server-managed fields, (2) recursively remove null values, (3) sort keys alphabetically, (4) pretty-print with 2-space indent.

```go
// internal/normalize/normalize.go

// serverManagedFields are fields that must be stripped during import.
// These are read-only server-managed properties per Microsoft Graph docs.
var serverManagedFields = []string{
    "id",
    "createdDateTime",
    "modifiedDateTime",
    "templateId",
}

// Normalize takes raw Graph API policy JSON and returns canonical form.
func Normalize(raw []byte) ([]byte, error) {
    var m map[string]interface{}
    if err := json.Unmarshal(raw, &m); err != nil {
        return nil, fmt.Errorf("unmarshalling: %w", err)
    }

    // Step 1: Strip server-managed fields
    for _, field := range serverManagedFields {
        delete(m, field)
    }

    // Step 2: Strip @odata metadata fields (recursive)
    stripODataFields(m)

    // Step 3: Recursively remove null values
    removeNulls(m)

    // Step 4: Marshal with sorted keys (Go maps sort automatically)
    //         and 2-space indent
    out, err := json.MarshalIndent(m, "", "  ")
    if err != nil {
        return nil, fmt.Errorf("marshalling: %w", err)
    }

    // Ensure trailing newline
    out = append(out, '\n')
    return out, nil
}

// removeNulls recursively deletes nil values from nested maps.
func removeNulls(m map[string]interface{}) {
    for k, v := range m {
        if v == nil {
            delete(m, k)
            continue
        }
        if nested, ok := v.(map[string]interface{}); ok {
            removeNulls(nested)
        }
        if arr, ok := v.([]interface{}); ok {
            for _, item := range arr {
                if nestedMap, ok := item.(map[string]interface{}); ok {
                    removeNulls(nestedMap)
                }
            }
        }
    }
}

// stripODataFields recursively removes @odata.* keys from nested maps.
func stripODataFields(m map[string]interface{}) {
    for k, v := range m {
        if strings.HasPrefix(k, "@odata.") {
            delete(m, k)
            continue
        }
        if nested, ok := v.(map[string]interface{}); ok {
            stripODataFields(nested)
        }
        if arr, ok := v.([]interface{}); ok {
            for _, item := range arr {
                if nestedMap, ok := item.(map[string]interface{}); ok {
                    stripODataFields(nestedMap)
                }
            }
        }
    }
}
```

### Pattern 7: Kebab-Case Slug Derivation (IMPORT-06)

**What:** Derive a filesystem-safe kebab-case slug from the policy display name.

```go
// internal/normalize/slug.go

var (
    nonAlphanumRegex = regexp.MustCompile(`[^a-z0-9]+`)
    leadTrailDash    = regexp.MustCompile(`^-+|-+$`)
)

// Slugify converts a display name to a kebab-case slug.
// "CA001: Require MFA for admins" -> "ca001-require-mfa-for-admins"
func Slugify(displayName string) string {
    s := strings.ToLower(displayName)
    s = nonAlphanumRegex.ReplaceAllString(s, "-")
    s = leadTrailDash.ReplaceAllString(s, "")
    return s
}
```

### Anti-Patterns to Avoid

- **Storing state in the working tree:** Policy files in the working tree (e.g., `policies/*.json`) pollute `git status`, can be accidentally edited, and create merge conflicts. Use Git refs for state storage.
- **Using go-git for blob-to-ref mapping:** go-git has documented issues where refs pointing directly to blobs return `ErrUnsupportedObject`. Stick with `os/exec` git plumbing.
- **Importing msgraph-sdk-go:** The auto-generated models package adds 8+ minutes to build/test times. Use raw `net/http` with `azcore.TokenCredential.GetToken()` for auth.
- **Structured Go types for policy JSON:** CA policy JSON has deeply nested, frequently changing sub-objects. Use `map[string]interface{}` for normalization and `json.RawMessage` for passthrough. Typed structs will break when Microsoft adds fields.
- **Non-deterministic JSON output:** Using `json.Marshal` on structs produces field-order-dependent output. Always unmarshal to map and re-marshal for deterministic key ordering.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Git object storage | Custom file-based state store | `git hash-object -w`, `git update-ref`, `git cat-file` | Git handles deduplication, integrity (SHA), compression, and transport (push/pull) natively. |
| HTTP auth token management | Custom OAuth2 token cache | `azcore.TokenCredential.GetToken()` | azidentity handles token refresh, caching, retry, and multi-tenant isolation. |
| JSON key sorting | Custom sort-and-serialize | `encoding/json` with `map[string]interface{}` | Go stdlib automatically sorts map keys during marshal. Zero code needed. |
| OData pagination | Custom pagination loop | Simple `@odata.nextLink` follow loop | Graph API uses consistent pagination pattern. A 10-line while loop handles it. |
| Kebab-case conversion | Full Unicode slug library | `regexp` + `strings.ToLower` | CA policy names are ASCII-dominant. Two regex replacements cover all cases. |

**Key insight:** Phase 2's value is wiring Git plumbing and Graph HTTP into a clean import pipeline. The stdlib and existing azcore dependency handle all the hard parts. No new external dependencies needed.

## Common Pitfalls

### Pitfall 1: Graph API Returns @odata Metadata in Nested Objects

**What goes wrong:** Graph API responses include `@odata.context` and `@odata.type` fields at various nesting levels (e.g., `grantControls.authenticationStrength@odata.context`). If not stripped, these pollute the normalized JSON and cause false diffs.
**Why it happens:** Graph API adds OData metadata to complex nested types and navigation properties.
**How to avoid:** The normalization pipeline must recursively strip all keys starting with `@odata.` at every nesting level, not just top-level.
**Warning signs:** Normalized JSON contains `@odata.context` URLs; diffs show changes in metadata fields.

### Pitfall 2: Null vs. Absent Fields in Graph API Responses

**What goes wrong:** Graph API returns explicit `null` for many optional fields (e.g., `"sessionControls": null`, `"platforms": null`). If not removed, these create noise in diffs when Microsoft adds or removes nullable fields.
**Why it happens:** Graph API serializes all declared properties, even when null.
**How to avoid:** Recursive null removal after unmarshal but before re-marshal. Must handle nested maps and arrays of maps.
**Warning signs:** Policy JSON contains many `null` fields; re-importing the same policy shows diffs.

### Pitfall 3: Git Tag on Blob Requires Explicit Object Type

**What goes wrong:** `git tag -a <name> <blob-sha>` may fail or behave unexpectedly in some git versions because tags traditionally point to commits.
**Why it happens:** While git supports tagging any object type, some tooling assumes tags point to commits.
**How to avoid:** Verify tag creation works with blob SHAs in tests. The `git tag` command handles this correctly; the risk is with third-party tools reading tags. For our use case (cactl reads its own tags), this is fine.
**Warning signs:** Tag creation succeeds but tag listing/reading fails.

### Pitfall 4: Refspec Duplication on Repeated Init

**What goes wrong:** Running `cactl init` twice (or `cactl import` which also checks refspec) could add duplicate refspec entries to `.git/config`.
**Why it happens:** `git config --add` always appends. Without a check, each run adds another entry.
**How to avoid:** Before adding, check if the refspec already exists using `git config --get-all remote.origin.fetch` and search for `refs/cactl/*`.
**Warning signs:** `.git/config` has multiple identical refspec lines; push/pull issues.

### Pitfall 5: Race Condition Between slug Derivation and Existing State

**What goes wrong:** Two policies with display names that produce the same slug (e.g., "CA 001" and "CA-001" both become "ca-001") would overwrite each other.
**Why it happens:** Slugification is lossy -- it collapses multiple naming patterns to the same output.
**How to avoid:** Before import, check if the slug already exists in the manifest. If it maps to a different `live_object_id`, error with a message suggesting `--force` or manual rename. Include the object ID in the collision check, not just the slug name.
**Warning signs:** Import succeeds but a previously tracked policy disappears from the manifest.

### Pitfall 6: No Remote Origin Configured

**What goes wrong:** `ConfigureRefspec()` fails because there is no `remote.origin` in `.git/config` -- the user hasn't added a remote yet.
**Why it happens:** Fresh repos or repos cloned without a standard remote name.
**How to avoid:** Check if remote `origin` exists before configuring refspec. If no remote exists, skip refspec setup with a warning ("Add a remote and run `cactl init --refspec` to enable ref sync"). Refspec is only needed for push/pull; local-only usage works without it.
**Warning signs:** `git config --get remote.origin.fetch` returns error; init fails.

### Pitfall 7: Graph API Pagination for Large Policy Sets

**What goes wrong:** Tenants with many CA policies (50+) may return paginated responses. Without following `@odata.nextLink`, import only gets the first page.
**Why it happens:** Graph API defaults to page sizes of 100 for collections, but can return fewer.
**How to avoid:** After each list response, check for `@odata.nextLink` in the JSON. If present, follow it until no more pages. Implement a simple pagination loop in the Graph client.
**Warning signs:** `import --all` only finds a subset of policies; `--policy <name>` fails for policies on later pages.

## Code Examples

### Complete Import Flow (IMPORT-01, IMPORT-02)

```go
// cmd/import.go (thin orchestrator)

func runImport(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()

    // 1. Load config, create auth credential
    cfg, err := config.LoadFromGlobal()
    if err != nil {
        return fmt.Errorf("loading config: %w", err)
    }

    factory, err := auth.NewClientFactory(cfg.Auth)
    if err != nil {
        return fmt.Errorf("creating auth factory: %w", err)
    }

    cred, err := factory.Credential(ctx, cfg.Tenant)
    if err != nil {
        return fmt.Errorf("getting credential: %w", err)
    }

    // 2. Create Graph client and state backend
    graphClient := graph.NewClient(cred, cfg.Tenant)
    backend, err := state.NewGitBackend(".")
    if err != nil {
        return fmt.Errorf("initializing state backend: %w", err)
    }

    // 3. Fetch policies from Graph API
    policies, err := graphClient.ListPolicies(ctx)
    if err != nil {
        return fmt.Errorf("listing policies: %w", err)
    }

    // 4. For each policy: normalize and write to state
    for _, p := range policies {
        slug := normalize.Slugify(p.DisplayName)
        normalized, err := normalize.Normalize(p.RawJSON)
        if err != nil {
            return fmt.Errorf("normalizing %s: %w", slug, err)
        }

        if err := backend.WritePolicy(cfg.Tenant, slug, normalized); err != nil {
            return fmt.Errorf("writing %s: %w", slug, err)
        }

        // Update manifest
        // Create version tag
    }

    return nil
}
```

### Interactive Policy Selection (IMPORT-08)

```go
// When neither --all nor --policy is specified, list untracked policies
func selectPolicies(policies []graph.Policy, manifest *state.Manifest, r output.Renderer) ([]graph.Policy, error) {
    var untracked []graph.Policy
    for _, p := range policies {
        slug := normalize.Slugify(p.DisplayName)
        if _, exists := manifest.Policies[slug]; !exists {
            untracked = append(untracked, p)
        }
    }

    if len(untracked) == 0 {
        r.Info("All policies are already tracked")
        return nil, nil
    }

    r.Print("Untracked policies:")
    for i, p := range untracked {
        r.Print(fmt.Sprintf("  [%d] %s (%s)", i+1, p.DisplayName, p.ID))
    }
    // Prompt for selection...
}
```

### Normalized Policy Output Example

Given a Graph API response:
```json
{
  "id": "2b31ac51-b855-40a5-a986-0a4ed23e9008",
  "templateId": null,
  "displayName": "CA001: Require MFA for admins",
  "createdDateTime": "2021-11-02T14:17:09Z",
  "modifiedDateTime": "2024-01-03T20:07:59Z",
  "state": "enabled",
  "sessionControls": null,
  "conditions": {
    "platforms": null,
    "locations": null,
    "clientAppTypes": ["all"],
    "users": {
      "includeUsers": [],
      "excludeUsers": [],
      "includeGuestsOrExternalUsers": null,
      "excludeGuestsOrExternalUsers": null
    }
  },
  "grantControls": {
    "operator": "OR",
    "builtInControls": ["mfa"],
    "authenticationStrength@odata.context": "https://graph.microsoft.com/...",
    "authenticationStrength": null
  }
}
```

After normalization (IMPORT-03..05):
```json
{
  "conditions": {
    "clientAppTypes": [
      "all"
    ],
    "users": {
      "excludeUsers": [],
      "includeUsers": []
    }
  },
  "displayName": "CA001: Require MFA for admins",
  "grantControls": {
    "builtInControls": [
      "mfa"
    ],
    "operator": "OR"
  },
  "state": "enabled"
}
```

Stripped: `id`, `createdDateTime`, `modifiedDateTime`, `templateId`, `@odata.context`.
Removed nulls: `sessionControls`, `platforms`, `locations`, `includeGuestsOrExternalUsers`, `excludeGuestsOrExternalUsers`, `authenticationStrength`.
Sorted keys. Pretty-printed with 2-space indent.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| msgraph-sdk-go for all Graph calls | Raw `net/http` + azcore auth | 2024-2025 (community shift) | Avoids 8-minute build penalty. Official SDK issue #129 documents the problem. |
| go-git for programmatic Git | `os/exec` with git plumbing | Ongoing pattern in CLIs | go-git is useful for pure-Go scenarios but `os/exec` is simpler when git binary is available |
| File-based state in working tree | Git refs for state storage | Pattern used by git-notes, Gerrit, GitHub | Zero working tree footprint; native push/pull via refspec |
| Single-file state (terraform.tfstate) | Per-resource refs + manifest | Design choice for cactl | Enables per-policy version tracking and granular push/pull |

**Deprecated/outdated:**
- `msgraph-sdk-go` for small surface area: Avoid unless using 20+ Graph endpoints. Binary bloat is severe.
- `go-git` for blob-in-ref patterns: Has `ErrUnsupportedObject` issues. Use for commit-based workflows only.

## Graph API Details for CA Policies

### Endpoints Used in Phase 2

| Operation | Method | Path | Returns |
|-----------|--------|------|---------|
| List all policies | GET | `/identity/conditionalAccess/policies` | `{ "value": [Policy...] }` |
| Get single policy | GET | `/identity/conditionalAccess/policies/{id}` | `Policy` |

### Required Permissions

- **Delegated:** `Policy.Read.All` (minimum for import; `Policy.ReadWrite.ConditionalAccess` needed for Phase 3 apply)
- **Application:** `Policy.Read.All`
- **Entra roles:** Conditional Access Administrator, Security Administrator, Global Reader

### Server-Managed Fields to Strip (IMPORT-03)

| Field | Type | Why Strip |
|-------|------|-----------|
| `id` | String | Server-assigned GUID. Stored in manifest as `live_object_id` instead. |
| `createdDateTime` | DateTimeOffset | Server-managed timestamp. Changes on server, not in policy definition. |
| `modifiedDateTime` | DateTimeOffset | Server-managed timestamp. Would cause false diffs on every import. |
| `templateId` | String | Inherited from template. Read-only, not part of policy definition. |

### OData Metadata Fields to Strip

Any key matching `@odata.*` pattern at any nesting level:
- `@odata.context` (top-level and nested)
- `@odata.type` (on complex nested types)
- `authenticationStrength@odata.context` (embedded in grantControls)
- `combinationConfigurations@odata.context` (embedded in authenticationStrength)

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CLI-04 | `cactl import` pulls live CA policies into backend with normalization | Pattern 5 (Graph client) + Pattern 6 (normalization) + Pattern 1 (Git state store). Import command orchestrates: fetch from Graph, normalize, write to refs. |
| STATE-01 | State manifest maps slug to live Entra Object ID | Pattern 2: Manifest struct with `Policies` map keyed by slug, each entry has `LiveObjectID` field. Stored as blob at `refs/cactl/tenants/<tenant>/manifest`. |
| STATE-02 | GitBackend stores state in `refs/cactl/tenants/<tenant-id>/policies/<slug>` | Pattern 1: `git hash-object -w` writes blob, `git update-ref` points custom ref to blob SHA. `git cat-file blob` reads back. |
| STATE-03 | Every `cactl apply` creates immutable annotated Git tag (`cactl/<tenant>/<slug>/<semver>`) | Pattern 3: `git tag -a <name> <blob-sha> -m <message>`. Tags are immutable; audit trail via tagger identity and timestamp. Phase 2 creates tags on import (v1.0.0); Phase 3 extends for apply. |
| STATE-04 | `cactl init` writes refspec for automatic push/pull of `refs/cactl/*` | Pattern 4: `git config --add remote.origin.fetch +refs/cactl/*:refs/cactl/*` and same for push. Idempotent check before adding. Graceful skip if no remote. |
| STATE-05 | State entry schema: schema_version, slug, tenant, live_object_id, version, last_deployed, deployed_by, auth_mode, backend_sha | Pattern 2: `Entry` struct defines all fields. `Manifest` wraps with `schema_version` and tenant. |
| IMPORT-01 | `cactl import --all` fetches all live CA policies as v1.0.0 | Pattern 5: `ListPolicies()` fetches all via GET. Each policy normalized and written to state as v1.0.0 with annotated tag. |
| IMPORT-02 | `cactl import --policy <slug>` imports specific policy by slug or display name | Pattern 5: List all, filter by slug match or display name match. Error if no match found. |
| IMPORT-03 | Import strips server-managed fields (id, createdDateTime, modifiedDateTime, templateId) | Pattern 6: `serverManagedFields` slice + top-level delete. Also strips `@odata.*` recursively. |
| IMPORT-04 | Import removes explicit null fields from Graph API responses | Pattern 6: `removeNulls()` recursive function handles nested maps and arrays of maps. |
| IMPORT-05 | Import normalizes field order (alphabetical) and pretty-prints with 2-space indent | Pattern 6: `json.Unmarshal` to `map[string]interface{}` (auto-sorts on remarshal) + `json.MarshalIndent(m, "", "  ")`. |
| IMPORT-06 | Import enforces kebab-case slug format derived from display name | Pattern 7: `Slugify()` uses regex to lowercase, strip non-alphanumeric, join with hyphens. |
| IMPORT-07 | `cactl import --force` overwrites existing backend JSON for already-tracked policies | Check manifest for existing entry. Without `--force`, skip with warning. With `--force`, overwrite blob, update manifest entry, bump version. |
| IMPORT-08 | Without --policy or --all, list untracked policies and prompt for selection | Interactive selection pattern: compare Graph policies against manifest, show untracked with indices, prompt for selection. Respect `--ci` flag (error if no explicit selection in CI mode). |
</phase_requirements>

## Open Questions

1. **Should `cactl import` create version tags at v1.0.0 for initial import?**
   - What we know: IMPORT-01 says "imports them as v1.0.0". STATE-03 says "every apply creates a tag".
   - What's unclear: Should import also create tags, or only apply? If import creates tags, re-importing with `--force` needs to handle existing v1.0.0 tags.
   - Recommendation: Import creates v1.0.0 tags on first import. Re-import with `--force` creates v1.0.1, v1.0.2 etc. (PATCH bump for state-only changes). This gives version history from day one.

2. **How should `cactl init` Phase 2 extension interact with Phase 1 init?**
   - What we know: STATE-04 requires refspec setup in init. Phase 1 init already exists and creates .cactl/ directory.
   - What's unclear: Should Phase 2 modify `cmd/init.go` to add refspec setup, or should refspec be configured lazily on first `cactl import`?
   - Recommendation: Add refspec setup to `cmd/init.go` (extend existing init). Also check/configure lazily on first import as a safety net. This way both `cactl init` in a new workspace and `cactl import` in an existing workspace ensure refspec is configured.

3. **Empty arrays: keep or strip?**
   - What we know: Graph API returns empty arrays like `"excludeUsers": []`. Null removal does not affect empty arrays.
   - What's unclear: Should empty arrays be preserved (they distinguish "explicitly empty" from "not set") or removed for cleanliness?
   - Recommendation: Preserve empty arrays. They are semantically meaningful in CA policies -- `"excludeUsers": []` means "no exclusions" which is different from the field being absent. Only null values are removed.

4. **Tenant ID format: GUID vs. domain?**
   - What we know: The `--tenant` flag accepts "tenant ID or primary domain". Refs use tenant ID in the path (`refs/cactl/tenants/<tenant-id>/`).
   - What's unclear: If the user passes a domain name, should we resolve it to a GUID for the ref path? Domains can change; GUIDs are immutable.
   - Recommendation: Always resolve to GUID for ref paths. If user passes a domain, resolve it via Graph API (`GET /organization`) on first use and store the GUID in the manifest. This ensures ref paths are stable even if the tenant domain changes.

## Sources

### Primary (HIGH confidence)
- [Microsoft Learn: conditionalAccessPolicy resource type](https://learn.microsoft.com/en-us/graph/api/resources/conditionalaccesspolicy?view=graph-rest-1.0) -- Full property list, read-only fields, JSON representation
- [Microsoft Learn: List CA policies](https://learn.microsoft.com/en-us/graph/api/conditionalaccessroot-list-policies?view=graph-rest-1.0) -- Permissions, query parameters, full JSON response example
- [Git Book: Git Internals - Git Objects](https://git-scm.com/book/en/v2/Git-Internals-Git-Objects) -- hash-object, cat-file, blob storage
- [Git Book: The Refspec](https://git-scm.com/book/en/v2/Git-Internals-The-Refspec) -- Custom refspec configuration, push/pull for custom refs
- [go-git v5 on pkg.go.dev](https://pkg.go.dev/github.com/go-git/go-git/v5) -- API reference (evaluated and rejected for this use case)
- [go-git v5 plumbing/object](https://pkg.go.dev/github.com/go-git/go-git/v5/plumbing/object) -- Blob, Tag, reference types
- [encoding/json on pkg.go.dev](https://pkg.go.dev/encoding/json) -- MarshalIndent, map key sorting, omitempty behavior
- [msgraph-sdk-go issue #129: API surface size](https://github.com/microsoftgraph/msgraph-sdk-go/issues/129) -- Documented build time regression from SDK import

### Secondary (MEDIUM confidence)
- [go-git issue #530: refs/pull parsing failure](https://github.com/go-git/go-git/issues/530) -- Custom ref handling limitations
- [Git Cookbook: refs and refspecs](https://git.seveas.net/the-meaning-of-refs-and-refspecs.html) -- Third-party tools using custom ref namespaces (Gerrit, git-svn)
- [Removing null values from maps in Go](https://www.ribice.ba/golang-null-values/) -- Recursive null removal patterns
- [Gerrit refs/for namespace docs](https://gerrit-review.googlesource.com/Documentation/concept-refs-for-namespace.html) -- Precedent for custom ref namespaces in production systems
- [go-git annotated tag example (Medium)](https://medium.com/@clm160/tag-example-with-go-git-library-4377a84bbf17) -- CreateTag API and CreateTagOptions

### Tertiary (LOW confidence)
- [gobeam/stringy](https://github.com/gobeam/stringy) -- Kebab-case library (evaluated, stdlib regex preferred)
- [ettle/strcase](https://github.com/ettle/strcase) -- Case conversion library (evaluated, stdlib regex preferred)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already in go.mod; stdlib covers new functionality; no new dependencies
- Architecture (git state store): HIGH -- git plumbing commands are stable, well-documented, and used by Gerrit/GitHub for similar patterns
- Architecture (graph client): HIGH -- raw HTTP with azcore auth is proven pattern; CA policy API endpoints verified against official docs
- Normalization: HIGH -- Go stdlib encoding/json handles key sorting and indentation natively; recursive null removal is straightforward
- Pitfalls: HIGH -- Graph API response format verified against official examples; @odata stripping and null handling needs verified against real responses

**Research date:** 2026-03-04
**Valid until:** 2026-04-04 (stable APIs, 30-day validity)
