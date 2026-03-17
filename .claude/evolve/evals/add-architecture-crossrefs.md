# Eval: Add Architecture Cross-References

## Code Graders (bash commands that must exit 0)
- `grep -q "token-optimization.md" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -q "self-learning.md" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -q "memory-hierarchy.md" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`

## Regression Evals (full test suite)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`

## Acceptance Checks (verification commands)
- `grep -c "docs/" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`

## Thresholds
- All checks: pass@1 = 1.0
