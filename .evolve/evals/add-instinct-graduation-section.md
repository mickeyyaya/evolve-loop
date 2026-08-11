# Eval: Add Instinct Graduation Section

## Code Graders (bash commands that must exit 0)

- `grep -c "## Graduation" docs/instincts.md`
- `grep -c "0\.75\|0.75" docs/instincts.md`
- `grep -ic "mandatory" docs/instincts.md`
- `grep -ic "reversal\|revert" docs/instincts.md`
- `grep -c "phase5-learn\|phase5" docs/instincts.md`
- `wc -l < docs/instincts.md | awk '{exit ($1 > 200)}'`

## Regression Evals (full test suite)

- `grep -c "## How It Works" docs/instincts.md | awk '{exit ($1 < 1)}'`
- `grep -c "## Confidence Scoring" docs/instincts.md | awk '{exit ($1 < 1)}'`

## Acceptance Checks (verification commands)

- `grep -A 20 "## Graduation" docs/instincts.md | grep -ic "threshold\|criteria\|condition"` — threshold documented
- `grep -A 20 "## Graduation" docs/instincts.md | grep -ic "builder"` — Builder behavior documented

## Thresholds
- All checks: pass@1 = 1.0
