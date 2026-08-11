# Eval: add-adversarial-eval-coevolution-doc

## Code Graders (bash commands that must exit 0)

- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md | awk '{exit ($1 < 60 || $1 > 140)}'`
- `grep -qi "Code-A1\|adversarial" /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`
- `grep -qi "Mistake Book" /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`
- `grep -qi "2603.15611" /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`
- `grep -qi "eval grader\|grader bank\|evolve.loop" /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md`

## Regression Evals (full test suite)

- `test -f docs/research-paper-index.md && grep -q "2603.15611" docs/research-paper-index.md`

## Acceptance Checks (verification commands)

- `grep -c "##" /Users/danleemh/ai/claude/evolve-loop/docs/adversarial-eval-coevolution.md | awk '{exit ($1 < 3)}'`

## Thresholds

- All checks: pass@1 = 1.0
