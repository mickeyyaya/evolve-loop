# Eval: Add "What the Loop Learned This Session" Template to phase5-learn.md

## Code Graders (bash commands that must exit 0)
- `grep -q "What the Loop Learned" skills/evolve-loop/phase5-learn.md`
- `grep -q "human-learning-guide" skills/evolve-loop/phase5-learn.md`
- `grep -q "Operator writes" skills/evolve-loop/phase5-learn.md`

## Regression Evals (full test suite)
- `test -f skills/evolve-loop/phase5-learn.md`

## Acceptance Checks (verification commands)
- `wc -l < skills/evolve-loop/phase5-learn.md | awk '{exit ($1 > 530)}'`
- `grep -q "session-learned.md\|session_learned\|What the Loop Learned" skills/evolve-loop/phase5-learn.md`

## Thresholds
- All checks: pass@1 = 1.0
