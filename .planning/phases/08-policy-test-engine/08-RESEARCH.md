# Phase 8: Policy Test Engine - Research

**Researched:** 2026-03-15
**Domain:** Conditional Access policy evaluation engine, YAML test framework, CLI test runner
**Confidence:** HIGH

## Summary

Phase 8 introduces a local policy evaluation engine that simulates Microsoft Entra Conditional Access policy evaluation without Azure API calls. Users write declarative YAML test scenarios describing sign-in contexts (who, what app, from where, on what platform, at what risk level) and assert expected outcomes (block, grant with MFA, session controls). The `cactl test` command loads policy JSON files from disk, evaluates them against each scenario, and reports pass/fail results.

The core challenge is faithfully implementing the CA policy evaluation model: all assignments are AND-combined, block always wins over grant, grant controls can be AND or OR, and all applicable policies are evaluated simultaneously (not in priority order). The evaluation engine must support the full condition surface area: users/groups/roles, applications, locations, platforms, client app types, risk levels, and device filters.

The codebase already has strong foundations: `internal/validate` demonstrates policy JSON traversal with `getNestedValue`/`getStringSlice` helpers, `internal/normalize` handles JSON canonicalization, and the `cmd/desired.go` pattern shows how to load policies from `policies/<tenantID>/`. The test engine builds on these patterns without requiring Graph API connectivity.

**Primary recommendation:** Build a self-contained `internal/testengine` package with a pure-function evaluation engine, YAML test spec parser, and test runner. Do NOT use OPA/Rego -- the CA policy model is well-defined and bounded; a custom evaluator is simpler than teaching users Rego. Wire to CLI via `cmd/test.go` using cobra.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| gopkg.in/yaml.v3 | 3.0.1 | YAML test file parsing | Already in go.mod, used by schema package |
| github.com/stretchr/testify | 1.11.1 | Unit test assertions | Already in go.mod, used across all test files |
| github.com/spf13/cobra | 1.10.2 | CLI command wiring | Already in go.mod, all commands use cobra |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| encoding/json | stdlib | Policy JSON loading | Loading policy files from disk |
| path/filepath | stdlib | Test file discovery | Globbing YAML test files |
| fmt/strings | stdlib | Result formatting | Test output rendering |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom evaluator | OPA/Rego | OPA adds a DSL users must learn; CA policies are a bounded domain with clear evaluation rules; custom is simpler and more maintainable |
| Custom YAML format | gdt framework | gdt is HTTP-focused; our test scenarios are policy evaluation, not HTTP requests; custom YAML is more natural |
| Test file per policy | Test suite files | Suite files can test multiple policies together, matching real CA evaluation (all policies evaluated simultaneously) |

**Installation:**
No new dependencies required. All libraries already in go.mod.

## Architecture Patterns

### Recommended Project Structure
```
internal/testengine/
    types.go          # TestSpec, Scenario, SignInContext, ExpectedOutcome
    parse.go          # YAML parsing and validation
    parse_test.go     # Parser unit tests
    evaluate.go       # Core evaluation engine (pure functions)
    evaluate_test.go  # Evaluation engine unit tests
    match.go          # Condition matching helpers (users, apps, locations, etc.)
    match_test.go     # Matcher unit tests
    runner.go         # Test runner: load policies, run scenarios, collect results
    runner_test.go    # Runner integration tests
    report.go         # Result formatting (human + JSON output)
    report_test.go    # Report formatting tests
cmd/
    test.go           # `cactl test` cobra command
    test_test.go      # Command-level tests
```

### Pattern 1: Test Specification YAML Format
**What:** Declarative YAML files that describe sign-in scenarios and expected policy outcomes
**When to use:** Every test file follows this structure

