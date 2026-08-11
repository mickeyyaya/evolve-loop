# Eval: Add Human Learning Guide

## Code Graders (bash commands that must exit 0)

- `test -f docs/human-learning-guide.md`
- `wc -l < docs/human-learning-guide.md | awk '{exit ($1 < 60 || $1 > 120)}'`
- `grep -q "human-learning-guide" docs/architecture.md`

## Regression Evals (full test suite)

- `grep -q "## Reference Documents" docs/architecture.md`
- `grep -c "^## " docs/architecture.md | awk '{exit ($1 < 5)}'`

## Acceptance Checks (verification commands)

- `grep -qi "instinct" docs/human-learning-guide.md`
- `grep -qi "ledger\|decision trace\|scout-report" docs/human-learning-guide.md`
- `grep -qi "audit\|human" docs/human-learning-guide.md`

## Thresholds

- All checks: pass@1 = 1.0
