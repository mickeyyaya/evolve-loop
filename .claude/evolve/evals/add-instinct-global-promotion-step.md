# Eval: add-instinct-global-promotion-step

## Code Graders (bash commands that must exit 0)
- `grep -n "global promotion\|~/.claude/instincts" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | wc -l | awk '{exit ($1 >= 2) ? 0 : 1}'`
- `grep -c "~/.claude/instincts/personal" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md`

## Regression Evals (full test suite)
- `bash /Users/danleemh/ai/claude/evolve-loop/install.sh --ci 2>&1 | grep -c "All checks passed\|plugin installed"`

## Acceptance Checks (verification commands)
- `grep -q "~/.claude/instincts/personal" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md`
- `grep -v "homunculus" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md | grep -c "instincts/personal" | awk '{exit ($1 >= 1) ? 0 : 1}'`
- `grep -q "global promotion\|promote.*instinct\|instinct.*promot" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -c "homunculus" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md | awk '{exit ($1 == 0) ? 0 : 1}'`
- `grep -c "homunculus" /Users/danleemh/ai/claude/evolve-loop/docs/configuration.md | awk '{exit ($1 == 0) ? 0 : 1}'`

## Thresholds
- All checks: pass@1 = 1.0
