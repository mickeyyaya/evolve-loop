# Eval: Regenerate Project Digest (Cycle 19)

## Code Graders (bash commands that must exit 0)

- `grep -q "phase4-ship.md" .evolve/runs/run-17739641253N-65b2/workspace/project-digest.md`
- `grep -q "eval-grader-best-practices.md" .evolve/runs/run-17739641253N-65b2/workspace/project-digest.md`
- `grep -qE "Cycle 19|Generated Cycle 19" .evolve/runs/run-17739641253N-65b2/workspace/project-digest.md`

## Regression Evals (full test suite)

- `test -s .evolve/runs/run-17739641253N-65b2/workspace/project-digest.md`

## Acceptance Checks (verification commands)

- `grep "phases.md" .evolve/runs/run-17739641253N-65b2/workspace/project-digest.md | grep -qv "672"` — phases.md no longer listed at 672 lines
- `grep -q "## Structure" .evolve/runs/run-17739641253N-65b2/workspace/project-digest.md` — structure section present
- `grep -q "## Hotspots" .evolve/runs/run-17739641253N-65b2/workspace/project-digest.md` — hotspots section present
- `grep -q "## Recent History" .evolve/runs/run-17739641253N-65b2/workspace/project-digest.md` — recent history present

## Thresholds
- All checks: pass@1 = 1.0
