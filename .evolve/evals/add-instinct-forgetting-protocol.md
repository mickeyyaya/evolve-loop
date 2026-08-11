# Eval: add-instinct-forgetting-protocol

## Code Graders (bash commands that must exit 0)
- `grep -qi "forgetting\|consolidat\|stale.*instinct\|instinct.*decay\|discard" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md`
- `grep -qi "causal\|usage.*frequency\|zero.*use\|merge" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md`
- `grep -qi "2505.00675\|2603.07670\|AgeMem\|continual.*consolidat\|memory.*operat" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md`

## Regression Evals (full test suite)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md | awk '{exit ($1 > 220)}'`

## Acceptance Checks (verification commands)
- `grep -q "Graduation\|graduation" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md`

## Thresholds
- All checks: pass@1 = 1.0
