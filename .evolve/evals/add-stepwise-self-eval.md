# Eval: Add Stepwise Self-Evaluation to Self-Learning Doc and Phase5

## Code Graders (bash commands that must exit 0)

- `grep -qi 'stepwise' docs/self-learning.md`
- `grep -qi 'stepwise' skills/evolve-loop/phase5-learn.md`
- `grep -qi 'per.step\|evidence step\|step-by-step' docs/self-learning.md`
- `wc -l < docs/self-learning.md | awk '{exit ($1 > 200)}'`
- `wc -l < skills/evolve-loop/phase5-learn.md | awk '{exit ($1 > 450)}'`

## Regression Evals (full test suite)

- `grep -qi 'LLM-as-a-Judge\|Self-Evaluation' docs/self-learning.md`
- `grep -qi 'Correctness\|Completeness\|Novelty\|Efficiency' skills/evolve-loop/phase5-learn.md`

## Acceptance Checks (verification commands)

- `grep -qi 'stepwise\|per.step' docs/self-learning.md`
- `grep -qi 'stepwise\|evidence' skills/evolve-loop/phase5-learn.md`
- `grep -qi 'stepwise\|step.*confidence\|2511' docs/accuracy-self-correction.md`

## Thresholds
- All checks: pass@1 = 1.0
