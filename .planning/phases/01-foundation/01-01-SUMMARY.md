---
phase: 01-foundation
plan: 01
subsystem: cli
tags: [go, cobra, viper, cli, config, exit-codes, renderer]

# Dependency graph
requires: []
provides:
  - "Buildable cactl binary with cobra root command"
  - "Global flags: --tenant, --output, --no-color, --ci, --config, --log-level, --auth-mode"
  - "Viper config loading with precedence: flags > env > config > defaults"
  - "Config and AuthConfig types with mapstructure tags"
  - "ExitError type with exit code propagation through main.go"
  - "Renderer interface with HumanRenderer (color) and JSONRenderer implementations"
  - "ShouldUseColor function respecting --no-color, NO_COLOR, --ci, terminal detection"
  - "Config validation returning ExitError code 3 for invalid values"
affects: [01-foundation, 02-auth, 03-state]

# Tech tracking
tech-stack:
  added: [go-1.24, cobra-v1.10.2, viper-v1.21.0, golangci-lint-v2]
  patterns: [cobra-persistent-prerun-config, viper-env-prefix, exit-code-contract, renderer-interface]

key-files:
  created:
    - main.go
    - cmd/root.go
    - pkg/types/exitcodes.go
    - pkg/types/config.go
    - internal/config/config.go
    - internal/config/validate.go
    - internal/output/renderer.go
    - internal/output/human.go
    - internal/output/json.go
    - internal/output/color.go
    - .golangci.yml
  modified: []

key-decisions:
  - "Used global viper singleton for Phase 1 (standard Cobra pattern, refactor if testing friction emerges)"
  - "Root command RunE shows help when invoked without subcommands"
  - "HumanRenderer uses text prefixes (OK, ERROR, INFO, WARN) when color disabled instead of Unicode symbols"

patterns-established:
  - "PersistentPreRunE config loading: viper reads config file, then binds flags (flag > env > config > defaults)"
  - "Exit code contract: ExitError carries code through cmd.Execute() to main.go os.Exit()"
  - "Renderer interface: all output goes through Renderer, enabling human/json format switching"
  - "Auth secrets from env vars only: ClientSecret never loaded from config file"

requirements-completed: [CLI-08, CLI-09, CLI-10, CONF-01, CONF-02, DISP-06]

# Metrics
duration: 3min
completed: 2026-03-04
---

# Phase 1 Plan 01: CLI Skeleton Summary

**Go CLI binary with cobra root command, 7 global flags, viper config precedence chain, human/JSON renderer with no-color support, and typed exit codes**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-04T20:06:53Z
- **Completed:** 2026-03-04T20:09:50Z
- **Tasks:** 2
- **Files modified:** 14

## Accomplishments
- Buildable single Go binary (`go build .`) with cobra CLI framework
- All 7 global flags registered: --tenant, --output, --no-color, --ci, --config, --log-level, --auth-mode
- Config precedence chain wired in PersistentPreRunE: flags > CACTL_* env vars > .cactl/config.yaml > defaults
- ExitError type propagates custom exit codes (0-3) through main.go
- Renderer interface with HumanRenderer (ANSI color) and JSONRenderer (structured JSON)
- ShouldUseColor respects --no-color, CACTL_NO_COLOR, NO_COLOR convention, --ci, and terminal detection
- Config validation returns ExitError code 3 for invalid output/log-level/auth-mode values

## Task Commits

Each task was committed atomically:

1. **Task 1: Initialize Go module and create project structure with types and exit codes** - `3bafa0b` (feat)
2. **Task 2: Create root command with global flags, Viper config loading, and output renderer** - `67897df` (feat)

## Files Created/Modified
- `main.go` - Entry point calling cmd.Execute() with ExitError exit code extraction
- `go.mod` / `go.sum` - Go module with cobra and viper dependencies
- `cmd/root.go` - Root command with global persistent flags and PersistentPreRunE config loading
- `pkg/types/exitcodes.go` - Exit code constants (0-3) and ExitError type with Unwrap
- `pkg/types/config.go` - Config and AuthConfig structs with mapstructure tags
- `internal/config/config.go` - Config loader using viper with env var override for auth secrets
- `internal/config/validate.go` - Config validator for output, log-level, auth-mode values
- `internal/output/renderer.go` - Renderer interface and NewRenderer factory
- `internal/output/human.go` - HumanRenderer with optional ANSI color output
- `internal/output/json.go` - JSONRenderer outputting structured JSON messages
- `internal/output/color.go` - ShouldUseColor with multi-signal color detection
- `.golangci.yml` - golangci-lint v2 config with standard linters

## Decisions Made
- Used global viper singleton for Phase 1 (standard Cobra pattern); can refactor to passed instance if testing friction emerges
- Added RunE on root command that shows help, so flags are visible in `--help` output
- HumanRenderer uses text prefixes (OK, ERROR, INFO, WARN) when color is disabled for accessibility

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added Long description and RunE to root command for proper help display**
- **Found during:** Task 2 (verification)
- **Issue:** Cobra root command without Long/RunE only showed Short description, hiding all flags from --help output
- **Fix:** Added Long description and RunE that calls cmd.Help()
- **Files modified:** cmd/root.go
- **Verification:** `./cactl --help` now shows all 7 global flags
- **Committed in:** 67897df (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Necessary for correct CLI UX. No scope creep.

## Issues Encountered
None beyond the auto-fixed deviation above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- CLI skeleton complete with all global flags and config precedence
- Ready for Plan 02 (auth provider) to wire authentication into the existing config/command structure
- Ready for Plan 03 (workspace init) to implement `cactl init` subcommand

## Self-Check: PASSED

All 11 created files verified on disk. Both task commits (3bafa0b, 67897df) verified in git log.

---
*Phase: 01-foundation*
*Completed: 2026-03-04*
