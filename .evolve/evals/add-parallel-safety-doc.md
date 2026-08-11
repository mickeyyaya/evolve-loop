# Eval: Add Parallel Safety Reference Doc

## Code Graders (bash commands that must exit 0)

- `test -f docs/parallel-safety.md`
- `grep -c "OCC\|optimistic concurrency\|version" docs/parallel-safety.md`
- `grep -ic "ship-lock\|ship lock" docs/parallel-safety.md`
- `grep -c "parallel-safety\|parallel safety\|parallel-safety.md" docs/architecture.md`
- `wc -l < docs/parallel-safety.md | awk '{exit ($1 < 40 || $1 > 80)}'`

## Regression Evals (full test suite)

- `grep -c "## Reference Documents" docs/architecture.md | awk '{exit ($1 < 1)}'`

## Acceptance Checks (verification commands)

- `grep -c "run-isolation\|run isolation" docs/parallel-safety.md | awk '{exit ($1 < 1)}'` — references run-isolation
- `grep -c "RUN_ID\|runId\|run ID" docs/parallel-safety.md | awk '{exit ($1 < 1)}'` — run isolation model covered

## Thresholds
- All checks: pass@1 = 1.0
