---
phase: 08-policy-test-engine
verified: 2026-03-15T12:00:00Z
status: passed
score: 14/14 must-haves verified
---

# Phase 8: Policy Test Engine Verification Report

**Phase Goal:** User can write declarative YAML test scenarios that assert CA policy behavior and run `cactl test` to verify policies produce expected outcomes locally without Azure API calls
**Verified:** 2026-03-15
**Status:** passed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | YAML test spec files can be parsed into typed Go structs | VERIFIED | `ParseTestFile`/`ParseTestBytes` in parse.go unmarshal YAML into `TestSpec`. 9 table-driven tests pass (valid, invalid, edge cases) |
| 2 | Each CA condition type (users, apps, platforms, locations, risk, client app types) can be matched against a sign-in context | VERIFIED | 7 matchers in match.go: `matchUsers`, `matchApplications`, `matchClientAppTypes`, `matchPlatforms`, `matchLocations`, `matchSignInRiskLevels`, `matchUserRiskLevels`. 40+ table-driven tests pass |
| 3 | Include/exclude semantics are correct: include first, then exclude overrides | VERIFIED | `matchStringList` and `matchUsers` implement include-then-exclude. Tests confirm: "user included but excluded overrides", "group excluded overrides user include", "exclude overrides include" |
| 4 | The "All" keyword matches everything in include lists | VERIFIED | Tested across users ("All"), applications ("All"), platforms ("all"), locations ("All"), clientAppTypes ("all"). Tests confirm match-all behavior |
| 5 | Missing condition blocks default to "matches all" | VERIFIED | Each matcher returns `true` when its condition block is absent (nil map). Tests confirm: "no user conditions matches all", "no clientAppTypes matches all", etc. |
| 6 | A single policy can be evaluated against a sign-in context yielding block, grant, or notApplicable | VERIFIED | `EvaluatePolicy` in evaluate.go. 9 test cases cover block, grant, notApplicable, session controls, missing conditions |
| 7 | Disabled policies always return notApplicable | VERIFIED | `EvaluatePolicy` checks `state == "disabled"` first, returns `ResultNotApplicable`. Test: "disabled policy returns notApplicable" |
| 8 | enabledForReportingButNotEnforced policies are evaluated as enabled | VERIFIED | `EvaluatePolicy` treats both "enabled" and "enabledForReportingButNotEnforced" identically. Test: "report-only policy evaluated as if enabled" |
| 9 | Block always wins when combining multiple policies | VERIFIED | `EvaluateAll` sets `hasBlock = true` if any policy blocks. Test: "block wins over grant" with 2 matching policies |
| 10 | Grant controls from all matching policies are collected | VERIFIED | `EvaluateAll` appends controls from all matching grant policies. Test: "two grant policies combine controls" yields ["mfa", "compliantDevice"] |
| 11 | Grant operator (AND/OR) is preserved per policy decision | VERIFIED | `PolicyDecision.Operator` set from `grantControls.operator`. Test: "enabled grant policy" verifies `wantOp: "AND"` |
| 12 | User can run "cactl test" and see pass/fail results for each scenario | VERIFIED | `cmd/test.go` registers testCmd on rootCmd. End-to-end test (`TestTestCmd_WithTestFiles`) creates temp policy + test YAML, runs command, sees "PASS" output |
| 13 | Exit code 0 when all tests pass, 1 when any test fails, 2 on fatal errors | VERIFIED | `runTest` returns nil (exit 0), `ExitChanges` (exit 1), or `ExitFatalError` (exit 2) based on `Summary()` results |
| 14 | Human output shows PASS/FAIL per scenario with details on failure; JSON output provides machine-readable results | VERIFIED | `RenderHuman` outputs "PASS"/"FAIL" with color, expected/got on failure, matching policies. `RenderJSON` outputs valid JSON with summary counts and scenario details. 6 report tests pass |

