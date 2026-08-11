# Eval: add-agent-error-taxonomy-to-accuracy-doc

## Code Graders (bash commands that must exit 0)
- `grep -qi "AgentDebug\|AgentErrorTaxonomy\|2509.25370" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md`
- `grep -qi "memory.*error\|reflection.*error\|planning.*error\|action.*error\|system.*error" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md`
- `grep -qi "targeted.*feedback\|root.cause\|iterative.*recover" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md`

## Regression Evals (full test suite)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md | awk '{exit ($1 > 250)}'`

## Acceptance Checks (verification commands)
- `grep -c "##" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md | awk '{exit ($1 < 6)}'`

## Thresholds
- All checks: pass@1 = 1.0
