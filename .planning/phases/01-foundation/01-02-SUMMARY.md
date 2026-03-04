---
phase: 01-foundation
plan: 02
subsystem: auth
tags: [go, azidentity, azcore, auth, per-tenant-isolation, credential-factory]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: "Config and AuthConfig types with mapstructure tags"
provides:
  - "AuthProvider interface with Credential(ctx, tenantID) and Mode() methods"
  - "ResolveAuthMode with priority chain: explicit > auto-detect > az-cli fallback"
  - "AzureCLI auth provider with TenantID option"
  - "ClientSecret auth provider with no credential leakage"
  - "ClientCertificate auth provider using ParseCertificates for PEM/PKCS#12"
  - "ClientFactory with per-tenant credential caching via RWMutex"
affects: [01-foundation, 02-auth, 03-state]

# Tech tracking
tech-stack:
  added: [azidentity-v1.13.1, azcore-v1.20.0, testify-v1.11.x]
  patterns: [auth-provider-interface, per-tenant-credential-isolation, rwmutex-double-check-cache, table-driven-tests]

key-files:
  created:
    - internal/auth/provider.go
    - internal/auth/azurecli.go
    - internal/auth/secret.go
    - internal/auth/certificate.go
    - internal/auth/factory.go
    - internal/auth/provider_test.go
    - internal/auth/factory_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "ClientFactory uses RWMutex double-check locking for thread-safe per-tenant credential caching"
  - "Auth providers are separate types implementing AuthProvider interface (not a single switch-case function)"
  - "Mock provider used in factory tests instead of real azidentity calls to avoid Azure dependency in CI"

patterns-established:
  - "AuthProvider interface: all auth flows go through Credential(ctx, tenantID) returning azcore.TokenCredential"
  - "Per-tenant isolation: ClientFactory creates one credential per tenantID, keyed in map, never shared"
  - "Credential safety: error messages include tenant ID and cert path but never secret values or cert contents"
  - "Table-driven tests: ResolveAuthMode uses standard Go table-driven test pattern with testify assertions"

requirements-completed: [AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-06]

# Metrics
duration: 2min
completed: 2026-03-04
---

# Phase 1 Plan 02: Auth Layer Summary

**Three credential providers (az-cli, client-secret, client-certificate) with per-tenant credential isolation via ClientFactory and auth mode resolution chain**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-04T20:12:07Z
- **Completed:** 2026-03-04T20:14:12Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- AuthProvider interface with Credential(ctx, tenantID) and Mode() enabling pluggable auth backends
- ResolveAuthMode implements full priority chain: explicit flag > auto-detect (secret > cert) > az-cli fallback
- Three credential providers wrapping azidentity: AzureCLI (with TenantID option), ClientSecret, ClientCertificate
- ClientFactory with per-tenant credential caching via RWMutex double-check pattern (prevents azidentity #19726)
- 14 unit tests covering auth mode resolution (6 cases), factory validation (5 cases), and credential isolation (3 cases)
- No credential values appear in error messages or logs (AUTH-06 compliance)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create AuthProvider interface, auth mode resolution, and three credential providers** - `f3166c7` (feat)
2. **Task 2: Create ClientFactory with per-tenant credential isolation and unit tests** - `37a4fa0` (feat)

## Files Created/Modified
- `internal/auth/provider.go` - AuthProvider interface, auth mode constants, ResolveAuthMode function
- `internal/auth/azurecli.go` - AzureCLIProvider wrapping azidentity.AzureCLICredential with TenantID option
- `internal/auth/secret.go` - ClientSecretProvider with no credential leakage in error paths
- `internal/auth/certificate.go` - ClientCertificateProvider using ParseCertificates for PEM/PKCS#12
- `internal/auth/factory.go` - ClientFactory with per-tenant credential caching and RWMutex protection
- `internal/auth/provider_test.go` - 6 table-driven tests for ResolveAuthMode priority chain
- `internal/auth/factory_test.go` - 8 tests for factory validation, isolation, caching, and concurrency
- `go.mod` / `go.sum` - Added azidentity v1.13.1, azcore v1.20.0, testify dependencies

## Decisions Made
- ClientFactory uses RWMutex with double-check locking pattern for thread-safe credential caching (avoids full lock on cache hits)
- Auth providers are separate struct types implementing AuthProvider interface rather than a single function with switch-case (enables independent testing and future extension)
- Factory tests use a mock provider to test caching and isolation logic without requiring Azure credentials in CI

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Auth layer complete with all three credential types and per-tenant isolation
- Ready for Plan 03 (workspace init) to implement `cactl init` subcommand
- ClientFactory can be wired into commands via NewClientFactory(cfg.Auth) in future command PreRunE hooks

## Self-Check: PASSED

All 7 created files verified on disk. Both task commits (f3166c7, 37a4fa0) verified in git log.

---
*Phase: 01-foundation*
*Completed: 2026-03-04*