```yaml
# tests/block-legacy-auth.yaml
name: "Block legacy authentication"
description: "Verify CAP001 blocks legacy auth protocols"

# Optional: limit which policies to evaluate (default: all in tenant dir)
policies:
  - "cap001-global-identityprotection-allapps-anyplatform-block-legacy-authentication"

scenarios:
  - name: "Legacy client should be blocked"
    context:
      user: "any"                    # "any" | specific GUID | "All"
      groups: []                     # group GUIDs the user belongs to
      roles: []                      # directory role GUIDs the user holds
      application: "All"             # app GUID or "All"
      clientAppType: "exchangeActiveSync"  # browser | mobileAppsAndDesktopClients | exchangeActiveSync | other | all
      platform: "windows"           # android | iOS | windows | macOS | linux | windowsPhone
      location: "trusted"           # GUID | "trusted" | "untrusted" | "All"
      signInRiskLevel: "none"       # none | low | medium | high
      userRiskLevel: "none"         # none | low | medium | high
    expect:
      result: "block"               # block | grant | notApplicable
      # Optional: for grant results, assert required controls
      # controls:
      #   - "mfa"
      # sessionControls:
      #   signInFrequency: { value: 9, type: "hours" }

  - name: "Modern browser client should not be blocked by this policy"
    context:
      user: "any"
      application: "All"
      clientAppType: "browser"
    expect:
      result: "notApplicable"       # Policy conditions don't match
```

### Pattern 2: Evaluation Engine (Pure Functions)
**What:** Stateless evaluation of a single policy against a sign-in context
**When to use:** Core engine logic, fully testable without side effects

```go
// evaluate.go

// EvalResult represents the outcome of evaluating a single policy.
type EvalResult int

const (
    ResultNotApplicable EvalResult = iota // Conditions don't match
    ResultBlock                            // Policy blocks access
    ResultGrant                            // Policy grants with controls
)

// PolicyDecision is the result of evaluating a single policy.
type PolicyDecision struct {
    PolicySlug     string
    Result         EvalResult
    GrantControls  []string   // e.g., ["mfa", "compliantDevice"]
    Operator       string     // "AND" or "OR"
    SessionControls map[string]interface{}
}

// EvaluatePolicy evaluates a single CA policy against a sign-in context.
// Returns NotApplicable if conditions don't match, Block or Grant if they do.
// This is a pure function with no side effects.
func EvaluatePolicy(policy map[string]interface{}, ctx *SignInContext) PolicyDecision {
    // 1. Check if policy is enabled (skip disabled policies)
    // 2. Check all conditions (AND logic):
    //    - Users/Groups/Roles match
    //    - Applications match
    //    - Client app types match
    //    - Platforms match (if specified)
    //    - Locations match (if specified)
    //    - Risk levels match (if specified)
    // 3. If all conditions match, determine grant vs block from grantControls
    // 4. Return decision
}

// EvaluateAll evaluates all policies and combines results.
// Implements CA evaluation semantics: block wins, all grants must be satisfied.
func EvaluateAll(policies []PolicyWithSlug, ctx *SignInContext) CombinedDecision {
    // 1. Evaluate each policy
    // 2. If ANY matching policy blocks -> result is Block
    // 3. Collect all grant controls from matching policies
    // 4. Return combined decision
}
```

### Pattern 3: Condition Matching with Include/Exclude Semantics
**What:** Each condition type follows the same include/exclude pattern
**When to use:** Every condition matcher uses this pattern

```go
// match.go

// matchUsers checks if the sign-in context user matches the policy's user conditions.
// Logic: user must be in includeUsers/includeGroups/includeRoles
//        AND must NOT be in excludeUsers/excludeGroups/excludeRoles
func matchUsers(conditions map[string]interface{}, ctx *SignInContext) bool {
    users := getNestedMap(conditions, "users")
    if users == nil {
        return true // No user conditions = matches all
    }

    // Check includes (OR logic: any include list match is sufficient)
    included := matchIncludes(users, ctx)
    if !included {
        return false
    }

    // Check excludes (any exclude match = not matched)
    excluded := matchExcludes(users, ctx)
    return !excluded
}
```

