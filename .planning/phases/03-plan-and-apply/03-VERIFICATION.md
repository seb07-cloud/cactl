---
phase: 03-plan-and-apply
verified: 2026-03-05T00:00:00Z
status: passed
score: 14/14 must-haves verified
re_verification: false
---

# Phase 3: Plan and Apply — Verification Report

**Phase Goal:** User can preview and deploy CA policy changes with colored diffs, semantic versioning, safety validations, and display name resolution
**Verified:** 2026-03-05
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                                           | Status     | Evidence                                                                                                               |
|----|-----------------------------------------------------------------------------------------------------------------|------------|------------------------------------------------------------------------------------------------------------------------|
| 1  | Reconcile correctly classifies create, update, noop, recreate, and untracked actions per the idempotency table | VERIFIED   | `internal/reconcile/engine.go` implements all 5 rows; 7 table-driven tests pass (`TestReconcile`)                     |
| 2  | Field-level diff detects added, removed, and changed fields with dot-separated paths                           | VERIFIED   | `internal/reconcile/diff.go` ComputeDiff with recursive descent; 7 tests pass (`TestComputeDiff`)                     |
| 3  | Running reconcile on identical backend and live state produces zero actions (idempotent noop)                  | VERIFIED   | Engine skips emit when `len(diffs) == 0`; noop test case passes                                                       |
| 4  | Semver bump level determined by field triggers with prefix matching (MAJOR/MINOR/PATCH)                        | VERIFIED   | `internal/semver/version.go` DetermineBump with matchesAny prefix logic; 8 tests pass                                 |
| 5  | BumpVersion correctly increments MAJOR, MINOR, or PATCH and resets lower components                           | VERIFIED   | `internal/semver/version.go` BumpVersion; 6 tests pass including reset cases                                          |
| 6  | Plan-time validations catch break-glass gaps, conflicting conditions, empty includes, overly broad policies    | VERIFIED   | `internal/validate/validate.go` 4 rules implemented; VALID-02 intentionally stubbed with TODO                         |
| 7  | Graph client can create, update, and delete CA policies via Graph API                                          | VERIFIED   | `internal/graph/policies.go` CreatePolicy/UpdatePolicy/DeletePolicy; 4 httptest tests pass                            |
| 8  | Resolver resolves GUIDs to display names using batched Graph API requests                                      | VERIFIED   | `internal/resolve/resolver.go` with chunked batches of 20; 6 tests pass                                               |
| 9  | Resolver gracefully degrades: returns GUID when resolution fails (404, deleted objects)                        | VERIFIED   | 404 cached as `{id} (deleted)`, errors cache raw ID; tests `TestResolveAll_404Deleted` passes                         |
| 10 | User can run `cactl plan` and see terraform-style colored diff with sigils and summary counts                  | VERIFIED   | `internal/output/diff.go` RenderPlan; `cmd/plan.go` wired end-to-end; all output tests pass                           |
| 11 | Plan output shows semver bump suggestion per changed policy with MAJOR bumps displaying explicit warnings      | VERIFIED   | SEMV-06 warning rendered in RenderPlan at BumpLevel==MAJOR; test `TestRenderPlan_MajorBumpWarning` passes             |
| 12 | Plan supports --output json with stable schema including schema_version                                        | VERIFIED   | `pkg/types/plan.go` PlanOutput with SchemaVersion=1; RenderPlanJSON tested; json schema verified                      |
| 13 | Plan exits code 0 when no changes, code 1 when changes detected, code 3 for validation errors                 | VERIFIED   | `cmd/plan.go` returns ExitChanges(1) or ExitValidationError(3) or nil(0); exitcodes.go matches contract               |
| 14 | User can run `cactl apply` with confirmation, --auto-approve, --dry-run, recreate escalation, state updates    | VERIFIED   | `cmd/apply.go` full pipeline; per-action WriteManifest + CreateVersionTag; all apply tests pass                       |

**Score:** 14/14 truths verified

---

### Required Artifacts

