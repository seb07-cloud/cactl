---
phase: 02-state-and-import
verified: 2026-03-04T21:30:00Z
status: passed
score: 15/15 must-haves verified
re_verification: false
---

# Phase 2: State and Import Verification Report

**Phase Goal:** User can import live CA policies into a Git-backed state store with full normalization and version tracking
**Verified:** 2026-03-04T21:30:00Z
**Status:** PASSED
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

All 15 truths verified across plans 01, 02, and 03.

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | Server-managed fields (id, createdDateTime, modifiedDateTime, templateId) are stripped | VERIFIED | `normalize.go:11-16` serverManagedFields slice; test `strip_server-managed_fields` PASS |
| 2  | Explicit null values are recursively removed from nested maps and arrays | VERIFIED | `removeNulls()` at normalize.go:71-88; test `remove_null_values_recursively` PASS |
| 3  | @odata.* metadata keys are recursively stripped at every nesting level | VERIFIED | `stripODataFields()` at normalize.go:51-68 uses `strings.Contains(k, "@odata.")`; test `strip_@odata_fields` PASS |
| 4  | Output JSON has alphabetically sorted keys, 2-space indent, and trailing newline | VERIFIED | `json.MarshalIndent(m, "", "  ")` + `append(out, '\n')` at normalize.go:40-47; test `pretty-print_with_2-space_indent` PASS |
| 5  | Display names are converted to kebab-case slugs | VERIFIED | `Slugify()` in slug.go using compiled regexes; all 8 test cases PASS |
| 6  | Empty arrays are preserved | VERIFIED | Test `preserve_empty_arrays` PASS; removeNulls only deletes nil, not empty arrays |
| 7  | Policy JSON blobs stored in Git object store via hash-object and retrievable via cat-file | VERIFIED | `hashObject()` + `catFile()` in backend.go; `TestWriteAndReadPolicy` PASS (real temp git repos) |
| 8  | Custom refs at refs/cactl/tenants/<tenant>/policies/<slug> with zero working tree footprint | VERIFIED | `policyRef()` format at backend.go:133; `TestWritePolicyCreatesRef` PASS |
| 9  | State manifest maps slugs to Entra Object IDs with all STATE-05 fields | VERIFIED | `Entry` struct in manifest.go:17-26 has all 8 fields; `TestEntryHasAllFields` PASS |
| 10 | Annotated tags at cactl/<tenant>/<slug>/<semver> with tagger identity and timestamp | VERIFIED | `CreateVersionTag()` uses `git tag -a` at backend.go:59-67; `TestCreateVersionTag` confirms type="tag" |
| 11 | Refspec is idempotent and skips when no remote origin | VERIFIED | `addRefspecIfMissing()` in refspec.go:38-53; `TestConfigureRefspecIdempotent` + `TestConfigureRefspecNoRemote` PASS |
| 12 | ListPolicies returns all tracked policy slugs for a tenant from refs | VERIFIED | `forEachRef()` in backend.go:103-130; `TestListPolicies` PASS |
| 13 | Graph client authenticates via azcore.TokenCredential and fetches CA policies with pagination | VERIFIED | `do()` calls `credential.GetToken()` at client.go:49; pagination loop in policies.go:34-68; 5 tests PASS |
| 14 | `cactl import --all/--policy/--force` imports through full pipeline | VERIFIED | import.go:47-211 wires graph->normalize->state->tag->manifest; binary compiles; help shows all flags |
| 15 | Interactive selection shows untracked policies with ? sigil; CI mode rejects interactive | VERIFIED | `interactiveSelect()` at import.go:229-285; `TestImportCIModeNoSelection` PASS; CI mode gives exit 3 |

