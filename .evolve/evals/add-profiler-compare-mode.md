# Eval: Add Profiler Compare Mode

## Code Graders
- `bash scripts/token-profiler.sh --save-baseline 2>/dev/null && test -f .evolve/token-baseline.json`
- `bash scripts/token-profiler.sh --compare 2>/dev/null | grep -qE "delta|change|[+-][0-9]|baseline"`
- `bash scripts/token-profiler.sh 2>/dev/null | grep -q "evolve-loop"`
- `grep -q "save-baseline\|baseline" scripts/token-profiler.sh`
- `grep -q "compare" scripts/token-profiler.sh`

## Regression Evals
- `bash scripts/token-profiler.sh > /tmp/tp-regression.txt 2>&1 && test -s /tmp/tp-regression.txt`

## Thresholds
- All checks: pass@1 = 1.0
