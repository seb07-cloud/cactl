# Phase 3: Plan and Apply - Research

**Researched:** 2026-03-04
**Domain:** Reconciliation engine (plan/apply), JSON diffing, semantic versioning, Graph API write operations (create/update/delete CA policies), display name resolution, plan-time validations
**Confidence:** HIGH

## Summary

Phase 3 is the core of cactl: the reconciliation engine that compares backend state (Git refs from Phase 2) against live tenant state (Graph API), produces a terraform-style diff, and applies changes. This phase spans five domains: (1) reconciliation engine with idempotency truth table, (2) colored diff output with sigils, (3) semantic versioning per policy, (4) display name resolution for GUIDs, and (5) plan-time safety validations.

The reconciliation engine compares two JSON documents per policy -- backend (desired) vs. live (actual) -- and classifies each into an action: create (+), update (~), recreate (-/+), noop, or untracked (?). The diff is computed using stdlib `encoding/json` unmarshal-to-map comparison (no external diff library needed for structured comparison). Colored output uses raw ANSI codes (the codebase already has this pattern in `internal/output/human.go`), not `fatih/color`, keeping the zero-dependency approach. Semantic versioning uses `golang.org/x/mod/semver` for comparison/validation -- it is lightweight, official, and sufficient since cactl does not need range constraints. Graph API write operations use the existing `internal/graph` client pattern from Phase 2, extending it with POST (create), PATCH (update), and DELETE (for recreate) methods. Display name resolution uses Graph API batch requests (`/$batch`) to resolve up to 20 GUIDs per call for groups, users, and named locations.

**Primary recommendation:** Build four new packages -- `internal/reconcile` (engine + truth table), `internal/semver` (version tracking with configurable field triggers), `internal/resolve` (display name cache), `internal/validate` (plan-time checks) -- plus two commands `cmd/plan.go` and `cmd/apply.go`. Extend `internal/graph` with write methods and batch resolution. Extend `internal/output` with diff rendering. Keep zero new external dependencies beyond `golang.org/x/mod/semver`.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `encoding/json` | N/A | JSON comparison via unmarshal-to-map, diff field detection | Already used in normalize package. Maps auto-sort keys. Deep comparison via `reflect.DeepEqual` on normalized maps. |
| Go stdlib `reflect` | N/A | Deep equality comparison of policy JSON maps | `reflect.DeepEqual` on `map[string]interface{}` gives field-level diff when combined with key iteration. |
| Go stdlib `fmt` + ANSI codes | N/A | Colored diff output with sigils (+, ~, -/+, ?) | Phase 1 already uses raw ANSI codes in `internal/output/human.go`. Consistent approach, zero dependency. |
| Go stdlib `os/exec` | N/A | Git plumbing for version tags (from Phase 2) | Continues Phase 2 pattern. Creates annotated tags on apply. |
| Go stdlib `net/http` | N/A | Graph API write operations (POST, PATCH, DELETE) and batch requests | Continues Phase 2 pattern. No new HTTP dependency needed. |
| golang.org/x/mod/semver | latest | Semantic version parsing, comparison, validation | Official Go module. Lightweight. Only need Compare, IsValid, Major, MajorMinor. No range/constraint support needed. |
| Azure/azure-sdk-for-go/sdk/azcore | v1.20.0 | TokenCredential for Graph API auth | Already in go.mod from Phase 1. |
| spf13/cobra | v1.10.2 | `plan` and `apply` command registration | Already in go.mod. |
| spf13/viper | v1.21.0 | Config reading (semver field triggers from config) | Already in go.mod. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stretchr/testify | v1.11.1 | Test assertions | Already in go.mod. Table-driven tests for reconcile, semver, validate, resolve packages. |
| Go stdlib `bufio` | N/A | Interactive confirmation prompts (apply, recreate) | Reading stdin for yes/no confirmation. |
| Go stdlib `strings` | N/A | Field path matching for semver triggers | Matching field paths like "conditions.users.includeUsers" against configured major/minor field lists. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Raw ANSI codes | `fatih/color` | fatih/color is the standard Go color library but adds an external dependency. The codebase already uses raw ANSI codes consistently. Adding fatih/color for one phase would create inconsistency. |
| `golang.org/x/mod/semver` | `Masterminds/semver/v3` | Masterminds is more feature-rich (ranges, constraints, wildcards) but cactl only needs parse/compare/bump. golang.org/x/mod is official and lighter. Requires `v` prefix which cactl can handle internally. |
| stdlib `reflect.DeepEqual` | `wI2L/jsondiff` (RFC 6902 patches) | jsondiff produces JSON Patch operations which are useful for APIs but overkill for human-readable diffs. We need field-level comparison, not patch generation. Stdlib reflection is sufficient. |
| Manual JSON comparison | `yudai/gojsondiff` | gojsondiff produces deltas but is unmaintained since 2017. Manual map comparison with key iteration gives us exactly the output format we need (terraform-style, not unified diff). |

### Installation

```bash
go get golang.org/x/mod/semver
```

No other new dependencies required. Phase 3 uses only what is already in `go.mod` plus `golang.org/x/mod`.

## Architecture Patterns

### Recommended Project Structure (Phase 3 additions)

```
cactl/
├── cmd/
│   ├── plan.go               # cactl plan command (CLI-02, PLAN-01..04)
│   ├── plan_test.go           # Plan command tests
│   ├── apply.go               # cactl apply command (CLI-03, PLAN-05..10)
│   ├── apply_test.go          # Apply command tests
│   ├── status.go              # cactl status command (DISP-05)
│   └── ... (existing)
├── internal/
│   ├── reconcile/
│   │   ├── engine.go          # Reconciliation engine: compare backend vs live
│   │   ├── engine_test.go     # Table-driven tests for truth table
│   │   ├── action.go          # Action types: Create, Update, Recreate, Noop, Untracked
│   │   ├── diff.go            # Field-level JSON diff computation
│   │   └── diff_test.go       # Diff computation tests
│   ├── semver/
│   │   ├── version.go         # Version tracking, bump logic, field trigger matching
│   │   ├── version_test.go    # Semver bump tests
│   │   └── config.go          # Configurable field trigger definitions
│   ├── resolve/
│   │   ├── resolver.go        # Display name resolution with cache
│   │   ├── resolver_test.go   # Resolution tests with mock Graph
│   │   └── batch.go           # Graph API batch request helper
│   ├── validate/
│   │   ├── validate.go        # Plan-time validation rules
│   │   └── validate_test.go   # Validation rule tests
│   ├── graph/
│   │   ├── client.go          # (extend) Add CreatePolicy, UpdatePolicy, DeletePolicy
│   │   ├── policies.go        # (extend) Add write methods
│   │   ├── batch.go           # Batch request helper for /$batch endpoint
│   │   └── ... (existing)
│   ├── output/
│   │   ├── diff.go            # Terraform-style diff renderer (sigils, colors, summary)
│   │   ├── diff_test.go       # Diff rendering tests
│   │   └── ... (existing)
│   └── ... (existing)
├── pkg/types/
│   ├── plan.go                # Plan types: PlanResult, PolicyAction, DiffEntry
│   └── ... (existing)
└── ... (existing)
```

