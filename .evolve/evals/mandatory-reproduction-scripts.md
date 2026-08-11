# Eval: mandatory-reproduction-scripts
## Code Graders (bash commands that must exit 0)
- `[code]` grep -q "Reproduction" agents/evolve-builder.md
- `[code]` grep -q "Blueprint" agents/evolve-builder.md
## Regression Evals (full test suite)
- `[code]` bash .evolve/calibrate.sh
## Acceptance Checks
- `[code]` grep -i "repro" agents/evolve-builder.md
## Thresholds
- All checks: pass@1 = 1.0
