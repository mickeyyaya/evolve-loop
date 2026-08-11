# Eval: add-bats-budget-aware-scaling

## Code Graders (bash commands that must exit 0)

- `grep -q "BATS\|Budget Aware\|budget-aware\|budget tracker" /Users/danleemh/ai/claude/evolve-loop/docs/performance-profiling.md`
- `grep -q "Pareto\|remaining_budget\|budgetRemaining\|budget remaining" /Users/danleemh/ai/claude/evolve-loop/docs/performance-profiling.md`
- `grep -q "arXiv:2511.17006\|BATS" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Regression Evals (full test suite)

- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/performance-profiling.md | awk '{exit ($1 > 200)}'`

## Acceptance Checks (verification commands)

- `grep -c "BATS\|budget-aware\|Budget Aware" /Users/danleemh/ai/claude/evolve-loop/docs/performance-profiling.md | awk '{exit ($1 < 1)}'`
- `grep -q "Cycle 144" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Thresholds

- All checks: pass@1 = 1.0
