# Eval: add-shared-values-protocol

## Code Graders (bash commands that must exit 0)
- `grep -c "shared.*values\|core.*rules\|agent.*alignment\|values.*protocol" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -c "shared.*values\|core.*rules\|agent.*alignment" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Regression Evals (full test suite)
- `grep -c "Layer" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`

## Acceptance Checks (verification commands)
- `grep -l "parallel.*agent\|agent.*parallel\|concurrent" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -c "do not\|must not\|always\|never" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`

## Thresholds
- All checks: pass@1 = 1.0
