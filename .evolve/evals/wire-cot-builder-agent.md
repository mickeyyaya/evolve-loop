# Eval: Wire CoT Enforcement into Builder Agent

## Code Graders (bash commands that must exit 0)
- `grep -c "reasoning steps\|chain-of-thought\|enumerate.*reason\|CoT" agents/evolve-builder.md | awk '{exit ($1 < 2)}'`
- `grep -q "Step 3" agents/evolve-builder.md`
- `wc -l < agents/evolve-builder.md | awk '{exit ($1 > 280)}'`

## Regression Evals (full test suite)
- `grep -q "^---" agents/evolve-builder.md`
- `grep -q "tools:" agents/evolve-builder.md`

## Acceptance Checks (verification commands)
- `grep -qi "chain-of-thought\|CoT\|reasoning steps" agents/evolve-builder.md`
- `grep -q "Risks" agents/evolve-builder.md`
- `grep -c "^###\|^##" agents/evolve-builder.md | awk '{exit ($1 < 5)}'`

## Thresholds
- All checks: pass@1 = 1.0
