# Eval: add-multi-agent-blackboard-doc

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-blackboard.md`
- `grep -qi "blackboard\|shared.state\|shared.memory" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-blackboard.md`
- `grep -qi "access.control\|slot\|read.write" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-blackboard.md`
- `grep -qi "consistency\|synchronization\|concurrency" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-blackboard.md`

## Regression Evals (full test suite)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-blackboard.md | awk '{exit ($1 < 80)}'`

## Acceptance Checks (verification commands)
- `grep -qi "message.passing\|file.passing" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-blackboard.md`
- `grep -qi "evolve.loop\|scout\|builder\|auditor" /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-blackboard.md`
- `head -1 /Users/danleemh/ai/claude/evolve-loop/docs/multi-agent-blackboard.md | grep -q "^#"`

## Thresholds
- All checks: pass@1 = 1.0