**Score:** 14/14 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/testengine/types.go` | Types: TestSpec, Scenario, SignInContext, etc. (min 60 lines) | VERIFIED | 111 lines. All types present: TestSpec, Scenario, SignInContext, ExpectedOutcome, PolicyWithSlug, EvalResult, PolicyDecision, CombinedDecision, ScenarioResult, TestReport, FileResult |
| `internal/testengine/parse.go` | YAML parsing and validation (ParseTestFile, ParseTestBytes) | VERIFIED | 66 lines. Both exports present. Validates name, scenarios, expect.result |
| `internal/testengine/parse_test.go` | Table-driven parser tests | VERIFIED | 9 test cases covering valid/invalid/edge cases. All pass |
| `internal/testengine/match.go` | Condition matchers for all CA types | VERIFIED | 381 lines. All 7 matchers plus matchStringList, matchStringListWithKeywords helpers |
| `internal/testengine/match_test.go` | Table-driven matcher tests (min 100 lines) | VERIFIED | 539 lines. 40+ test cases across 7 matcher test suites plus matchStringList tests |
| `internal/testengine/evaluate.go` | Single policy evaluation and multi-policy combination | VERIFIED | 150 lines. EvaluatePolicy and EvaluateAll exported. extractSessionControls helper |
| `internal/testengine/evaluate_test.go` | TDD tests covering all evaluation rules (min 150 lines) | VERIFIED | 467 lines. 9 EvaluatePolicy tests + 1 session controls test + 5 EvaluateAll tests |
| `internal/testengine/runner.go` | Test orchestration (RunTests, RunTestFile) | VERIFIED | 165 lines. LoadPolicies, RunTests, RunTestFile, filterPolicies, evaluateScenario, containsAllControls |
| `internal/testengine/runner_test.go` | Runner tests with in-memory policies | VERIFIED | 204 lines. Tests for all-pass, one-fail, policy filter, grant controls, filter logic, containsAllControls |
| `internal/testengine/report.go` | Human and JSON output formatting (RenderHuman, RenderJSON) | VERIFIED | 144 lines. Both renderers plus Summary helper. Color support consistent with project conventions |
| `internal/testengine/report_test.go` | Report output tests | VERIFIED | 151 lines. Tests for human format, color, JSON validity, JSON details, summary computation, error scenarios |
| `cmd/test.go` | cactl test cobra command (testCmd) | VERIFIED | 117 lines. Registered on rootCmd. Handles tenant, test file discovery, policy dir, output format, exit codes |
| `cmd/test_test.go` | Command-level tests (min 30 lines) | VERIFIED | 84 lines. 3 tests: registration, missing tenant, full end-to-end with temp files |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `parse.go` | `types.go` | `yaml.Unmarshal into TestSpec struct` | WIRED | Line 29: `yaml.Unmarshal(data, &spec)` where spec is `TestSpec` |
| `match.go` | `types.go` | `SignInContext as input to matchers` | WIRED | All 7 matchers accept `*SignInContext` parameter |
| `evaluate.go` | `match.go` | `condition matchers called from EvaluatePolicy` | WIRED | Lines 31-58: calls matchUsers, matchApplications, matchClientAppTypes, matchPlatforms, matchLocations, matchSignInRiskLevels, matchUserRiskLevels |
| `evaluate.go` | `types.go` | `PolicyDecision, CombinedDecision, SignInContext types` | WIRED | All three types used as parameters and return values |
| `cmd/test.go` | `runner.go` | `testengine.RunTests call` | WIRED | Line 78: `testengine.RunTests(testPaths, policyDir)` |
| `runner.go` | `evaluate.go` | `EvaluateAll for each scenario` | WIRED | Line 128: `EvaluateAll(policies, ctx)` in evaluateScenario |
| `cmd/test.go` | `cmd/root.go` | `rootCmd.AddCommand(testCmd)` | WIRED | Line 29: `rootCmd.AddCommand(testCmd)` in init() |

### Requirements Coverage

No explicit requirement IDs mapped to this phase in ROADMAP.md or plans. Phase goal is fully covered by the observable truths above.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/testengine/runner.go` | 147 | `// TODO: SessionControls comparison deferred` | Info | SessionControls comparison not yet implemented in scenario evaluation. Grant controls and result comparison work correctly. This is a known deferral documented in the plan |

No blockers. No stubs. No placeholder implementations. No console.log-only functions. No empty returns.

### Human Verification Required

### 1. End-to-end CLI test with real policy files

**Test:** Run `cactl test tests/<tenant>/block-legacy.yaml --tenant <id>` with actual policy JSON files from the policies directory
**Expected:** PASS/FAIL output with correct policy matching against real CA policy JSON structure
**Why human:** Verifying that real-world Entra CA policy JSON structures (with nested conditions, GUIDs, etc.) are correctly parsed and evaluated requires actual policy files

### 2. Color output rendering

**Test:** Run `cactl test` in a terminal that supports ANSI colors
**Expected:** PASS appears in green, FAIL appears in red
**Why human:** Terminal color rendering cannot be verified programmatically

### Gaps Summary

No gaps found. All 14 observable truths verified. All 13 artifacts exist, are substantive, and are properly wired. All 7 key links confirmed. The project builds cleanly, all tests pass (testengine: all pass, cmd: 3/3 pass), go vet reports no issues, and no imports of internal/graph exist in the testengine package (confirming local-only evaluation).

The single TODO for SessionControls comparison is a documented, planned deferral that does not block the phase goal -- users can write and run YAML test scenarios that verify block/grant/notApplicable outcomes and grant controls.

---

_Verified: 2026-03-15_
_Verifier: Claude (gsd-verifier)_
