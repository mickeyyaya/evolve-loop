# Eval: implement-scoped-context
## Code Graders (bash commands that must exit 0)
- `[code]` grep -q "Scoped Context" skills/evolve-loop/SKILL.md
- `[code]` grep -q "Scoped Fields" skills/evolve-loop/memory-protocol.md
## Regression Evals (full test suite)
- `[code]` bash .evolve/calibrate.sh
## Acceptance Checks
- `[code]` grep -i "filter" skills/evolve-loop/SKILL.md
## Thresholds
- All checks: pass@1 = 1.0
