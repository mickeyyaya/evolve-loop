# Eval: Extract Scout Discovery Guide

## Code Graders (bash commands that must exit 0)

- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/scout-discovery-guide.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md | awk '{exit ($1 >= 300)}'`
- `grep -q 'scout-discovery-guide' /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/scout-discovery-guide.md | awk '{exit ($1 > 200)}'`

## Regression Evals (full test suite)

- `grep -q 'mode.*full\|mode.*incremental\|mode.*convergence' /Users/danleemh/ai/claude/evolve-loop/docs/scout-discovery-guide.md`
- `grep -q 'Codebase Analysis\|Stability\|Security\|Architecture' /Users/danleemh/ai/claude/evolve-loop/docs/scout-discovery-guide.md`

## Acceptance Checks (verification commands)

- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/scout-discovery-guide.md`
- `grep -qiE 'Discover|scout.discovery.guide|discovery mode' /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -qi 'scout-discovery-guide\|scout discovery' /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md | awk '{exit ($1 >= 300)}'`

## Thresholds

- All checks: pass@1 = 1.0
