# Eval: add-cot-grader-patterns

## Code Graders (bash commands that must exit 0)
- `grep -qi 'grader\|chain-of-thought\|CoT' docs/accuracy-self-correction.md`
- `grep -c '^## ' docs/accuracy-self-correction.md | awk '{exit ($1 < 5)}'`
- `wc -l < docs/accuracy-self-correction.md | awk '{exit ($1 < 100)}'`

## Regression Evals (full test suite)
- `wc -l < docs/accuracy-self-correction.md | awk '{exit ($1 > 250)}'`

## Acceptance Checks (verification commands)
- `grep -qi 'implement\|example\|how to\|pattern\|concrete' docs/accuracy-self-correction.md`
- `test -f docs/accuracy-self-correction.md`

## Thresholds
- All checks: pass@1 = 1.0
