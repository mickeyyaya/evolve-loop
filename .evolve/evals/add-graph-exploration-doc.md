# Eval: add-graph-exploration-doc

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/graph-exploration.md`
- `grep -q "GraphReader\|RepoMaster\|graph.*exploration\|dependency.*graph\|call.*graph" /Users/danleemh/ai/claude/evolve-loop/docs/graph-exploration.md`
- `grep -q "scout\|Scout\|DISCOVER" /Users/danleemh/ai/claude/evolve-loop/docs/graph-exploration.md`
- `grep -q "token.*reduc\|95%\|token sav" /Users/danleemh/ai/claude/evolve-loop/docs/graph-exploration.md`

## Regression Evals (full test suite)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`

## Acceptance Checks (verification commands)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/graph-exploration.md | awk '{exit ($1 < 40 || $1 > 150)}'`
- `grep -q "graph-exploration" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`

## Thresholds
- All checks: pass@1 = 1.0
