# Eval: Add Run Isolation Doc

## Code Graders (bash commands that must exit 0)
- `test -f docs/run-isolation.md`

## Regression Evals (full test suite)
- `test -d docs && test "$(ls docs/*.md 2>/dev/null | wc -l)" -gt 0`

## Acceptance Checks (verification commands)
- `grep -q "RUN_ID" docs/run-isolation.md`
- `grep -q "WORKSPACE_PATH" docs/run-isolation.md`
- `grep -q "run-isolation" docs/architecture.md`
- `wc -l < docs/run-isolation.md | awk '{exit ($1 < 30 || $1 > 120)}'`

## Thresholds
- All checks: pass@1 = 1.0
