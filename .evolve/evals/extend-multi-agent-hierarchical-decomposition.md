# Eval: extend-multi-agent-hierarchical-decomposition

## Code Graders (bash commands that must exit 0)

- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-coordination.md | awk '{exit ($1 < 100 || $1 > 160)}'`
- `grep -qi "hierarchical\|TextGrad\|decompos" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-coordination.md`
- `grep -qi "2602.21670" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-coordination.md`
- `grep -qi "PDDL\|meta-prompt\|prompt optimization" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-coordination.md`
- `grep -q "AdaptOrch" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-coordination.md`

## Regression Evals (full test suite)

- `test -f docs/research-paper-index.md && grep -q "2602.21670" docs/research-paper-index.md`

## Acceptance Checks (verification commands)

- `grep -qi "evolve.loop\|scout\|builder" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-coordination.md`

## Thresholds

- All checks: pass@1 = 1.0