### Pattern 1: Reconciliation Engine with Idempotency Truth Table (PLAN-01, PLAN-09, PLAN-10)

**What:** The engine takes two inputs -- backend policies (from Git refs) and live policies (from Graph API) -- and produces a plan: a list of actions per policy.

**Truth Table (PLAN-10):**

| Backend Exists | Live Exists | Backend == Live | Action | Sigil |
|----------------|-------------|-----------------|--------|-------|
| Yes | No | N/A | Create | + |
| Yes | Yes | No | Update | ~ |
| Yes | Yes | Yes | Noop | (omitted) |
| No | Yes | N/A | Untracked | ? |
| Yes (but live ID gone) | No (ghost) | N/A | Recreate | -/+ |

The "recreate" case occurs when the manifest has a `live_object_id` for a policy but that ID no longer exists in the tenant (policy was deleted outside cactl). The engine must create a new policy and update the manifest with the new object ID.

```go
// internal/reconcile/action.go

type ActionType int

const (
    ActionNoop      ActionType = iota
    ActionCreate               // + backend exists, no live match
    ActionUpdate               // ~ backend and live differ
    ActionRecreate             // -/+ manifest has ID but live policy gone
    ActionUntracked            // ? live exists, not in backend
)

// PolicyAction represents a planned action for one policy.
type PolicyAction struct {
    Slug         string
    Action       ActionType
    DisplayName  string
    BackendJSON  map[string]interface{} // desired state
    LiveJSON     map[string]interface{} // current state (nil for Create)
    LiveObjectID string                 // Entra object ID (empty for Create)
    Diff         []FieldDiff            // field-level differences
    VersionBump  BumpLevel              // MAJOR, MINOR, PATCH
    Warnings     []string               // validation warnings
}
```

```go
// internal/reconcile/engine.go

// Reconcile compares backend state against live state and returns a plan.
func Reconcile(backend map[string]PolicyState, live map[string]LivePolicy, manifest *state.Manifest) []PolicyAction {
    var actions []PolicyAction

    // 1. Check each backend policy against live state
    for slug, backendState := range backend {
        entry, tracked := manifest.Policies[slug]

        if !tracked {
            // Backend policy not in manifest = Create
            actions = append(actions, PolicyAction{
                Slug: slug, Action: ActionCreate,
                BackendJSON: backendState.Data,
            })
            continue
        }

        livePolicy, liveExists := live[entry.LiveObjectID]
        if !liveExists {
            // Manifest has ID but live policy gone = Recreate
            actions = append(actions, PolicyAction{
                Slug: slug, Action: ActionRecreate,
                BackendJSON: backendState.Data,
            })
            continue
        }

        // Compare backend vs live (both normalized)
        diffs := ComputeDiff(backendState.Data, livePolicy.NormalizedData)
        if len(diffs) == 0 {
            // Noop -- skip (PLAN-09: idempotent)
            continue
        }

        actions = append(actions, PolicyAction{
            Slug: slug, Action: ActionUpdate,
            BackendJSON: backendState.Data,
            LiveJSON:    livePolicy.NormalizedData,
            LiveObjectID: entry.LiveObjectID,
            Diff:        diffs,
        })
    }

    // 2. Check for untracked live policies
    trackedIDs := make(map[string]bool)
    for _, entry := range manifest.Policies {
        trackedIDs[entry.LiveObjectID] = true
    }
    for id, livePolicy := range live {
        if !trackedIDs[id] {
            actions = append(actions, PolicyAction{
                Slug: livePolicy.Slug, Action: ActionUntracked,
                LiveJSON:     livePolicy.NormalizedData,
                LiveObjectID: id,
            })
        }
    }

    return actions
}
```

### Pattern 2: Field-Level JSON Diff (PLAN-02, DISP-01)

**What:** Compute field-level differences between two normalized JSON maps. Walk both maps recursively, reporting added/removed/changed fields with their paths.

```go
// internal/reconcile/diff.go

type DiffType int

const (
    DiffAdded   DiffType = iota // field in backend, not in live
    DiffRemoved                 // field in live, not in backend
    DiffChanged                 // field in both but different value
)

type FieldDiff struct {
    Path     string      // dot-separated path, e.g., "conditions.users.includeGroups"
    Type     DiffType
    OldValue interface{} // live value (nil for Added)
    NewValue interface{} // backend value (nil for Removed)
}

// ComputeDiff recursively compares two normalized JSON maps.
func ComputeDiff(desired, actual map[string]interface{}) []FieldDiff {
    var diffs []FieldDiff
    computeDiffRecursive("", desired, actual, &diffs)
    return diffs
}

func computeDiffRecursive(prefix string, desired, actual map[string]interface{}, diffs *[]FieldDiff) {
    // Check fields in desired
    for key, dVal := range desired {
        path := joinPath(prefix, key)
        aVal, exists := actual[key]
        if !exists {
            *diffs = append(*diffs, FieldDiff{Path: path, Type: DiffAdded, NewValue: dVal})
            continue
        }

        // Both exist -- compare
        dMap, dIsMap := dVal.(map[string]interface{})
        aMap, aIsMap := aVal.(map[string]interface{})
        if dIsMap && aIsMap {
            computeDiffRecursive(path, dMap, aMap, diffs)
        } else if !reflect.DeepEqual(dVal, aVal) {
            *diffs = append(*diffs, FieldDiff{Path: path, Type: DiffChanged, OldValue: aVal, NewValue: dVal})
        }
    }

    // Check fields in actual but not in desired
    for key, aVal := range actual {
        path := joinPath(prefix, key)
        if _, exists := desired[key]; !exists {
            *diffs = append(*diffs, FieldDiff{Path: path, Type: DiffRemoved, OldValue: aVal})
        }
    }
}
```

### Pattern 3: Terraform-Style Colored Diff Output (DISP-01, PLAN-02, PLAN-04)

**What:** Render plan results with colored sigils, field-level changes, and summary counts.

```
cactl plan

  ~ ca001-require-mfa-for-admins (MINOR bump: 1.0.0 -> 1.1.0)
      ~ conditions.users.includeGroups:
          - "Admins" (ba8e7ded-8b0f-4836-ba06-8ff1ecc5c8ba)
          + "All Admins" (c92a4f2b-1234-5678-abcd-ef0123456789)

  + ca002-block-legacy-auth
      (new policy, initial version 1.0.0)

  ? ca-unknown-policy (untracked: exists in tenant, not in backend)

Plan: 1 to create, 1 to update, 0 to recreate, 1 untracked.
```

