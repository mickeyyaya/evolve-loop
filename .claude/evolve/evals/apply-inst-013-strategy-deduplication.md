# Eval: Apply inst-013 Strategy Deduplication

## Code Graders (bash commands that must exit 0)
- `grep -c "SKILL.md\|strategy presets\|Strategy Presets" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `grep -c "SKILL.md\|strategy presets\|Strategy Presets" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`
- `test -f /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/cycle-21-instincts.yaml`
- `grep "confidence: 0.7" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/cycle-21-instincts.yaml`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `wc -l /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `wc -l /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`
- `grep -n "inst-013" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/cycle-21-instincts.yaml`
- `grep -n "strategy" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`

## Thresholds
- All checks: pass@1 = 1.0
