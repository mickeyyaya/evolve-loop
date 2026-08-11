# Eval: Fix Skill and Doc Internal Link Paths

## Code Graders (bash commands that must exit 0)

- `grep -roh '\[.*\]([^)]*\.md)' skills/ agents/ docs/ 2>/dev/null | grep -oE '\([^)]+\)' | tr -d '()' | while read f; do test -f "$f" || echo "BROKEN: $f"; done | grep -c "BROKEN" | awk '{exit ($1 >= 6)}'`
- `grep -q 'skills/evolve-loop/benchmark-eval.md' skills/evolve-loop/phases.md`
- `grep -q 'skills/evolve-loop/phase5-learn.md' skills/evolve-loop/phases.md`
- `grep -q 'skills/evolve-loop/benchmark-eval.md' skills/evolve-loop/eval-runner.md`

## Regression Evals (full test suite)

- `grep -roh '\[.*\]([^)]*\.md)' skills/ agents/ docs/ 2>/dev/null | grep -oE '\([^)]+\)' | tr -d '()' | while read f; do test -f "$f" || echo "BROKEN: $f"; done | grep -c "BROKEN" | awk '{exit ($1 >= 6)}'`

## Acceptance Checks (verification commands)

- `grep -c 'skills/evolve-loop/' skills/evolve-loop/SKILL.md | awk '{exit ($1 < 1)}'`
- `grep -q 'docs/architecture.md\|docs/self-learning.md\|docs/domain-adapters.md' docs/architecture.md || grep -q 'docs/' docs/architecture.md`

## Thresholds
- All checks: pass@1 = 1.0
