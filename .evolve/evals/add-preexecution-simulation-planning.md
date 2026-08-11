# Eval: add-preexecution-simulation-planning

## Code Graders (bash commands that must exit 0)
- `grep -qi "pre.execution\|simul\|WebDreamer\|world model" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-coordination.md`
- `grep -qi "2411.06559\|deliberat" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-coordination.md`

## Regression Evals (full test suite)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-coordination.md | awk '{exit ($1 < 108 || $1 > 160)}'`

## Acceptance Checks (verification commands)
- `grep -q "AdaptOrch" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-coordination.md`
- `grep -q "research-paper-index" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-coordination.md`

## Thresholds
- All checks: pass@1 = 1.0
