# Eval: Add Prompt Caching API Guidance to phases.md

## Code Graders (bash commands that must exit 0)
- `grep -q "cache_control" skills/evolve-loop/phases.md`
- `grep -q "ephemeral" skills/evolve-loop/phases.md`
- `grep -q "prompt.cach" skills/evolve-loop/phases.md`

## Regression Evals (full test suite)
- `test -f skills/evolve-loop/phases.md`

## Acceptance Checks (verification commands)
- `wc -l < skills/evolve-loop/phases.md | awk '{exit ($1 > 700)}'`
- `grep -q "NEVER launch the Builder without" skills/evolve-loop/phases.md`
- `grep -c "cache_control\|ephemeral" skills/evolve-loop/phases.md | awk '{exit ($1 < 2)}'`

## Thresholds
- All checks: pass@1 = 1.0