| Artifact                          | Provides                                                  | Exists | Substantive | Wired      | Status    |
|-----------------------------------|-----------------------------------------------------------|--------|-------------|------------|-----------|
| `internal/reconcile/action.go`    | ActionType enum + PolicyAction struct                     | YES    | YES (53 lines, all 5 action types + PolicyAction) | Used by engine.go, output/diff.go, cmd/plan.go, cmd/apply.go | VERIFIED |
| `internal/reconcile/diff.go`      | Recursive field-level JSON diff computation               | YES    | YES (103 lines, ComputeDiff + DiffType + FieldDiff) | Called by engine.go ComputeDiff | VERIFIED |
| `internal/reconcile/engine.go`    | Reconciliation engine comparing backend vs live state     | YES    | YES (100 lines, full truth table) | Called by cmd/plan.go and cmd/apply.go | VERIFIED |
| `internal/semver/version.go`      | DetermineBump and BumpVersion functions                   | YES    | YES (107 lines, full implementation) | Called from cmd/plan.go and cmd/apply.go via manual FieldDiff conversion | VERIFIED |
| `internal/semver/config.go`       | SemverConfig with default major/minor field triggers      | YES    | YES (30 lines, 7 major fields, 3 minor fields) | DefaultSemverConfig() called in cmd/plan.go and cmd/apply.go | VERIFIED |
| `internal/validate/validate.go`   | Plan-time validation rules for policy safety              | YES    | YES (270 lines, 4 rules + ValidatePlan) | Called from cmd/plan.go and cmd/apply.go | VERIFIED |
| `internal/graph/policies.go`      | CreatePolicy, UpdatePolicy, DeletePolicy on Graph Client  | YES    | YES (169 lines, 3 write methods) | Called from cmd/apply.go | VERIFIED |
| `internal/graph/batch.go`         | ExecuteBatch method and batch types                       | YES    | YES (64 lines, batch types + ExecuteBatch) | Used by resolve/resolver.go via BatchClient interface | VERIFIED |
| `internal/resolve/resolver.go`    | Display name resolver with caching and batch resolution   | YES    | YES (212 lines, full impl with GUID detection) | Called from cmd/plan.go and cmd/apply.go | VERIFIED |
| `internal/output/diff.go`         | Terraform-style diff renderer with sigils, colors, summary | YES   | YES (275 lines, RenderPlan + RenderPlanJSON) | Called from cmd/plan.go and cmd/apply.go | VERIFIED |
| `pkg/types/plan.go`               | PlanOutput, ActionOutput, DiffOutput, SummaryOutput types | YES    | YES (39 lines, all 4 types) | Used by output/diff.go RenderPlanJSON | VERIFIED |
| `cmd/plan.go`                     | cactl plan command registered on root                     | YES    | YES (256 lines, full pipeline) | Registered via init() on rootCmd | VERIFIED |
| `cmd/apply.go`                    | cactl apply command with confirmation flow                | YES    | YES (518 lines, full pipeline + helpers) | Registered via init() on rootCmd | VERIFIED |

---

### Key Link Verification

| From                              | To                                 | Via                                      | Pattern Found                        | Status    |
|-----------------------------------|------------------------------------|------------------------------------------|--------------------------------------|-----------|
| `internal/reconcile/engine.go`    | `internal/state/manifest.go`       | manifest.Policies map for lookup         | `manifest.Policies` (lines 28, 74)   | WIRED     |
| `internal/reconcile/engine.go`    | `internal/reconcile/diff.go`       | ComputeDiff call for update detection    | `ComputeDiff` (line 54)              | WIRED     |
| `internal/semver/version.go`      | `internal/reconcile/diff.go`       | FieldDiff.Path for field trigger matching | Local FieldDiff type (design choice, manual conversion at call site in cmd/plan.go line 151-159) | WIRED (via adapter) |
| `internal/validate/validate.go`   | `internal/reconcile/action.go`     | PolicyAction.BackendJSON for checks      | Local mirror PolicyAction (design choice, manual conversion in cmd/plan.go line 189-196) | WIRED (via adapter) |
| `internal/resolve/resolver.go`    | `internal/graph/batch.go`          | ExecuteBatch for batched GUID resolution | `ExecuteBatch` (lines 20, 106)       | WIRED     |
| `internal/graph/policies.go`      | `internal/graph/client.go`         | c.do() for authenticated HTTP requests   | `c.do(` (lines 36, 76, 110, 139, 156) | WIRED   |
| `cmd/plan.go`                     | `internal/reconcile/engine.go`     | Reconcile() to generate actions          | `reconcile.Reconcile` (line 135)     | WIRED     |
| `cmd/plan.go`                     | `internal/semver/version.go`       | DetermineBump for version suggestions    | `semver.DetermineBump` (line 161)    | WIRED     |
| `cmd/plan.go`                     | `internal/validate/validate.go`    | ValidatePlan for safety checks           | `validate.ValidatePlan` (line 197)   | WIRED     |
| `cmd/plan.go`                     | `internal/resolve/resolver.go`     | ResolveAll + DisplayName for GUIDs       | `resolve.NewResolver` (line 208)     | WIRED     |
| `internal/output/diff.go`         | `internal/reconcile/action.go`     | PolicyAction and ActionType for rendering | `reconcile.ActionType`, `reconcile.PolicyAction` (line 21, 39) | WIRED |
| `cmd/apply.go`                    | `internal/reconcile/engine.go`     | Reconcile() to generate plan             | `reconcile.Reconcile` (line 144)     | WIRED     |
| `cmd/apply.go`                    | `internal/graph/policies.go`       | CreatePolicy, UpdatePolicy for writes    | `graphClient.CreatePolicy` (line 295, 394), `graphClient.UpdatePolicy` (line 343) | WIRED |
| `cmd/apply.go`                    | `internal/state/backend.go`        | WritePolicy + CreateVersionTag           | `backend.WritePolicy` (lines 307, 358, 415), `backend.CreateVersionTag` (lines 315, 366, 423) | WIRED |
| `cmd/apply.go`                    | `internal/state/manifest.go`       | WriteManifest for manifest updates       | `state.WriteManifest` (lines 332, 383, 440) | WIRED |

