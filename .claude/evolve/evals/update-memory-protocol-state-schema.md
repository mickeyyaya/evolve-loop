# Eval: update-memory-protocol-state-schema

## Code Graders (bash commands that must exit 0)
- `grep -c "processRewards" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -c "mastery" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`

## Regression Evals (full test suite)
- `bash /Users/danleemh/ai/claude/evolve-loop/install.sh --ci 2>&1 | grep -c "All checks passed\|plugin installed"`

## Acceptance Checks (verification commands)
- `grep -q "processRewards" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -q "mastery" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -q "synthesizedTools" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -q "planCache" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`

## Thresholds
- All checks: pass@1 = 1.0
