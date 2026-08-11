# Eval: Add Confidence-Correctness Alignment to Self-Evaluation Protocol

## Code Graders (bash commands that must exit 0)

- `grep -q "Confidence-Correctness" docs/self-learning.md`
- `grep -q "2603.06604" docs/self-learning.md`
- `grep -q "miscalibrat" docs/self-learning.md`
- `grep -qi "confidence-correctness\|Confidence-Correctness" docs/accuracy-self-correction.md`

## Regression Evals (full test suite)

- `wc -l < docs/self-learning.md | awk '{exit ($1 > 235)}'`
- `grep -q "### b\. LLM-as-a-Judge\|### Stepwise Confidence" docs/self-learning.md`

## Acceptance Checks (verification commands)

- `grep -q "evalHistory\|eval.*pass rate\|pass rate\|correctness.*score" docs/self-learning.md`
- `grep -c "###" docs/self-learning.md | awk '{exit ($1 < 3)}'`

## Thresholds
- All checks: pass@1 = 1.0
