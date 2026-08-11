# Eval: observer-migration-acs
## Code Graders (bash commands that must exit 0)
- `[code]` `test -f acs/cycle-100/001-observer-enforce-default-on.sh`
- `[code]` `test -f acs/cycle-100/002-observer-spawned-when-default.sh`
- `[code]` `test -f acs/cycle-100/003-watchdog-not-spawned-when-default.sh`
- `[code]` `test -f acs/cycle-100/004-watchdog-glob-includes-observer-events.sh`
- `[code]` `test -f acs/cycle-100/005-observer-events-file-mtime-fresh.sh`
- `[code]` `test -f scripts/tests/observer-no-false-fire-test.sh`
## Regression Evals (full test suite)
- `[code]` `bash tests/markdown-structure-test.sh`
## Thresholds
- All checks: pass@1 = 1.0
