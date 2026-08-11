# Eval: agent-native-refactor
## Code Graders (bash commands that must exit 0)
- `[code]` test -d skills/evolve-loop/phases
- `[code]` test -d skills/evolve-loop/utils
- `[code]` bash .evolve/calibrate.sh | grep -q '"modularity": [9][0-9]'
## Regression Evals (full test suite)
- `[code]` bash .evolve/calibrate.sh
## Acceptance Checks
- `[code]` grep -q "phases/" skills/evolve-loop/SKILL.md
## Thresholds
- All checks: pass@1 = 1.0
