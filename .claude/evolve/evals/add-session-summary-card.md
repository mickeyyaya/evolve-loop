# Eval: Add Session Summary Card to Operator Output

## Code Graders (bash commands that must exit 0)
- `grep -qi "session summary\|session-summary" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md`
- `grep -qi "session.summary\|session-summary" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`

## Regression Evals (full test suite)
- n/a (documentation project — no test suite)

## Acceptance Checks (verification commands)
- `grep -c "isLastCycle\|last.*cycle\|lastCycle" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md | grep -vq "^0$"`
- `grep -c "session-summary\.md" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md | grep -vq "^0$"`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/agents/evolve-operator.md | awk '{exit ($1 > 800)}'`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md | awk '{exit ($1 > 800)}'`

## Thresholds
- All checks: pass@1 = 1.0
