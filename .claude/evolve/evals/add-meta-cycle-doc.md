# Eval: add-meta-cycle-doc

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/meta-cycle.md`
- `grep -q "cycle % 5" /Users/danleemh/ai/claude/evolve-loop/docs/meta-cycle.md`
- `grep -q "split-role" /Users/danleemh/ai/claude/evolve-loop/docs/meta-cycle.md`
- `grep -q "mutation" /Users/danleemh/ai/claude/evolve-loop/docs/meta-cycle.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -q "meta-cycle" /Users/danleemh/ai/claude/evolve-loop/README.md`
- `grep -q "meta-cycle.md\|meta_cycle" /Users/danleemh/ai/claude/evolve-loop/docs/meta-cycle.md || grep -q "Meta-Cycle" /Users/danleemh/ai/claude/evolve-loop/docs/meta-cycle.md`

## Thresholds
- All checks: pass@1 = 1.0
