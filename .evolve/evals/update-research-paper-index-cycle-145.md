# Eval: update-research-paper-index-cycle-145

## Code Graders (bash commands that must exit 0)

- `grep -qi "AUQ\|2602.05073\|2601.15703" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`
- `grep -qi "DCCD\|2603.03305\|2501.10868" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`
- `grep -qi "DocAgent\|2504.08725" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`
- `grep -q "Cycle 145\|### Cycle 145" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Regression Evals (full test suite)

- `grep -c "arXiv:" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md | awk '{exit ($1 < 20)}'`

## Acceptance Checks (verification commands)

- `grep -q "Cycle 144" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`
- `grep -q "Research Coverage Map" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Thresholds

- All checks: pass@1 = 1.0