```go
// internal/output/diff.go

const (
    colorCyan    = "\033[36m"
    colorMagenta = "\033[35m"
)

// Sigils map action types to their display characters and colors
var sigils = map[reconcile.ActionType]struct {
    symbol string
    color  string
}{
    reconcile.ActionCreate:    {"+", colorGreen},
    reconcile.ActionUpdate:    {"~", colorYellow},
    reconcile.ActionRecreate:  {"-/+", colorRed},
    reconcile.ActionUntracked: {"?", colorCyan},
}

// RenderPlan outputs the full plan to stdout.
func RenderPlan(actions []reconcile.PolicyAction, useColor bool) {
    counts := map[reconcile.ActionType]int{}
    for _, a := range actions {
        if a.Action == reconcile.ActionNoop {
            continue
        }
        counts[a.Action]++
        renderAction(a, useColor)
    }
    renderSummary(counts, useColor)
}

func renderSummary(counts map[reconcile.ActionType]int, useColor bool) {
    fmt.Printf("\nPlan: %d to create, %d to update, %d to recreate, %d untracked.\n",
        counts[reconcile.ActionCreate],
        counts[reconcile.ActionUpdate],
        counts[reconcile.ActionRecreate],
        counts[reconcile.ActionUntracked],
    )
}
```

### Pattern 4: Apply with Confirmation and Idempotency (PLAN-05..10)

**What:** `cactl apply` generates a plan (reuses reconcile engine), displays it, prompts for confirmation, then executes Graph API calls.

```go
// cmd/apply.go (simplified flow)

func runApply(cmd *cobra.Command, args []string) error {
    // 1. Generate plan (same as cactl plan)
    actions := reconcile.Reconcile(backend, live, manifest)

    // 2. Display plan
    output.RenderPlan(actions, useColor)

    // Filter to actionable items (no Noop, no Untracked)
    actionable := filterActionable(actions)
    if len(actionable) == 0 {
        renderer.Success("No changes. Infrastructure is up-to-date.")
        return nil // Exit 0 (PLAN-09)
    }

    // 3. Confirmation
    autoApprove, _ := cmd.Flags().GetBool("auto-approve")
    dryRun, _ := cmd.Flags().GetBool("dry-run")

    if dryRun {
        renderer.Info("Dry run: no changes applied.")
        return nil
    }

    if !autoApprove {
        // Standard confirmation
        if !confirm("Do you want to apply these changes?") {
            renderer.Info("Apply cancelled.")
            return nil
        }

        // Escalated confirmation for recreate actions (PLAN-08)
        hasRecreate := hasAction(actionable, reconcile.ActionRecreate)
        if hasRecreate {
            if !confirmExplicit("Type 'yes' to confirm recreate actions: ") {
                renderer.Info("Apply cancelled (recreate not confirmed).")
                return nil
            }
        }
    }

    // 4. Execute actions
    for _, a := range actionable {
        switch a.Action {
        case reconcile.ActionCreate:
            newID, err := graphClient.CreatePolicy(ctx, a.BackendJSON)
            // Update manifest with new ID
        case reconcile.ActionUpdate:
            err := graphClient.UpdatePolicy(ctx, a.LiveObjectID, a.BackendJSON)
        case reconcile.ActionRecreate:
            newID, err := graphClient.CreatePolicy(ctx, a.BackendJSON)
            // Clean up ghost ref, update manifest
        }
        // Create version tag
        // Update manifest
    }

    return nil
}
```

### Pattern 5: Semantic Versioning with Configurable Field Triggers (SEMV-01..06)

**What:** Each policy has an independent semver. Bump level is determined by which fields changed, configurable via `semver.major_fields` and `semver.minor_fields` in config.

**Config schema extension:**

```yaml
# .cactl/config.yaml
semver:
  major_fields:
    - "conditions.users.includeUsers"
    - "conditions.users.includeGroups"
    - "conditions.users.excludeUsers"
    - "conditions.users.excludeGroups"
    - "conditions.applications.includeApplications"
    - "conditions.applications.excludeApplications"
    - "state"  # enabling/disabling is always MAJOR
  minor_fields:
    - "conditions"  # any conditions change not in major
    - "grantControls"
    - "sessionControls"
  # All other fields = PATCH
```

```go
// internal/semver/version.go

type BumpLevel int

const (
    BumpPatch BumpLevel = iota
    BumpMinor
    BumpMajor
)

// DetermineBump analyzes field diffs against configured triggers.
func DetermineBump(diffs []reconcile.FieldDiff, majorFields, minorFields []string) BumpLevel {
    bump := BumpPatch
    for _, d := range diffs {
        if matchesAny(d.Path, majorFields) {
            return BumpMajor // Short-circuit: any major field = MAJOR
        }
        if matchesAny(d.Path, minorFields) {
            bump = BumpMinor
        }
    }
    return bump
}

// matchesAny checks if a field path matches any configured trigger.
// Supports prefix matching: "conditions" matches "conditions.users.includeGroups".
func matchesAny(path string, triggers []string) bool {
    for _, trigger := range triggers {
        if path == trigger || strings.HasPrefix(path, trigger+".") {
            return true
        }
    }
    return false
}

// BumpVersion increments a semver string by the given level.
func BumpVersion(current string, level BumpLevel) (string, error) {
    // Parse "1.2.3" into major, minor, patch integers
    // Increment appropriate component, reset lower components
    // Return new version string
}
```

### Pattern 6: Display Name Resolution with Caching (DISP-03, DISP-04)

**What:** Resolve GUIDs (groups, users, named locations, service principals) to human-readable display names using Graph API batch requests. Cache results for the duration of a plan/apply run.

**Graph API endpoints:**
- Groups: `GET /groups/{id}?$select=id,displayName`
- Users: `GET /users/{id}?$select=id,displayName`
- Named locations: `GET /identity/conditionalAccess/namedLocations/{id}`
- Service principals: `GET /servicePrincipals/{id}?$select=id,displayName`

**Batch approach:** Collect all GUIDs from the plan, deduplicate, batch into groups of 20 (Graph API limit), resolve via `POST /$batch`.

