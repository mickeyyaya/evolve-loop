# Eval: observer-migration-default
## Code Graders (bash commands that must exit 0)
- `[code]` `grep -q 'EVOLVE_OBSERVER_ENFORCE:-1' scripts/dispatch/run-cycle.sh`
- `[code]` `! grep -q 'EVOLVE_OBSERVER_ENFORCE:-0' scripts/dispatch/run-cycle.sh`
- `[code]` `grep -q '\*-observer-events.ndjson' scripts/dispatch/phase-watchdog.sh`
## Regression Evals (full test suite)
- `[code]` `bash tests/markdown-structure-test.sh`
## Acceptance Checks
- `[code]` `grep -q 'Replaces phase-watchdog.sh' docs/architecture/phase-observer.md`
## Thresholds
- All checks: pass@1 = 1.0
