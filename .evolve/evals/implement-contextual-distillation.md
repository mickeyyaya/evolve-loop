# Eval: implement-contextual-distillation
## Code Graders (bash commands that must exit 0)
- `[code]` grep -q "Distillation" skills/evolve-loop/phase6-learn.md
## Regression Evals (full test suite)
- `[code]` bash .evolve/calibrate.sh
## Acceptance Checks
- `[code]` grep -i "compress" skills/evolve-loop/phase6-learn.md
## Thresholds
- All checks: pass@1 = 1.0
