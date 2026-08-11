# Eval: extract-observability-python

## Code Graders (bash commands that must exit 0)
- `python3 -m py_compile evolve_status.py`

## Regression Evals (full test suite)
- `test -d .evolve/evals && test "$(ls .evolve/evals/*.md 2>/dev/null | wc -l)" -gt 0`

## Acceptance Checks (verification commands)
- `grep -q "import json" evolve_status.py`

## Thresholds
- All checks: pass@1 = 1.0