# Eval: add-agent-failure-tracing-doc

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/agent-failure-tracing.md`
- `grep -qi "AgentRx\|violation.log\|root.cause" /Users/danleemh/ai/claude/evolve-loop/docs/agent-failure-tracing.md`
- `grep -qi "cascading.fail\|failure.propagation" /Users/danleemh/ai/claude/evolve-loop/docs/agent-failure-tracing.md`
- `grep -qi "memory\|reflection\|planning\|action" /Users/danleemh/ai/claude/evolve-loop/docs/agent-failure-tracing.md`

## Regression Evals (full test suite)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/agent-failure-tracing.md | awk '{exit ($1 < 80)}'`

## Acceptance Checks (verification commands)
- `grep -qi "recovery\|remediation" /Users/danleemh/ai/claude/evolve-loop/docs/agent-failure-tracing.md`
- `grep -qi "evolve.loop\|phase.gate\|auditor" /Users/danleemh/ai/claude/evolve-loop/docs/agent-failure-tracing.md`
- `head -1 /Users/danleemh/ai/claude/evolve-loop/docs/agent-failure-tracing.md | grep -q "^#"`

## Thresholds
- All checks: pass@1 = 1.0
