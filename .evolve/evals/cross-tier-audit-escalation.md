# Eval: cross-tier-audit-escalation
## Code Graders (bash commands that must exit 0)
- `[code]` grep -q "tier-1" agents/evolve-auditor.md
- `[code]` grep -q "Asymmetry" docs/model-routing.md
## Regression Evals (full test suite)
- `[code]` bash .evolve/calibrate.sh
## Acceptance Checks
- `[code]` grep -i "upgrade" agents/evolve-auditor.md
## Thresholds
- All checks: pass@1 = 1.0
