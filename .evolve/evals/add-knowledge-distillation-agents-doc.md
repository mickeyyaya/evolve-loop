# Eval: add-knowledge-distillation-agents-doc

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/knowledge-distillation-agents.md`
- `grep -qi "distillation\|knowledge.transfer" /Users/danleemh/ai/claude/evolve-loop/docs/knowledge-distillation-agents.md`
- `grep -qi "reasoning.trace\|chain.of.thought\|multi.teacher" /Users/danleemh/ai/claude/evolve-loop/docs/knowledge-distillation-agents.md`
- `grep -qi "cross.generation\|compress\|transfer" /Users/danleemh/ai/claude/evolve-loop/docs/knowledge-distillation-agents.md`

## Regression Evals (full test suite)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/knowledge-distillation-agents.md | awk '{exit ($1 < 80)}'`

## Acceptance Checks (verification commands)
- `grep -qi "gene\|instinct\|evolve.loop" /Users/danleemh/ai/claude/evolve-loop/docs/knowledge-distillation-agents.md`
- `grep -qi "cross.modal\|efficiency\|compression" /Users/danleemh/ai/claude/evolve-loop/docs/knowledge-distillation-agents.md`
- `head -1 /Users/danleemh/ai/claude/evolve-loop/docs/knowledge-distillation-agents.md | grep -q "^#"`

## Thresholds
- All checks: pass@1 = 1.0