### Pattern 4: Test Runner with Result Collection
**What:** Load test files, load policies, run scenarios, collect results
**When to use:** The runner orchestrates parsing and evaluation

```go
// runner.go

// RunTests discovers and executes test files against policies.
func RunTests(testPaths []string, policyDir string) (*TestReport, error) {
    // 1. Load all policies from policyDir
    // 2. Parse each test file
    // 3. For each scenario in each test:
    //    a. Select applicable policies (all or filtered by spec.Policies)
    //    b. Build SignInContext from scenario.Context
    //    c. EvaluateAll policies against context
    //    d. Compare CombinedDecision against scenario.Expect
    //    e. Record pass/fail with details
    // 4. Return TestReport
}
```

### Anti-Patterns to Avoid
- **Calling Graph API from test engine:** The entire point is local-only evaluation. Never import `internal/graph` from `internal/testengine`.
- **Implementing full Azure AD group resolution:** Test scenarios specify group membership directly in the context; don't try to resolve nested groups or dynamic groups.
- **Supporting policy state "enabledForReportingButNotEnforced" differently:** For testing purposes, treat report-only mode same as enabled -- the user wants to test what the policy WOULD do.
- **Building a policy priority/ordering system:** CA policies have NO priority. All applicable policies are evaluated simultaneously. Block always wins.
- **Parsing displayName to infer behavior:** Use the actual JSON fields (conditions, grantControls, sessionControls), never the displayName string.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| YAML parsing | Custom tokenizer | gopkg.in/yaml.v3 with struct tags | Already in go.mod, battle-tested |
| JSON traversal | New dot-path walker | Reuse pattern from `internal/validate` (getNestedValue, getStringSlice) | Proven pattern, consistent codebase |
| Test file discovery | Custom recursive walker | filepath.Glob + filepath.WalkDir | Standard library handles edge cases |
| CLI output formatting | Raw fmt.Println | Pattern from `internal/output` (human/JSON modes, color support) | Consistent with existing commands |
| Exit codes | Ad-hoc integers | Reuse `types.ExitError` with existing codes | ExitSuccess=0 for all pass, ExitChanges=1 for failures, ExitFatalError=2 for errors |

**Key insight:** The codebase already has all the primitives needed. The `internal/validate` package's `getNestedValue` and `getStringSlice` functions are exactly what the condition matchers need. The `cmd/desired.go` ReadDesiredPolicies function shows how to load policies from disk. The test engine is a new assembly of existing patterns, not a from-scratch build.

## Common Pitfalls

### Pitfall 1: Forgetting "All" is a Special Keyword
**What goes wrong:** Treating "All" as a literal GUID instead of a wildcard
**Why it happens:** The include/exclude lists contain both GUIDs and the keyword "All"
**How to avoid:** Every condition matcher must check for "All" as the first step. If includeUsers contains "All", all users match (subject to excludes). If includeApplications contains "All", all apps match.
**Warning signs:** Tests pass for specific GUIDs but fail for "All" keyword

### Pitfall 2: AND vs OR Confusion in Grant Controls
**What goes wrong:** Treating grant controls as always-AND when the policy specifies OR operator
**Why it happens:** The `grantControls.operator` field can be "AND" or "OR", changing evaluation semantics
**How to avoid:** Always read the operator field. When asserting grant controls in test expectations, the runner must account for the operator.
**Warning signs:** Policies with OR operator produce unexpected test results

### Pitfall 3: Disabled Policies Should Not Match
**What goes wrong:** Evaluating disabled policies as if they were active
**Why it happens:** The `state` field defaults to checking conditions before checking state
**How to avoid:** First check in EvaluatePolicy: if state == "disabled", return NotApplicable immediately. Treat "enabledForReportingButNotEnforced" as enabled for test purposes (user wants to test what it would do).
**Warning signs:** All policies appear to match in test results

