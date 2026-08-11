# Eval: Add Non-Code Eval Grader Patterns

## Code Graders (bash commands that must exit 0)
- `grep -q "## Non-Code Eval Graders" skills/evolve-loop/eval-runner.md`
- `grep -q "rubric" skills/evolve-loop/eval-runner.md`
- `grep -q "groundedness" skills/evolve-loop/eval-runner.md`
- `grep -q "LLM" skills/evolve-loop/eval-runner.md`

## Regression Evals (full test suite)
- N/A — no test runner. Regression check: existing bash grader sections must not be removed.
- `grep -q "exit 0" skills/evolve-loop/eval-runner.md`

## Acceptance Checks (verification commands)
- `grep -qi "coverage check\|coverage" skills/evolve-loop/eval-runner.md`
- `wc -l < skills/evolve-loop/eval-runner.md | awk '{exit ($1 > 230)}'`

## Thresholds
- All checks: pass@1 = 1.0
