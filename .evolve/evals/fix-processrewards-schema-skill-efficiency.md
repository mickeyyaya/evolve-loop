# Eval: Fix processRewards Schema — Add skillEfficiency Field

## Code Graders (bash commands that must exit 0)

- `grep -q "skillEfficiency" skills/evolve-loop/memory-protocol.md`
- `grep -c "skillEfficiency" skills/evolve-loop/memory-protocol.md | awk '{exit ($1 < 2)}'`

## Regression Evals (full test suite)

- `wc -l < skills/evolve-loop/memory-protocol.md | awk '{exit ($1 > 435)}'`
- `grep -q '"discover"' skills/evolve-loop/memory-protocol.md`

## Acceptance Checks (verification commands)

- `grep -q '"skillEfficiency": 0' skills/evolve-loop/memory-protocol.md`
- `grep -A10 "processRewardsHistory" skills/evolve-loop/memory-protocol.md | grep -q "skillEfficiency"`

## Thresholds
- All checks: pass@1 = 1.0
