# Eval: Fix Link-Checker Eval Grader Regex

## Code Graders (bash commands that must exit 0)

- `grep -q "grep -oP" skills/evolve-loop/benchmark-eval.md`
- `grep -q "grep -oP" docs/eval-grader-best-practices.md`
- `grep -q "grep -oP" .evolve/evals/fix-skill-doc-internal-links.md`

## Regression Evals (full test suite)

- `grep -roh '\]\([^)]*\.md\)' skills/ agents/ docs/ 2>/dev/null | tr -d '()' | while read f; do test -f "$f" || echo "$f"; done | wc -l | awk '{exit ($1 >= 3)}'`

## Acceptance Checks (verification commands)

- `grep -c "grep -oP" skills/evolve-loop/benchmark-eval.md | awk '{exit ($1 < 1)}'`
- `grep -q "link text\|href\|false.positive\|false positive" docs/eval-grader-best-practices.md`

## Thresholds
- All checks: pass@1 = 1.0
