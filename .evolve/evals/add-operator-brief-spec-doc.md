# Eval: Add Operator Brief Spec Doc

## Code Graders (bash commands that must exit 0)
- `test -f docs/operator-brief.md`

## Regression Evals (full test suite)
- `test -d docs && test "$(ls docs/*.md 2>/dev/null | wc -l)" -gt 0`

## Acceptance Checks (verification commands)
- `grep -q "weakestDimension" docs/operator-brief.md`
- `grep -q "recommendedStrategy" docs/operator-brief.md`
- `grep -q "taskTypeBoosts" docs/operator-brief.md`
- `grep -q "avoidAreas" docs/operator-brief.md`
- `grep -q "operator-brief" docs/architecture.md`
- `wc -l < docs/operator-brief.md | awk '{exit ($1 < 30 || $1 > 120)}'`

## Thresholds
- All checks: pass@1 = 1.0
