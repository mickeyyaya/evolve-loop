# Eval: update-changelog-cycles-8-12

## Code Graders (bash commands that must exit 0)
- `grep -q "\[7.0.0\]" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`

## Regression Evals (full test suite)
- `grep -c "^## \[" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md | awk '{exit ($1 < 8)}'`

## Acceptance Checks (verification commands)
- `grep -q "plan cache schema\|Plan Cache Schema\|plan-cache-schema" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q "instinct graduation\|Instinct Graduation\|instinct-graduation" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q "performance.profiling\|Performance Profiling" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q "security.considerations\|Security Considerations" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q "APC\|Agentic Plan Caching\|token.optim" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md | awk '{exit ($1 > 340)}'`

## Thresholds
- All checks: pass@1 = 1.0
