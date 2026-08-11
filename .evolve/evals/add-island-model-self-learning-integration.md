# Eval: Add Island Model Cross-Reference in Self-Learning Doc

## Code Graders (bash commands that must exit 0)
- `grep -q "island-model" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -qi "self-learning\|architecture" /Users/danleemh/ai/claude/evolve-loop/docs/island-model.md`

## Regression Evals (full test suite)
- `grep -q "Seven Mechanisms\|Self-Improvement" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -q "Island Model" /Users/danleemh/ai/claude/evolve-loop/docs/island-model.md`
- `grep -q "Migration" /Users/danleemh/ai/claude/evolve-loop/docs/island-model.md`

## Acceptance Checks (verification commands)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md | awk '{exit ($1 > 290)}'`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/island-model.md | awk '{exit ($1 > 90)}'`

## Thresholds
- All checks: pass@1 = 1.0
