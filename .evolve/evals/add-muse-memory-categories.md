# Eval: Add MUSE Hierarchical Memory Categories

## Code Graders (bash commands that must exit 0)
- `grep -q 'Functional Memory Categories' docs/memory-hierarchy.md`
- `grep -q 'strategic' docs/memory-hierarchy.md`
- `grep -q 'tool-use' docs/memory-hierarchy.md`
- `grep -q 'procedural' docs/memory-hierarchy.md`

## Regression Evals (full test suite)
- `grep -roh '\[.*\]([^)]*\.md)' skills/ agents/ docs/ 2>/dev/null | grep -oE '\([^)]+\)' | tr -d '()' | while read f; do test -f "$f" || echo "$f"; done | wc -l | awk '{exit ($1 > 2)}'`

## Acceptance Checks (verification commands)
- `grep -qi 'memory-hierarchy' docs/self-learning.md`
- `wc -l < docs/memory-hierarchy.md | awk '{exit ($1 > 220)}'`
- `grep -c '##' docs/memory-hierarchy.md | awk '{exit ($1 < 5)}'`

## Thresholds
- All checks: pass@1 = 1.0