**Design Note on Adapter Pattern:** The plan specified `internal/semver/version.go` should import `reconcile.FieldDiff` and `internal/validate/validate.go` should import `reconcile.PolicyAction`. Both packages instead define local mirror types to avoid circular dependencies. Manual conversion happens at the call site in `cmd/plan.go` and `cmd/apply.go`. This is functionally equivalent and architecturally sound — not a gap.

---

### Requirements Coverage

| Requirement | Plan  | Description                                                              | Status    | Evidence                                                          |
|-------------|-------|--------------------------------------------------------------------------|-----------|-------------------------------------------------------------------|
| CLI-02      | 03-04 | `cactl plan` shows reconciliation diff                                   | SATISFIED | cmd/plan.go full pipeline; planCmd registered on root             |
| CLI-03      | 03-05 | `cactl apply` deploys with confirmation                                  | SATISFIED | cmd/apply.go with confirm/confirmExplicit helpers                 |
| PLAN-01     | 03-01/04 | Plan compares backend JSON against live tenant via Graph API          | SATISFIED | Reconcile() + graphClient.ListPolicies() wired in cmd/plan.go     |
| PLAN-02     | 03-01 | Sigils: + (create), ~ (update), -/+ (recreate), ? (untracked)           | SATISFIED | output/diff.go sigil() function maps all 4 action types           |
| PLAN-03     | 03-04 | Semver bump suggestion per policy                                        | SATISFIED | DetermineBump called per ActionUpdate in cmd/plan.go; BumpLevel set on action |
| PLAN-04     | 03-04 | Summary line: N to create, N to update, N to recreate, N untracked      | SATISFIED | RenderPlan prints "Plan: N to create..." line                     |
| PLAN-05     | 03-05 | Apply presents plan diff and requests confirmation                       | SATISFIED | confirm() called before Phase E in cmd/apply.go                   |
| PLAN-06     | 03-05 | --auto-approve skips confirmation                                        | SATISFIED | `--auto-approve` flag wired; CI mode also enforces                |
| PLAN-07     | 03-05 | --dry-run generates plan but makes no writes                             | SATISFIED | dryRun check at Phase C; returns before execute loop              |
| PLAN-08     | 03-05 | Recreate escalates to explicit 'yes' confirmation                        | SATISFIED | confirmExplicit() called when hasAction(actionable, ActionRecreate) |
| PLAN-09     | 03-01 | Apply is idempotent: no changes on unchanged policy set                  | SATISFIED | Noop case emits no action; apply prints "No changes. Infrastructure is up-to-date." |
| PLAN-10     | 03-01 | Full idempotency truth table: create, update, noop, recreate, untracked  | SATISFIED | 7 table-driven engine tests cover all cases                       |
| SEMV-01     | 03-02/05 | Per-policy MAJOR.MINOR.PATCH versioning                               | SATISFIED | BumpVersion applied per action; version stored in manifest        |
| SEMV-02     | 03-02 | MAJOR triggered by scope expansion (configurable major_fields)           | SATISFIED | DefaultSemverConfig major fields include includeUsers, includeGroups, state, etc. |
| SEMV-03     | 03-02 | MINOR triggered by conditions/controls changes                           | SATISFIED | DefaultSemverConfig minor fields include conditions, grantControls, sessionControls |
| SEMV-04     | 03-02 | PATCH for all other fields                                               | SATISFIED | DetermineBump default return is BumpPatch                         |
| SEMV-06     | 03-04 | MAJOR bumps display explicit warning in plan output                      | SATISFIED | RenderPlan checks `a.BumpLevel == "MAJOR"` and prints warning     |
| DISP-01     | 03-04 | Human-readable terraform-style colored diffs                             | SATISFIED | RenderPlan with ANSI sigils, colors, field-level diffs            |
| DISP-02     | 03-04/05 | --output json with stable schema (schema_version)                    | SATISFIED | RenderPlanJSON sets SchemaVersion=1; PlanOutput schema            |
| DISP-03     | 03-03 | Named locations resolved to display names                                | SATISFIED | CollectRefs maps "includeLocations"/"excludeLocations" to "namedLocation" type |
| DISP-04     | 03-03 | Groups and users resolved to display names                               | SATISFIED | CollectRefs maps includeUsers/excludeUsers to "user", groups to "group" |
| VALID-01    | 03-02 | Break-glass account exclusion validated at plan time                     | SATISFIED | checkBreakGlass() warns when account not in excludeUsers           |
| VALID-03    | 03-02 | Detect conflicting conditions (include and exclude same group)           | SATISFIED | checkConflictingConditions() checks users, groups, applications    |
| VALID-04    | 03-02 | Detect empty include lists                                               | SATISFIED | checkEmptyIncludes() warns when all user include lists empty       |
| VALID-05    | 03-02 | Detect overly broad policies (All users with no exclusions)              | SATISFIED | checkOverlyBroad() warns when includeUsers="All" with no exclusions and enabled |