```go
// internal/resolve/resolver.go

type Resolver struct {
    graphClient *graph.Client
    cache       map[string]string // GUID -> display name
    mu          sync.Mutex
}

func NewResolver(client *graph.Client) *Resolver {
    return &Resolver{
        graphClient: client,
        cache:       make(map[string]string),
    }
}

// ResolveAll takes a list of GUIDs with their types and resolves display names.
func (r *Resolver) ResolveAll(ctx context.Context, refs []ObjectRef) error {
    // 1. Filter out already-cached GUIDs
    // 2. Group by type (user, group, namedLocation, servicePrincipal)
    // 3. Batch into groups of 20
    // 4. Execute batch requests via POST /$batch
    // 5. Cache results
    return nil
}

// DisplayName returns the cached display name for a GUID, or the GUID itself as fallback.
func (r *Resolver) DisplayName(id string) string {
    r.mu.Lock()
    defer r.mu.Unlock()
    if name, ok := r.cache[id]; ok {
        return name
    }
    return id // Graceful degradation: show GUID if resolution fails
}

type ObjectRef struct {
    ID   string
    Type string // "user", "group", "namedLocation", "servicePrincipal"
}
```

```go
// internal/graph/batch.go

type BatchRequest struct {
    Requests []BatchRequestItem `json:"requests"`
}

type BatchRequestItem struct {
    ID     string `json:"id"`
    Method string `json:"method"`
    URL    string `json:"url"`
}

type BatchResponse struct {
    Responses []BatchResponseItem `json:"responses"`
}

type BatchResponseItem struct {
    ID     string          `json:"id"`
    Status int             `json:"status"`
    Body   json.RawMessage `json:"body"`
}

// ExecuteBatch sends a batch request to Graph API /$batch endpoint.
func (c *Client) ExecuteBatch(ctx context.Context, requests []BatchRequestItem) ([]BatchResponseItem, error) {
    batch := BatchRequest{Requests: requests}
    body, _ := json.Marshal(batch)

    resp, err := c.do(ctx, "POST", "/$batch", bytes.NewReader(body))
    // ... handle response
}
```

### Pattern 7: Plan-Time Validations (VALID-01..05)

**What:** A set of validation rules that run against the plan before apply. Each rule returns warnings (non-blocking) or errors (blocking).

```go
// internal/validate/validate.go

type Severity int

const (
    SeverityWarning Severity = iota
    SeverityError
)

type ValidationResult struct {
    Rule     string
    Severity Severity
    Message  string
    Policy   string // slug of the affected policy
}

// ValidatePlan runs all validation rules against the plan.
func ValidatePlan(actions []reconcile.PolicyAction, cfg ValidationConfig) []ValidationResult {
    var results []ValidationResult
    for _, a := range actions {
        if a.Action == reconcile.ActionNoop || a.Action == reconcile.ActionUntracked {
            continue
        }
        results = append(results, checkBreakGlass(a, cfg)...)
        results = append(results, checkSchema(a, cfg)...)
        results = append(results, checkConflictingConditions(a)...)
        results = append(results, checkEmptyIncludes(a)...)
        results = append(results, checkOverlyBroad(a)...)
    }
    return results
}
```

**VALID-01: Break-glass account exclusion.**
Check that `conditions.users.excludeUsers` contains the configured break-glass account IDs. Config: `validation.break_glass_accounts: ["<guid1>", "<guid2>"]`.

```go
func checkBreakGlass(a reconcile.PolicyAction, cfg ValidationConfig) []ValidationResult {
    if len(cfg.BreakGlassAccounts) == 0 {
        return nil // Not configured, skip
    }

    excludeUsers := getStringSlice(a.BackendJSON, "conditions.users.excludeUsers")
    excludeSet := toSet(excludeUsers)

    var results []ValidationResult
    for _, bg := range cfg.BreakGlassAccounts {
        if !excludeSet[bg] {
            results = append(results, ValidationResult{
                Rule:     "VALID-01",
                Severity: SeverityWarning,
                Message:  fmt.Sprintf("Break-glass account %s not excluded from policy", bg),
                Policy:   a.Slug,
            })
        }
    }
    return results
}
```

**VALID-03: Conflicting conditions.**
Detect when the same GUID appears in both include and exclude lists.

```go
func checkConflictingConditions(a reconcile.PolicyAction) []ValidationResult {
    checks := []struct {
        include string
        exclude string
        label   string
    }{
        {"conditions.users.includeUsers", "conditions.users.excludeUsers", "users"},
        {"conditions.users.includeGroups", "conditions.users.excludeGroups", "groups"},
        {"conditions.applications.includeApplications", "conditions.applications.excludeApplications", "applications"},
        {"conditions.locations.includeLocations", "conditions.locations.excludeLocations", "locations"},
    }

    var results []ValidationResult
    for _, c := range checks {
        includes := toSet(getStringSlice(a.BackendJSON, c.include))
        excludes := getStringSlice(a.BackendJSON, c.exclude)
        for _, id := range excludes {
            if includes[id] {
                results = append(results, ValidationResult{
                    Rule:     "VALID-03",
                    Severity: SeverityError,
                    Message:  fmt.Sprintf("Conflicting %s: %s is both included and excluded", c.label, id),
                    Policy:   a.Slug,
                })
            }
        }
    }
    return results
}
```

**VALID-04: Empty include lists.**
Detect policies where `includeUsers` is empty AND `includeGroups` is empty AND no `includeRoles` -- policy applies to no one.

**VALID-05: Overly broad policies.**
Detect policies where `includeUsers` contains "All" but `excludeUsers` and `excludeGroups` are both empty, and `state` is "enabled".

### Pattern 8: Graph API Write Operations

**What:** Extend the Phase 2 Graph client with create, update, and delete methods for CA policies.

**Endpoints:**

| Operation | Method | Path | Request Body | Response |
|-----------|--------|------|-------------|----------|
| Create | POST | `/identity/conditionalAccess/policies` | Full policy JSON | 201 Created + policy JSON (includes new `id`) |
| Update | PATCH | `/identity/conditionalAccess/policies/{id}` | Partial or full policy JSON | 204 No Content |
| Delete | DELETE | `/identity/conditionalAccess/policies/{id}` | None | 204 No Content |

**Permissions required:** `Policy.Read.All` AND `Policy.ReadWrite.ConditionalAccess` (both delegated and application).

**Important:** PATCH is a partial update -- only fields included in the body are changed. For cactl, we send the full normalized policy body on update to ensure desired state matches exactly.

```go
// internal/graph/policies.go (extensions)

// CreatePolicy creates a new CA policy and returns its server-assigned ID.
func (c *Client) CreatePolicy(ctx context.Context, policyJSON map[string]interface{}) (string, error) {
    body, _ := json.Marshal(policyJSON)
    resp, err := c.do(ctx, "POST", "/identity/conditionalAccess/policies", bytes.NewReader(body))
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        respBody, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("create policy: HTTP %d: %s", resp.StatusCode, string(respBody))
    }

    var result struct {
        ID string `json:"id"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.ID, nil
}

