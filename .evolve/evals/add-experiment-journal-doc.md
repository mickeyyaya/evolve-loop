# Eval: Add Experiment Journal Doc

## Code Graders (bash commands that must exit 0)
- `test -f docs/experiment-journal.md`

## Regression Evals (full test suite)
- `test -d docs && test "$(find docs/ -name '*.md' -newer .evolve/evals/add-experiment-journal-doc.md 2>/dev/null | head -1)" != "" || test -f docs/experiment-journal.md`

## Acceptance Checks (verification commands)
- `grep -q "experiments.jsonl" docs/experiment-journal.md`
- `grep -q "verdict" docs/experiment-journal.md`
- `grep -q "experiment-journal" docs/architecture.md`
- `wc -l < docs/experiment-journal.md | awk '{exit ($1 < 30 || $1 > 120)}'`

## Thresholds
- All checks: pass@1 = 1.0
