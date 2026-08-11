# Eval: Add Eval Grader Best Practices

## Code Graders (bash commands that must exit 0)
- `test -f docs/eval-grader-best-practices.md`
- `wc -l < docs/eval-grader-best-practices.md | awk '{exit ($1 < 60)}'`
- `wc -l < docs/eval-grader-best-practices.md | awk '{exit ($1 > 200)}'`
- `grep -q "best practice\|Best Practice" docs/eval-grader-best-practices.md`
- `grep -q "exit 0\|exit code\|grep\|wc -l" docs/eval-grader-best-practices.md`

## Regression Evals (full test suite)
- `grep -q "## Non-Code Eval Graders" skills/evolve-loop/eval-runner.md`
- `grep -q "## Mutation Testing" skills/evolve-loop/eval-runner.md`
- `grep -q "## Benchmark Eval Execution" skills/evolve-loop/eval-runner.md`

## Acceptance Checks (verification commands)
- `grep -qi "eval.grader\|grader.*best\|best.*practice" docs/eval-grader-best-practices.md`
- `grep -q "eval-grader-best-practices\|eval_grader_best" skills/evolve-loop/eval-runner.md`

## Thresholds
- All checks: pass@1 = 1.0
