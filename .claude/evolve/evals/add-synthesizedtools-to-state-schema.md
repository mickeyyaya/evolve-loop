# Eval: add-synthesizedtools-to-state-schema

## Code Graders (bash commands that must exit 0)
- `grep -q "synthesizedTools" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md && echo OK`
- `grep -q "synthesizedTools" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md && echo OK`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -q '"synthesizedTools"' /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md && echo "init schema OK"`

## Thresholds
- All checks: pass@1 = 1.0
