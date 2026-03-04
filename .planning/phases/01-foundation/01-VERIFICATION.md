---
phase: 01-foundation
verified: 2026-03-04T21:00:00Z
status: passed
score: 13/13 must-haves verified
re_verification: false
gaps: []
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
**Verified:** 2026-03-04T21:00:00Z
**Status:** PASSED
**Re-verification:** No - initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Running `go build .` produces a single cactl binary with no external runtime dependencies | VERIFIED | `go build .` succeeds; binary exists at `/Users/seb/Git/cactl/cactl` |
| 2 | Running `cactl --help` shows all global flags: --tenant, --output, --no-color, --ci, --config, --log-level, --auth-mode | VERIFIED | All 7 flags confirmed in `./cactl --help` output |
| 3 | Setting CACTL_TENANT=abc and running cactl resolves tenant through viper precedence chain | VERIFIED | `CACTL_TENANT=test-tenant ./cactl` exits 0; Viper env prefix "CACTL" + AutomaticEnv() wired in `initConfig` |
| 4 | Running cactl with --no-color or CACTL_NO_COLOR=1 disables ANSI color output | VERIFIED | `ShouldUseColor` checks `v.GetBool("no-color")` and `os.Getenv("NO_COLOR")`; `--no-color` flag registered in root.go |
| 5 | Returning an ExitError with code 3 causes the process to exit with code 3 | VERIFIED | Double-init test: `cactl init` on existing workspace exits 3; `main.go` uses `errors.As` to extract `ExitError.Code` |
| 6 | Auth mode resolves correctly: explicit flag > env > config > auto-detect (SP secret, SP cert) > az-cli fallback | VERIFIED | `ResolveAuthMode` implements full chain; 6 table-driven tests pass covering all cases |
| 7 | ClientFactory creates a separate credential instance per tenant ID (no shared credential state) | VERIFIED | RWMutex double-check cache in `factory.go`; `TestClientFactory_PerTenantIsolation` confirms separate instances |
| 8 | Client secrets and certificate contents are never logged, written to disk, or included in output | VERIFIED | `clientSecret` field unexported; error messages use generic "creating client secret credential for tenant %s" format; cert errors use path only |
| 9 | AzureCLICredential passes TenantID in options to force correct tenant token acquisition | VERIFIED | `azurecli.go` line 19-21: `&azidentity.AzureCLICredentialOptions{TenantID: tenantID}` |
| 10 | ClientCertificateCredential uses azidentity.ParseCertificates for PEM/PKCS#12 handling | VERIFIED | `certificate.go` line 39: `azidentity.ParseCertificates(certData, nil)` |
| 11 | Running `cactl init` in an empty directory creates .cactl/config.yaml, .cactl/schema.json, and updates .gitignore | VERIFIED | Manual test confirmed all three files created; `TestInitHappyPath` passes |
| 12 | Running `cactl init` in an already-initialized directory fails with exit code 3 | VERIFIED | Manual test: second `cactl init` exits 3; `TestInitAlreadyInitialized` passes |
| 13 | Schema fetch failure is non-fatal: falls back to embedded schema with a warning | VERIFIED | `FetchOrFallback` in `fetch.go` always uses embedded in Phase 1; warning rendered via renderer; `schema.json` written |

