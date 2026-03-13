# Eval: fix-install-usage-and-ci-docs-check

## Code Graders (bash commands that must exit 0)
- `grep -q "\[cycles\] \[strategy\] \[goal\]" /Users/danleemh/ai/claude/evolve-loop/install.sh`
- `grep -q "genes.md\|island-model.md" /Users/danleemh/ai/claude/evolve-loop/.github/workflows/ci.yml`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -q "strategy" /Users/danleemh/ai/claude/evolve-loop/install.sh`
- `! grep -q "\[cycles\] \[goal\]$" /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Thresholds
- All checks: pass@1 = 1.0
