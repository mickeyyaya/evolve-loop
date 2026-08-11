# Eval: Add Coefficient of Self-Improvement Metric

## Code Graders (bash commands that must exit 0)
- `grep -qi 'Coefficient of Self-Improvement' docs/self-learning.md`
- `grep -q 'CSI' docs/self-learning.md`
- `grep -q 'fitnessScore' docs/self-learning.md`
- `grep -q 'rolling' docs/self-learning.md`

## Regression Evals (full test suite)
- `grep -roh '\[.*\]([^)]*\.md)' skills/ agents/ docs/ 2>/dev/null | grep -oE '\([^)]+\)' | tr -d '()' | while read f; do test -f "$f" || echo "$f"; done | wc -l | awk '{exit ($1 > 2)}'`

## Acceptance Checks (verification commands)
- `wc -l < docs/self-learning.md | awk '{exit ($1 > 220)}'`
- `grep -q 'Feedback Loop' docs/self-learning.md`
- `grep -qi 'operator\|Operator' docs/self-learning.md`

## Thresholds
- All checks: pass@1 = 1.0
