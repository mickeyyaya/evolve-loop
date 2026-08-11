# Eval: implement-state-manifest
## Code Graders (bash commands that must exit 0)
- `[code]` bash .evolve/calibrate.sh | grep -q '"schemaHygiene": [4-9][0-9]'
## Regression Evals (full test suite)
- `[code]` bash .evolve/calibrate.sh
## Acceptance Checks
- `[code]` grep -q "State Manifest" skills/evolve-loop/memory-protocol.md
## Thresholds
- All checks: pass@1 = 1.0