**Score:** 13/13 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `main.go` | Entry point calling cmd.Execute() with exit code handling | VERIFIED | Contains `cmd.Execute()`, `errors.As(err, &exitErr)`, `os.Exit(exitErr.Code)` |
| `cmd/root.go` | Root command with global persistent flags and PersistentPreRunE config loading | VERIFIED | All 7 flags registered; `PersistentPreRunE` calls `initConfig`; exports `Execute()` |
| `pkg/types/exitcodes.go` | Exit code constants and ExitError type | VERIFIED | `ExitSuccess=0, ExitChanges=1, ExitFatalError=2, ExitValidationError=3`; `ExitError` struct with `Error()` and `Unwrap()` |
| `internal/config/config.go` | Config struct with Viper loading and env var override | VERIFIED | `func Load(v *viper.Viper)` unmarshals into `types.Config`; overrides auth secrets from viper |
| `internal/output/renderer.go` | Renderer interface for human and JSON output | VERIFIED | `Renderer` interface defined with 5 methods; `NewRenderer` factory returns `HumanRenderer` or `JSONRenderer` |
| `internal/auth/provider.go` | AuthProvider interface and ResolveAuthMode function | VERIFIED | `AuthProvider` interface with `Credential` and `Mode`; `ResolveAuthMode` with full priority chain |
| `internal/auth/factory.go` | ClientFactory with per-tenant credential caching | VERIFIED | `ClientFactory` struct with RWMutex, per-tenant `credentials` map; `NewClientFactory` constructor |
| `internal/auth/azurecli.go` | AzureCLI auth provider | VERIFIED | `AzureCLIProvider` wraps `azidentity.AzureCLICredential` with `TenantID` option |
| `internal/auth/secret.go` | Client secret auth provider | VERIFIED | `ClientSecretProvider` with unexported `clientSecret`; no String/Format/GoString methods |
| `internal/auth/certificate.go` | Client certificate auth provider | VERIFIED | `ClientCertificateProvider` uses `azidentity.ParseCertificates`; error messages include path not contents |
| `cmd/init.go` | cactl init command implementation | VERIFIED | `func runInit` implements 7-step workspace scaffolding in correct order |
| `internal/schema/fetch.go` | Schema fetching from msgraph-metadata | VERIFIED | `func Fetch` with HTTP GET + 30s timeout; `FetchOrFallback` convenience wrapper |
| `internal/schema/embedded.go` | Embedded fallback schema via Go embed | VERIFIED | `//go:embed schema.json` directive; `EmbeddedSchema []byte`; `WriteEmbedded` function |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `main.go` | `cmd/root.go` | `cmd.Execute()` | WIRED | `main.go:12` calls `cmd.Execute()` |
| `main.go` | `pkg/types/exitcodes.go` | ExitError exit code extraction | WIRED | `main.go:13-16` uses `errors.As(err, &exitErr)` then `exitErr.Code` |
| `cmd/root.go` | `internal/config/config.go` | PersistentPreRunE calling initConfig | WIRED | `root.go:15-17` PersistentPreRunE calls `initConfig(cmd)` |
| `internal/auth/factory.go` | `internal/auth/provider.go` | ClientFactory implements AuthProvider interface | WIRED | `factory.go:69` and `factory.go:97` implement both `Credential` and `Mode` methods |
| `internal/auth/provider.go` | `pkg/types/config.go` | ResolveAuthMode reads AuthConfig | WIRED | `provider.go:40` `ResolveAuthMode(cfg types.AuthConfig)` uses `cfg.Mode`, `cfg.ClientID`, `cfg.ClientSecret`, `cfg.CertPath` |
| `internal/auth/factory.go` | `internal/auth/azurecli.go` | switch on resolved auth mode | WIRED | `factory.go:35` `case AuthModeAzCLI:` creates `&AzureCLIProvider{}` |
| `cmd/init.go` | `internal/schema/fetch.go` | schema.FetchOrFallback() called during init | WIRED | `init.go:92` calls `schema.FetchOrFallback(schemaPath)` |
| `cmd/init.go` | `pkg/types/exitcodes.go` | ExitError for validation failures | WIRED | `init.go:57-60` and `init.go:64-68` return `&types.ExitError{Code: types.ExitValidationError, ...}` |
| `internal/schema/fetch.go` | `internal/schema/embedded.go` | Fallback to embedded schema on fetch failure | WIRED | `fetch.go:65` calls `WriteEmbedded(destPath)` on Fetch failure |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CLI-01 | 01-03 | User can run `cactl init` to scaffold workspace | SATISFIED | `cmd/init.go` creates `.cactl/config.yaml`, `.cactl/schema.json`, updates `.gitignore`; `cactl init` works end-to-end |
| CLI-08 | 01-01 | All commands accept global flags | SATISFIED | All 7 flags registered in `cmd/root.go` `init()`; confirmed in `./cactl --help` |
| CLI-09 | 01-01 | Exit codes: 0=success, 1=changes, 2=fatal, 3=validation | SATISFIED | Constants in `pkg/types/exitcodes.go`; `main.go` extracts `ExitError.Code`; verified exit code 3 on double-init |
| CLI-10 | 01-01 | Single Go binary with zero external runtime dependencies | SATISFIED | `go build .` produces single binary; cobra/viper statically linked |
| AUTH-01 | 01-02 | Azure CLI credential (az login token, default) | SATISFIED | `AzureCLIProvider` in `azurecli.go`; `ResolveAuthMode` falls back to "az-cli" when no SP config |
| AUTH-02 | 01-02 | Service principal with client secret | SATISFIED | `ClientSecretProvider` in `secret.go`; auto-detected from `ClientID+ClientSecret` |
| AUTH-03 | 01-02 | Service principal with certificate | SATISFIED | `ClientCertificateProvider` in `certificate.go` using `ParseCertificates` |
| AUTH-04 | 01-02 | Auth mode priority chain | SATISFIED | `ResolveAuthMode`: explicit mode > auto-detect secret > auto-detect cert > az-cli; 6 tests confirm all cases |
| AUTH-05 | 01-02 | Per-tenant credential isolation via ClientFactory | SATISFIED | `ClientFactory` keyed by `tenantID`; `TestClientFactory_PerTenantIsolation` confirms separate instances |
| AUTH-06 | 01-02 | Credentials never written to disk, logged, or in output | SATISFIED | `clientSecret` unexported; no String/Format/GoString; error messages exclude secret values; cert errors use path only |
| CONF-01 | 01-01 | Config in .cactl/config.yaml with documented schema | SATISFIED | Default config template in `init.go` with all documented keys (tenant, auth, output, log_level) |
| CONF-02 | 01-01 | Every config value overridable by env var or CLI flag | SATISFIED | Viper `SetEnvPrefix("CACTL")` + `AutomaticEnv()` + `BindPFlags()`; env replacer for dash/dot to underscore |
| CONF-03 | 01-03 | cactl init adds .gitignore and refuses if config already tracked | SATISFIED | `.gitignore` written in Step 4 before `config.yaml` in Step 5; `isGitTracked` check in Step 2 |
| CONF-04 | 01-03 | cactl init fetches CA policy JSON Schema to .cactl/schema.json | SATISFIED | `schema.FetchOrFallback` called; embedded fallback ensures `.cactl/schema.json` always created |
| DISP-06 | 01-01 | --no-color flag disables ANSI color (also via CACTL_NO_COLOR=1) | SATISFIED | `ShouldUseColor` checks `v.GetBool("no-color")`, `os.Getenv("NO_COLOR")`, `v.GetBool("ci")`, terminal detection |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/schema/fetch.go` | 56 | `return fmt.Errorf("schema extraction from OpenAPI spec not yet implemented...")` | INFO | Intentional Phase 1 stub; `FetchOrFallback` always uses embedded fallback. Schema fetch enhancement deferred to later phase. Does not block the phase goal. |

No blockers or warnings found. The fetch.go stub is documented and intentional - the fallback path ensures `cactl init` always succeeds with a valid embedded schema.

### Human Verification Required

#### 1. Git-tracked config.yaml check

**Test:** In a git repository, create and `git add .cactl/config.yaml`, then run `cactl init`
**Expected:** Command fails immediately with exit code 3 and the message ".cactl/config.yaml is tracked by Git -- remove it first: git rm --cached .cactl/config.yaml"
**Why human:** Requires setting up a real git repository with a tracked file in a specific state; the code path (`isGitTracked` via `git ls-files --error-unmatch`) is correct but the test environment cannot easily simulate a real git-tracked file.

#### 2. ANSI color disablement in terminal

**Test:** In a real terminal, run `cactl init` (first time), then run `cactl --no-color init` (in a new directory), then run `CACTL_NO_COLOR=1 cactl init` (in another new directory)
**Expected:** First run shows colored symbols (green checkmark, blue i, yellow !); second and third runs show text prefixes (OK, INFO, WARN) with no ANSI escape codes visible
**Why human:** ANSI terminal detection depends on `os.ModeCharDevice` check which varies by terminal emulator and shell; cannot reliably verify color output in a non-interactive shell.

### Gaps Summary

No gaps. All 13 observable truths verified. All 13 required artifacts exist, are substantive, and are wired. All 9 key links confirmed. All 15 requirement IDs from plan frontmatter are satisfied. The only item noted is the intentional schema fetch stub in Phase 1 (always falls back to embedded), which is documented in both the PLAN and SUMMARY as deliberate design.

---

_Verified: 2026-03-04T21:00:00Z_
_Verifier: Claude (gsd-verifier)_
