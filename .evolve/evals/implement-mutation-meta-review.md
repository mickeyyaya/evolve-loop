# Eval: implement-mutation-meta-review
## Code Graders (bash commands that must exit 0)
- `[code]` grep -q "Mutation" skills/evolve-loop/phases/phase7-meta.md
## Regression Evals (full test suite)
- `[code]` bash .evolve/calibrate.sh
## Acceptance Checks
- `[code]` grep -i "mutant" skills/evolve-loop/phases/phase7-meta.md
## Thresholds
- All checks: pass@1 = 1.0