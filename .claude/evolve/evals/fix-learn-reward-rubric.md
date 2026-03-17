# Eval: Fix Learn ProcessReward Rubric

## Code Graders (bash commands that must exit 0)
- `grep -A3 "learn" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | grep -i "cited\|citation\|applied"`
- `grep -c "| \*\*learn\*\*" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -A8 "\*\*learn\*\*" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -n "0\.5" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Thresholds
- All checks: pass@1 = 1.0
