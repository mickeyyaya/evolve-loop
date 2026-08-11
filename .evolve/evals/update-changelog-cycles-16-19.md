# Eval: Update CHANGELOG for Cycles 16-19

## Code Graders (bash commands that must exit 0)

- `grep -q "7.2.0\|8.0.0" CHANGELOG.md`
- `grep -q "stepwise\|Stepwise" CHANGELOG.md`
- `grep -q "CSI\|Coefficient of Self-Improvement" CHANGELOG.md`
- `grep -q "confidence-correctness\|Confidence-Correctness" CHANGELOG.md`
- `grep -q "MUSE\|muse memory" CHANGELOG.md`
- `grep -q "phase4-ship\|Phase 4.*ship\|phase4" CHANGELOG.md`
- `grep -q "7.1.0" CHANGELOG.md`

## Regression Evals (full test suite)

- `grep -roh '\]([^)]*\.md)' skills/ agents/ docs/ 2>/dev/null | grep -oE '\([^)]+\)' | tr -d '()' | while read f; do test -f "$f" || echo "BROKEN: $f"; done | wc -l | awk '{exit ($1 > 2)}'`

## Acceptance Checks (verification commands)

- `grep -c "^## \[" CHANGELOG.md | awk '{exit ($1 < 2)}'` — at least 2 version entries exist
- `head -30 CHANGELOG.md | grep -qE "## \[7\.[2-9]\|8\."` — new entry is at top

## Thresholds
- All checks: pass@1 = 1.0
