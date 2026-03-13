# Eval: add-example-instinct-file

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/examples/instinct-example.yaml`
- `grep -c "^- id:" /Users/danleemh/ai/claude/evolve-loop/examples/instinct-example.yaml`
- `grep -c "confidence:" /Users/danleemh/ai/claude/evolve-loop/examples/instinct-example.yaml`
- `grep -c "category:" /Users/danleemh/ai/claude/evolve-loop/examples/instinct-example.yaml`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -c "type:" /Users/danleemh/ai/claude/evolve-loop/examples/instinct-example.yaml`
- `grep -c "source:" /Users/danleemh/ai/claude/evolve-loop/examples/instinct-example.yaml`

## Thresholds
- All checks: pass@1 = 1.0
