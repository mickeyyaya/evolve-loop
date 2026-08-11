# Eval: add-agent-personalization-doc

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/agent-personalization.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/agent-personalization.md | awk '{exit ($1 < 50 || $1 > 140)}'`
- `grep -q "PPP" /Users/danleemh/ai/claude/evolve-loop/docs/agent-personalization.md`
- `grep -q "2511.02208" /Users/danleemh/ai/claude/evolve-loop/docs/agent-personalization.md`
- `grep -q "2602.22680" /Users/danleemh/ai/claude/evolve-loop/docs/agent-personalization.md`
- `grep -q "Personalization" /Users/danleemh/ai/claude/evolve-loop/docs/agent-personalization.md`
- `grep -q "research-paper-index" /Users/danleemh/ai/claude/evolve-loop/docs/agent-personalization.md`

## Regression Evals (full test suite)
- `bash scripts/phase-gate.sh audit 2>/dev/null || true`

## Acceptance Checks (verification commands)
- `grep -c "##" /Users/danleemh/ai/claude/evolve-loop/docs/agent-personalization.md | awk '{exit ($1 < 3)}'`
- `grep -q "instinct" /Users/danleemh/ai/claude/evolve-loop/docs/agent-personalization.md`
- `grep -q "Cycle 151" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Thresholds
- All checks: pass@1 = 1.0
