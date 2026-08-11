# Eval: update-research-paper-index-cycle-147

## Code Graders (bash commands that must exit 0)

- `grep -q "Cycle 147" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`
- `grep -q "2603.15611" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`
- `grep -q "2602.21670" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`
- `grep -c "arXiv:" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md | awk '{exit ($1 < 24)}'`
- `grep -q "AgentAssay" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Regression Evals (full test suite)

- `grep -q "Cycle 146" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`
- `grep -q "AdaptOrch" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Acceptance Checks (verification commands)

- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md | awk '{exit ($1 > 150)}'`

## Thresholds

- All checks: pass@1 = 1.0
