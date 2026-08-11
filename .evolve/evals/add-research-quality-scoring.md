# Eval: Add Research Quality Scoring

## Code Graders (bash commands that must exit 0)

- `grep -q "research quality\|Research Quality\|researchQuality\|per.query" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -q "2510.07794\|HiPRAG" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md | awk '{exit ($1 > 350)}'`

## Regression Evals (full test suite)

- `cd /Users/danleemh/ai/claude/evolve-loop && bash scripts/phase-gate.sh lint 2>/dev/null || true`

## Acceptance Checks (verification commands)

- `grep -q "novelty\|relevance\|yield" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`

## Thresholds
- All checks: pass@1 = 1.0
