# Eval: add-cgi-actionable-critique

## Code Graders (bash commands that must exit 0)
- `grep -qi "CGI\|critique-guided\|Lighthouse" /Users/danleemh/ai/claude/evolve-loop/docs/agent-observability.md`
- `grep -qi "actionable" /Users/danleemh/ai/claude/evolve-loop/docs/agent-observability.md`
- `grep -qi "2503.16024" /Users/danleemh/ai/claude/evolve-loop/docs/agent-observability.md`

## Regression Evals (full test suite)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/agent-observability.md`

## Acceptance Checks (verification commands)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/agent-observability.md | awk '{exit ($1 < 100 || $1 > 130)}'`
- `grep -q "State Tracking" /Users/danleemh/ai/claude/evolve-loop/docs/agent-observability.md`
- `grep -qi "CGI\|2503.16024" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Thresholds
- All checks: pass@1 = 1.0
