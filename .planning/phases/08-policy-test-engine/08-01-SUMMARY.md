---
phase: 08-policy-test-engine
plan: 01
subsystem: testing
tags: [yaml, conditional-access, policy-evaluation, testengine]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: "Project structure, Go module, cobra CLI"
provides:
  - "TestSpec YAML parsing with validation"
  - "SignInContext and ExpectedOutcome types for policy evaluation"
  - "Condition matchers for all CA condition types (users, apps, platforms, locations, risk, client app types)"
  - "EvalResult, PolicyDecision, CombinedDecision types for evaluation engine"
affects: [08-02, 08-03]

# Tech tracking
tech-stack:
  added: []
  patterns: ["include/exclude condition matching with All keyword", "table-driven matcher tests", "YAML test spec format"]

key-files:
  created:
    - "internal/testengine/types.go"
    - "internal/testengine/parse.go"
    - "internal/testengine/parse_test.go"
    - "internal/testengine/match.go"
    - "internal/testengine/match_test.go"
  modified: []

key-decisions:
  - "Copied getNestedValue/getStringSlice/splitPath helpers into testengine package (no import coupling with validate)"
  - "matchStringList shared helper for include/exclude with All keyword across all matchers"
  - "matchStringListWithKeywords extends base helper for location-specific AllTrusted keyword"
  - "GuestsOrExternalUsers matches when ctx.User == guest (simple keyword mapping)"
  - "Empty platform in context matches any platform condition (unspecified = any)"

patterns-established:
  - "Condition matcher pattern: accept conditions map + SignInContext, return bool"
  - "YAML test spec format: name, optional policies filter, scenarios with context and expected outcome"
  - "Include-first-then-exclude evaluation order for all CA conditions"

requirements-completed: []

# Metrics
duration: 3min
completed: 2026-03-15
---

# Phase 8 Plan 1: Types, Parser, and Condition Matchers Summary

**TestSpec YAML parser with validation and 7 condition matchers implementing CA include/exclude/All semantics with 49 table-driven tests**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-15T06:34:59Z
- **Completed:** 2026-03-15T06:38:12Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Complete type system for test engine: TestSpec, Scenario, SignInContext, ExpectedOutcome, PolicyWithSlug, EvalResult, PolicyDecision, CombinedDecision, ScenarioResult, TestReport
- YAML parser with validation: required fields (name, scenarios, expect.result), valid result values (block/grant/notApplicable), descriptive errors
- 7 condition matchers: users (with groups/roles/GuestsOrExternalUsers), applications, clientAppTypes, platforms, locations (with AllTrusted), signInRiskLevels, userRiskLevels
- All matchers implement correct include/exclude semantics with "All" keyword handling

## Task Commits

Each task was committed atomically:

1. **Task 1: Types and YAML parser** - `1575f2d` (feat)
2. **Task 2: Condition matchers with include/exclude semantics** - `f3f7444` (feat)

## Files Created/Modified
- `internal/testengine/types.go` - All types: TestSpec, Scenario, SignInContext, ExpectedOutcome, EvalResult, PolicyDecision, CombinedDecision, ScenarioResult, TestReport
- `internal/testengine/parse.go` - ParseTestFile/ParseTestBytes with YAML validation
- `internal/testengine/parse_test.go` - 9 table-driven parser tests (valid, invalid, edge cases)
- `internal/testengine/match.go` - 7 condition matchers + matchStringList helper + copied JSON traversal helpers
- `internal/testengine/match_test.go` - 40 table-driven matcher tests covering All keyword, exclude overrides, missing conditions, GuestsOrExternalUsers

## Decisions Made
- Copied getNestedValue/getStringSlice/splitPath into testengine package to avoid import coupling with validate package
- matchStringList as shared helper for standard include/exclude/All logic across matchers
- Separate matchStringListWithKeywords for location-specific AllTrusted keyword
- GuestsOrExternalUsers matched by checking ctx.User == "guest" (simple keyword-based approach)
- Empty platform in context matches any platform condition (unspecified = any platform)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Types ready for evaluate.go (Plan 02) to build EvaluatePolicy/EvaluateAll on top of matchers
- Condition matchers ready for the evaluation engine to call for each policy condition type
- YAML parser ready for the test runner (Plan 02) to load test files

---
*Phase: 08-policy-test-engine*
*Completed: 2026-03-15*
