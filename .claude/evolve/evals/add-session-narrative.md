# Eval: add-session-narrative

## Code Graders (bash commands that must exit 0)
- `grep -c "Session Narrative" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md | grep -E '^[2-9]'`
- `grep -c "narrative" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md | grep -E '^[3-9]|^[0-9]{2,}'`

## Regression Evals (full test suite)
- `grep -c "HALT" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md | grep -E '^[1-9]'`
- `grep -c "operator-log.md" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md | grep -E '^[1-9]'`

## Acceptance Checks (verification commands)
- `grep "Session Narrative" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md`
- `grep -A 5 "Session Narrative" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md`

## Thresholds
- All checks: pass@1 = 1.0
