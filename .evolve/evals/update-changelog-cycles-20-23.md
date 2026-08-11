# Eval: Update Changelog Cycles 20-23

## Code Graders (bash commands that must exit 0)

- `grep -c "scout-discovery-guide\|scout discovery guide\|extract.*scout\|Scout Discovery" CHANGELOG.md`
- `grep -c "operator-brief\|operator brief\|Operator Brief" CHANGELOG.md`
- `grep -c "run-isolation\|run isolation\|Run Isolation" CHANGELOG.md`
- `grep -c "security.*self-check\|security self.check\|Security Self" CHANGELOG.md`
- `grep -iE "^\## \[7\.[3-9]\.[0-9]|^\## \[8\." CHANGELOG.md`

## Regression Evals (full test suite)

- `grep -c "^\## \[" CHANGELOG.md | awk '{exit ($1 < 10)}'`

## Acceptance Checks (verification commands)

- `head -10 CHANGELOG.md | grep -E "\[7\.[3-9]|8\.[0-9]"` — new version entry at top
- `tail -20 CHANGELOG.md | grep -cE "\[7\.[3-9]|8\.[0-9]\]:"` — version links updated

## Thresholds
- All checks: pass@1 = 1.0
