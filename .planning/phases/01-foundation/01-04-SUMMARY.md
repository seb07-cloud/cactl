---
phase: 01-foundation
plan: 04
subsystem: cli
tags: [cobra, error-handling, config-validation, stderr]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: "ExitError type, config.Load, config.Validate, root command, init command"
provides:
  - "ExitError messages printed to stderr before os.Exit"
  - "Config validation invoked on every command via PersistentPreRunE"
affects: [02-graph-client, 03-state-storage]

# Tech tracking
tech-stack:
  added: []
  patterns: ["stderr error display in main.go exit handler", "config.Load + config.Validate in initConfig pipeline"]

key-files:
  created:
    - main_test.go
    - cmd/root_test.go
  modified:
    - main.go
    - cmd/root.go

key-decisions:
  - "Error output via fmt.Fprintln(os.Stderr) in main.go -- single error display point, SilenceErrors stays true"

patterns-established:
  - "Exit handler pattern: errors.As ExitError -> print Message to stderr -> os.Exit(Code)"
  - "Config pipeline: ReadInConfig -> BindPFlags -> Load -> Validate in initConfig"

requirements-completed: [CLI-09, CLI-10, CONF-02]

# Metrics
duration: 2min
completed: 2026-03-04
---

# Phase 1 Plan 4: Gap Closure Summary

**Wired ExitError stderr output in main.go and config validation in initConfig to close UAT gaps 4 and 5**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-04T20:43:40Z
- **Completed:** 2026-03-04T20:46:07Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- ExitError.Message now printed to stderr before os.Exit in main.go (both ExitError and generic error paths)
- config.Load + config.Validate called in initConfig after flag binding, validating output/log-level/auth-mode on every command
- Tests verify stderr output and exit codes via binary execution and in-process cobra testing

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire ExitError stderr output and config validation** - `13cb379` (fix)
2. **Task 2: Add tests for both gap fixes** - `8ec1e28` (test)

## Files Created/Modified
- `main.go` - Added fmt.Fprintln(os.Stderr) for ExitError.Message and generic error before os.Exit
- `cmd/root.go` - Added config.Load + config.Validate call in initConfig after BindPFlags
- `main_test.go` - Binary tests for ExitError stderr output and invalid output flag
- `cmd/root_test.go` - Unit tests for invalid output format validation and valid format pass-through

## Decisions Made
- Error output via fmt.Fprintln(os.Stderr) in main.go -- keeps SilenceErrors true on rootCmd, single error display point prevents double-printing

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 01 UAT gaps 4 and 5 are now fixed
- All error paths produce visible stderr output with correct exit codes
- Config validation runs on every command invocation
- Ready for UAT re-verification and phase sign-off

---
*Phase: 01-foundation*
*Completed: 2026-03-04*
