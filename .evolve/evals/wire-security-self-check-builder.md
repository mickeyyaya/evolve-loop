# Eval: Wire Security Self-Check Into Builder Agent

## Code Graders (bash commands that must exit 0)
- `grep -q "Security Self-Check" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `grep -q "strategy: harden" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `grep -q "security-considerations.md" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `grep -q "prompt injection" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`

## Regression Evals (full test suite)
- `grep -oP '\]\(([^)]+\.md)\)' /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md | wc -l | awk '{exit ($1 > 6)}'`

## Acceptance Checks (verification commands)
- `grep -q "shell injection\|command injection\|unvalidated" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md | awk '{exit ($1 > 280)}'`

## Thresholds
- All checks: pass@1 = 1.0
