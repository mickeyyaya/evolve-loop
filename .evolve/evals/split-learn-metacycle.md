# Eval: Split Phase 5 LEARN from meta-cycle

## Code Graders (bash commands that must exit 0)
- `[code]` `test -f skills/evolve-loop/phase6-metacycle.md`
- `[code]` `bash -c 'test $(wc -l < skills/evolve-loop/phase5-learn.md) -lt 350'`
- `[code]` `grep -q "phase6-metacycle" skills/evolve-loop/SKILL.md`

## Acceptance Checks
- `[code]` `bash -c 'test $(wc -l < skills/evolve-loop/phase6-metacycle.md) -gt 100'`
- `[code]` `grep -q "meta-cycle\|Meta-Cycle\|meta_cycle" skills/evolve-loop/phase6-metacycle.md`

## Thresholds
- All checks: pass@1 = 1.0
