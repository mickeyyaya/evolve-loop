# Eval: add-token-optimization-doc

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`
- `grep -c "model.routing\|model routing\|haiku\|sonnet\|opus" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`
- `grep -c "KV.cache\|kv.cache\|prompt.cache\|cache.hit" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`
- `grep -c "instinct.summar\|plan.cache\|incremental.scan\|research.cooldown" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`

## Regression Evals (full test suite)
- `grep -c "##" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`

## Acceptance Checks (verification commands)
- `grep -l "token.budget\|perTask\|perCycle" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`

## Thresholds
- All checks: pass@1 = 1.0
