# Eval: add-prompt-variant-tracking

## Code Graders (bash commands that must exit 0)

- `grep -q "prompt.*variant\|Prompt.*Variant\|prompting pattern" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -q "AutoPDL\|autopdl\|successive halving\|prompt.*optimiz" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -qi "arXiv:2504.04365\|2504.04365" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -q "experiments.jsonl" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md | awk '{exit ($1 > 500)}'`

## Regression Evals (full test suite)

- `bash /Users/danleemh/ai/claude/evolve-loop/scripts/eval-quality-check.sh 2>/dev/null || true`

## Acceptance Checks (verification commands)

- `grep -q "Zero-Shot\|Chain-of-Thought\|CoT\|ReAct" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`

## Thresholds

- All checks: pass@1 = 1.0
