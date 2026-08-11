# Eval: Wire Scout Performance Tracking

## Code Graders (bash commands that must exit 0)
- `grep -qi "token.*efficien\|token.*budget\|token.*track\|performance.*track" agents/evolve-scout.md`
- `wc -l < agents/evolve-scout.md | awk '{exit ($1 > 380)}'`
- `grep -q "## Token Budget Awareness\|token budget\|tokenBudget\|perTask\|perCycle" agents/evolve-scout.md`

## Regression Evals (full test suite)
- `grep -q "## Responsibilities" agents/evolve-scout.md`
- `grep -q "## Inputs" agents/evolve-scout.md`
- `grep -q "mode.*full\|mode.*incremental\|mode.*convergence" agents/evolve-scout.md`
- `grep -q "## Output" agents/evolve-scout.md`

## Acceptance Checks (verification commands)
- `grep -qi "token" agents/evolve-scout.md | wc -l | awk '{exit ($1 < 1)}'`
- `grep -c "token" agents/evolve-scout.md | awk '{exit ($1 < 2)}'`

## Thresholds
- All checks: pass@1 = 1.0
