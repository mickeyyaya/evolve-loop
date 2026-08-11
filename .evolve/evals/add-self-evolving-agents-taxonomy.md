# Eval: Add Self-Evolving Agents Taxonomy

## Code Graders (bash commands that must exit 0)

- `grep -qi "self-evolving" docs/self-learning.md`
- `grep -q "2507.21046" docs/self-learning.md`
- `grep -qi "taxonomy" docs/self-learning.md`
- `grep -qiE "memory mechanism|training signal|action space" docs/self-learning.md`

## Regression Evals (full test suite)

- `grep -roh '\]([^)]*\.md)' skills/ agents/ docs/ 2>/dev/null | grep -oE '\([^)]+\)' | tr -d '()' | while read f; do test -f "$f" || echo "BROKEN: $f"; done | wc -l | awk '{exit ($1 > 2)}'`

## Acceptance Checks (verification commands)

- `wc -l < docs/self-learning.md | awk '{exit ($1 > 260)}'` — file stays under 260 lines (net add ≤ 42 lines from current 218)
- `grep -qiE "evolve-loop|instinct|\.evolve" docs/self-learning.md` — taxonomy mapped to evolve-loop mechanisms

## Thresholds
- All checks: pass@1 = 1.0
