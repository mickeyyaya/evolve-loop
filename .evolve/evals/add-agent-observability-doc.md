# Eval: add-agent-observability-doc

## Code Graders (bash commands that must exit 0)

- `test -f docs/agent-observability.md`
- `wc -l < docs/agent-observability.md | awk '{exit ($1 < 60 || $1 > 130)}'`
- `grep -q "trajectory" docs/agent-observability.md`
- `grep -q "state tracking\|state-tracking" docs/agent-observability.md`
- `grep -q "reasoning trace\|reasoning traces" docs/agent-observability.md`
- `grep -q "tool.call\|tool call" docs/agent-observability.md`
- `grep -q "2602.06841" docs/agent-observability.md`

## Regression Evals (full test suite)

- `grep -rq "agent-observability" docs/research-paper-index.md`

## Acceptance Checks (verification commands)

- `grep -q "agent-observability.md" docs/research-paper-index.md`

## Thresholds
- All checks: pass@1 = 1.0
