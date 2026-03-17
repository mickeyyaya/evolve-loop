# Eval: add-llm-judge-eval-rubric

## Code Graders (bash commands that must exit 0)
- `grep -c "LLM-as-a-Judge\|llm-judge\|self-evaluation" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -c "binary\|rubric\|scoring" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Regression Evals (full test suite)
- `grep -c "Phase 5" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Acceptance Checks (verification commands)
- `grep -l "judge\|self-eval\|output quality" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -c "chain-of-thought\|step-by-step\|justification" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Thresholds
- All checks: pass@1 = 1.0