### Pitfall 4: Missing Condition Means "All Match"
**What goes wrong:** Treating a missing condition block as "none match"
**Why it happens:** If a policy has no `conditions.locations` block, it applies to ALL locations, not no locations
**How to avoid:** Each condition matcher must default to "matches" when the condition is absent from the policy JSON
**Warning signs:** Policies without location/platform conditions report NotApplicable when they should match

### Pitfall 5: Exclude Logic Runs After Include
**What goes wrong:** Checking excludes independently of includes
**Why it happens:** The evaluation order matters: first check if the user is included, THEN check if excluded
**How to avoid:** Pattern: `included := matchIncludes(); if !included { return false }; excluded := matchExcludes(); return !excluded`
**Warning signs:** Users in exclude lists still being matched

### Pitfall 6: Block Always Wins Across All Policies
**What goes wrong:** Reporting "grant" when one policy grants and another blocks
**Why it happens:** Evaluating policies individually without combining results
**How to avoid:** EvaluateAll must combine: if ANY policy blocks, the combined result is Block, regardless of other grant policies
**Warning signs:** Scenarios expect "block" but get "grant" because the grant policy was evaluated last

### Pitfall 7: Test File Discovery Path Confusion
**What goes wrong:** Test files not found because of incorrect path resolution
**Why it happens:** Running `cactl test` from different directories, or test files referencing policies by wrong path
**How to avoid:** Use the same tenant-relative policy directory pattern as `cmd/desired.go`. Test files should reference policy slugs, not file paths.
**Warning signs:** "no test files found" errors or "policy not found" errors

## Code Examples

### Loading and Evaluating Policies Locally (reusing existing patterns)
```go
// Reuse the existing pattern from cmd/desired.go
policies, err := cmd.ReadDesiredPolicies(tenantID)
// policies is map[string]reconcile.BackendPolicy where each .Data is map[string]interface{}

// Convert to testengine format
var policyList []testengine.PolicyWithSlug
for slug, bp := range policies {
    policyList = append(policyList, testengine.PolicyWithSlug{
        Slug: slug,
        Data: bp.Data,
    })
}
```

### YAML Test Spec Parsing
```go
type TestSpec struct {
    Name        string     `yaml:"name"`
    Description string     `yaml:"description,omitempty"`
    Policies    []string   `yaml:"policies,omitempty"` // Filter to specific policy slugs
    Scenarios   []Scenario `yaml:"scenarios"`
}

type Scenario struct {
    Name    string          `yaml:"name"`
    Context SignInContext    `yaml:"context"`
    Expect  ExpectedOutcome `yaml:"expect"`
}

type SignInContext struct {
    User            string   `yaml:"user"`            // GUID, "any", "All"
    Groups          []string `yaml:"groups,omitempty"` // Group GUIDs user belongs to
    Roles           []string `yaml:"roles,omitempty"`  // Role GUIDs user holds
    Application     string   `yaml:"application"`     // App GUID or "All"
    ClientAppType   string   `yaml:"clientAppType"`   // browser, mobileAppsAndDesktopClients, exchangeActiveSync, other, all
    Platform        string   `yaml:"platform,omitempty"`        // android, iOS, windows, macOS, linux, windowsPhone
    Location        string   `yaml:"location,omitempty"`        // GUID, "trusted", "untrusted", "All"
    SignInRiskLevel string   `yaml:"signInRiskLevel,omitempty"` // none, low, medium, high
    UserRiskLevel   string   `yaml:"userRiskLevel,omitempty"`   // none, low, medium, high
}

type ExpectedOutcome struct {
    Result          string   `yaml:"result"`                    // block, grant, notApplicable
    Controls        []string `yaml:"controls,omitempty"`        // Required grant controls
    SessionControls map[string]interface{} `yaml:"sessionControls,omitempty"`
}
```

