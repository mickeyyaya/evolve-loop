# Eval: Add CHANGELOG Entry and Update README Features List for v6.8

## Code Graders (bash commands that must exit 0)
- `grep -q "\[6\.8\.0\]" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -qi "session narrative" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -qi "bandit" /Users/danleemh/ai/claude/evolve-loop/README.md`
- `grep -qi "session narrative" /Users/danleemh/ai/claude/evolve-loop/README.md`

## Regression Evals (full test suite)
- n/a (documentation project — no test suite)

## Acceptance Checks (verification commands)
- `grep -c "next-cycle brief\|next-cycle-brief" /Users/danleemh/ai/claude/evolve-loop/README.md | grep -vq "^0$"`
- `grep -c "mailbox\|Mailbox" /Users/danleemh/ai/claude/evolve-loop/README.md | grep -vq "^0$"`
- `grep -c "crossover\|Crossover" /Users/danleemh/ai/claude/evolve-loop/README.md | grep -vq "^0$"`

## Thresholds
- All checks: pass@1 = 1.0
