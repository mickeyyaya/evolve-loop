# Eval: cli-bash-wrapper

## Code Graders (bash commands that must exit 0)
- `bash -n status.sh`

## Regression Evals (full test suite)
- `test -d .evolve/evals && test "$(ls .evolve/evals/*.md 2>/dev/null | wc -l)" -gt 0`

## Acceptance Checks (verification commands)
- `grep -q "evolve_status.py" status.sh`

## Thresholds
- All checks: pass@1 = 1.0