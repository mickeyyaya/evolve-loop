# Eval: add-install-ci-mode

## Code Graders (bash commands that must exit 0)
- `bash -c 'CI=true ./install.sh'`

## Regression Evals (full test suite)
- `./install.sh`

## Acceptance Checks (verification commands)
- `grep -n "\-\-ci\|CI=" /Users/danleemh/ai/claude/evolve-loop/install.sh`
- `bash -c 'test $(grep -c "non-interactive\|CI mode\|--ci" /Users/danleemh/ai/claude/evolve-loop/install.sh) -gt 0'`

## Thresholds
- All checks: pass@1 = 1.0
