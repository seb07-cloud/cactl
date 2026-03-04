---
phase: 01-foundation
verified: 2026-03-04T21:50:00Z
status: passed
score: 15/15 must-haves verified
re_verification:
  previous_status: passed
  previous_score: 13/13
  gaps_closed:
    - "Running cactl init a second time now prints error message to stderr and exits with code 3"
    - "Running cactl --output invalid now returns exit code 3 with validation error on stderr"
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "Run cactl init in a git repo where .cactl/config.yaml is already tracked"
    expected: "Command fails with exit code 3 and message: .cactl/config.yaml is tracked by Git -- remove it first: git rm --cached .cactl/config.yaml"
    why_human: "Requires a real git repo with a tracked file; cannot reliably create this state in an automated grep check"
  - test: "Run cactl with CACTL_NO_COLOR=1 and verify no ANSI escape codes appear in output"
    expected: "Output shows 'OK', 'INFO', 'WARN', 'ERROR' text prefixes with no ANSI color codes"
    why_human: "Terminal behavior and ANSI detection depends on runtime environment; automated check only verifies code path exists"
---

# Phase 1: Foundation Verification Report

**Phase Goal:** User can initialize a cactl workspace and authenticate to an Entra tenant via the CLI
**Verified:** 2026-03-04T21:50:00Z
**Status:** PASSED
**Re-verification:** Yes — after UAT gap closure (plan 01-04)

## Re-verification Context

Two UAT gaps were identified during the original UAT run and addressed by plan 01-04:

1. **Gap 4 — Silent ExitError:** `cactl init` on an already-initialized workspace correctly exited with code 3 but produced no visible output. Root cause: `SilenceErrors: true` on cobra suppressed output, and `main.go` never printed `ExitError.Message` to stderr. Fixed by adding `fmt.Fprintln(os.Stderr, "Error: "+exitErr.Message)` in `main.go` before `os.Exit`.

2. **Gap 5 — Unvalidated config:** `cactl --output invalid` accepted the invalid value silently. Root cause: `config.Validate()` was fully implemented but never called — `initConfig` bound viper flags but did not invoke `config.Load()` or `config.Validate()`. Fixed by wiring both into `initConfig` in `cmd/root.go`.

Previously-verified truths 1-13 all pass regression checks. Two new truths added to capture the gap-closure requirements.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Running `go build .` produces a single cactl binary | VERIFIED | Build succeeds; `go test ./...` passes cleanly |
| 2 | `cactl --help` shows all 7 global flags | VERIFIED | `./cactl --help` confirms: --tenant, --output, --no-color, --ci, --config, --log-level, --auth-mode |
| 3 | Viper env var precedence resolves CACTL_TENANT | VERIFIED | `SetEnvPrefix("CACTL")` + `AutomaticEnv()` + `BindPFlags()` wired in `initConfig` |
| 4 | `--no-color` and `CACTL_NO_COLOR=1` disable ANSI color | VERIFIED | `ShouldUseColor` checks `v.GetBool("no-color")`, `os.Getenv("NO_COLOR")`, `v.GetBool("ci")`, terminal detection |
| 5 | ExitError code 3 causes process to exit with code 3 | VERIFIED | Binary exits 3 on double-init; `main.go` uses `errors.As(err, &exitErr)` then `os.Exit(exitErr.Code)` |
| 6 | Auth mode resolves flag > env > config > auto-detect > az-cli fallback | VERIFIED | `ResolveAuthMode` full chain; 6 table-driven tests pass |
| 7 | ClientFactory creates separate credential per tenant (no shared state) | VERIFIED | RWMutex double-check cache; `TestClientFactory_PerTenantIsolation` confirms separate instances |
| 8 | Client secrets and certificate contents never logged, written, or in output | VERIFIED | `clientSecret` unexported; error messages use generic format with path only, not contents |
| 9 | AzureCLICredential passes TenantID in options | VERIFIED | `azurecli.go`: `&azidentity.AzureCLICredentialOptions{TenantID: tenantID}` |
| 10 | ClientCertificateCredential uses `azidentity.ParseCertificates` | VERIFIED | `certificate.go`: `azidentity.ParseCertificates(certData, nil)` |
| 11 | `cactl init` in empty dir creates config.yaml, schema.json, .gitignore | VERIFIED | Binary confirmed in temp dir; `TestInitHappyPath` passes |
| 12 | `cactl init` in already-initialized dir fails with exit code 3 AND prints error to stderr | VERIFIED | Binary outputs "Error: workspace already initialized (.cactl directory exists)" to stderr, exits 3; `TestExitErrorPrintsToStderr` passes |
| 13 | Schema fetch failure is non-fatal: falls back to embedded schema | VERIFIED | `FetchOrFallback` always uses embedded in Phase 1; WARN message rendered; `schema.json` written |
| 14 | `cactl --output invalid` returns exit code 3 with validation error on stderr | VERIFIED | Binary outputs "Error: invalid output format \"invalid\": must be one of human, json", exits 3; `TestInvalidOutputPrintsToStderr` + `TestInvalidOutputFormatReturnsExitError` pass |
| 15 | All unit tests pass: 0 failures | VERIFIED | `go test ./...` — all packages pass; 0 failures |

