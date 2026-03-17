# Eval: fix-eval-runner-stale-refs

## Code Graders (bash commands that must exit 0)
- `bash -c 'test $(grep -c "Phase 5.5\|Phase 6\|Phase 7\|Developer agent\|Planner in Phase" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/eval-runner.md) -eq 0'`

## Regression Evals (full test suite)
- `./install.sh`

## Acceptance Checks (verification commands)
- `grep -n "Scout" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/eval-runner.md`
- `grep -n "Builder" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/eval-runner.md`
- `grep -n "Phase 3\|Phase 4\|Phase 5" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/eval-runner.md`

## Thresholds
- All checks: pass@1 = 1.0
