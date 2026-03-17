# Eval: Bump Version to 6.7.0 with Full Changelog

## Code Graders (bash commands that must exit 0)
- `grep -q "\[6.7.0\]" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q '"version": "6.7.0"' /Users/danleemh/ai/claude/evolve-loop/.claude-plugin/plugin.json`
- `grep -q "v6.7" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`

## Regression Evals (full test suite)
- `grep -q "\[6.6.0\]" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q "\[6.5.0\]" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q "evolve-loop" /Users/danleemh/ai/claude/evolve-loop/.claude-plugin/plugin.json`

## Acceptance Checks (verification commands)
- `grep -c "6.7.0" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md | grep -q "^[2-9]\|^[1-9][0-9]"`
- `grep -q "bandit\|Bandit\|counterfactual\|Counterfactual\|crossover\|Crossover\|novelty\|Novelty\|decision trace\|Decision Trace\|prerequisite\|Prerequisite" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`

## Thresholds
- All checks: pass@1 = 1.0
