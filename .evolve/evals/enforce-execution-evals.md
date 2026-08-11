# Eval: enforce-execution-evals
## Code Graders (bash commands that must exit 0)
- `[code]` grep -q 'fail "eval-quality-check flagged' scripts/phase-gate.sh
## Regression Evals (full test suite)
- `[code]` bash .evolve/calibrate.sh
## Acceptance Checks
- `[code]` grep -i "fail" scripts/phase-gate.sh
## Thresholds
- All checks: pass@1 = 1.0