### Condition Matching (include/exclude with "All" handling)
```go
// matchStringList implements the standard CA include/exclude pattern.
// Returns true if the value matches the include list and is not in the exclude list.
// "All" in the include list matches everything.
func matchStringList(includeList, excludeList []string, values []string) bool {
    // Check includes
    included := false
    for _, inc := range includeList {
        if inc == "All" {
            included = true
            break
        }
        for _, v := range values {
            if inc == v {
                included = true
                break
            }
        }
        if included {
            break
        }
    }
    if !included {
        return false
    }

    // Check excludes
    excludeSet := make(map[string]bool, len(excludeList))
    for _, ex := range excludeList {
        excludeSet[ex] = true
    }
    for _, v := range values {
        if excludeSet[v] {
            return false
        }
    }
    return true
}
```

### Test Runner Output Format
```go
// Human-readable output (matching cactl's existing output style):
//
// === cactl test ===
//
// tests/block-legacy-auth.yaml
//   PASS  Legacy client should be blocked (1 policy matched, result: block)
//   PASS  Modern browser should not be blocked (0 policies matched, result: notApplicable)
//
// tests/admin-mfa.yaml
//   PASS  Admin user requires MFA (1 policy matched, result: grant [mfa])
//   FAIL  Guest user should not require MFA
//         expected: notApplicable
//         got:      grant [mfa]
//         matching policies: cap100-admin-identityprotection-...
//
// Results: 3 passed, 1 failed, 0 errors
// Exit code: 1

// JSON output (for CI integration):
// {
//   "summary": { "total": 4, "passed": 3, "failed": 1, "errors": 0 },
//   "files": [
//     {
//       "file": "tests/block-legacy-auth.yaml",
//       "scenarios": [
//         { "name": "Legacy client should be blocked", "status": "pass", ... }
//       ]
//     }
//   ]
// }
```

## CA Policy Evaluation Model Reference

This section documents the Microsoft Entra CA evaluation model that the engine must faithfully implement.

### Evaluation Rules
1. **All applicable policies evaluated simultaneously** -- no priority order
2. **All assignments within a policy are AND-combined** -- user AND app AND conditions must all match
3. **Block always wins** -- if any matching policy blocks, access is denied
4. **Disabled policies are skipped** -- only `enabled` and `enabledForReportingButNotEnforced` policies are evaluated
5. **Grant controls combine across policies** -- all grant requirements from all matching policies must be satisfied
6. **Grant operator is per-policy** -- `grantControls.operator` ("AND" or "OR") controls whether all or any controls suffice within that policy

### Condition Surface Area
| Condition | Policy JSON Path | Values | Default (if absent) |
|-----------|-----------------|--------|---------------------|
| Users | `conditions.users.includeUsers/excludeUsers` | GUIDs, "All", "GuestsOrExternalUsers" | Must match |
| Groups | `conditions.users.includeGroups/excludeGroups` | Group GUIDs | No group filter |
| Roles | `conditions.users.includeRoles/excludeRoles` | Role GUIDs | No role filter |
| Applications | `conditions.applications.includeApplications/excludeApplications` | App GUIDs, "All", "Office365" | Must match |
| Client App Types | `conditions.clientAppTypes` | browser, mobileAppsAndDesktopClients, exchangeActiveSync, other, all | All types |
| Platforms | `conditions.platforms.includePlatforms/excludePlatforms` | android, iOS, windows, macOS, linux, windowsPhone, all | All platforms |
| Locations | `conditions.locations.includeLocations/excludeLocations` | Location GUIDs, "All", "AllTrusted" | All locations |
| Sign-in Risk | `conditions.signInRiskLevels` | none, low, medium, high | All levels (empty = no filter) |
| User Risk | `conditions.userRiskLevels` | none, low, medium, high | All levels (empty = no filter) |
| Service Principal Risk | `conditions.servicePrincipalRiskLevels` | low, medium, high | No filter |

### Grant Controls
| Control | builtInControls Value |
|---------|----------------------|
| Block | "block" |
| MFA | "mfa" |
| Compliant Device | "compliantDevice" |
| Hybrid Joined | "domainJoinedDevice" |
| Approved Client App | "approvedApplication" |
| App Protection Policy | "compliantApplication" |
| Password Change | "passwordChange" |

