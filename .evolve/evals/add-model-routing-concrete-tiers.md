# Eval: add-model-routing-concrete-tiers

## Code Graders (bash commands that must exit 0)
- `grep -qi "haiku\|claude-haiku" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`
- `grep -qi "sonnet\|claude-sonnet" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`
- `grep -qi "opus\|claude-opus" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`
- `grep -q "52.*cost\|52-70%\|MasRouter\|routing.*phase\|phase.*routing" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`

## Regression Evals (full test suite)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`
- `grep -q "Model Routing" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`

## Acceptance Checks (verification commands)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md | awk '{exit ($1 > 430)}'`
- `grep -c "Haiku\|Sonnet\|Opus" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md | awk '{exit ($1 < 3)}'`

## Thresholds
- All checks: pass@1 = 1.0
