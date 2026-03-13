# Eval: add-example-gene-file

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/examples/gene-example.yaml`
- `grep -q "selector" /Users/danleemh/ai/claude/evolve-loop/examples/gene-example.yaml`
- `grep -q "validation" /Users/danleemh/ai/claude/evolve-loop/examples/gene-example.yaml`
- `grep -q "capsule" /Users/danleemh/ai/claude/evolve-loop/examples/gene-example.yaml`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -q "successCount" /Users/danleemh/ai/claude/evolve-loop/examples/gene-example.yaml`
- `grep -q "confidence" /Users/danleemh/ai/claude/evolve-loop/examples/gene-example.yaml`

## Thresholds
- All checks: pass@1 = 1.0
