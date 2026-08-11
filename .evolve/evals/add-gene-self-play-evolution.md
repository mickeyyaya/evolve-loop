# Eval: Add Gene Self-Play Evolution Section

## Code Graders (bash commands that must exit 0)
- `grep -qi "Tool-R0\|self-play\|co-evol" /Users/danleemh/ai/claude/evolve-loop/docs/genes.md`
- `grep -qi "2602.21320" /Users/danleemh/ai/claude/evolve-loop/docs/genes.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/genes.md | awk '{exit ($1 > 140)}'`

## Regression Evals (full test suite)
- `grep -q "successCount" /Users/danleemh/ai/claude/evolve-loop/docs/genes.md`
- `grep -q "Gene Extraction" /Users/danleemh/ai/claude/evolve-loop/docs/genes.md`

## Acceptance Checks (verification commands)
- `grep -qi "adversarial\|curriculum\|self-play" /Users/danleemh/ai/claude/evolve-loop/docs/genes.md`
- `test $(wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/genes.md) -gt 91`

## Thresholds
- All checks: pass@1 = 1.0
