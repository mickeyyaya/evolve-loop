# Eval: fix-readme-operator-model

## Code Graders (bash commands that must exit 0)
- `grep -n "Operator.*haiku" /Users/danleemh/ai/claude/evolve-loop/README.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `bash -c '! grep -n "Operator.*evolve-operator.*sonnet" /Users/danleemh/ai/claude/evolve-loop/README.md'`

## Thresholds
- All checks: pass@1 = 1.0