**Score:** 15/15 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/normalize/normalize.go` | Normalize function: strip/null-remove/sort/pretty-print | VERIFIED | 89 lines, full implementation, exports `Normalize` with godoc |
| `internal/normalize/slug.go` | Slugify function: display name to kebab-case | VERIFIED | 20 lines, compiled regex singletons, exports `Slugify` |
| `internal/normalize/normalize_test.go` | Table-driven normalization tests | VERIFIED | 8 test cases including full Graph API pipeline test |
| `internal/normalize/slug_test.go` | Table-driven slug tests | VERIFIED | 8 test cases covering edge cases |
| `internal/state/backend.go` | GitBackend with WritePolicy, ReadPolicy, ListPolicies, CreateVersionTag | VERIFIED | 136 lines, all 4 exported methods plus 4 internal git plumbing helpers |
| `internal/state/manifest.go` | Manifest and Entry types with read/write to Git refs | VERIFIED | 82 lines, all STATE-05 fields present, ReadManifest/WriteManifest implemented |
| `internal/state/refspec.go` | ConfigureRefspec for .git/config push/pull setup | VERIFIED | 55 lines, idempotent fetch+push setup, graceful no-remote skip |
| `internal/state/backend_test.go` | Integration tests using temp git repos | VERIFIED | 9 tests against real `t.TempDir()` + `git init` repos |
| `internal/state/manifest_test.go` | Manifest read/write round-trip tests | VERIFIED | 4 tests including all-fields verification |
| `internal/state/refspec_test.go` | Refspec idempotency and no-remote tests | VERIFIED | 3 tests |
| `internal/graph/client.go` | Graph HTTP client with azcore token auth | VERIFIED | 66 lines, 30s timeout, Bearer header injection |
| `internal/graph/policies.go` | ListPolicies and GetPolicy with pagination | VERIFIED | 99 lines, @odata.nextLink loop, RawJSON preserved |
| `internal/graph/client_test.go` | Tests using httptest mock server | VERIFIED | 5 tests including pagination and auth header verification |
| `cmd/import.go` | cactl import command with --all, --policy, --force | VERIFIED | 300 lines, full pipeline orchestration, interactive selection |
| `cmd/import_test.go` | Import flag validation and registration tests | VERIFIED | 4 tests including bumpPatchVersion |

---

### Key Link Verification

| From | To | Via | Status | Evidence |
|------|----|-----|--------|---------|
| `internal/normalize/normalize.go` | `encoding/json` | `json.MarshalIndent` for sorted keys | WIRED | normalize.go:40 `json.MarshalIndent(m, "", "  ")` |
| `internal/normalize/slug.go` | `regexp` | Compiled regex for non-alphanumeric replacement | WIRED | slug.go:9-10 `regexp.MustCompile(...)` |
| `internal/state/backend.go` | git plumbing | `exec.Command("git", ...)` | WIRED | backend.go:21,61,71,83,93,104 — 6 distinct git plumbing calls |
| `internal/state/manifest.go` | `internal/state/backend.go` | Uses GitBackend read/write for manifest blob | WIRED | manifest.go:35,64 `func ReadManifest(backend *GitBackend...)` |
| `internal/state/refspec.go` | git config | `exec.Command("git", "config", ...)` | WIRED | refspec.go:16,39,47 — check and add git config entries |
| `cmd/import.go` | `internal/graph/client.go` | `graph.NewClient(cred, cfg.Tenant)` | WIRED | import.go:95 |
| `cmd/import.go` | `internal/normalize/normalize.go` | `normalize.Normalize(p.RawJSON)` | WIRED | import.go:163 |
| `cmd/import.go` | `internal/state/backend.go` | `state.NewGitBackend(".")` | WIRED | import.go:97 |
| `cmd/import.go` | `internal/auth/factory.go` | `auth.NewClientFactory(cfg.Auth)` | WIRED | import.go:85 |
| `internal/graph/client.go` | `azcore.TokenCredential` | `credential.GetToken(ctx, ...)` | WIRED | client.go:49 |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|---------|
| CLI-04 | 02-03 | User can run `cactl import` with normalization | SATISFIED | `cmd/import.go` registered; binary compiles; `cactl import --help` shows all flags |
| STATE-01 | 02-02 | State manifest records 1:1 slug-to-ObjectID mapping | SATISFIED | `manifest.go` Entry.LiveObjectID; slug collision detection in import.go:150-155 |
| STATE-02 | 02-02 | GitBackend stores state in refs/cactl/tenants/<tenant>/policies/<slug> | SATISFIED | `policyRef()` format verified; `TestWritePolicyCreatesRef` passes |
| STATE-03 | 02-02 | Every apply creates immutable annotated Git tag cactl/<tenant>/<slug>/<semver> | SATISFIED | `CreateVersionTag()` uses `git tag -a`; tag type confirmed "tag" in test |
| STATE-04 | 02-02 | `cactl init` writes refspec to .git/config | SATISFIED | `ConfigureRefspec()` implemented and tested; called lazily in import.go:103 |
| STATE-05 | 02-02 | State entry schema has 8 required fields | SATISFIED | `Entry` struct: slug, tenant, live_object_id, version, last_deployed, deployed_by, auth_mode, backend_sha |
| IMPORT-01 | 02-03 | `cactl import --all` fetches all live CA policies as v1.0.0 | SATISFIED | import.go:124-126 sets toImport=policies when --all; version="1.0.0" for new entries |
| IMPORT-02 | 02-03 | `cactl import --policy <slug>` imports specific policy | SATISFIED | `filterPolicy()` at import.go:214-226 matches slug or display name case-insensitively |
| IMPORT-03 | 02-01 | Strip server-managed fields (id, createdDateTime, modifiedDateTime, templateId) | SATISFIED | `serverManagedFields` slice + delete loop in normalize.go:11-30 |
| IMPORT-04 | 02-01 | Remove explicit null fields from Graph API responses | SATISFIED | `removeNulls()` recursively deletes nil values in normalize.go:71-88 |
| IMPORT-05 | 02-01 | Normalize field order (alphabetical) with 2-space indent | SATISFIED | `json.MarshalIndent(m, "", "  ")` — Go map marshaling sorts keys |
| IMPORT-06 | 02-01 | Enforce kebab-case slug format | SATISFIED | `Slugify()` in slug.go enforces lowercase kebab-case via compiled regex |
| IMPORT-07 | 02-03 | `cactl import --force` overwrites existing backend JSON | SATISFIED | import.go:156-160 checks `--force` flag; `bumpPatchVersion()` increments patch |
| IMPORT-08 | 02-03 | Without flags, list untracked (?) policies and prompt for selection | SATISFIED | `interactiveSelect()` at import.go:229-285; prints `? [N] DisplayName (ID)` |

**All 14 requirements satisfied. No orphaned requirements.**

---

### Anti-Patterns Found

No blocking or warning anti-patterns detected across all 15 phase 2 files.

A comment in `internal/graph/client_test.go:69` uses the word "placeholder" in a test helper string — this is a test fixture comment, not a stub implementation.

---

### Human Verification Required

The following behaviors require live Entra credentials to verify end-to-end:

#### 1. Full Import Pipeline Against Real Tenant

**Test:** Run `cactl import --all --tenant <real-tenant-id>` with an authenticated Entra session
**Expected:** All live CA policies fetched, normalized, written as Git blobs, version tagged, manifest updated
**Why human:** Requires live Azure credentials and Graph API access; cannot mock end-to-end

#### 2. Interactive Selection UI

**Test:** Run `cactl import` without flags in a terminal with live credentials
**Expected:** Untracked policies displayed with `? [N]` sigil; user can enter "all", "none", or comma-separated numbers
**Why human:** Requires TTY interaction; stdin mocking in tests is limited to unit cases

#### 3. Pagination with Multi-Page Tenant

**Test:** Run `cactl import --all` on a tenant with >100 CA policies
**Expected:** All policies returned across multiple @odata.nextLink pages
**Why human:** Requires a tenant with sufficient policy count to trigger pagination; unit test uses mock with 2 pages

#### 4. --force Version Bump on Re-Import

**Test:** Run `cactl import --all`, then run `cactl import --all --force`
**Expected:** Second run bumps all existing policy versions from 1.0.0 to 1.0.1; manifest updated
**Why human:** Requires real Git repo state across two import runs

---

### Gaps Summary

None. All automated checks pass.

- All 15 source files exist and are fully implemented (not stubs)
- All 10 key links are wired with verified pattern matches
- All 14 requirements (CLI-04, STATE-01 through STATE-05, IMPORT-01 through IMPORT-08) satisfied
- Full test suite passes: 16 normalize tests, 16 state tests, 5 graph tests, 4 import tests
- Binary builds cleanly; `go vet ./...` reports no issues
- `cactl import --help` shows --all, --policy, --force flags with correct descriptions
- CI mode correctly rejects interactive selection with exit code 3

---

*Verified: 2026-03-04T21:30:00Z*
*Verifier: Claude (gsd-verifier)*
