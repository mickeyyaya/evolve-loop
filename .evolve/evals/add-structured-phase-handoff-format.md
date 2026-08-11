# Eval: Add Structured Phase Handoff Format

## Code Graders (bash commands that must exit 0)
- `awk '/phaseHandoff/{found=1} END{exit !found}' skills/evolve-loop/phases.md`
- `awk '/files_modified/{found=1} END{exit !found}' skills/evolve-loop/phases.md`
- `awk '/next_phase_context/{found=1} END{exit !found}' skills/evolve-loop/phases.md`
- `awk '/Inter-Phase Handoff/{found=1} END{exit !found}' skills/evolve-loop/phases.md`
- `awk '/handoff-/{found=1} END{exit !found}' skills/evolve-loop/phases.md`
- `[ $(wc -l < skills/evolve-loop/phases.md) -gt 640 ]`

## Regression Evals (full test suite)
- `awk '/Phase 2: BUILD/{found=1} END{exit !found}' skills/evolve-loop/phases.md`
- `awk '/Stagnation detection/{found=1} END{exit !found}' skills/evolve-loop/phases.md`

## Acceptance Checks (verification commands)
- `awk '/findings/{found=1} END{exit !found}' skills/evolve-loop/phases.md`
- `awk '/decisions/{found=1} END{exit !found}' skills/evolve-loop/phases.md`

## Thresholds
- All checks: pass@1 = 1.0
