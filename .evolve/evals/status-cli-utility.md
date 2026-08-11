# Eval: status-cli-utility

## Code Graders (bash commands that must exit 0)
- `chmod +x status.sh`
- `./status.sh`

## Regression Evals (full test suite)
- `test -f scripts/phase-gate.sh`

## Acceptance Checks (verification commands)
- `grep -q "lastCycleNumber" status.sh`
- `grep -q "mastery" status.sh`

## Thresholds
- All checks: pass@1 = 1.0
