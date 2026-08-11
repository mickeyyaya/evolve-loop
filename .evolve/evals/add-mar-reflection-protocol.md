# Eval: add-mar-reflection-protocol

## Code Graders (bash commands that must exit 0)
- `grep -qi "MAR\|multi-agent reflex" /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`
- `grep -qi "degenerat" /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`
- `grep -qi "2512.20845" /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`

## Regression Evals (full test suite)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`

## Acceptance Checks (verification commands)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md | awk '{exit ($1 < 100 || $1 > 140)}'`
- `grep -q "Code-A1" /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`
- `grep -qi "MAR" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Thresholds
- All checks: pass@1 = 1.0