// UpdatePolicy updates an existing CA policy by ID.
func (c *Client) UpdatePolicy(ctx context.Context, id string, policyJSON map[string]interface{}) error {
    body, _ := json.Marshal(policyJSON)
    path := fmt.Sprintf("/identity/conditionalAccess/policies/%s", id)
    resp, err := c.do(ctx, "PATCH", path, bytes.NewReader(body))
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusNoContent {
        respBody, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("update policy %s: HTTP %d: %s", id, resp.StatusCode, string(respBody))
    }
    return nil
}

// DeletePolicy deletes a CA policy by ID.
func (c *Client) DeletePolicy(ctx context.Context, id string) error {
    path := fmt.Sprintf("/identity/conditionalAccess/policies/%s", id)
    resp, err := c.do(ctx, "DELETE", path, nil)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusNoContent {
        respBody, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("delete policy %s: HTTP %d: %s", id, resp.StatusCode, string(respBody))
    }
    return nil
}
```

### Pattern 9: JSON Output Schema (DISP-02)

**What:** All commands support `--output json` with a stable schema including `schema_version`.

```go
// pkg/types/plan.go

type PlanOutput struct {
    SchemaVersion int            `json:"schema_version"`
    Actions       []ActionOutput `json:"actions"`
    Summary       SummaryOutput  `json:"summary"`
}

type ActionOutput struct {
    Slug        string       `json:"slug"`
    DisplayName string       `json:"display_name"`
    Action      string       `json:"action"` // "create", "update", "recreate", "untracked"
    VersionFrom string       `json:"version_from,omitempty"`
    VersionTo   string       `json:"version_to,omitempty"`
    BumpLevel   string       `json:"bump_level,omitempty"` // "major", "minor", "patch"
    Diffs       []DiffOutput `json:"diffs,omitempty"`
    Warnings    []string     `json:"warnings,omitempty"`
}

type DiffOutput struct {
    Path     string      `json:"path"`
    Type     string      `json:"type"` // "added", "removed", "changed"
    OldValue interface{} `json:"old_value,omitempty"`
    NewValue interface{} `json:"new_value,omitempty"`
}

type SummaryOutput struct {
    Create    int `json:"create"`
    Update    int `json:"update"`
    Recreate  int `json:"recreate"`
    Untracked int `json:"untracked"`
}
```

### Anti-Patterns to Avoid

- **Full JSON text diff instead of structured comparison:** Using `diff` or unified-diff on raw JSON strings produces noisy output (whitespace changes, key reordering). Always compare normalized `map[string]interface{}` values.
- **Sending partial updates on PATCH:** While Graph API supports partial PATCH, sending only changed fields risks drift if the local normalization strips fields the API expects. Send the full normalized body on every update.
- **Hardcoding semver field triggers:** What constitutes a "MAJOR" change is organization-specific. Always read triggers from config, with sensible defaults.
- **Resolving display names synchronously per-GUID:** Each Graph API call adds latency. Batch all resolution into one or two `/$batch` calls before rendering.
- **Blocking apply on validation warnings:** Warnings (VALID-01 break-glass) should be loud but non-blocking. Only schema violations (VALID-02) and conflicting conditions (VALID-03) should block apply.
- **Mutating state before all actions succeed:** If apply fails midway, partial state updates create inconsistency. Either use per-action state updates with clear error reporting, or batch state updates at the end. Per-action is safer (shows progress) but must handle partial failure gracefully.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Semantic version parsing/comparison | Custom string parsing | `golang.org/x/mod/semver` | Edge cases with prerelease, metadata, comparison precedence |
| Token acquisition for Graph API | Custom OAuth2 flow | `azcore.TokenCredential.GetToken()` | Already handled by Phase 1 ClientFactory |
| JSON key sorting for comparison | Custom sort | `encoding/json` marshal of `map[string]interface{}` | Go stdlib auto-sorts map keys |
| Interactive terminal prompts | Custom stdin reader | `bufio.Scanner` with simple yes/no/Enter logic | Standard pattern, no library needed |
| Colored terminal output | Full TUI framework | Raw ANSI codes (existing pattern) | Consistent with Phase 1; zero dependency |
| GUID batch resolution | Sequential per-GUID calls | Graph API `/$batch` endpoint | 20x fewer HTTP calls; single round-trip per batch |

**Key insight:** Phase 3 is primarily a composition layer. The reconciliation engine, diff computation, and validation rules are all custom logic specific to cactl's domain. But the underlying I/O (Graph API, Git plumbing, JSON handling, semver) is handled by existing dependencies and stdlib.

## Common Pitfalls

### Pitfall 1: Graph API PATCH Returns 204 With No Body

**What goes wrong:** After updating a policy, you cannot read back the updated state from the PATCH response (it returns 204 No Content with empty body). If you need to verify the update, you must make a separate GET call.
**Why it happens:** Graph API design -- PATCH is fire-and-forget for CA policies.
**How to avoid:** After PATCH, do NOT assume success means the state matches. For idempotency verification, re-fetch the policy with GET and compare. For plan-time validation, rely on the pre-PATCH comparison, not post-PATCH verification.
**Warning signs:** Tests that try to read the PATCH response body get empty results.

### Pitfall 2: Recreate Requires Delete + Create (Not Atomic)

**What goes wrong:** When a policy needs recreation (ghost cleanup), you must DELETE the old ID (if it still exists) then POST a new one. These are two separate Graph API calls -- not atomic. If the create fails after delete, the policy is gone.
**Why it happens:** Graph API has no "replace" operation for CA policies.
**How to avoid:** For the recreate case where the old policy is already deleted (ghost), only POST is needed. For true recreate (old exists but must be replaced), consider: (1) create new policy in disabled state, (2) verify creation, (3) delete old policy, (4) enable new policy. This is safer than delete-first.
**Warning signs:** Apply fails midway through recreate; policy exists in neither old nor new form.

### Pitfall 3: Display Name Resolution Fails Silently for Deleted Objects

**What goes wrong:** GUIDs in policy JSON may reference deleted groups, users, or locations. Graph API returns 404 for these. If not handled, the batch response fails or the plan crashes.
**Why it happens:** CA policies can reference objects that have since been deleted from the tenant.
**How to avoid:** Handle 404 in batch responses gracefully -- show the GUID with a "(deleted)" or "(unknown)" suffix instead of crashing. The resolver should never fail the plan.
**Warning signs:** Plan crashes with "resource not found" on seemingly valid policies.

### Pitfall 4: Normalization Drift Between Import and Plan

**What goes wrong:** If plan normalizes live JSON differently than import normalized backend JSON, you get false diffs on every plan run (never reaches noop state).
**Why it happens:** Different code paths for normalization, or Graph API returns different field sets over time.
**How to avoid:** Both import and plan MUST use the exact same `normalize.Normalize()` function on both backend and live JSON before comparison. This is the single most important design constraint for idempotency.
**Warning signs:** `cactl plan` always shows changes even after `cactl apply` just ran.

### Pitfall 5: Config Semver Fields Not Matching JSON Paths

**What goes wrong:** User configures `semver.major_fields: ["conditions.users"]` but the actual JSON path is `conditions.users.includeUsers`. The prefix matching either over-matches (any conditions.users change is MAJOR) or under-matches.
**Why it happens:** Ambiguity between exact match and prefix match in field trigger configuration.
**How to avoid:** Document explicitly that triggers use prefix matching -- `"conditions.users"` matches any field under conditions.users. Provide a `cactl validate-config` check or plan-time warning if a configured path never matches any field in any policy.
**Warning signs:** All changes are PATCH because configured paths never match.

### Pitfall 6: Break-Glass Validation Requires Pre-Configuration

**What goes wrong:** VALID-01 does nothing if `validation.break_glass_accounts` is not configured. Users get no warning about missing break-glass exclusions.
**Why it happens:** The tool cannot automatically discover break-glass accounts -- they must be explicitly configured.
**How to avoid:** On first run (or if config section is missing), emit an INFO message: "Configure break-glass accounts in .cactl/config.yaml for policy safety validation." Make it opt-in but visible.
**Warning signs:** Users deploy policies that lock out emergency accounts with no warning.

### Pitfall 7: Graph API Rate Limiting on Batch Requests

**What goes wrong:** Display name resolution for large tenants (many groups, many users) hits Graph API throttling (429 Too Many Requests).
**Why it happens:** Each batch can contain 20 requests, but Graph applies per-app rate limits.
**How to avoid:** Implement retry with exponential backoff on 429 responses. Cache resolved names aggressively (they rarely change). Consider pre-fetching all named locations at plan start (usually <100 items) instead of per-GUID resolution.
**Warning signs:** Plan intermittently fails with "429 Too Many Requests"; works on retry.

### Pitfall 8: `cactl apply --auto-approve` Without Safety Net in CI

**What goes wrong:** CI pipeline runs `cactl apply --auto-approve` and a MAJOR scope expansion deploys without human review.
**Why it happens:** Auto-approve bypasses all confirmation prompts including recreate escalation.
**How to avoid:** Consider a `--auto-approve-level` flag: `--auto-approve-level=minor` would auto-approve PATCH and MINOR but still require confirmation for MAJOR bumps. Or: in CI mode, always output the full plan summary before executing, and fail with exit code 1 if any MAJOR bumps exist (require explicit `--allow-major`).
**Warning signs:** MAJOR scope changes deployed to production without review.

## Code Examples

### Complete Plan Flow

```go
// cmd/plan.go

