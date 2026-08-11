# Eval: implement-mutation-testing
## Code Graders (bash commands that must exit 0)
- `[code]` grep -q "Mutation" agents/evolve-auditor.md
## Regression Evals (full test suite)
- `[code]` bash .evolve/calibrate.sh
## Acceptance Checks
- `[code]` grep -i "mutant" agents/evolve-auditor.md
## Thresholds
- All checks: pass@1 = 1.0