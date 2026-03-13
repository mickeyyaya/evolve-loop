# Eval: add-memory-consolidation-trigger

## Code Graders (bash commands that must exit 0)
- `grep -c "cycle % 3\|every 3 cycles\|consolidation.*trigger\|should.*consolidate" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Regression Evals (full test suite)
- `bash /Users/danleemh/ai/claude/evolve-loop/install.sh --ci 2>&1 | grep -c "All checks passed\|plugin installed"`

## Acceptance Checks (verification commands)
- `grep -q "cycle % 3\|N % 3\|cycle.*mod.*3" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -n "Memory Consolidation" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | wc -l | awk '{exit ($1 >= 2) ? 0 : 1}'`

## Thresholds
- All checks: pass@1 = 1.0
