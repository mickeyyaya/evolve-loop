# Eval: add-semantic-task-crossover

## Code Graders (bash commands that must exit 0)
- `grep -c "crossover\|recombine\|offspring\|parent" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -c "crossover\|recombine" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`

## Regression Evals (full test suite)
- `grep -c "Selected Tasks" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -c "Task Selection" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`

## Acceptance Checks (verification commands)
- `grep -c "taskCrossover\|crossoverEnabled\|semanticCrossover" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -c "Crossover\|crossover" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`

## Thresholds
- All checks: pass@1 = 1.0