func runPlan(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()

    // 1. Load config and create auth
    cfg, _ := config.LoadFromGlobal()
    factory, _ := auth.NewClientFactory(cfg.Auth)
    cred, _ := factory.Credential(ctx, cfg.Tenant)

    // 2. Create clients
    graphClient := graph.NewClient(cred, cfg.Tenant)
    backend, _ := state.NewGitBackend(".")
    resolver := resolve.NewResolver(graphClient)

    // 3. Load backend state (all policies from Git refs)
    backendPolicies, _ := loadBackendPolicies(backend, cfg.Tenant)

    // 4. Load live state (all policies from Graph API)
    livePolicies, _ := graphClient.ListPolicies(ctx)
    normalizedLive := normalizeLivePolicies(livePolicies)

    // 5. Load manifest
    manifest, _ := backend.ReadManifest(cfg.Tenant)

    // 6. Reconcile
    actions := reconcile.Reconcile(backendPolicies, normalizedLive, manifest)

    // 7. Compute semver bumps
    semverCfg := loadSemverConfig(cfg)
    for i := range actions {
        if actions[i].Action == reconcile.ActionUpdate {
            actions[i].VersionBump = semver.DetermineBump(
                actions[i].Diff, semverCfg.MajorFields, semverCfg.MinorFields)
        }
    }

    // 8. Run validations
    validationResults := validate.ValidatePlan(actions, loadValidationConfig(cfg))

    // 9. Resolve display names
    refs := collectGUIDRefs(actions)
    resolver.ResolveAll(ctx, refs)

    // 10. Render output
    format := cfg.Output
    if format == "json" {
        output.RenderPlanJSON(actions, validationResults)
    } else {
        output.RenderPlan(actions, validationResults, resolver, output.ShouldUseColor(viper.GetViper()))
    }

    // 11. Exit code: 0 if no changes, 1 if changes detected
    if hasChanges(actions) {
        return &types.ExitError{Code: types.ExitChanges, Message: "changes detected"}
    }
    return nil
}
```

### Config Extension for Phase 3

```yaml
# .cactl/config.yaml additions for Phase 3
semver:
  major_fields:
    - "conditions.users.includeUsers"
    - "conditions.users.includeGroups"
    - "conditions.users.excludeUsers"
    - "conditions.users.excludeGroups"
    - "conditions.applications.includeApplications"
    - "conditions.applications.excludeApplications"
    - "state"
  minor_fields:
    - "conditions"
    - "grantControls"
    - "sessionControls"

validation:
  break_glass_accounts: []  # GUIDs of emergency access accounts
  # break_glass_accounts:
  #   - "12345678-1234-1234-1234-123456789012"
  #   - "87654321-4321-4321-4321-210987654321"
```

### Confirmation Prompt Patterns

```go
// Standard confirmation (Enter = yes)
func confirm(prompt string) bool {
    fmt.Printf("%s [Y/n]: ", prompt)
    scanner := bufio.NewScanner(os.Stdin)
    scanner.Scan()
    input := strings.TrimSpace(strings.ToLower(scanner.Text()))
    return input == "" || input == "y" || input == "yes"
}

