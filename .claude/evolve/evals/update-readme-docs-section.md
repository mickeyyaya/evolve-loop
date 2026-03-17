# Eval: Update README Docs Section

## Code Graders (bash commands that must exit 0)
- `grep -q "token-optimization" /Users/danleemh/ai/claude/evolve-loop/README.md`
- `grep -q "self-learning" /Users/danleemh/ai/claude/evolve-loop/README.md`
- `grep -q "memory-hierarchy" /Users/danleemh/ai/claude/evolve-loop/README.md`

## Regression Evals (full test suite)
- `test -f /Users/danleemh/ai/claude/evolve-loop/README.md`

## Acceptance Checks (verification commands)
- `grep -c "docs/" /Users/danleemh/ai/claude/evolve-loop/README.md`

## Thresholds
- All checks: pass@1 = 1.0
