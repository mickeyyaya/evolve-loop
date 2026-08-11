# Eval: Add Mutation Testing Pattern to Eval Runner

## Code Graders (bash commands that must exit 0)
- `grep -ci "mutation\|mutant\|kill.rate\|mutation.*test" skills/evolve-loop/eval-runner.md | awk '{exit ($1 < 3)}'`
- `wc -l < skills/evolve-loop/eval-runner.md | awk '{exit ($1 > 310)}'`
- `grep -q "## Mutation" skills/evolve-loop/eval-runner.md`

## Regression Evals (full test suite)
- `grep -q "## Eval Definition Format" skills/evolve-loop/eval-runner.md`
- `grep -q "## Retry Protocol" skills/evolve-loop/eval-runner.md`
- `grep -q "## Non-Code Eval Graders" skills/evolve-loop/eval-runner.md`
- `grep -q "## Benchmark Eval Execution" skills/evolve-loop/eval-runner.md`

## Acceptance Checks (verification commands)
- `grep -qi "mutation" skills/evolve-loop/eval-runner.md`
- `grep -q "kill" skills/evolve-loop/eval-runner.md`
- `grep -c "^##" skills/evolve-loop/eval-runner.md | awk '{exit ($1 < 6)}'`

## Thresholds
- All checks: pass@1 = 1.0
