# Eval: implement-skill-crystallization
## Code Graders (bash commands that must exit 0)
- `[code]` grep -q "Crystallization" skills/evolve-loop/phase6-learn.md
- `[code]` grep -q "crystallizedSkills" skills/evolve-loop/memory-protocol.md
## Regression Evals (full test suite)
- `[code]` bash .evolve/calibrate.sh
## Acceptance Checks
- `[code]` grep -i "crystallize" skills/evolve-loop/phase6-learn.md
## Thresholds
- All checks: pass@1 = 1.0
