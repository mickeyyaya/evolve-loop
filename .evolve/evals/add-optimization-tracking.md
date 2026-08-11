# Eval: Add Optimization Tracking

## Code Graders
- `grep -q "Tracking\|tracking\|Baseline\|baseline" docs/token-optimization-guide.md`
- `grep -q "token-profiler" docs/token-optimization-guide.md`
- `grep -q "policies.md" docs/token-optimization-guide.md`
- `grep -q "44%" docs/token-optimization-guide.md`

## Regression Evals
- `test -f docs/token-optimization-guide.md`
- `[ $(wc -l < docs/token-optimization-guide.md) -gt 50 ]`

## Thresholds
- All checks: pass@1 = 1.0
