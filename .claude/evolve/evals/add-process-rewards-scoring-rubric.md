# Eval: add-process-rewards-scoring-rubric

## Code Graders (bash commands that must exit 0)
- `grep -q "discover.*=" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md || grep -q "discover score\|discover:" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -qE "1\.0|0\.5|0\.0" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -q "scoring\|rubric\|formula\|= 1.0\|= 0.5\|= 0.0" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -c "processRewards" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | grep -qv "^0$"`

## Thresholds
- All checks: pass@1 = 1.0
