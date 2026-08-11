# Eval: add-parallel-run-research-dedup-spec

## Code Graders (bash commands that must exit 0)
- `grep -q "parallel\|cross-run\|concurrent" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`

## Regression Evals (full test suite)
- `grep -q "Research Cooldown" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`

## Acceptance Checks (verification commands)
- `grep -q "parallel run\|parallel invocation\|cross-run\|Parallel Run\|Parallel Invocation" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`
- `grep -q "latest-brief\|shared.*research\|research.*shared\|cross-run" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md | awk '{exit ($1 > 290)}'`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md | awk '{exit ($1 > 420)}'`

## Thresholds
- All checks: pass@1 = 1.0
