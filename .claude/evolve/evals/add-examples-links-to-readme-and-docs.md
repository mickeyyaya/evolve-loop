# Eval: add-examples-links-to-readme-and-docs

## Code Graders (bash commands that must exit 0)
- `grep -q "examples/" README.md`
- `grep -q "instinct-example" docs/instincts.md`
- `grep -q "gene-example" docs/genes.md`
- `grep -q "eval-definition" skills/evolve-loop/eval-runner.md`

## Regression Evals (full test suite)
- `CI=true ./install.sh`

## Acceptance Checks (verification commands)
- `grep -n "examples" README.md | head -5`
- `grep -n "instinct-example\|examples/" docs/instincts.md | head -5`
- `grep -n "gene-example\|examples/" docs/genes.md | head -5`
- `grep -n "eval-definition\|examples/" skills/evolve-loop/eval-runner.md | head -5`

## Thresholds
- All checks: pass@1 = 1.0
