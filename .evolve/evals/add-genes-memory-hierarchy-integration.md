# Eval: Add Genes Doc Link in Memory Hierarchy Layer 6

## Code Graders (bash commands that must exit 0)
- `grep -q "genes\.md" /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md`
- `grep -q "memory-hierarchy" /Users/danleemh/ai/claude/evolve-loop/docs/genes.md`

## Regression Evals (full test suite)
- `grep -q "Layer 6" /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md`
- `grep -q "Gene Schema" /Users/danleemh/ai/claude/evolve-loop/docs/genes.md`
- `grep -q "successCount" /Users/danleemh/ai/claude/evolve-loop/docs/genes.md`

## Acceptance Checks (verification commands)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md | awk '{exit ($1 > 200)}'`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/genes.md | awk '{exit ($1 > 100)}'`

## Thresholds
- All checks: pass@1 = 1.0