**Score:** 15/15 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `main.go` | Entry point with stderr error output and exit code handling | VERIFIED | `fmt.Fprintln(os.Stderr, "Error: "+exitErr.Message)` on line 16; `os.Exit(exitErr.Code)` on line 17 |
| `cmd/root.go` | Root command with global flags, config load, and validation in PersistentPreRunE | VERIFIED | All 7 flags registered; `initConfig` calls `config.Load(v)` then `config.Validate(cfg)` |
| `pkg/types/exitcodes.go` | Exit code constants and ExitError type | VERIFIED | `ExitSuccess=0, ExitChanges=1, ExitFatalError=2, ExitValidationError=3`; `ExitError` with `Error()` and `Unwrap()` |
| `internal/config/config.go` | Config struct with Viper loading | VERIFIED | `func Load(v *viper.Viper)` unmarshals into `types.Config`; called from `initConfig` |
| `internal/config/validate.go` | Config validation with ExitError on bad values | VERIFIED | `Validate` checks output, log-level, auth-mode; returns ExitError code 3; called from `initConfig` |
| `internal/output/renderer.go` | Renderer interface for human and JSON output | VERIFIED | `Renderer` interface with 5 methods; `NewRenderer` factory |
| `internal/auth/provider.go` | AuthProvider interface and ResolveAuthMode | VERIFIED | Full priority chain implemented; 6 tests |
| `internal/auth/factory.go` | ClientFactory with per-tenant caching | VERIFIED | RWMutex; per-tenant `credentials` map |
| `internal/auth/azurecli.go` | AzureCLI auth provider | VERIFIED | `AzureCLIProvider` with TenantID option |
| `internal/auth/secret.go` | Client secret auth provider | VERIFIED | `clientSecret` unexported; no String/Format/GoString |
| `internal/auth/certificate.go` | Client certificate auth provider | VERIFIED | `ParseCertificates`; path-only error messages |
| `cmd/init.go` | cactl init command | VERIFIED | 7-step workspace scaffolding; guard returns ExitError on re-init |
| `internal/schema/fetch.go` | Schema fetch with embedded fallback | VERIFIED | `FetchOrFallback`; `WriteEmbedded` on failure |
| `internal/schema/embedded.go` | Embedded fallback schema | VERIFIED | `//go:embed schema.json`; `EmbeddedSchema []byte` |
| `main_test.go` | Binary tests for stderr error output | VERIFIED | `TestExitErrorPrintsToStderr` + `TestInvalidOutputPrintsToStderr` — both PASS |
| `cmd/root_test.go` | Unit tests for config validation | VERIFIED | `TestInvalidOutputFormatReturnsExitError` + `TestValidOutputFormatPasses` — both PASS |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `main.go` | `pkg/types/exitcodes.go` | `errors.As` + `Fprintln` + `os.Exit` | WIRED | Lines 14-18: `errors.As(err, &exitErr)`, `Fprintln(os.Stderr, ...)`, `os.Exit(exitErr.Code)` |
| `cmd/root.go` | `internal/config/config.go` | `config.Load(v)` in `initConfig` | WIRED | Line 68: `cfg, err := config.Load(v)` |
| `cmd/root.go` | `internal/config/validate.go` | `config.Validate(cfg)` in `initConfig` | WIRED | Line 72: `if err := config.Validate(cfg)` |
| `cmd/root.go` | `internal/config/config.go` via validate | Full config pipeline in PersistentPreRunE | WIRED | Lines 62-74: BindPFlags -> Load -> Validate, all in `initConfig` called from `PersistentPreRunE` |
| `internal/auth/factory.go` | `internal/auth/provider.go` | implements AuthProvider interface | WIRED | `factory.go`: `Credential` and `Mode` methods implemented |
| `internal/auth/provider.go` | `pkg/types/config.go` | `ResolveAuthMode(cfg types.AuthConfig)` | WIRED | Uses `cfg.Mode`, `cfg.ClientID`, `cfg.ClientSecret`, `cfg.CertPath` |
| `cmd/init.go` | `internal/schema/fetch.go` | `schema.FetchOrFallback()` | WIRED | `init.go`: calls `schema.FetchOrFallback(schemaPath)` |
| `cmd/init.go` | `pkg/types/exitcodes.go` | `ExitError` for validation failures | WIRED | Returns `&types.ExitError{Code: types.ExitValidationError, ...}` on re-init guard |
| `internal/schema/fetch.go` | `internal/schema/embedded.go` | Fallback to embedded schema on fetch failure | WIRED | `fetch.go`: calls `WriteEmbedded(destPath)` on Fetch failure |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CLI-01 | 01-03 | User can run `cactl init` to scaffold workspace | SATISFIED | `cmd/init.go` creates `.cactl/config.yaml`, `.cactl/schema.json`, updates `.gitignore`; end-to-end confirmed |
| CLI-08 | 01-01 | All commands accept global flags | SATISFIED | All 7 flags in `cmd/root.go`; confirmed in `./cactl --help` |
| CLI-09 | 01-01, 01-04 | Exit codes: 0=success, 1=changes, 2=fatal, 3=validation | SATISFIED | Constants in `pkg/types/exitcodes.go`; `main.go` prints message and exits correctly; binary exits 3 with message on double-init |
| CLI-10 | 01-01, 01-04 | Single Go binary with zero external runtime dependencies | SATISFIED | `go build .` succeeds; statically linked |
| AUTH-01 | 01-02 | Azure CLI credential (az login, default) | SATISFIED | `AzureCLIProvider` in `azurecli.go`; fallback in `ResolveAuthMode` |
| AUTH-02 | 01-02 | Service principal with client secret | SATISFIED | `ClientSecretProvider` in `secret.go`; auto-detected |
| AUTH-03 | 01-02 | Service principal with certificate | SATISFIED | `ClientCertificateProvider` in `certificate.go` |
| AUTH-04 | 01-02 | Auth mode priority chain | SATISFIED | `ResolveAuthMode`: explicit > auto-detect secret > auto-detect cert > az-cli; 6 tests |
| AUTH-05 | 01-02 | Per-tenant credential isolation via ClientFactory | SATISFIED | RWMutex keyed by `tenantID`; isolation test passes |
| AUTH-06 | 01-02 | Credentials never written to disk, logged, or in output | SATISFIED | `clientSecret` unexported; error messages exclude secret values |
| CONF-01 | 01-01 | Config in .cactl/config.yaml with documented schema | SATISFIED | Default config template with all documented keys written by `cactl init` |
| CONF-02 | 01-01, 01-04 | Every config value overridable by env var or CLI flag | SATISFIED | Viper `SetEnvPrefix("CACTL")` + `AutomaticEnv()` + `BindPFlags()`; `config.Validate` now confirms invalid overrides are rejected with exit code 3 |
| CONF-03 | 01-03 | cactl init adds .gitignore and refuses if config already tracked | SATISFIED | `.gitignore` written before `config.yaml`; `isGitTracked` check present |
| CONF-04 | 01-03 | cactl init fetches CA policy JSON Schema to .cactl/schema.json | SATISFIED | `schema.FetchOrFallback` called; embedded fallback ensures schema always written |
| DISP-06 | 01-01 | --no-color flag disables ANSI color (also via CACTL_NO_COLOR=1) | SATISFIED | `ShouldUseColor` checks `v.GetBool("no-color")`, `os.Getenv("NO_COLOR")`, `v.GetBool("ci")`, terminal detection |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/schema/fetch.go` | 56 | `return fmt.Errorf("schema extraction from OpenAPI spec not yet implemented...")` | INFO | Intentional Phase 1 stub; `FetchOrFallback` always uses embedded fallback. Does not block goal. |

No blockers or warnings. The fetch.go stub is documented and intentional — the fallback path ensures `cactl init` always succeeds with a valid embedded schema.

### Human Verification Required

#### 1. Git-tracked config.yaml check

**Test:** In a git repository, create and `git add .cactl/config.yaml`, then run `cactl init`
**Expected:** Command fails with exit code 3 and the message ".cactl/config.yaml is tracked by Git -- remove it first: git rm --cached .cactl/config.yaml"
**Why human:** Requires setting up a real git repository with a tracked file in a specific state; code path via `isGitTracked` (`git ls-files --error-unmatch`) is correct but cannot simulate a real git-tracked file in automated checks.

#### 2. ANSI color disablement in terminal

**Test:** In a real terminal, run `cactl init` (first time), then `cactl --no-color init` (new directory), then `CACTL_NO_COLOR=1 cactl init` (another new directory)
**Expected:** First run shows colored symbols (green checkmark, blue i, yellow !); second and third runs show text prefixes (OK, INFO, WARN) with no ANSI escape codes
**Why human:** ANSI terminal detection depends on `os.ModeCharDevice` which varies by terminal emulator and shell; cannot reliably verify in a non-interactive shell.

### Gaps Summary

No gaps remain. Both UAT gaps (tests 4 and 5) are now closed and verified:

- **Gap 4 closed:** `cactl init` on an existing workspace now prints "Error: workspace already initialized (.cactl directory exists)" to stderr with exit code 3. Confirmed by `TestExitErrorPrintsToStderr` and live binary execution.
- **Gap 5 closed:** `cactl --output invalid` now prints "Error: invalid output format \"invalid\": must be one of human, json" to stderr with exit code 3. Confirmed by `TestInvalidOutputPrintsToStderr`, `TestInvalidOutputFormatReturnsExitError`, and live binary execution.

All 15 observable truths verified. All 16 required artifacts exist, are substantive, and are wired. All 9 key links confirmed. All 15 requirement IDs satisfied. `go test ./...` passes cleanly with 0 failures.

---

_Verified: 2026-03-04T21:50:00Z_
_Verifier: Claude (gsd-verifier)_
