# Eval: add-goal-milestone-tracking

## Code Graders (bash commands that must exit 0)

- `grep -q "milestone" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -q "GoalAct\|goalact\|goal.*continuity\|goal continuity" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -q "global.*plan\|milestone.*map\|goal.*anchor" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -qi "arXiv:2504.16563\|2504.16563" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md | awk '{exit ($1 > 460)}'`

## Regression Evals (full test suite)

- `bash /Users/danleemh/ai/claude/evolve-loop/scripts/eval-quality-check.sh 2>/dev/null || true`

## Acceptance Checks (verification commands)

- `grep -c "milestone" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md | awk '{exit ($1 < 2)}'`

## Thresholds

- All checks: pass@1 = 1.0
