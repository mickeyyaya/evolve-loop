# Eval: add-dccd-structured-output-pattern

## Code Graders (bash commands that must exit 0)

- `grep -qi "DCCD\|Draft-Conditioned\|constrained decoding\|two-stage" /Users/danleemh/ai/claude/evolve-loop/docs/performance-profiling.md`
- `grep -qi "projection tax\|feasible mass\|structured output" /Users/danleemh/ai/claude/evolve-loop/docs/performance-profiling.md`
- `grep -qi "2603.03305\|2501.10868" /Users/danleemh/ai/claude/evolve-loop/docs/performance-profiling.md`

## Regression Evals (full test suite)

- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/performance-profiling.md | awk '{exit ($1 > 200)}'`

## Acceptance Checks (verification commands)

- `grep -q "Budget-Aware\|BATS" /Users/danleemh/ai/claude/evolve-loop/docs/performance-profiling.md`
- `grep -q "Model Routing\|tier-1\|tier-2" /Users/danleemh/ai/claude/evolve-loop/docs/performance-profiling.md`

## Thresholds

- All checks: pass@1 = 1.0
