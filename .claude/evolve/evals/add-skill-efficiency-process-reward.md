# Eval: add-skill-efficiency-process-reward

## Code Graders (bash commands that must exit 0)
- `grep -q "skillEfficiency\|skill_efficiency\|skill efficiency" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -A3 -i "skillEfficiency\|skill efficiency" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | grep -q "1\.0\|0\.5\|0\.0"`
- `grep -q "processRewards\|process.reward" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Thresholds
- All checks: pass@1 = 1.0
