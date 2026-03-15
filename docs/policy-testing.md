# Policy Testing

`cactl test` evaluates sign-in scenarios against your local CA policy JSON files without making any Microsoft Graph API calls. Tests run instantly, work offline, and catch policy logic errors before you deploy.

## Overview

A test spec is a YAML file that describes one or more scenarios. Each scenario defines a simulated sign-in context (who is signing in, from where, on what device, at what risk level) and the expected outcome (block, grant, or notApplicable, with optional required controls).

The test engine loads your policy JSON files from disk, evaluates every enabled policy against the sign-in context, combines the results using the same logic Entra uses (AND across conditions, block wins across policies), and compares the combined outcome to your expectation.

```
YAML test spec ──┐
                  ├─ Test Engine ─── PASS / FAIL
Policy JSON files ┘
```

## Directory layout

```
your-repo/
  policies/<tenant-id>/
    require-mfa-all-users.json
    block-legacy-auth.json
    require-compliant-device.json
  tests/<tenant-id>/
    mfa-scenarios.yaml
    legacy-auth.yaml
    device-compliance.yaml
```

Test files live in `tests/<tenant-id>/` and policy files in `policies/<tenant-id>/`. When you run `cactl test` without arguments, it auto-discovers all `*.yaml` files in `tests/<tenant-id>/` using the tenant from your config chain (flag, env var, or `az` CLI context).

## Running tests

```bash
# Auto-discover tests for the configured tenant
cactl test

# Run specific test files
cactl test tests/a793bcba-.../mfa-scenarios.yaml tests/a793bcba-.../legacy-auth.yaml

# Explicit tenant
cactl test --tenant YOUR_TENANT_ID

# JSON output (for CI parsing)
cactl test --output json
```

## Writing test specs

### Minimal example

```yaml
name: MFA enforcement
description: Verify MFA is required for all cloud apps
scenarios:
  - name: regular user from untrusted location
    context:
      user: "e1d2c3b4-..."
      application: "All"
      clientAppType: browser
      location: "untrusted"
    expect:
      result: grant
      controls:
        - mfa
```

### Full spec structure

```yaml
name: <string, required>         # Test spec name
description: <string, optional>  # Human-readable description
policies:                        # Optional: filter to specific policy slugs
  - require-mfa-all-users        #   Empty list = evaluate all policies
  - block-legacy-auth

scenarios:
  - name: <string, required>     # Scenario name (shown in output)
    context:                     # Sign-in context to simulate
      user: <string>             # User GUID, "All", "any", or "guest"
      groups:                    # Group GUIDs the user belongs to
        - "group-guid-1"
        - "group-guid-2"
      roles:                     # Directory role GUIDs the user holds
        - "role-guid-1"
      application: <string>     # App GUID or "All"
      clientAppType: <string>   # browser | mobileAppsAndDesktopClients | exchangeActiveSync | other | all
      platform: <string>        # android | iOS | windows | macOS | linux | windowsPhone
      location: <string>        # Location GUID, "trusted", "untrusted", or "All"
      signInRiskLevel: <string> # none | low | medium | high
      userRiskLevel: <string>   # none | low | medium | high
    expect:
      result: <string, required> # block | grant | notApplicable
      controls:                  # Optional: expected grant controls
        - mfa
        - compliantDevice
      sessionControls:           # Optional: expected session controls
        signInFrequency: ...
```

All `context` fields are optional. Omitted fields match permissively (a missing `platform` means the scenario applies regardless of platform, just like Entra does when no platform condition is set on a policy).

## Context field reference

| Field | Values | Behavior when omitted |
|---|---|---|
| `user` | Any GUID, `"All"`, `"guest"` | Matches only if policy has no user conditions |
| `groups` | List of group GUIDs | User is not in any groups |
| `roles` | List of directory role GUIDs | User holds no roles |
| `application` | App GUID or `"All"` | Matches only if policy has no app conditions |
| `clientAppType` | `browser`, `mobileAppsAndDesktopClients`, `exchangeActiveSync`, `other`, `all` | Matches any client app type |
| `platform` | `android`, `iOS`, `windows`, `macOS`, `linux`, `windowsPhone` | Matches any platform |
| `location` | Location GUID, `"trusted"`, `"untrusted"`, `"All"` | Matches any location |
| `signInRiskLevel` | `none`, `low`, `medium`, `high` | Matches any risk level |
| `userRiskLevel` | `none`, `low`, `medium`, `high` | Matches any risk level |

## Expected outcomes

| Result | Meaning |
|---|---|
| `block` | At least one matching policy contains a `block` grant control |
| `grant` | One or more policies match and none block; access is granted (possibly with controls) |
| `notApplicable` | No enabled policy's conditions match the sign-in context |

### Controls

When `result` is `grant`, you can optionally assert which grant controls are required:

```yaml
expect:
  result: grant
  controls:
    - mfa
    - compliantDevice
```

The assertion checks that all listed controls are present in the combined result. It does not fail if additional controls are present (subset matching).

## Evaluation rules

The test engine mirrors how Entra evaluates CA policies:

1. **Disabled policies are skipped.** Only `enabled` and `enabledForReportingButNotEnforced` policies are evaluated.

2. **All conditions use AND logic.** A policy matches only if the sign-in context satisfies every condition dimension: users AND applications AND client app types AND platforms AND locations AND sign-in risk AND user risk.