**Note on VALID-02:** Not in the requirement list for this phase (correctly excluded). Plan 03-02 intentionally stubs schema validation with a TODO comment referencing VALID-02 as deferred. This matches REQUIREMENTS.md which marks VALID-02 as Pending for Phase 3.

---

### Anti-Patterns Found

| File                              | Line | Pattern                                    | Severity | Impact                                                     |
|-----------------------------------|------|--------------------------------------------|----------|------------------------------------------------------------|
| `internal/validate/validate.go`   | 75   | `// TODO (VALID-02): Add checkSchema...`   | Info     | Intentional deferral; VALID-02 is out-of-scope for Phase 3 |
| `cmd/apply.go`                    | 42   | `// TODO: SEMV-05 --bump-level flag...`    | Info     | Intentional deferral; SEMV-05 is out-of-scope for Phase 3  |

Both TODOs are intentional, explicitly documented in plan files as deferred to future work. Neither blocks goal achievement.

---

### Human Verification Required

#### 1. Terminal Color Output

**Test:** Run `cactl plan --tenant <id>` against a real or mock tenant with at least one changed policy.
**Expected:** Green `+` for create, yellow `~` for update, red `-/+` for recreate, cyan `?` for untracked; MAJOR bump shows yellow/red warning line.
**Why human:** ANSI escape codes are present in code and tested with buffers, but visual rendering in an actual terminal cannot be verified programmatically.

#### 2. Confirmation Prompt UX

**Test:** Run `cactl apply --tenant <id>` against a tenant with changes. Answer the standard confirmation with Enter (should proceed), then run again and answer "n" (should cancel).
**Expected:** Enter proceeds, "n" cancels with "Apply cancelled." message.
**Why human:** Interactive stdin behavior cannot be fully tested end-to-end without a real terminal; unit tests cover the helper functions in isolation.

#### 3. Recreate Escalation Prompt

**Test:** Create a ghost scenario (policy in manifest but deleted from tenant), run `cactl apply`, and verify the escalated "Type 'yes'" prompt appears and rejects anything other than "yes".
**Expected:** Only "yes" (case-insensitive) proceeds; "y", "", Enter all cancel with "Apply cancelled."
**Why human:** Requires live Graph API or complex mock setup to produce an ActionRecreate in the apply path.

#### 4. Display Name Resolution in Plan Output

**Test:** Run `cactl plan --tenant <id>` where a policy contains GUIDs in includeUsers/includeGroups. Verify plan output shows human-readable names in parentheses next to GUIDs.
**Expected:** Diff lines show `"guid-uuid" (Jane Doe)` format when resolver successfully resolves the GUID.
**Why human:** Requires live Graph API with real objects; unit tests use mock batch client.

---

### Gaps Summary

No gaps found. All 14 observable truths are verified, all artifacts exist at substantive depth and are correctly wired, all 25 declared requirement IDs are satisfied, and the build compiles cleanly with no vet issues. The two TODO comments represent intentional, explicitly planned deferrals (VALID-02, SEMV-05) that are correctly out of scope for Phase 3.

The only structural deviation from plans is the adapter pattern used for `semver.FieldDiff` and `validate.PolicyAction` (local mirror types instead of importing from reconcile to avoid circular dependencies). The manual conversion at call sites in `cmd/plan.go` and `cmd/apply.go` is correct and complete — this is better architecture, not a gap.

---

_Verified: 2026-03-05_
_Verifier: Claude (gsd-verifier)_
