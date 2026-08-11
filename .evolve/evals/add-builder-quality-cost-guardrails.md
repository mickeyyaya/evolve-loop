# Eval: Add Builder Quality-Cost Guardrails

## Code Graders (bash commands that must exit 0)

- `grep -q "Quality Signal\|quality signal\|Quality signal" agents/evolve-builder.md`
- `grep -q "consecutiveClean\|consecutive_clean\|consecutive clean" agents/evolve-builder.md`
- `wc -l < agents/evolve-builder.md | awk '{exit ($1 > 280)}'`

## Regression Evals (full test suite)

- `grep -q "Step 1: Read Instincts" agents/evolve-builder.md`
- `grep -q "Worktree Isolation" agents/evolve-builder.md`
- `grep -q "Integrity Notice" agents/evolve-builder.md`

## Acceptance Checks (verification commands)

- `grep -qi "confidence\|0\.0.*1\.0\|0\.0-1\.0" agents/evolve-builder.md`
- `grep -qi "token-optimization\|quality guardrail" agents/evolve-builder.md`

## Thresholds

- All checks: pass@1 = 1.0
