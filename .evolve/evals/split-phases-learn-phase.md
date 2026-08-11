# Eval: Split phases.md — Extract Phase 5 LEARN into Standalone Doc

## Code Graders (bash commands that must exit 0)

- `test -f skills/evolve-loop/phase5-learn.md`
- `grep -q "Phase 5" skills/evolve-loop/phase5-learn.md`
- `grep -q "phase5-learn.md" skills/evolve-loop/phases.md`

## Regression Evals (full test suite)

- `grep -q "Phase 0" skills/evolve-loop/phases.md`
- `grep -q "Phase 1" skills/evolve-loop/phases.md`
- `grep -q "Phase 2" skills/evolve-loop/phases.md`
- `grep -q "Phase 3" skills/evolve-loop/phases.md`
- `grep -q "Phase 4" skills/evolve-loop/phases.md`
- `grep -q "Phase 5" skills/evolve-loop/phases.md`

## Acceptance Checks (verification commands)

- `wc -l < skills/evolve-loop/phases.md | awk '{exit ($1 > 800)}'` → phases.md drops below 800 lines
- `grep -c "^#" skills/evolve-loop/phase5-learn.md | awk '{exit ($1 < 3)}'` → phase5-learn.md has at least 3 headings
- `grep -q "phase5-learn" skills/evolve-loop/phases.md` → phases.md links to the new file

## Thresholds

- All checks: pass@1 = 1.0
