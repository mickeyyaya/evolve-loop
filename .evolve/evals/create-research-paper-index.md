# Eval: Create Research Paper Index

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md | awk '{exit ($1 < 40)}'`
- `grep -q "arXiv" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Regression Evals (full test suite)
- `grep -qi "DGM\|ACE\|DAAO" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`
- `grep -qi "AgentDebug\|SWE-CI\|ARTEMIS" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Acceptance Checks (verification commands)
- `grep -c "arXiv:" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md | awk '{exit ($1 < 10)}'`
- `grep -qi "cycle" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Thresholds
- All checks: pass@1 = 1.0
