# Eval: add-operator-next-cycle-brief

## Code Graders (bash commands that must exit 0)
- `grep -c "next-cycle-brief.json" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md | grep -E '^[3-9]|^[0-9]{2,}'`
- `grep -c "next-cycle-brief.json" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md | grep -E '^[1-9]'`
- `grep -c "next-cycle-brief.json" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md | grep -E '^[2-9]'`
- `grep -c "weakestDimension" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md | grep -E '^[1-9]'`
- `grep -c "taskTypeBoosts" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md | grep -E '^[1-9]'`

## Regression Evals (full test suite)
- `grep -c "MAP-Elites" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md | grep -E '^[1-9]'`
- `grep -c "HALT" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md | grep -E '^[1-9]'`

## Acceptance Checks (verification commands)
- `grep "next-cycle-brief.json" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md`
- `grep "next-cycle-brief.json" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep "next-cycle-brief.json" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep "weakestDimension" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md`

## Thresholds
- All checks: pass@1 = 1.0
