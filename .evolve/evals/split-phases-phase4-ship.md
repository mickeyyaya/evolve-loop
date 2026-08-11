# Eval: Split Phase 4 SHIP into Dedicated Skill File

## Code Graders (bash commands that must exit 0)
- `test -f skills/evolve-loop/phase4-ship.md`
- `grep -q 'phase4-ship' skills/evolve-loop/phases.md`
- `wc -l < skills/evolve-loop/phases.md | awk '{exit ($1 >= 600)}'`
- `grep -q 'name:' skills/evolve-loop/phase4-ship.md`
- `grep -q 'SHIP' skills/evolve-loop/phase4-ship.md`

## Regression Evals (full test suite)
- `grep -q 'Phase 5' skills/evolve-loop/phases.md`
- `grep -q 'Phase 4' skills/evolve-loop/phases.md`
- `grep -roh '\[.*\]([^)]*\.md)' skills/ agents/ docs/ 2>/dev/null | grep -oE '\([^)]+\)' | tr -d '()' | while read f; do test -f "$f" || echo "$f"; done | wc -l | awk '{exit ($1 > 2)}'`

## Acceptance Checks (verification commands)
- `grep -q '^---' skills/evolve-loop/phase4-ship.md`
- `grep -q 'description:' skills/evolve-loop/phase4-ship.md`
- `grep -q 'ship-lock\|SHIP Lock\|ship lock' skills/evolve-loop/phase4-ship.md`
- `find skills/evolve-loop/ -name "*.md" | wc -l | awk '{exit ($1 < 7)}'`

## Thresholds
- All checks: pass@1 = 1.0