3. **Within a condition, include/exclude follows standard CA logic.** Include is evaluated first (OR across the include list). If the context matches any include entry, exclusions are checked. Any exclusion match removes the context from scope.

4. **Special include values are supported.** `"All"` in `includeUsers` or `includeApplications` matches everything. `"GuestsOrExternalUsers"` matches when the context user is `"guest"`.

5. **Block wins across policies.** When multiple policies match, if any of them contain a `block` grant control, the combined result is `block`, regardless of what other policies grant.

6. **Grant controls are collected across all matching policies.** If policy A requires `mfa` and policy B requires `compliantDevice`, the combined result includes both.

7. **Absent conditions match permissively.** If a policy has no `platforms` block, it matches all platforms. This matches Entra's behavior.

## Practical examples

### Testing break-glass account exclusion

Verify that your emergency access accounts are never blocked:

```yaml
name: break-glass exclusion
scenarios:
  - name: break-glass account bypasses all policies
    context:
      user: "break-glass-guid-here"
      application: "All"
      clientAppType: browser
      platform: windows
    expect:
      result: notApplicable
```

### Testing legacy auth blocking

```yaml
name: legacy authentication
scenarios:
  - name: ActiveSync is blocked for all users
    context:
      user: "All"
      application: "All"
      clientAppType: exchangeActiveSync
    expect:
      result: block

  - name: browser access is not blocked by legacy auth policy
    context:
      user: "All"
      application: "All"
      clientAppType: browser
    expect:
      result: grant
      controls:
        - mfa
```

### Testing location-based access

```yaml
name: location restrictions
scenarios:
  - name: untrusted location requires MFA
    context:
      user: "e1d2c3b4-..."
      groups:
        - "sales-team-guid"
      application: "All"
      clientAppType: browser
      location: "untrusted"
    expect:
      result: grant
      controls:
        - mfa

  - name: trusted location does not require MFA
    context:
      user: "e1d2c3b4-..."
      groups:
        - "sales-team-guid"
      application: "All"
      clientAppType: browser
      location: "trusted"
    expect:
      result: grant
```

### Testing risk-based policies

```yaml
name: risk-based access control
scenarios:
  - name: high sign-in risk is blocked
    context:
      user: "All"
      application: "All"
      clientAppType: browser
      signInRiskLevel: high
    expect:
      result: block

  - name: medium user risk requires password change
    context:
      user: "All"
      application: "All"
      clientAppType: browser
      userRiskLevel: medium
    expect:
      result: grant
      controls:
        - passwordChange
        - mfa
```

### Scoping tests to specific policies

Use the `policies` field to limit which policies the engine evaluates. This is useful when you want to test a single policy in isolation:

```yaml
name: MFA policy isolation test
policies:
  - require-mfa-all-users
scenarios:
  - name: guest user requires MFA
    context:
      user: "guest"
      application: "All"
      clientAppType: browser
    expect:
      result: grant
      controls:
        - mfa
```

## Output

### Human output (default)

```
=== cactl test ===

  tests/a793bcba-.../mfa-scenarios.yaml
    PASS  regular user from untrusted location (2 policies matched, result: grant)
    PASS  break-glass account bypasses MFA (0 policies matched, result: notApplicable)
    FAIL  guest user on mobile
          expected: block
          got:      grant [mfa]
          matching policies: require-mfa-all-users

  Results: 2 passed, 1 failed, 0 errors
```

### JSON output

```bash
cactl test --output json
```

```json
{
  "summary": {
    "total": 3,
    "passed": 2,
    "failed": 1,
    "errors": 0
  },
  "files": [
    {
      "file": "tests/a793bcba-.../mfa-scenarios.yaml",
      "scenarios": [
        {
          "name": "regular user from untrusted location",
          "passed": true,
          "expectedResult": "grant",
          "gotResult": "grant",
          "gotControls": ["mfa"],
          "matchingPolicies": ["require-mfa-all-users", "require-compliant-device"]
        }
      ]
    }
  ]
}
```

## Exit codes

| Exit code | Meaning |
|---|---|
| `0` | All tests passed |
| `1` | One or more tests failed |
| `2` | Fatal error (missing policy directory, invalid YAML, etc.) |

## CI integration

Run tests in your pipeline before plan/apply to catch logic errors early:

```yaml
- name: Test policy logic
  run: cactl test --ci --tenant ${{ vars.AZURE_TENANT_ID }}

- name: Plan changes
  run: cactl plan --ci --tenant ${{ vars.AZURE_TENANT_ID }}

- name: Apply changes
  if: github.ref == 'refs/heads/main'
  run: cactl apply --ci --auto-approve --tenant ${{ vars.AZURE_TENANT_ID }}
```

Tests run entirely locally -- no Azure credentials are needed. You can run the test step before the `azure/login` action.

## Tips

- **Start with break-glass tests.** The single most important test is verifying that your emergency access accounts are excluded from every blocking policy. Write this test first.
- **One file per concern.** Group scenarios by what they test (MFA enforcement, legacy auth blocking, device compliance) rather than by policy name. A single scenario often exercises multiple policies.
- **Use the `policies` filter for isolation.** When debugging a specific policy, scope the test to just that slug to avoid interference from other policies.
- **Combine with `cactl plan`.** Run `cactl test` to verify logic, then `cactl plan` to verify the diff. Both are read-only and safe to run at any time.
- **Test report-only policies too.** Policies in `enabledForReportingButNotEnforced` state are evaluated by the test engine just like enabled policies. This lets you validate behavior before enabling enforcement.
