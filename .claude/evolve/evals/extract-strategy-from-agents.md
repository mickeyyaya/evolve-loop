# Eval: extract-strategy-from-agents

## Code Graders (bash commands that must exit 0)
- `grep -c "Strategy Handling" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md | grep -q "^1$" && exit 0 || exit 1`
- `grep -c "Strategy Handling" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md | grep -q "^1$" && exit 0 || exit 1`
- `grep -c "Strategy Handling" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md | grep -q "^1$" && exit 0 || exit 1`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md | awk '{if ($1 <= 235) exit 0; else exit 1}'`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md | awk '{if ($1 <= 147) exit 0; else exit 1}'`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md | awk '{if ($1 <= 143) exit 0; else exit 1}'`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -q "SKILL.md" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -q "SKILL.md" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `grep -q "SKILL.md" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`
- `grep -q "balanced\|innovate\|harden\|repair" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`

## Thresholds
- All checks: pass@1 = 1.0
