# Eval: Add Context Engineering Maturity Model (CEMM) Checklist

## Code Graders (bash commands that must exit 0)

- `grep -q "CEMM\|Context Engineering Maturity" skills/evolve-loop/phases.md`
- `grep -q "Relevance" skills/evolve-loop/phases.md`
- `grep -q "Sufficiency" skills/evolve-loop/phases.md`
- `grep -q "Isolation" skills/evolve-loop/phases.md`
- `grep -q "Economy" skills/evolve-loop/phases.md`
- `grep -q "Provenance" skills/evolve-loop/phases.md`

## Regression Evals (full test suite)

- `grep -q "Phase 1: DISCOVER" skills/evolve-loop/phases.md`
- `grep -q "Inter-Phase Handoff Format" skills/evolve-loop/phases.md`

## Acceptance Checks (verification commands)

- `wc -l < skills/evolve-loop/phases.md | awk '{exit ($1 > 680)}'`
- `git diff --name-only HEAD | grep -v "phases.md" | wc -l | awk '{exit ($1 > 0)}'` (only phases.md modified)

## Thresholds

- All checks: pass@1 = 1.0
