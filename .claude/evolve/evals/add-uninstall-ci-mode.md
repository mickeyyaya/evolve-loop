# Eval: add-uninstall-ci-mode

## Code Graders (bash commands that must exit 0)
- `grep -q "\-\-ci" /Users/danleemh/ai/claude/evolve-loop/uninstall.sh`
- `grep -q "CI_MODE\|CI=" /Users/danleemh/ai/claude/evolve-loop/uninstall.sh`
- `bash -c "CI=true /Users/danleemh/ai/claude/evolve-loop/uninstall.sh; echo exit:$?"  | grep -q "exit:0"`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -q "dry.run\|DRY_RUN\|validate" /Users/danleemh/ai/claude/evolve-loop/uninstall.sh`

## Thresholds
- All checks: pass@1 = 1.0
