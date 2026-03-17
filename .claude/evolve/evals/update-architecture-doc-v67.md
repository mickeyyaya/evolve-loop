# Eval: Update Architecture Doc to v6.7

## Code Graders (bash commands that must exit 0)
- `grep -q "Bandit" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -q "Crossover" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -q "Novelty" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -qi "decision trace" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -qi "mailbox" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -qi "retrospective" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -qi "session narrative" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`

## Regression Evals (full test suite)
- n/a (documentation project — no test suite)

## Acceptance Checks (verification commands)
- `grep -c "Bandit\|bandit" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md | grep -vq "^0$"`
- `grep -c "Prerequisite\|prerequisite" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md | grep -vq "^0$"`
- `grep -c "Adaptive\|adaptive" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md | grep -vq "^0$"`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md | awk '{exit ($1 > 800)}'`

## Thresholds
- All checks: pass@1 = 1.0
