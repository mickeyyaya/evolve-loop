# Eval: Add Lazy Tool Loading and System Reminder Patterns to token-optimization.md

## Code Graders (bash commands that must exit 0)
- `grep -q "lazy.tool\|Lazy Tool" docs/token-optimization.md`
- `grep -q "system.reminder\|System Reminder\|instruction.fade\|fade-out" docs/token-optimization.md`
- `grep -q "OPENDEV\|lazy.*load\|tool.*schema" docs/token-optimization.md`

## Regression Evals (full test suite)
- `test -f docs/token-optimization.md`

## Acceptance Checks (verification commands)
- `wc -l < docs/token-optimization.md | awk '{exit ($1 > 480)}'`
- `grep -c "lazy\|reminder\|fade" docs/token-optimization.md | awk '{exit ($1 < 3)}'`

## Thresholds
- All checks: pass@1 = 1.0
