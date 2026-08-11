# Eval: Wire Operator Brief Benchmark Sync

## Code Graders (bash commands that must exit 0)
- `grep -q "benchmarkWeaknesses\|benchmark.*weak\|weakest.*dimension\|taskTypeBoosts" agents/evolve-operator.md`
- `wc -l < agents/evolve-operator.md | awk '{exit ($1 > 260)}'`
- `grep -q "next-cycle-brief" agents/evolve-operator.md`
- `grep -q "taskTypeBoosts\|task.*type.*boost" agents/evolve-operator.md`

## Regression Evals (full test suite)
- `grep -q "## Responsibilities" agents/evolve-operator.md`
- `grep -q "## Next-Cycle Brief" agents/evolve-operator.md`
- `grep -q "## HALT Protocol" agents/evolve-operator.md`
- `grep -q "recommendedStrategy\|weakestDimension" agents/evolve-operator.md`

## Acceptance Checks (verification commands)
- `grep -qi "benchmark" agents/evolve-operator.md | wc -l | awk '{exit ($1 < 1)}'`
- `grep -c "benchmark" agents/evolve-operator.md | awk '{exit ($1 < 3)}'`

## Thresholds
- All checks: pass@1 = 1.0
