# Eval: Compress policies.md

## Code Graders (bash commands that must exit 0)
- `test -f skills/evolve-loop/reference/policies.md`
- `LINES=$(wc -l < skills/evolve-loop/reference/policies.md); [ "$LINES" -lt 270 ]`
- `ORIGINAL=318; CURRENT=$(wc -l < skills/evolve-loop/reference/policies.md); REDUCTION=$(( (ORIGINAL - CURRENT) * 100 / ORIGINAL )); [ "$REDUCTION" -ge 15 ]`

## Regression Evals (key content preserved)
- `grep -q "Graduated Instincts" skills/evolve-loop/reference/policies.md`
- `grep -q "Token Budgets" skills/evolve-loop/reference/policies.md`
- `grep -q "Context Window Strategy" skills/evolve-loop/reference/policies.md`
- `grep -q "Rate Limit" skills/evolve-loop/reference/policies.md`
- `grep -q "Session Break" skills/evolve-loop/reference/policies.md`
- `grep -q "AgentDiet" skills/evolve-loop/reference/policies.md`
- `grep -q "context-budget.sh" skills/evolve-loop/reference/policies.md`
- `grep -q "handoff.md" skills/evolve-loop/reference/policies.md`
- `grep -q "80K tokens" skills/evolve-loop/reference/policies.md`
- `grep -q "200K tokens" skills/evolve-loop/reference/policies.md`
- `grep -q "GREEN\|YELLOW\|RED" skills/evolve-loop/reference/policies.md`

## Acceptance Checks (structural integrity)
- `grep -q "Plan Reuse" skills/evolve-loop/reference/policies.md`
- `grep -q "Final Session Report" skills/evolve-loop/reference/policies.md`
- `grep -q "Context Management" skills/evolve-loop/reference/policies.md`
- `grep -qE "Inline S-tasks|inst-007" skills/evolve-loop/reference/policies.md`

## Thresholds
- All checks: pass@1 = 1.0
