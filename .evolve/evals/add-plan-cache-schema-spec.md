# Eval: add-plan-cache-schema-spec

## Code Graders (bash commands that must exit 0)
- `grep -q "planCache" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`
- `grep -q "taskType\|similarity\|template\|priorPlan" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`
- `grep -q '```json' /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`

## Regression Evals (full test suite)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`

## Acceptance Checks (verification commands)
- `grep -q "## Plan Cache" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md | awk '{exit ($1 > 260)}'`

## Thresholds
- All checks: pass@1 = 1.0
