# Eval: add-bandit-task-selector

## Code Graders (bash commands that must exit 0)
- `grep -c "bandit\|UCB\|Thompson\|exploration\|exploitation" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`
- `grep -c "arm\|reward\|pull" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`

## Regression Evals (full test suite)
- `grep -c "Strategy Presets" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`
- `grep -c "balanced\|innovate\|harden\|repair" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`

## Acceptance Checks (verification commands)
- `grep -c "taskArms\|armRewards\|banditState\|explorationRate" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -c "Multi-Armed Bandit\|bandit" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`

## Thresholds
- All checks: pass@1 = 1.0
