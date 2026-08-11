# Eval: implement-metadata-linter
## Code Graders (bash commands that must exit 0)
- `[code]` bash scripts/doc-lint.sh
- `[code]` bash .evolve/calibrate.sh | grep -q '"documentationCompleteness": [8-9][0-9]'
## Regression Evals (full test suite)
- `[code]` bash .evolve/calibrate.sh
## Acceptance Checks
- `[code]` test -x scripts/doc-lint.sh
## Thresholds
- All checks: pass@1 = 1.0
