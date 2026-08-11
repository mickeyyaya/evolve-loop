# Eval: update-research-paper-index-cycle-148

## Code Graders (bash commands that must exit 0)
- `grep -q "Cycle 148" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`
- `grep -qi "2411.06559\|2512.20677\|2508.14419\|2603.11808" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Regression Evals (full test suite)
- `grep -c "arXiv:" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md | awk '{exit ($1 < 27)}'`

## Acceptance Checks (verification commands)
- `grep -q "Cycle 147" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`
- `grep -q "Code-A1" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md | awk '{exit ($1 < 108)}'`

## Thresholds
- All checks: pass@1 = 1.0
