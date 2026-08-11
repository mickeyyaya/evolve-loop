# Eval: implement-checkpoint-handshake
## Code Graders (bash commands that must exit 0)
- `[code]` grep -q "Checkpoint" agents/evolve-auditor.md
## Regression Evals (full test suite)
- `[code]` bash .evolve/calibrate.sh
## Acceptance Checks
- `[code]` grep -i "json" agents/evolve-auditor.md
## Thresholds
- All checks: pass@1 = 1.0