### Session Controls
| Control | JSON Path |
|---------|-----------|
| Sign-in Frequency | `sessionControls.signInFrequency` |
| Persistent Browser | `sessionControls.persistentBrowser` |
| App Enforced Restrictions | `sessionControls.applicationEnforcedRestrictions` |
| Cloud App Security | `sessionControls.cloudAppSecurity` |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Azure "What If" tool only | Local evaluation + What If | This phase | Users can test policies offline in CI |
| Manual policy review | Automated declarative tests | This phase | Catches policy conflicts before deployment |
| OPA/Rego for all policy engines | Domain-specific evaluators for bounded domains | Ongoing trend | Simpler for users, no DSL learning curve |

## Open Questions

1. **How to handle "GuestsOrExternalUsers" in user matching?**
   - What we know: This is a valid value for includeUsers/excludeUsers
   - What's unclear: How to represent this in test context (is the test user a guest?)
   - Recommendation: Add optional `userType: "member" | "guest"` field to SignInContext, default to "member"

2. **How to handle location GUIDs vs named location semantics?**
   - What we know: Policies reference location GUIDs, but test writers may want to say "trusted" or "untrusted"
   - What's unclear: Whether to support both GUID and keyword in test context
   - Recommendation: Support keywords ("trusted", "untrusted", "All") plus raw GUIDs. For GUID matching, do literal comparison against policy include/exclude lists.

3. **Should test engine support testing multiple policies as a group (combined evaluation)?**
   - What we know: CA evaluation combines ALL matching policies. A single-policy test is useful but incomplete.
   - What's unclear: Whether users need combined evaluation testing in v1
   - Recommendation: Support both modes. Default evaluates all policies in the tenant dir. `policies:` filter in YAML narrows scope. Combined evaluation is the more valuable mode.

4. **Where should test YAML files live?**
   - What we know: Policies live in `policies/<tenantID>/`
   - What's unclear: Standard location for test files
   - Recommendation: `tests/<tenantID>/` directory parallel to `policies/`, discovered via glob `tests/**/*.yaml`. Also support explicit path arguments to `cactl test`.

## Sources

### Primary (HIGH confidence)
- Microsoft Learn: [Building Conditional Access policies](https://learn.microsoft.com/en-us/entra/identity/conditional-access/concept-conditional-access-policies) - Full evaluation model, condition types, grant/session controls
- Codebase: `internal/validate/validate.go` - Existing JSON traversal patterns (getNestedValue, getStringSlice)
- Codebase: `cmd/desired.go` - Policy loading from disk pattern
- Codebase: `internal/reconcile/engine.go` - BackendPolicy/LivePolicy types
- Codebase: `cmd/pipeline.go` - Command pipeline bootstrap pattern
- Codebase: `pkg/types/exitcodes.go` - Exit code conventions

### Secondary (MEDIUM confidence)
- Microsoft Learn: [Conditional Access What If tool](https://learn.microsoft.com/en-us/entra/identity/conditional-access/what-if-tool) - Reference for evaluation semantics
- [How CA policies are evaluated](https://www.cswrld.com/2026/02/how-conditional-access-policies-are-evaluated-in-microsoft-entra-id/) - Cross-verification of evaluation rules

### Tertiary (LOW confidence)
- gdt-dev/gdt framework - Considered but rejected; HTTP-focused, not applicable to policy evaluation domain

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - No new dependencies, all libraries already in go.mod
- Architecture: HIGH - Follows established codebase patterns, clear domain model from Microsoft docs
- Evaluation model: HIGH - Well-documented by Microsoft, cross-verified across multiple sources
- YAML test format: MEDIUM - Custom design informed by similar tools, but needs user feedback
- Pitfalls: HIGH - Derived from official CA evaluation documentation

**Research date:** 2026-03-15
**Valid until:** 2026-04-15 (CA evaluation model is stable; YAML format is our design)
