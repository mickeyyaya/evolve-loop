# Eval: add-counterfactual-cycle-log

## Code Graders (bash commands that must exit 0)
- `grep -c "counterfactual\|what-if\|alternate\|shadow" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -c "counterfactual\|what-if" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`

## Regression Evals (full test suite)
- `grep -c "Phase 5: LEARN" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -c "Instinct Extraction" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Acceptance Checks (verification commands)
- `grep -c "counterfactualLog\|shadowRun\|alternateTask" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -c "Counterfactual" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`

## Thresholds
- All checks: pass@1 = 1.0
