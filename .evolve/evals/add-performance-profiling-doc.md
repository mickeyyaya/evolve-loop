# Eval: add-performance-profiling-doc

## Code Graders (bash commands that must exit 0)
- `test -f docs/performance-profiling.md`
- `wc -l < docs/performance-profiling.md | awk '{exit ($1 < 40)}'`
- `grep -q 'per.*phase\|phase.*token\|token.*phase' docs/performance-profiling.md`
- `grep -qi 'telemetry\|profil' docs/performance-profiling.md`

## Regression Evals (full test suite)
- `grep -roh '\[.*\](docs/performance-profiling\.md)' docs/ agents/ skills/ 2>/dev/null | wc -l | awk '{exit ($1 == 0)}'`

## Acceptance Checks (verification commands)
- `grep -c '^## ' docs/performance-profiling.md | awk '{exit ($1 < 2)}'`
- `wc -l < docs/performance-profiling.md | awk '{exit ($1 > 150)}'`

## Thresholds
- All checks: pass@1 = 1.0
