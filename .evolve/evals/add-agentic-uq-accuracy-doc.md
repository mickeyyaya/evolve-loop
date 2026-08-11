# Eval: add-agentic-uq-accuracy-doc

## Code Graders (bash commands that must exit 0)

- `grep -qi "AUQ\|agentic uncertainty\|UAM\|UAR" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md`
- `grep -qi "action-conditional\|state-changing\|information-gathering" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md`
- `grep -qi "2602.05073\|2601.15703" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md`

## Regression Evals (full test suite)

- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md | awk '{exit ($1 > 250)}'`

## Acceptance Checks (verification commands)

- `grep -q "Anti-Conformity" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md`
- `grep -q "AgentDebug\|Five-dimension" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md`

## Thresholds

- All checks: pass@1 = 1.0
