# Eval: add-automated-threat-taxonomy

## Code Graders (bash commands that must exit 0)
- `grep -qi "reward hacking\|sandbagging\|data exfiltration\|chain-of-thought manipulation" /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`
- `grep -qi "2512.20677\|red.team" /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`

## Regression Evals (full test suite)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md | awk '{exit ($1 < 80 || $1 > 140)}'`

## Acceptance Checks (verification commands)
- `grep -q "Code-A1" /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`
- `grep -q "Mistake Book" /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`

## Thresholds
- All checks: pass@1 = 1.0
