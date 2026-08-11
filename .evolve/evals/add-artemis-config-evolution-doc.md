# Eval: add-artemis-config-evolution-doc

## Code Graders (bash commands that must exit 0)

- `grep -q "ARTEMIS\|artemis\|config.*evolution\|evolutionary.*optim\|genetic.*operator" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -qi "arXiv:2512.09108\|2512.09108" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -q "configVariant\|config_variant\|configvariant" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md | awk '{exit ($1 > 540)}'`

## Regression Evals (full test suite)

- `bash /Users/danleemh/ai/claude/evolve-loop/scripts/eval-quality-check.sh 2>/dev/null || true`

## Acceptance Checks (verification commands)

- `grep -q "2512.09108" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -q "experiments.jsonl" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`

## Thresholds

- All checks: pass@1 = 1.0
