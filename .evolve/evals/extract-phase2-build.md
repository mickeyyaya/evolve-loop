# Eval: Extract Phase 2 BUILD into standalone module

## Code Graders (bash commands that must exit 0)
- `[code]` `test -f skills/evolve-loop/phase2-build.md`
- `[code]` `bash -c 'test $(wc -l < skills/evolve-loop/phases.md) -lt 500'`
- `[code]` `grep -q "phase2-build.md" skills/evolve-loop/phases.md`

## Acceptance Checks
- `[code]` `bash -c 'test $(wc -l < skills/evolve-loop/phase2-build.md) -gt 100'`
- `[code]` `grep -q "Phase 2" skills/evolve-loop/phase2-build.md`

## Thresholds
- All checks: pass@1 = 1.0