// Escalated confirmation for recreate (PLAN-08: must type 'yes')
func confirmExplicit(prompt string) bool {
    fmt.Print(prompt)
    scanner := bufio.NewScanner(os.Stdin)
    scanner.Scan()
    return strings.TrimSpace(scanner.Text()) == "yes"
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Full resource replacement on update | PATCH with partial body (Graph API standard) | Stable since Graph v1.0 | Can send only changed fields, though cactl sends full body for safety |
| Single version file for all policies | Per-resource independent semver | Terraform popularized this | Enables granular version tracking; each policy evolves independently |
| Text-based unified diff | Structured field-level comparison | Modern IaC tools (Terraform, Pulumi) | Meaningful diffs: shows field paths, not raw text changes |
| Manual GUID lookup | Batch resolution via `/$batch` | Graph API batch since 2020 | 20x fewer API calls for display name resolution |
| No plan-time validation | Configurable safety rules | Best practice from Terraform Sentinel, OPA | Catches dangerous changes before they reach production |

**Deprecated/outdated:**
- Text diff of JSON files: Produces noisy output (whitespace, key order). Use structural comparison.
- Sequential GUID resolution: Each call adds ~200ms latency. Use batch.

## Graph API Details for Phase 3

### Endpoints Used in Phase 3 (extending Phase 2)

| Operation | Method | Path | Request | Response |
|-----------|--------|------|---------|----------|
| List all policies | GET | `/identity/conditionalAccess/policies` | None | `200 OK` + `{ "value": [...] }` |
| Get single policy | GET | `/identity/conditionalAccess/policies/{id}` | None | `200 OK` + policy JSON |
| Create policy | POST | `/identity/conditionalAccess/policies` | Full policy JSON | `201 Created` + policy JSON with `id` |
| Update policy | PATCH | `/identity/conditionalAccess/policies/{id}` | Policy JSON (partial or full) | `204 No Content` |
| Delete policy | DELETE | `/identity/conditionalAccess/policies/{id}` | None | `204 No Content` |
| List named locations | GET | `/identity/conditionalAccess/namedLocations` | None | `200 OK` + `{ "value": [...] }` |
| Batch requests | POST | `/$batch` | `{ "requests": [...] }` (max 20) | `200 OK` + `{ "responses": [...] }` |

### Required Permissions (Phase 3 write operations)

| Permission Type | Required Permissions |
|-----------------|---------------------|
| Delegated (work/school) | `Policy.Read.All` AND `Policy.ReadWrite.ConditionalAccess` |
| Application | `Policy.Read.All` AND `Policy.ReadWrite.ConditionalAccess` |
| Display name resolution | `User.Read.All`, `Group.Read.All`, `Directory.Read.All` (for batch resolution) |

### Entra Roles (least privileged)

- **Conditional Access Administrator** -- can read/write CA policies
- **Security Administrator** -- can read/write CA policies
- Additional: **Global Reader** for read-only operations (plan without apply)

### Important: CA Policy Soft Delete

Deleted CA policies enter a soft-deleted state for 30 days. The recreate flow should be aware that the old policy ID may still exist in soft-deleted state. Creating a new policy will get a new ID regardless.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CLI-02 | `cactl plan` to see reconciliation diff | Pattern 1 (reconcile engine) + Pattern 2 (field diff) + Pattern 3 (colored output). Plan command orchestrates: load backend, fetch live, reconcile, render. |
| CLI-03 | `cactl apply` to deploy with confirmation | Pattern 4 (apply flow). Reuses plan engine, adds confirmation prompts, executes Graph API writes. |
| PLAN-01 | Plan compares backend JSON against live via Graph API | Pattern 1: Reconcile function takes backend (Git refs) and live (Graph API) maps. Both normalized via same function. |
| PLAN-02 | Plan shows sigils: + (create), ~ (update), -/+ (recreate), ? (untracked) | Pattern 3: Sigil map in diff renderer. Each ActionType maps to symbol + color. |
| PLAN-03 | Plan shows semver bump suggestion per policy | Pattern 5: DetermineBump analyzes field diffs against configurable major/minor field triggers. |
| PLAN-04 | Plan summary line with counts | Pattern 3: renderSummary counts each ActionType. "Plan: N to create, N to update, N to recreate, N untracked." |
| PLAN-05 | Apply presents plan diff and requests confirmation | Pattern 4: Apply reuses plan rendering, then prompts via bufio.Scanner before executing. |
| PLAN-06 | Apply --auto-approve skips confirmation (required in --ci) | Pattern 4: Boolean flag check. In CI mode, auto-approve must be set or apply errors. |
| PLAN-07 | Apply --dry-run generates plan and runs validation but no writes | Pattern 4: After rendering plan and running validations, exit before Graph API write calls. |
| PLAN-08 | Recreate actions require typing 'yes' | Pattern 4: confirmExplicit function requires exact "yes" text, not just Enter. |
| PLAN-09 | Apply on unchanged policy set produces no changes | Pattern 1 truth table: when backend == live (after normalization), action is Noop. Empty actionable list means exit 0 with "No changes." |
| PLAN-10 | Full idempotency truth table | Pattern 1 truth table: Create, Update, Noop, Recreate (ghost), Untracked. Covers all state combinations. |
| SEMV-01 | Every policy versioned independently with MAJOR.MINOR.PATCH | Pattern 5: Version stored per-entry in state manifest. `golang.org/x/mod/semver` for parsing. |
| SEMV-02 | MAJOR bump for scope expansion (configurable) | Pattern 5: DetermineBump with majorFields from config. Default includes user/group include/exclude lists and state field. |
| SEMV-03 | MINOR bump for conditions/controls changes (configurable) | Pattern 5: DetermineBump with minorFields from config. Default includes conditions, grantControls, sessionControls. |
| SEMV-04 | PATCH for state-only or cosmetic changes | Pattern 5: DetermineBump falls through to BumpPatch when no field matches major or minor triggers. |
| SEMV-05 | User can override suggested bump at apply time | Apply flow: prompt "Suggested bump: MINOR. Override? [MAJOR/minor/patch]:" before executing. |
| SEMV-06 | MAJOR bumps display explicit warning | Pattern 3 + 5: When rendering a MAJOR bump action, emit warning with red color: "WARNING: MAJOR version bump -- scope expansion detected." |
| DISP-01 | Terraform-style colored diffs with sigils | Pattern 3: Full diff renderer with ANSI colors, sigils, field-level changes, indented sub-diffs. |
| DISP-02 | --output json with stable schema (schema_version) | Pattern 9: PlanOutput struct with SchemaVersion field. JSON renderer serializes this instead of human-readable output. |
| DISP-03 | Named locations resolved to display names | Pattern 6: Resolver fetches `/identity/conditionalAccess/namedLocations` and caches id->displayName. Diff renderer shows "Office Network (guid)" instead of raw GUID. |
| DISP-04 | Groups and users resolved to display names | Pattern 6: Resolver uses `/$batch` to resolve `/groups/{id}` and `/users/{id}` with `$select=id,displayName`. |
| DISP-05 | `cactl status` shows per-policy version tree | Status command reads manifest, lists all tracked policies with version, last_deployed timestamp, deployed_by identity. |
| VALID-01 | Break-glass account exclusion validated at plan time | Pattern 7: checkBreakGlass validates configured break-glass GUIDs are in excludeUsers for each policy. Warning severity (non-blocking). |
| VALID-02 | Policy JSON validated against schema from init | Pattern 7: checkSchema validates backend JSON against .cactl/schema.json fetched during init. Error severity (blocking). |
| VALID-03 | Detect conflicting conditions (include AND exclude same group) | Pattern 7: checkConflictingConditions cross-checks include/exclude lists for users, groups, apps, locations. Error severity. |
| VALID-04 | Detect empty include lists (policy applies to no one) | Pattern 7: checkEmptyIncludes verifies at least one of includeUsers, includeGroups, includeRoles is non-empty. Warning severity. |
| VALID-05 | Detect overly broad policies (block all users) | Pattern 7: checkOverlyBroad detects "All" in includeUsers with no exclusions on enabled policies. Warning severity. |
</phase_requirements>

## Open Questions

1. **Should apply execute actions sequentially or in parallel?**
   - What we know: Graph API supports concurrent requests, but CA policy changes can have dependencies (e.g., one policy's conditions reference groups modified by another).
   - What's unclear: Whether parallel execution is safe for CA policy mutations.
   - Recommendation: Execute sequentially in v1 (MTNT-03 already specifies sequential for multi-tenant). Parallel execution is a v2 optimization. Sequential is safer and simpler to reason about for error handling.

2. **How to handle apply failure midway through multiple policy updates?**
   - What we know: Each Graph API call is independent. If policy 3 of 5 fails, policies 1-2 are already applied.
   - What's unclear: Should cactl attempt rollback of applied changes, or report partial success?
   - Recommendation: Report partial success with clear error output: "Applied 2 of 5 policies. Failed on: ca003-policy (HTTP 403: insufficient permissions). Remaining 2 policies not applied." Do NOT attempt rollback -- it adds complexity and its own failure modes. Let the user re-run after fixing the issue.

3. **Should `cactl status` be part of Phase 3 or Phase 4?**
   - What we know: DISP-05 is listed in Phase 3 requirements. CLI-07 (`cactl status`) is in Phase 4.
   - What's unclear: The status command shows version tree, timestamp, and deployer identity which overlaps with Phase 4's drift/rollback status needs.
   - Recommendation: Implement the basic `cactl status` in Phase 3 (reads manifest, shows per-policy version/timestamp/deployer). Phase 4 extends it with sync status (drift detection). This satisfies DISP-05 without pulling in drift logic.

4. **PATCH with full body vs. only changed fields?**
   - What we know: Graph API PATCH supports partial updates. Sending only changed fields is more efficient.
   - What's unclear: Whether sending the full normalized body causes issues (e.g., read-only fields rejected by PATCH).
   - Recommendation: Send the full normalized body (minus server-managed fields already stripped by normalize). This ensures desired state == live state after apply. The normalization pipeline already strips read-only fields, so PATCH should accept the full body. Test with real Graph API to verify.

5. **How to handle the `golang.org/x/mod/semver` `v` prefix requirement?**
   - What we know: `golang.org/x/mod/semver` requires version strings to start with `v` (e.g., `v1.0.0`). Manifest stores versions as `1.0.0` (no prefix).
   - What's unclear: Whether to store with `v` prefix or add/strip it at the boundary.
   - Recommendation: Store without `v` prefix in manifest and tags (matches user expectation for policy versions). Add `v` prefix when calling semver library functions, strip it on output. Two small helper functions: `toSemver("1.0.0") -> "v1.0.0"` and `fromSemver("v1.0.0") -> "1.0.0"`.

## Sources

### Primary (HIGH confidence)
- [Microsoft Learn: Update conditionalAccessPolicy (v1.0)](https://learn.microsoft.com/en-us/graph/api/conditionalaccesspolicy-update?view=graph-rest-1.0) -- PATCH endpoint, permissions, 204 response
- [Microsoft Learn: Create conditionalAccessPolicy (v1.0)](https://learn.microsoft.com/en-us/graph/api/conditionalaccessroot-post-policies?view=graph-rest-1.0) -- POST endpoint, permissions, 201 response with id
- [Microsoft Learn: Delete conditionalAccessPolicy (v1.0)](https://learn.microsoft.com/en-us/graph/api/conditionalaccesspolicy-delete?view=graph-rest-1.0) -- DELETE endpoint, permissions, 204 response, soft-delete behavior
- [Microsoft Learn: List namedLocations (v1.0)](https://learn.microsoft.com/en-us/graph/api/conditionalaccessroot-list-namedlocations?view=graph-rest-1.0) -- Named location types, response format
- [Microsoft Learn: JSON batching](https://learn.microsoft.com/en-us/graph/json-batching) -- Batch request format, 20-request limit, response ordering
- [Microsoft Learn: Emergency access accounts](https://learn.microsoft.com/en-us/entra/identity/role-based-access-control/security-emergency-access) -- Break-glass account best practices, CA policy exclusion guidance
- [golang.org/x/mod/semver](https://pkg.go.dev/golang.org/x/mod/semver) -- API: Compare, IsValid, Major, MajorMinor; requires `v` prefix
- [encoding/json](https://pkg.go.dev/encoding/json) -- MarshalIndent, map key sorting, Unmarshal to interface{}

### Secondary (MEDIUM confidence)
- [Masterminds/semver/v3](https://pkg.go.dev/github.com/Masterminds/semver/v3) -- Evaluated but rejected (too feature-rich for this use case)
- [wI2L/jsondiff](https://pkg.go.dev/github.com/wI2L/jsondiff) -- Evaluated for RFC 6902 patches (overkill for human-readable diffs)
- [fatih/color](https://pkg.go.dev/github.com/fatih/color) -- Evaluated for terminal colors (rejected: codebase uses raw ANSI codes)
- [hashicorp/terraform-json](https://pkg.go.dev/github.com/hashicorp/terraform-json) -- Referenced for plan output schema design inspiration

### Tertiary (LOW confidence)
- [yudai/gojsondiff](https://github.com/yudai/gojsondiff) -- Unmaintained since 2017; evaluated and rejected
- [CA policy What-If tool discussion](https://blog.admindroid.com/graph-based-what-if-tool-conditional-access/) -- Context on CA policy assessment (out of scope for cactl v1)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- only one new dependency (golang.org/x/mod/semver); everything else is stdlib or already in go.mod
- Architecture (reconcile engine): HIGH -- truth table is well-defined; JSON map comparison is straightforward stdlib pattern
- Architecture (Graph API writes): HIGH -- POST/PATCH/DELETE endpoints verified against official Microsoft Learn docs; permissions confirmed
- Architecture (display name resolution): HIGH -- batch endpoint documented; 20-request limit confirmed
- Architecture (semver): HIGH -- golang.org/x/mod/semver API verified; v-prefix requirement noted and handled
- Pitfalls: HIGH -- normalization drift and PATCH response handling verified against official API behavior docs
- Validation rules: MEDIUM -- break-glass pattern is well-documented but VALID-02 schema validation depends on schema quality from init (which is currently embedded/minimal)

**Research date:** 2026-03-04
**Valid until:** 2026-04-04 (stable APIs, 30-day validity)
